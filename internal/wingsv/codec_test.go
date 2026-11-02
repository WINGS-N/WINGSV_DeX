package wingsv

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/WINGS-N/wingsv-dex/internal/gen/wingsvpb"
)

func sampleConfig() *wingsvpb.Config {
	return &wingsvpb.Config{
		Ver:     1,
		Type:    wingsvpb.ConfigType_CONFIG_TYPE_VK_TURN_PROFILE,
		Backend: wingsvpb.BackendType_BACKEND_TYPE_VK_TURN,
		Turn: &wingsvpb.Turn{
			Endpoint:    &wingsvpb.Endpoint{Host: "203.0.113.10", Port: 51820},
			Threads:     proto.Uint32(24),
			UseUdp:      proto.Bool(true),
			SessionMode: wingsvpb.TurnSessionMode_TURN_SESSION_MODE_MUX,
			TunnelMode:  wingsvpb.TunnelMode_TUNNEL_MODE_WIREGUARD,
			Title:       "VK TURN",
		},
		Wg: &wingsvpb.WireGuard{
			Iface: &wingsvpb.Interface{Addrs: []string{"10.8.0.2/32"}},
			Peer: &wingsvpb.Peer{
				AllowedIps: []*wingsvpb.Cidr{{Addr: []byte{0, 0, 0, 0}, Prefix: 0}},
			},
			Endpoint: &wingsvpb.Endpoint{Host: "203.0.113.10", Port: 51820},
		},
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	link, err := Encode(sampleConfig())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.HasPrefix(link, SchemePrefix) {
		t.Fatalf("link missing scheme prefix: %q", link)
	}
	got, err := Decode(link)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !proto.Equal(got, sampleConfig()) {
		t.Fatalf("round-trip mismatch:\n got:  %v\n want: %v", got, sampleConfig())
	}
}

func TestDecodeRejectsNonWings(t *testing.T) {
	if _, err := Decode("vless://whatever"); err == nil {
		t.Fatal("expected error for non-wingsv link")
	}
}

func TestDecodeRejectsBadFormatByte(t *testing.T) {
	// A valid base64url payload whose first byte is not the protobuf-deflate tag.
	if _, err := Decode("wingsv://AAAA"); err == nil {
		t.Fatal("expected error for unknown format byte")
	}
}

// TestDecodeFromSurroundingText lenient parser: a link embedded
// in other text, with whitespace, must still decode.
func TestDecodeFromSurroundingText(t *testing.T) {
	link, err := Encode(sampleConfig())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	wrapped := "Мой конфиг:\n  " + link + "  \nспасибо"
	got, err := Decode(wrapped)
	if err != nil {
		t.Fatalf("Decode from text: %v", err)
	}
	if !proto.Equal(got, sampleConfig()) {
		t.Fatal("decoded config from surrounding text does not match")
	}
}

func TestExtractLink(t *testing.T) {
	if got := ExtractLink("prefix wingsv://ABC-_123 suffix"); got != "wingsv://ABC-_123" {
		t.Fatalf("ExtractLink = %q", got)
	}
	if got := ExtractLink("no link here"); got != "" {
		t.Fatalf("ExtractLink should be empty, got %q", got)
	}
}

// Оба формата обязаны читаться: ссылки, выданные раньше, никуда не делись
func TestBothFormatsDecode(t *testing.T) {
	config := &wingsvpb.Config{
		Ver:  1,
		Type: wingsvpb.ConfigType_CONFIG_TYPE_XRAY,
		Xray: &wingsvpb.Xray{Profiles: []*wingsvpb.VlessProfile{{Id: "p1", RawLink: "vless://x@1.2.3.4:443#p1"}}},
	}
	brotliLink, err := Encode(config)
	if err != nil {
		t.Fatal(err)
	}
	deflateLink, err := EncodeDeflate(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(brotliLink) >= len(deflateLink) {
		t.Errorf("brotli не короче: %d против %d", len(brotliLink), len(deflateLink))
	}
	for _, link := range []string{brotliLink, deflateLink} {
		got, err := Decode(link)
		if err != nil {
			t.Fatalf("ссылка не разобралась: %v", err)
		}
		if len(got.GetXray().GetProfiles()) != 1 {
			t.Errorf("профили потерялись: %+v", got.GetXray())
		}
	}
}

// Подчёркивания в новом формате не бывает вовсе: мессенджеры читают пару подряд
// как разметку и съедают кусок ссылки. Проверяем на данных, где в base64url
// подчёркивания гарантированно были бы
func TestLinkCarriesNoUnderscore(t *testing.T) {
	profiles := make([]*wingsvpb.VlessProfile, 0, 24)
	for i := range 24 {
		profiles = append(profiles, &wingsvpb.VlessProfile{
			Id:      fmt.Sprintf("p%d", i),
			RawLink: fmt.Sprintf("vless://%x@%d.%d.%d.%d:443?flow=xtls-rprx-vision#node%d", i*7919, i, i+3, i+7, i+11, i),
		})
	}
	config := &wingsvpb.Config{
		Ver:  1,
		Type: wingsvpb.ConfigType_CONFIG_TYPE_XRAY,
		Xray: &wingsvpb.Xray{Profiles: profiles},
	}
	link, err := Encode(config)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(link, "_") {
		t.Errorf("в ссылке осталось подчёркивание: %s", link)
	}
	if strings.HasSuffix(link, ".") {
		t.Errorf("ссылка кончается точкой, автолинковка её обрежет: %s", link)
	}
	got, err := Decode(link)
	if err != nil {
		t.Fatalf("своя же ссылка не разобралась: %v", err)
	}
	if len(got.GetXray().GetProfiles()) != len(profiles) {
		t.Errorf("профилей на выходе %d, ждали %d", len(got.GetXray().GetProfiles()), len(profiles))
	}
}

// Хвостовая точка закрывается знаком равенства, и такая ссылка обязана читаться
func TestTrailingDotIsClosed(t *testing.T) {
	for _, framed := range [][]byte{{0x13, 0xff}, {0x13, 0xbf, 0xff}, {0x13, 0x0f, 0xff, 0xff}} {
		encoded := encodeLinkPayload(framed)
		if strings.HasSuffix(encoded, ".") {
			t.Errorf("хвостовая точка осталась: %s", encoded)
		}
		round := strings.TrimRight(encoded, "=")
		round = strings.ReplaceAll(round, ".", "_")
		if _, err := base64.RawURLEncoding.DecodeString(round); err != nil {
			t.Errorf("тело %s не декодируется назад: %v", encoded, err)
		}
	}
}

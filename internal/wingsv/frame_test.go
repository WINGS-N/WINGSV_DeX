package wingsv

import (
	"testing"

	wingsvpb "github.com/WINGS-N/wingsv-dex/internal/gen/wingsvpb"
)

// TestDecodeFrameMatchesLink checks that the raw config frame (what the federation
// subscription serves under application/x-wingsv-config) decodes to the same Config as the
// equivalent wingsv:// link, so the subscription parser recovers the xray profiles from it.
func TestDecodeFrameMatchesLink(t *testing.T) {
	cfg := &wingsvpb.Config{
		Ver:  1,
		Type: wingsvpb.ConfigType_CONFIG_TYPE_XRAY,
		Xray: &wingsvpb.Xray{
			Profiles: []*wingsvpb.VlessProfile{
				{RawLink: "vless://11111111-2222-3333-4444-555555555555@node.example:443?security=tls#Donor-A"},
				{RawLink: "vless://66666666-7777-8888-9999-000000000000@node2.example:8443?security=reality#Donor-B"},
			},
		},
	}
	link, err := Encode(cfg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// The bytes inside the link are exactly what a federation body carries raw.
	frame, err := decodePayload(link)
	if err != nil {
		t.Fatalf("decodePayload: %v", err)
	}
	got, err := DecodeFrame(frame)
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}
	profiles := got.GetXray().GetProfiles()
	if len(profiles) != 2 {
		t.Fatalf("got %d profiles, want 2", len(profiles))
	}
	if profiles[0].GetRawLink() != cfg.Xray.Profiles[0].RawLink {
		t.Fatalf("raw link = %q, want %q", profiles[0].GetRawLink(), cfg.Xray.Profiles[0].RawLink)
	}
}

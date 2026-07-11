package xray

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/WINGS-N/wingsv-dex/internal/config"
)

// xrayBin locates the built helper; the test is skipped when it is absent (e.g. CI that
// has not run build:xray), since Build shells out to it for link conversion.
func xrayBin(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "bin", "xray"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Skip("bin/xray not built; run `task build:xray`")
	}
	return path
}

func TestBuildProducesValidConfig(t *testing.T) {
	bin := xrayBin(t)
	settings := config.DefaultXraySettings()
	settings.LocalProxyEnabled = true
	settings.LocalProxyUsername = "u"
	settings.LocalProxyPassword = "p"

	out, err := Build(Options{
		XrayBin:    bin,
		Profile:    config.XrayProfile{RawLink: "vless://uuid@v.wingsnet.org:443?encryption=none&security=tls&type=tcp#Cyprus"},
		Settings:   settings,
		IncludeTun: true,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var cfg struct {
		Inbounds []struct {
			Tag      string `json:"tag"`
			Protocol string `json:"protocol"`
		} `json:"inbounds"`
		Outbounds []map[string]any `json:"outbounds"`
		Routing   struct {
			Rules []map[string]any `json:"rules"`
		} `json:"routing"`
	}
	if err := json.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("built config is not valid json: %v", err)
	}

	tags := map[string]string{}
	for _, in := range cfg.Inbounds {
		tags[in.Tag] = in.Protocol
	}
	if tags["tun-in"] != "tun" {
		t.Errorf("missing tun inbound, got %v", tags)
	}
	if tags["socks-in"] != "socks" {
		t.Errorf("missing socks inbound, got %v", tags)
	}

	if cfg.Outbounds[0]["tag"] != "proxy" {
		t.Errorf("first outbound should be tagged proxy, got %v", cfg.Outbounds[0]["tag"])
	}
	if cfg.Outbounds[0]["protocol"] != "vless" {
		t.Errorf("proxy outbound protocol = %v, want vless", cfg.Outbounds[0]["protocol"])
	}
	if _, bad := cfg.Outbounds[0]["sendThrough"]; bad {
		t.Errorf("invalid sendThrough should have been stripped")
	}
	if len(cfg.Routing.Rules) == 0 {
		t.Errorf("routing rules missing")
	}
}

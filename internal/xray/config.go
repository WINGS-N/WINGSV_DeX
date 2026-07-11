// Package xray builds the xray-core config JSON for an Xray/VLESS profile. The proxy
// outbound is produced by shelling out to the bin/xray convert helper (libXray) so this
// package never has to import xray-core itself.
package xray

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/WINGS-N/wingsv-dex/internal/config"
)

// TunDeviceName is the TUN interface the fork's proxy/tun inbound creates and the helper
// routes through; it must agree with cmd/xrayhelper's run wrapper.
const TunDeviceName = "wingsv-tun"

// Options carries everything the config build needs.
type Options struct {
	XrayBin    string             // path to the bin/xray helper (for link conversion)
	Profile    config.XrayProfile // active node
	Settings   config.XraySettings
	IncludeTun bool // false in proxy-only runtime mode (local socks/http only)
	// ByeDPIFrontSocks, when set, routes the proxy outbound through this local SOCKS
	// (host:port) as a DPI-bypass front (filled by the ByeDPI integration).
	ByeDPIFrontSocks string
}

// ProbeConfig builds a minimal proxy-only config for a single node: a no-auth SOCKS inbound
// on socksPort plus the node's outbound, used by the Real Delay test to ping through it.
func ProbeConfig(xrayBin, rawLink string, socksPort int) (string, error) {
	s := config.DefaultXraySettings()
	s.RuntimeMode = "proxy"
	s.SniffingEnabled = false
	s.LocalProxyEnabled = true
	s.LocalProxyAuthEnabled = false
	s.LocalProxyListenAddress = "127.0.0.1"
	s.LocalProxyPort = socksPort
	return Build(Options{
		XrayBin:    xrayBin,
		Profile:    config.XrayProfile{RawLink: rawLink},
		Settings:   s,
		IncludeTun: false,
	})
}

// Build assembles the full xray config JSON for the given profile and settings.
func Build(opts Options) (string, error) {
	if strings.TrimSpace(opts.Profile.RawLink) == "" {
		return "", errors.New("xray: profile has no share link")
	}
	proxy, err := proxyOutbound(opts)
	if err != nil {
		return "", err
	}

	cfg := map[string]any{
		"log":       map[string]any{"loglevel": "warning"},
		"dns":       buildDNS(opts.Settings),
		"inbounds":  buildInbounds(opts),
		"outbounds": buildOutbounds(proxy, opts),
		"routing":   buildRouting(opts.Settings),
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// proxyOutbound converts the profile's share link into an xray outbound, tags it "proxy",
// sanitizes the libXray quirk of stuffing the node remark into sendThrough, applies the
// allow-insecure override, and chains it through the ByeDPI front when requested.
func proxyOutbound(opts Options) (map[string]any, error) {
	raw, err := convertLink(opts.XrayBin, opts.Profile.RawLink)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("xray: parse converted outbound: %w", err)
	}
	out["tag"] = "proxy"
	// share-link conversion can put the node remark into sendThrough, which xray then
	// rejects because it must be a bind IP; drop it unless it is a valid address.
	if st, ok := out["sendThrough"].(string); ok && net.ParseIP(st) == nil {
		delete(out, "sendThrough")
	}
	if opts.Settings.AllowInsecure {
		applyAllowInsecure(out)
	}
	if opts.ByeDPIFrontSocks != "" {
		out["proxySettings"] = map[string]any{"tag": "byedpi-front", "transportLayer": true}
	}
	return out, nil
}

// convertLink runs `bin/xray convert` with the share link on stdin and returns the first
// outbound from the resulting xray config json. xray logs warnings to stderr; only stdout
// carries the json.
func convertLink(xrayBin, link string) (json.RawMessage, error) {
	if strings.TrimSpace(xrayBin) == "" {
		return nil, errors.New("xray: helper binary path is empty")
	}
	cmd := exec.Command(xrayBin, "convert")
	cmd.Stdin = strings.NewReader(link)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	hideWindow(cmd)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("xray: convert link: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var parsed struct {
		Outbounds []json.RawMessage `json:"outbounds"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return nil, fmt.Errorf("xray: decode converted config: %w", err)
	}
	if len(parsed.Outbounds) == 0 {
		return nil, errors.New("xray: share link produced no outbound")
	}
	return parsed.Outbounds[0], nil
}

func buildInbounds(opts Options) []any {
	var in []any
	sniff := map[string]any{
		"enabled":      opts.Settings.SniffingEnabled,
		"destOverride": []string{"http", "tls", "quic"},
		"routeOnly":    false,
	}
	if opts.IncludeTun {
		in = append(in, map[string]any{
			"tag":      "tun-in",
			"protocol": "tun",
			"settings": map[string]any{
				"name":      TunDeviceName,
				"mtu":       1500,
				"userLevel": 0,
				// bind outbound sockets to the physical default-route interface so the
				// proxy's own upstream traffic does not loop back into the tun.
				"autoOutboundsInterface": "auto",
			},
			"sniffing": sniff,
		})
	}
	if opts.Settings.LocalProxyEnabled {
		socks := map[string]any{"udp": true}
		if opts.Settings.LocalProxyAuthEnabled && opts.Settings.LocalProxyUsername != "" {
			socks["auth"] = "password"
			socks["accounts"] = []any{map[string]any{
				"user": opts.Settings.LocalProxyUsername,
				"pass": opts.Settings.LocalProxyPassword,
			}}
		} else {
			socks["auth"] = "noauth"
		}
		in = append(in, map[string]any{
			"tag":      "socks-in",
			"protocol": "socks",
			"listen":   listenAddr(opts.Settings.LocalProxyListenAddress),
			"port":     opts.Settings.LocalProxyPort,
			"settings": socks,
			"sniffing": sniff,
		})
	}
	if opts.Settings.HTTPProxyEnabled {
		http := map[string]any{}
		if opts.Settings.HTTPProxyAuthEnabled && opts.Settings.HTTPProxyUsername != "" {
			http["accounts"] = []any{map[string]any{
				"user": opts.Settings.HTTPProxyUsername,
				"pass": opts.Settings.HTTPProxyPassword,
			}}
		}
		in = append(in, map[string]any{
			"tag":      "http-in",
			"protocol": "http",
			"listen":   listenAddr(opts.Settings.HTTPProxyListenAddress),
			"port":     opts.Settings.HTTPProxyPort,
			"settings": http,
			"sniffing": sniff,
		})
	}
	return in
}

func buildOutbounds(proxy map[string]any, opts Options) []any {
	out := []any{
		proxy,
		map[string]any{"tag": "direct", "protocol": "freedom", "settings": map[string]any{}},
		map[string]any{"tag": "block", "protocol": "blackhole", "settings": map[string]any{}},
		map[string]any{"tag": "dns-out", "protocol": "dns"},
	}
	if opts.ByeDPIFrontSocks != "" {
		host, port := splitHostPort(opts.ByeDPIFrontSocks)
		out = append(out, map[string]any{
			"tag":      "byedpi-front",
			"protocol": "socks",
			"settings": map[string]any{"servers": []any{map[string]any{"address": host, "port": port}}},
		})
	}
	return out
}

func buildRouting(s config.XraySettings) map[string]any {
	rules := []any{
		// DNS queries to the dns outbound so the fake-dns/redirect path works.
		map[string]any{"type": "field", "port": "53", "outboundTag": "dns-out"},
	}
	if !s.ProxyQuicEnabled {
		// Block QUIC so browsers fall back to TCP/TLS, which the proxy handles cleanly.
		rules = append(rules, map[string]any{
			"type": "field", "network": "udp", "port": "443", "outboundTag": "block",
		})
	}
	rules = append(rules,
		map[string]any{"type": "field", "protocol": []string{"bittorrent"}, "outboundTag": "block"},
		map[string]any{"type": "field", "network": "tcp,udp", "outboundTag": "proxy"},
	)
	return map[string]any{"domainStrategy": "IPIfNonMatch", "rules": rules}
}

func buildDNS(s config.XraySettings) map[string]any {
	strategy := "UseIP"
	if !s.IPv6 {
		strategy = "UseIPv4"
	}
	servers := []any{}
	if s.RemoteDNS != "" {
		servers = append(servers, s.RemoteDNS)
	}
	// A plain fallback resolver keeps name resolution working if the DoH endpoint is
	// unreachable during startup.
	servers = append(servers, "1.1.1.1")
	return map[string]any{"servers": servers, "queryStrategy": strategy}
}

// applyAllowInsecure sets allowInsecure on the outbound's TLS/REALITY stream settings so a
// self-signed or hostname-mismatched cert is accepted (only when the user opts in).
func applyAllowInsecure(out map[string]any) {
	ss, ok := out["streamSettings"].(map[string]any)
	if !ok {
		return
	}
	for _, key := range []string{"tlsSettings", "realitySettings"} {
		if t, ok := ss[key].(map[string]any); ok {
			t["allowInsecure"] = true
		}
	}
}

func listenAddr(addr string) string {
	if strings.TrimSpace(addr) == "" {
		return "127.0.0.1"
	}
	return addr
}

func splitHostPort(hostPort string) (string, int) {
	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		return hostPort, 0
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	return host, port
}

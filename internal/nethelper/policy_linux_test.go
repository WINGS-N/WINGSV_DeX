//go:build linux

package nethelper

import (
	"encoding/base64"
	"testing"

	"github.com/WINGS-N/wingsv-dex/internal/wg"
	"github.com/vishvananda/netlink"
)

func validWGConfig() *wg.Config {
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	return &wg.Config{Interface: "wingsv0", PrivateKey: key, PeerPublicKey: key, PeerEndpoint: "127.0.0.1:9000", Addresses: []string{"10.0.0.2/32"}, AllowedIPs: []string{"0.0.0.0/0"}, FwMark: daemonMark, Table: daemonMark}
}

func TestValidateDaemonCommandRejectsPolicyViolations(t *testing.T) {
	baseStart := command{Cmd: "start", ProtectSocket: "wingsv-dex-protect", FwMark: daemonMark, TunName: "wingsv-tun"}
	for _, tc := range []struct {
		name    string
		cmd     command
		started bool
		active  *wg.Config
	}{
		{"wrong start socket", command{Cmd: "start", ProtectSocket: "other", FwMark: daemonMark, TunName: "wingsv-tun"}, false, nil},
		{"repeated start", baseStart, true, nil},
		{"foreign interface", command{Cmd: "wgup", Config: &wg.Config{Interface: "eth0", FwMark: daemonMark, Table: daemonMark, MTU: 1420}}, true, nil},
		{"wrong mark", command{Cmd: "wgup", Config: &wg.Config{Interface: "wingsv0", FwMark: 1, Table: daemonMark, MTU: 1420}}, true, nil},
		{"active wg", command{Cmd: "wgup", Config: &wg.Config{Interface: "wingsv0", FwMark: daemonMark, Table: daemonMark, MTU: 1420}}, true, &wg.Config{}},
		{"bad mtu", command{Cmd: "wgup", Config: &wg.Config{Interface: "wingsv0", FwMark: daemonMark, Table: daemonMark, MTU: 999999}}, true, nil},
		{"foreign pid", command{Cmd: "cgadd", Pid: -1}, true, nil},
		{"malformed xray", command{Cmd: "xrayup", TunName: "wingsv-tun", XrayConfig: "{"}, true, nil},
		{"unknown", command{Cmd: "wat"}, true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateDaemonCommand(tc.cmd, tc.started, tc.active, 12345); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestValidateDaemonCommandAllowsStartAndWGUp(t *testing.T) {
	start := command{Cmd: "start", ProtectSocket: "wingsv-dex-protect", FwMark: daemonMark, TunName: "wingsv-tun"}
	if err := validateDaemonCommand(start, false, nil, 12345); err != nil {
		t.Fatalf("valid start rejected: %v", err)
	}
	for _, mtu := range []int{0, 576, 1420, 65535} {
		c := validWGConfig()
		c.MTU = mtu
		cmd := command{Cmd: "wgup", Config: c}
		if err := validateDaemonCommand(cmd, true, nil, 12345); err != nil {
			t.Errorf("valid MTU %d rejected: %v", mtu, err)
		}
	}
	for _, mtu := range []int{-1, 1, 575, 65536} {
		c := validWGConfig()
		c.MTU = mtu
		cmd := command{Cmd: "wgup", Config: c}
		if err := validateDaemonCommand(cmd, true, nil, 12345); err == nil {
			t.Errorf("invalid MTU %d accepted", mtu)
		}
	}
}

func TestDaemonWGRejectsMalformedInputs(t *testing.T) {
	for _, mutate := range []func(*wg.Config){func(c *wg.Config) { c.PrivateKey = "bad" }, func(c *wg.Config) { c.PeerEndpoint = "example.com:80" }, func(c *wg.Config) { c.PeerEndpoint = "127.0.0.1:80\nX" }, func(c *wg.Config) { c.PeerEndpoint = "8.8.8.8:80" }, func(c *wg.Config) { c.PeerEndpoint = "127.0.0.1:abc" }, func(c *wg.Config) { c.Addresses = []string{"broken"} }, func(c *wg.Config) { c.AllowedIPs = []string{"10/8"} }, func(c *wg.Config) { c.Jc = "1\nPrivateKey=x" }} {
		c := validWGConfig()
		mutate(c)
		if err := validateDaemonCommand(command{Cmd: "wgup", Config: c}, true, nil, 1); err == nil {
			t.Fatal("invalid WG config accepted")
		}
	}
}

func TestXrayRequiresNamedSingleTun(t *testing.T) {
	if err := validateXrayConfig(`{"inbounds":[{"tag":"tun-in","protocol":"tun","settings":{"name":"wingsv-tun"}},{"protocol":"socks","tag":"s"}]}`); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{`{"inbounds":[]}`, `{"inbounds":[{"tag":"tun-in","protocol":"tun","settings":{"name":"wrong"}}]}`, `{"inbounds":[{"protocol":"tun","settings":{"name":"wingsv-tun"}},{"protocol":"tun","settings":{"name":"wingsv-tun"}}]}`} {
		if validateXrayConfig(s) == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}

func TestByeDPIRouteGuard(t *testing.T) {
	if validateByeDPIRoute(1, &netlink.Route{}) != nil {
		t.Fatal("valid route rejected")
	}
	for _, x := range []struct {
		mark  int
		route *netlink.Route
	}{{0, &netlink.Route{}}, {1, nil}} {
		if validateByeDPIRoute(x.mark, x.route) == nil {
			t.Fatal("missing route/mark accepted")
		}
	}
}

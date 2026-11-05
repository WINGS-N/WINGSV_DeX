//go:build linux

package nethelper

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/WINGS-N/wingsv-dex/internal/wg"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const daemonMark = 0x8888

func validateDaemonCommand(cmd command, started bool, active *wg.Config, uid uint32) error {
	if cmd.Cmd == "start" {
		if started {
			return fmt.Errorf("start: already started")
		}
		if cmd.ProtectSocket != "wingsv-dex-protect" || cmd.FwMark != daemonMark || cmd.TunName != "wingsv-tun" {
			return fmt.Errorf("start: invalid protect policy")
		}
		return nil
	}
	if cmd.Cmd == "stop" {
		return nil
	}
	if !started {
		return fmt.Errorf("%s: helper not started", cmd.Cmd)
	}
	switch cmd.Cmd {
	case "cgadd":
		return validateOwnedTurnPID(cmd.Pid, uid)
	case "wgup":
		if cmd.Config == nil {
			return fmt.Errorf("wgup: missing config")
		}
		c := cmd.Config
		if active != nil {
			return fmt.Errorf("wgup: already active")
		}
		if c.Interface != "wingsv0" || c.FwMark != daemonMark || c.Table != daemonMark {
			return fmt.Errorf("wgup: invalid interface or mark/table")
		}
		if err := validateWGConfig(c); err != nil {
			return fmt.Errorf("wgup: %w", err)
		}
		if c.MTU != 0 && (c.MTU < 576 || c.MTU > 65535) {
			return fmt.Errorf("wgup: invalid MTU")
		}
	case "apps":
		if cmd.SelfPid > 0 {
			if err := validatePIDUID(cmd.SelfPid, uid); err != nil {
				return fmt.Errorf("apps: self pid: %w", err)
			}
		}
		if (!cmd.Whitelist && cmd.AppMark != 0x8888) || (cmd.Whitelist && cmd.AppMark != 0x8889) {
			return fmt.Errorf("apps: invalid mark")
		}
	case "appsdown", "wgdown", "xraydown", "byedpidown":
	case "xrayup":
		if cmd.TunName != "wingsv-tun" || len(cmd.XrayConfig) > 4*1024*1024 {
			return fmt.Errorf("xrayup: invalid tun name or config size")
		}
		if err := validateXrayConfig(cmd.XrayConfig); err != nil {
			return fmt.Errorf("xrayup: %w", err)
		}
	case "byedpiup":
		if _, err := validateBundledBinary(cmd.ByeDPIBin, "byedpi"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown command")
	}
	return nil
}

func hasControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func validateWGConfig(c *wg.Config) error {
	if len(c.Addresses) == 0 || len(c.Addresses) > 16 || len(c.AllowedIPs) == 0 || len(c.AllowedIPs) > 256 {
		return fmt.Errorf("invalid address/allowed IP count")
	}
	for _, key := range []string{c.PrivateKey, c.PeerPublicKey} {
		if hasControl(key) {
			return fmt.Errorf("control character in key")
		}
		if _, err := wgtypes.ParseKey(key); err != nil {
			return fmt.Errorf("invalid key")
		}
	}
	if c.PresharedKey != "" {
		if hasControl(c.PresharedKey) {
			return fmt.Errorf("control character in key")
		}
		if _, err := wgtypes.ParseKey(c.PresharedKey); err != nil {
			return fmt.Errorf("invalid preshared key")
		}
	}
	if len(c.PeerEndpoint) > 255 || hasControl(c.PeerEndpoint) {
		return fmt.Errorf("invalid endpoint")
	}
	host, port, err := net.SplitHostPort(c.PeerEndpoint)
	ip := net.ParseIP(host)
	portNum, portErr := strconv.Atoi(port)
	if err != nil || ip == nil || !ip.IsLoopback() || portErr != nil || portNum < 1 || portNum > 65535 {
		return fmt.Errorf("endpoint must be literal IP:port")
	}
	for _, list := range [][]string{c.Addresses, c.AllowedIPs} {
		for _, cidr := range list {
			if len(cidr) > 128 {
				return fmt.Errorf("CIDR too long")
			}
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				return fmt.Errorf("invalid CIDR")
			}
		}
	}
	for _, v := range []string{c.Jc, c.Jmin, c.Jmax, c.S1, c.S2, c.S3, c.S4, c.H1, c.H2, c.H3, c.H4} {
		if len(v) > 128 || hasControl(v) {
			return fmt.Errorf("invalid AWG field")
		}
	}
	return nil
}

func validateXrayConfig(s string) error {
	if !json.Valid([]byte(s)) {
		return fmt.Errorf("malformed config JSON")
	}
	var cfg struct {
		Inbounds []struct {
			Protocol string `json:"protocol"`
			Tag      string `json:"tag"`
			Settings struct {
				Name string `json:"name"`
			} `json:"settings"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal([]byte(s), &cfg); err != nil {
		return err
	}
	n := 0
	for _, in := range cfg.Inbounds {
		if in.Protocol == "tun" {
			n++
			if in.Settings.Name != "wingsv-tun" {
				return fmt.Errorf("invalid tun inbound name")
			}
		}
	}
	if n != 1 {
		return fmt.Errorf("expected exactly one tun inbound")
	}
	return nil
}

func validateByeDPIRoute(fwmark int, physical *netlink.Route) error {
	if fwmark == 0 || physical == nil {
		return fmt.Errorf("byedpiup: physical default route and nonzero fwmark required")
	}
	return nil
}

func validateOwnedTurnPID(pid int, uid uint32) error {
	if err := validatePIDUID(pid, uid); err != nil {
		return err
	}
	exe, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	if err != nil {
		return fmt.Errorf("cannot inspect pid executable")
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot resolve helper executable")
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return fmt.Errorf("cannot resolve helper executable")
	}
	if exe != filepath.Join(filepath.Dir(self), "vkturn") {
		return fmt.Errorf("pid executable is not bundled vkturn")
	}
	return nil
}

func validatePIDUID(pid int, uid uint32) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid")
	}
	// /proc checks are subject to PID reuse between validation and subsequent use;
	// this narrows authorization but does not make process identity race-free.
	b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return fmt.Errorf("pid is not accessible")
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				break
			}
			got, e := strconv.ParseUint(fields[1], 10, 32)
			if e == nil && uint32(got) == uid {
				return nil
			}
			return fmt.Errorf("pid is not owned by allowed uid")
		}
	}
	return fmt.Errorf("pid ownership unavailable")
}

package config

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"runtime"
	"strings"
)

// DeviceInfo identifies this machine to the panel. The federation subscription pins access to
// an account's devices by fingerprint, so it is mandatory there: a missing or forged
// fingerprint means the link could be handed to anyone and the slots would be counted against
// made-up devices, which the panel rejects with 403.
type DeviceInfo struct {
	HWID      string // stable per-machine id (x-hwid)
	OS        string // x-device-os
	OSVersion string // x-ver-os
	Model     string // x-device-model
}

// DeviceFingerprint builds this machine's fingerprint. HWID is taken from the OS machine id
// where there is one and falls back to a stable hash of the hostname, so it survives restarts
// without any stored state.
func DeviceFingerprint() DeviceInfo {
	return DeviceInfo{
		HWID:      machineID(),
		OS:        deviceOS(),
		OSVersion: osVersion(),
		Model:     hostname(),
	}
}

func machineID() string {
	// Linux: the systemd/d-bus machine id is stable and world-readable.
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if b, err := os.ReadFile(p); err == nil {
			if s := strings.TrimSpace(string(b)); s != "" {
				return s
			}
		}
	}
	// Fallback (Windows/macOS or an id-less host): a stable hash of hostname + platform. Not a
	// true hardware id, but deterministic across runs, which is what slot counting needs.
	sum := sha256.Sum256([]byte(hostname() + "|" + runtime.GOOS))
	return hex.EncodeToString(sum[:16])
}

func deviceOS() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	default:
		return "Linux"
	}
}

func osVersion() string {
	if b, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		if s := strings.TrimSpace(string(b)); s != "" {
			return s
		}
	}
	return runtime.GOARCH
}

// SubscriptionHwidSettings is the user-facing control over the device fingerprint the client
// sends when refreshing subscriptions. Enabled gates sending it at all (default on); Manual
// swaps the automatic values for the hand-entered ones below. The federation subscription
// ignores these and always sends the real automatic fingerprint - its access is pinned to the
// account's devices, so a disabled or faked fingerprint there would let the link be shared.
type SubscriptionHwidSettings struct {
	Enabled     bool   `json:"enabled"`
	Manual      bool   `json:"manual"`
	HWID        string `json:"hwid"`
	DeviceOS    string `json:"deviceOs"`
	VerOS       string `json:"verOs"`
	DeviceModel string `json:"deviceModel"`
}

// DefaultSubscriptionHwidSettings has sending on and manual overrides off (Android parity).
func DefaultSubscriptionHwidSettings() SubscriptionHwidSettings {
	return SubscriptionHwidSettings{Enabled: true}
}

// Effective resolves the fingerprint to send for an ordinary subscription: an empty DeviceInfo
// when sending is off, otherwise each field taken from the manual value (when Manual is on and
// the field is non-empty) or the automatic one.
func (h SubscriptionHwidSettings) Effective() DeviceInfo {
	if !h.Enabled {
		return DeviceInfo{}
	}
	auto := DeviceFingerprint()
	return DeviceInfo{
		HWID:      pickHwid(h.Manual, h.HWID, auto.HWID),
		OS:        pickHwid(h.Manual, h.DeviceOS, auto.OS),
		OSVersion: pickHwid(h.Manual, h.VerOS, auto.OSVersion),
		Model:     pickHwid(h.Manual, h.DeviceModel, auto.Model),
	}
}

func pickHwid(manual bool, manualValue, autoValue string) string {
	if manual {
		if v := strings.TrimSpace(manualValue); v != "" {
			return v
		}
	}
	return autoValue
}

func hostname() string {
	if h, err := os.Hostname(); err == nil && strings.TrimSpace(h) != "" {
		return strings.TrimSpace(h)
	}
	return "wingsv-dex"
}

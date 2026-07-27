//go:build windows

package deeplink

import "golang.org/x/sys/windows/registry"

// RegisterSchemes registers wingsv:// and vless:// under HKCU\Software\Classes so the OS
// launches this executable when such a link is opened. Per-user (HKCU) needs no elevation and
// is idempotent, so it runs on every startup. Last writer wins for vless:// across VPN clients.
func RegisterSchemes(exePath string) {
	for _, scheme := range []string{"wingsv", "vless"} {
		_ = registerScheme(scheme, exePath)
	}
}

func registerScheme(scheme, exe string) error {
	base := `Software\Classes\` + scheme
	root, _, err := registry.CreateKey(registry.CURRENT_USER, base, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.SetStringValue("", "URL:"+scheme+" Protocol"); err != nil {
		return err
	}
	// The empty-named "URL Protocol" value marks the key as a URL scheme handler.
	if err := root.SetStringValue("URL Protocol", ""); err != nil {
		return err
	}
	cmd, _, err := registry.CreateKey(registry.CURRENT_USER, base+`\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer cmd.Close()
	return cmd.SetStringValue("", `"`+exe+`" "%1"`)
}

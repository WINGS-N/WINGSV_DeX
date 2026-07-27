// Package deeplink funnels profile-import links into the running app from two sources that
// avoid the per-OS pain of custom URL schemes being the only channel: the OS scheme handler
// (wingsv://, vless://) delivered as a process argument, and a loopback HTTP callback a web
// page can redirect to (http://127.0.0.1:PORT/callback?url=...). Both hand the raw link to a
// deliver callback; the app routes it through the same import/confirm flow as clipboard import.
package deeplink

import "strings"

// schemeLinks are the URL schemes the app registers as an OS handler. http(s) is deliberately
// excluded here (hijacking it would capture ordinary web navigation); it is still accepted on
// the loopback channel as a subscription URL.
var schemeLinks = []string{"wingsv://", "vless://"}

// ExtractSchemeLink returns the first wingsv:// or vless:// argument (the OS launches the app
// as `wingsv-dex <link>` when a registered scheme is opened).
func ExtractSchemeLink(args []string) string {
	for _, a := range args {
		if isSchemeLink(a) {
			return strings.TrimSpace(a)
		}
	}
	return ""
}

func isSchemeLink(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	for _, p := range schemeLinks {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	return false
}

// IsImportLink reports whether s is something SmartImport can accept over the loopback channel:
// a registered scheme link or an http(s) subscription URL.
func IsImportLink(s string) bool {
	if isSchemeLink(s) {
		return true
	}
	low := strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://")
}

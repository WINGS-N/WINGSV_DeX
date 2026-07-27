//go:build !windows

package deeplink

// RegisterSchemes is a no-op off Windows: Linux registers the schemes through the installed
// .desktop MimeType (x-scheme-handler/wingsv, x-scheme-handler/vless), and macOS is out of scope.
func RegisterSchemes(exePath string) {}

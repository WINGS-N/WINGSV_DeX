//go:build linux

package updater

import "testing"

func TestDetectKindNixStore(t *testing.T) {
	t.Setenv("APPIMAGE", "/tmp/wingsv-dex.AppImage")
	if got := detectKind("/nix/store/abc-wingsv-dex/bin/wingsv-dex"); got != KindNix {
		t.Fatalf("detectKind(nix store) = %q, want %q", got, KindNix)
	}
	if got := detectKind("/tmp/wingsv-dex"); got != KindAppImage {
		t.Fatalf("detectKind(non-store AppImage) = %q, want %q", got, KindAppImage)
	}
}

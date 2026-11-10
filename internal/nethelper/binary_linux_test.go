//go:build linux

package nethelper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSiblingBinary(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "wingsv-dex")
	if err := os.WriteFile(self, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"xray", "byedpi"} {
		t.Run(name, func(t *testing.T) {
			canonical := filepath.Join(dir, name)
			if err := os.WriteFile(canonical, nil, 0o700); err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(dir, name+"-alias")
			if err := os.Symlink(canonical, alias); err != nil {
				t.Fatal(err)
			}
			got, err := validateSiblingBinary(self, alias, name)
			if err != nil || got != canonical {
				t.Fatalf("resolve alias: got %q, err %v; want %q", got, err, canonical)
			}
			outside := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(outside, nil, 0o700); err != nil {
				t.Fatal(err)
			}
			for _, injected := range []string{outside, "relative", ""} {
				got, err := validateSiblingBinary(self, injected, name)
				if err == nil || got != "" || !strings.Contains(err.Error(), "refusing "+name+" binary") {
					t.Errorf("%q: expected explicit rejection, got path %q, err %v", injected, got, err)
				}
			}
		})
	}
}

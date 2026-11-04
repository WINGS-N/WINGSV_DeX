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
	sibling := filepath.Join(dir, "xray")
	if err := os.WriteFile(sibling, nil, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := validateSiblingBinary(self, sibling, "xray"); err != nil {
		t.Fatalf("accept sibling: %v", err)
	}
	for _, injected := range []string{"/bin/sh", filepath.Join(dir, "other")} {
		err := validateSiblingBinary(self, injected, "xray")
		if err == nil || !strings.Contains(err.Error(), "refusing xray binary") {
			t.Errorf("%q: expected explicit rejection, got %v", injected, err)
		}
	}
	if err := validateSiblingBinary(self, "xray", "xray"); err == nil {
		t.Error("accepted relative path")
	}
}

//go:build linux

package wg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommandPathPrefersPinnedNft(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nft")
	if err := os.WriteFile(path, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	nixNftPath = path
	t.Cleanup(func() { nixNftPath = "" })
	got, err := commandPath([]string{"missing-command"})
	if err != nil || got != path {
		t.Fatalf("commandPath = %q, %v; want %q", got, err, path)
	}
}

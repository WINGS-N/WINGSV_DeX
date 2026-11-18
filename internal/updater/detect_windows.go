//go:build windows

package updater

import (
	"os"
	"path/filepath"
	"strings"
)

func detectInstall(exe string) Install {
	l := strings.ToLower(exe)
	bases := []string{
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("ProgramW6432"),
	}
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		bases = append(bases, filepath.Join(la, "Programs"))
	}
	for _, base := range bases {
		if base != "" && strings.HasPrefix(l, strings.ToLower(base)) {
			return Install{Kind: KindWindowsSetup}
		}
	}
	return Install{Kind: KindWindowsPortable}
}

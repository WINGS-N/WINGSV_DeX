//go:build linux

package updater

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func detectInstall(exe string) Install {
	if strings.HasPrefix(filepath.Clean(exe), "/nix/store/") {
		return Install{Kind: KindNix}
	}
	if os.Getenv("APPIMAGE") != "" {
		return Install{Kind: KindAppImage}
	}
	if out, err := exec.Command("pacman", "-Qo", "--quiet", "--", exe).Output(); err == nil {
		name := strings.TrimSpace(string(out))
		result := Install{Kind: KindArch, Package: name}
		if out, err := exec.Command("pacman", "-Q", "--", name).Output(); err == nil {
			if fields := strings.Fields(string(out)); len(fields) == 2 {
				result.Version = fields[1]
			}
		}
		return result
	}
	if owns("dpkg", "-S", exe) {
		return Install{Kind: KindDeb}
	}
	if owns("rpm", "-qf", exe) {
		return Install{Kind: KindRPM}
	}
	return Install{Kind: KindBinary}
}

func owns(bin string, args ...string) bool {
	if _, err := exec.LookPath(bin); err != nil {
		return false
	}
	return exec.Command(bin, args...).Run() == nil
}

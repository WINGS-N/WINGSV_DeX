//go:build linux

package updater

import (
	"os"
	"os/exec"
)

func detectKind(exe string) Kind {
	if os.Getenv("APPIMAGE") != "" {
		return KindAppImage
	}
	if owns("dpkg", "-S", exe) {
		return KindDeb
	}
	if owns("rpm", "-qf", exe) {
		return KindRPM
	}
	return KindBinary
}

func owns(bin string, args ...string) bool {
	if _, err := exec.LookPath(bin); err != nil {
		return false
	}
	return exec.Command(bin, args...).Run() == nil
}

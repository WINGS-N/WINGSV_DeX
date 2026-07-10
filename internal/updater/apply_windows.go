//go:build windows

package updater

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

func packageInstall(inst Install, file string) error {
	l := strings.ToLower(file)
	switch {
	case strings.HasSuffix(l, ".exe"):
		// NSIS silent install to the same location; the app quits after so files unlock.
		cmd := exec.Command(file, "/S")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return cmd.Start()
	case strings.HasSuffix(l, ".msi"):
		cmd := exec.Command("msiexec", "/i", file, "/qb")
		return cmd.Start()
	}
	return fmt.Errorf("updater: unsupported package %s", file)
}

// replaceExe cannot overwrite a running exe, so it stages the new binary and hands the swap
// to a detached helper that waits for this process to exit, moves it into place and relaunches.
func replaceExe(inst Install, newBin string) error {
	staged := inst.ExePath + ".new"
	if err := copyFile(newBin, staged, 0o755); err != nil {
		return err
	}
	script := fmt.Sprintf(`ping -n 3 127.0.0.1 >nul & move /y "%s" "%s" & start "" "%s"`, staged, inst.ExePath, inst.ExePath)
	cmd := exec.Command("cmd", "/c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}

// replacePlain overwrites vkturn directly; the child is stopped before the update runs.
func replacePlain(inst Install, path, newBin string) error {
	return copyFile(newBin, path, 0o755)
}

// Restart is a no-op on Windows: the portable swap helper and the NSIS installer both
// relaunch the app themselves after this process exits.
func Restart(inst Install) error { return nil }

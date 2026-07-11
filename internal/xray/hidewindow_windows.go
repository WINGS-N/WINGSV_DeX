//go:build windows

package xray

import (
	"os/exec"
	"syscall"
)

// hideWindow keeps the short-lived convert helper from flashing a console window.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

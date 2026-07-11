//go:build !windows

package xray

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}

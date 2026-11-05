//go:build linux

package nethelper

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

// setXrayPdeathsig makes the kernel SIGKILL the xray child if the helper dies, so a
// crashed helper never orphans xray still holding the TUN device.
func setXrayPdeathsig(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}

func setChildCredential(cmd *exec.Cmd, credential *childCredential) {
	if credential == nil {
		return
	}
	attr := cmd.SysProcAttr
	if attr == nil {
		attr = &syscall.SysProcAttr{}
	}
	attr.Credential = &syscall.Credential{Uid: credential.Uid, Gid: credential.Gid, Groups: credential.Groups}
	attr.AmbientCaps = []uintptr{unix.CAP_NET_ADMIN}
	cmd.SysProcAttr = attr
}

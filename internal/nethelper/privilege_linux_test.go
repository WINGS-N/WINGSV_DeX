//go:build linux

package nethelper

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func TestSetChildCredentialPreservesPdeathsigAndAddsCapability(t *testing.T) {
	cmd := exec.Command("true")
	setXrayPdeathsig(cmd)
	credential := &childCredential{Uid: 1234, Gid: 2345, Groups: []uint32{3456, 4567}}
	setChildCredential(cmd, credential)

	attr := cmd.SysProcAttr
	if attr.Pdeathsig != syscall.SIGKILL {
		t.Fatalf("Pdeathsig = %v, want SIGKILL", attr.Pdeathsig)
	}
	if attr.Credential == nil || attr.Credential.Uid != credential.Uid || attr.Credential.Gid != credential.Gid || !reflect.DeepEqual(attr.Credential.Groups, credential.Groups) {
		t.Fatalf("Credential = %#v, want uid/gid/groups %#v", attr.Credential, credential)
	}
	if !reflect.DeepEqual(attr.AmbientCaps, []uintptr{unix.CAP_NET_ADMIN}) {
		t.Fatalf("AmbientCaps = %v, want [%d] (CAP_NET_ADMIN)", attr.AmbientCaps, unix.CAP_NET_ADMIN)
	}
}

func TestRemoveXrayScratchRemovesFixedFilesOnly(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"config.json", "run.json", "user-data"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := removeXrayScratch(dir); err == nil {
		t.Fatal("expected rmdir to fail while unexpected user file remains")
	}
	if _, err := os.Stat(filepath.Join(dir, "user-data")); err != nil {
		t.Fatalf("unexpected user file was removed: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "user-data")); err != nil {
		t.Fatal(err)
	}
	if err := removeXrayScratch(dir); err != nil {
		t.Fatalf("remove fixed scratch files and directory: %v", err)
	}
}

func TestRunForUIDRejectsRoot(t *testing.T) {
	if err := RunForUID(nil, nil, 0); err == nil {
		t.Fatal("RunForUID accepted root UID")
	}
}

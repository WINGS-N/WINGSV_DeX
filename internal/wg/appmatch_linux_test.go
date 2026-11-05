//go:build linux

package wg

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestProcessUID(t *testing.T) {
	status := filepath.Join(t.TempDir(), "status")
	if err := os.WriteFile(status, []byte("Name:\ttest\nUid:\t1234\t1234\t1234\t1234\n"), 0600); err != nil {
		t.Fatal(err)
	}
	uid, err := processUID(status)
	if err != nil || uid != 1234 {
		t.Fatalf("processUID() = %d, %v; want 1234, nil", uid, err)
	}
}

func TestAppMatcherForUID(t *testing.T) {
	pid := os.Getpid()
	uid, err := processUID("/proc/" + strconv.Itoa(pid) + "/status")
	if err != nil {
		t.Fatal(err)
	}
	name, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil {
		t.Fatal(err)
	}
	exec := string(name[:len(name)-1])
	m := &AppMatcher{execs: map[string]bool{exec: true}, uid: &uid}
	if !m.matches(pid) {
		t.Fatal("current process did not match its own UID")
	}
	wrongUID := uid + 1
	m.uid = &wrongUID
	if m.matches(pid) {
		t.Fatal("current process matched a different UID")
	}
	if m.matches(0) {
		t.Fatal("PID zero matched")
	}
}

//go:build linux

package nethelper

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testListener(t *testing.T) (*net.UnixListener, string, os.FileInfo) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "helper.sock")
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	l.SetUnlinkOnClose(false)
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l, path, info
}

func TestServeAcceptsAllowedUIDAndCancelClosesSession(t *testing.T) {
	l, path, info := testListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	running := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- serve(ctx, l, path, info, uint32(os.Getuid()), func(r io.Reader, w io.Writer) error { close(running); _, _ = io.Copy(io.Discard, r); return nil })
	}()
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-running:
	case <-time.After(time.Second):
		t.Fatal("allowed peer was not dispatched")
	}
	cancel()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop after cancellation")
	}
	_ = c.Close()
}

func TestServeRejectsOtherUID(t *testing.T) {
	l, path, info := testListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dispatched := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, l, path, info, uint32(os.Getuid())+1, func(io.Reader, io.Writer) error { dispatched <- struct{}{}; return nil })
	}()
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	var b [1]byte
	_, _ = c.Read(b[:])
	select {
	case <-dispatched:
		t.Fatal("unauthorized peer dispatched")
	default:
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
}

func TestServeDoesNotRemoveReplacementSocket(t *testing.T) {
	l, path, info := testListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	entered := make(chan struct{})
	go func() {
		done <- serve(ctx, l, path, info, uint32(os.Getuid()), func(io.Reader, io.Writer) error { close(entered); <-ctx.Done(); return nil })
	}()
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("handler not entered")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	replacement, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
	if _, err := os.Lstat(path); err != nil {
		t.Fatal(err)
	}
	_ = replacement.Close()
}

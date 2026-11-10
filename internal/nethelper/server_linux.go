//go:build linux

package nethelper

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

// RunServer serves one authenticated helper session at a time until shutdown.
func RunServer(path string, allowedUID uint32) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("net-helper server requires root")
	}
	parent := filepath.Dir(path)
	st, err := os.Lstat(parent)
	if err != nil {
		return err
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || st.Mode().Perm()&0022 != 0 || st.Sys().(*syscall.Stat_t).Uid != 0 || parent != "/run/wingsv-dex" {
		return fmt.Errorf("unsafe socket parent %s", parent)
	}
	if st, err := os.Lstat(path); err == nil {
		if st.Mode()&os.ModeSocket == 0 {
			return fmt.Errorf("refusing existing non-socket %s", path)
		}
		return fmt.Errorf("socket already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return err
	}
	l := net.Listener(listener)
	listener.SetUnlinkOnClose(false)
	created, err := os.Lstat(path)
	if err != nil {
		_ = l.Close()
		return err
	}
	if err = os.Chmod(path, 0600); err != nil {
		return err
	}
	if err = os.Chown(path, int(allowedUID), -1); err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	return serve(ctx, listener, path, created, allowedUID, func(r io.Reader, w io.Writer) error { return RunForUID(r, w, allowedUID) })
}

func serve(ctx context.Context, l *net.UnixListener, path string, created os.FileInfo, allowedUID uint32, run func(io.Reader, io.Writer) error) error {
	var active net.Conn
	var activeMu sync.Mutex
	defer func() {
		activeMu.Lock()
		if active != nil {
			_ = active.Close()
		}
		activeMu.Unlock()
		_ = l.Close()
		if st, e := os.Lstat(path); e == nil && os.SameFile(created, st) && st.Mode()&os.ModeSocket != 0 {
			_ = os.Remove(path)
		}
	}()
	go func() {
		<-ctx.Done()
		_ = l.Close()
		activeMu.Lock()
		if active != nil {
			_ = active.Close()
		}
		activeMu.Unlock()
	}()
	for {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		uc, ok := conn.(*net.UnixConn)
		if !ok {
			_ = conn.Close()
			continue
		}
		raw, err := uc.SyscallConn()
		if err != nil {
			_ = conn.Close()
			continue
		}
		var cred *syscall.Ucred
		var sockErr error
		if err = raw.Control(func(fd uintptr) {
			cred, sockErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		}); err != nil || sockErr != nil || cred == nil || cred.Uid != allowedUID {
			_ = conn.Close()
			continue
		}
		activeMu.Lock()
		active = conn
		activeMu.Unlock()
		_ = run(conn, conn)
		activeMu.Lock()
		active = nil
		activeMu.Unlock()
		_ = conn.Close()
	}
}

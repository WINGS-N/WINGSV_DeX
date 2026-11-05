//go:build linux

package wg

import (
	"net"
	"os"
	"testing"
	"time"
)

func TestListenProtectForUID(t *testing.T) {
	uid := uint32(os.Getuid())
	for _, tc := range []struct {
		name      string
		allowed   uint32
		connected bool
	}{
		{name: "authorized", allowed: uid, connected: true},
		{name: "unauthorized", allowed: uid + 1, connected: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := "wg-protect-test-" + tc.name + "-" + time.Now().Format("150405.000000000")
			server, err := ListenProtectForUID(name, 0, tc.allowed)
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()

			conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: "@" + name, Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			if _, err := conn.Write([]byte{1}); err != nil {
				t.Fatal(err)
			}
			var ack [1]byte
			_, err = conn.Read(ack[:])
			if tc.connected {
				if err != nil || ack[0] != 1 {
					t.Fatalf("authorized connection was not handled: ack=%v err=%v", ack, err)
				}
			} else if err == nil {
				t.Fatal("unauthorized connection was handled")
			}
		})
	}
}

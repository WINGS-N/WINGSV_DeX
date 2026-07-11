//go:build windows

package services

import (
	"net"

	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

// interfaceCounters reads a network interface's rx/tx byte counters via GetIfEntry2, so the
// wintun-based xray TUN reports traffic on Windows the same way sysfs does on Linux. rx =
// InOctets (download), tx = OutOctets (upload).
func interfaceCounters(name string) (int64, int64, bool) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return 0, 0, false
	}
	luid, err := winipcfg.LUIDFromIndex(uint32(iface.Index))
	if err != nil {
		return 0, 0, false
	}
	row, err := luid.Interface()
	if err != nil {
		return 0, 0, false
	}
	return int64(row.InOctets), int64(row.OutOctets), true
}

//go:build linux

package wg

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
)

func TestSameRoute(t *testing.T) {
	base := netlink.Route{Gw: net.ParseIP("192.0.2.1"), LinkIndex: 3, Table: BypassTable, Protocol: bypassRouteProtocol}
	if !sameRoute(base, base) {
		t.Fatal("identical default routes must match")
	}
	kernelRoute := base
	kernelRoute.Scope = netlink.SCOPE_UNIVERSE
	kernelRoute.Flags = 0x40
	kernelRoute.Dst = &net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)}
	if !sameRoute(base, kernelRoute) {
		t.Fatal("kernel-normalized fields should not prevent an ownership match")
	}
	for _, foreign := range []netlink.Route{
		func() netlink.Route { r := base; r.Protocol++; return r }(),
		func() netlink.Route { r := base; r.Gw = net.ParseIP("192.0.2.2"); return r }(),
		func() netlink.Route { r := base; r.LinkIndex++; return r }(),
	} {
		if sameRoute(base, foreign) {
			t.Fatal("foreign route identity must not match")
		}
	}
}

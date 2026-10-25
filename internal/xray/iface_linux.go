//go:build linux

package xray

import "github.com/vishvananda/netlink"

// mainRouteTable is the kernel main routing table id (RT_TABLE_MAIN).
const mainRouteTable = 254

// physicalOutboundInterface returns the name of the main-table default-route interface,
// skipping the wingsv tun. The tun inbound's "auto" detection runs after the tun exists
// and on some networks its periodic lookup returns no interface; xray-core then fails the
// dial outright ("proxy/tun: iface not found", upstream #7173), which kills even the DNS
// resolvers. Pinning a concrete name makes the fork resolve it with InterfaceByName, which
// never yields that nil. Computed while the config is built, before any tun exists, so only
// the physical default is present. Empty means detection failed; the caller keeps "auto".
func physicalOutboundInterface() string {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Table: mainRouteTable}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return ""
	}
	var best *netlink.Route
	for i := range routes {
		route := routes[i]
		// A default route has either no Dst or a zero-length (/0) mask; the kernel
		// reports it as 0.0.0.0/0 rather than nil, so a plain nil check misses it.
		if route.Dst != nil {
			if ones, _ := route.Dst.Mask.Size(); ones != 0 {
				continue
			}
		}
		if route.LinkIndex == 0 {
			continue
		}
		link, err := netlink.LinkByIndex(route.LinkIndex)
		if err != nil || link.Attrs().Name == TunDeviceName {
			continue
		}
		if best == nil || route.Priority < best.Priority {
			best = &routes[i]
		}
	}
	if best == nil {
		return ""
	}
	link, err := netlink.LinkByIndex(best.LinkIndex)
	if err != nil {
		return ""
	}
	return link.Attrs().Name
}

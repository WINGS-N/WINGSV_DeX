//go:build linux

package wg

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// BypassTable holds a single default route over the physical link, reached only by
// traffic carrying the bypass fwmark. It exists because the Xray runtime puts the tun's
// default route in the main table, where it outranks the physical one: a helper process
// fronting the tunnel (ByeDPI) would otherwise dial the node through the very tunnel it
// fronts, and each dial would arrive back at the tun inbound - a loop that ends with the
// front's connection pool exhausted and every request failing.
//
// The mark must be on the socket (SO_MARK) before connect, not on the packet: the kernel
// picks the route and the source address at connect time, so a packet-level mark applied
// afterwards only redirects an already-misaddressed SYN. The vkturn path does not need
// this table - there the tunnel's default lives in its own table and unmarked traffic is
// what gets diverted, so marked traffic reaches the physical link through main.
const (
	BypassTable         = 8889
	bypassRulePriority  = 100
	bypassRouteProtocol = 99
)

// MarkBypass is the installed table + rule pair.
type MarkBypass struct {
	fwmark    int
	route     netlink.Route
	ruleOwned bool
}

// PhysicalDefaultRoute returns the gateway and link of the main-table IPv4 default route,
// skipping tunName so a stale tun from a previous session cannot be mistaken for the
// physical link. Call it before the tun is up.
func PhysicalDefaultRoute(tunName string) (*netlink.Route, error) {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Table: mainTable}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return nil, fmt.Errorf("wg: list main routes: %w", err)
	}
	var best *netlink.Route
	for i := range routes {
		route := routes[i]
		if route.Dst != nil || route.Gw == nil {
			continue
		}
		if link, err := netlink.LinkByIndex(route.LinkIndex); err == nil && link.Attrs().Name == tunName {
			continue
		}
		if best == nil || route.Priority < best.Priority {
			best = &routes[i]
		}
	}
	if best == nil {
		return nil, fmt.Errorf("wg: no physical default route")
	}
	return best, nil
}

// SetupMarkBypass points fwmark-carrying traffic at the physical default route.
func SetupMarkBypass(fwmark int, physical *netlink.Route) (*MarkBypass, error) {
	if fwmark == 0 || physical == nil {
		return nil, fmt.Errorf("wg: mark bypass needs a mark and a physical route")
	}
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Table: BypassTable}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return nil, fmt.Errorf("wg: list bypass routes: %w", err)
	}
	// An unclean shutdown (SIGKILL, no teardown) can leave our own route here. Clear routes
	// we own, marked with bypassRouteProtocol, so startup self-heals; a route we do not
	// recognize still blocks, since the table is ours to manage.
	for i := range routes {
		if routes[i].Protocol != netlink.RouteProtocol(bypassRouteProtocol) {
			return nil, fmt.Errorf("wg: bypass table %d is occupied", BypassTable)
		}
		if err := netlink.RouteDel(&routes[i]); err != nil {
			return nil, fmt.Errorf("wg: clear stale bypass route: %w", err)
		}
	}
	route := netlink.Route{
		Gw:        physical.Gw,
		LinkIndex: physical.LinkIndex,
		Table:     BypassTable,
		Protocol:  netlink.RouteProtocol(bypassRouteProtocol),
	}
	if err := netlink.RouteAdd(&route); err != nil {
		return nil, fmt.Errorf("wg: bypass default route: %w", err)
	}
	m := &MarkBypass{fwmark: fwmark, route: route}
	rule := m.rule()
	if err := netlink.RuleAdd(rule); err != nil {
		_ = m.deleteOwnedRoute()
		return nil, fmt.Errorf("wg: bypass fwmark rule: %w", err)
	}
	m.ruleOwned = true
	return m, nil
}

func (m *MarkBypass) rule() *netlink.Rule {
	r := netlink.NewRule()
	r.Family = netlink.FAMILY_V4
	r.Table = BypassTable
	r.Mark = uint32(m.fwmark)
	r.Priority = bypassRulePriority
	return r
}

// Close removes only the rule and route installed by this instance.
func (m *MarkBypass) Close() error {
	if m == nil {
		return nil
	}
	if m.ruleOwned {
		_ = netlink.RuleDel(m.rule())
		m.ruleOwned = false
	}
	return m.deleteOwnedRoute()
}

func (m *MarkBypass) deleteOwnedRoute() error {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Table: BypassTable}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return err
	}
	for i := range routes {
		if sameRoute(routes[i], m.route) {
			return netlink.RouteDel(&routes[i])
		}
	}
	return nil
}

func sameRoute(a, b netlink.Route) bool {
	return a.Table == b.Table && a.LinkIndex == b.LinkIndex &&
		a.Protocol == b.Protocol && a.Priority == b.Priority &&
		equalIP(a.Gw, b.Gw) && equalIPNet(a.Dst, b.Dst)
}

func equalIP(a, b net.IP) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(b)
}

func equalIPNet(a, b *net.IPNet) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		if a == nil {
			return isIPv4Default(b)
		}
		return isIPv4Default(a)
	}
	return a.IP.Equal(b.IP) && reflectIPMaskEqual(a.Mask, b.Mask)
}

func isIPv4Default(dst *net.IPNet) bool {
	return dst != nil && dst.IP.To4() != nil && dst.IP.To4().IsUnspecified() &&
		dst.Mask.String() == net.CIDRMask(0, 32).String()
}

func reflectIPMaskEqual(a, b net.IPMask) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

//go:build linux

package wg

import (
	"fmt"

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
	BypassTable        = 8889
	bypassRulePriority = 100
)

// MarkBypass is the installed table + rule pair.
type MarkBypass struct {
	fwmark int
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
	m := &MarkBypass{fwmark: fwmark}
	// Replace rather than add: a previous session that died without teardown would
	// otherwise leave a stale default here and every bypassed dial would follow it.
	if err := netlink.RouteReplace(&netlink.Route{
		Gw:        physical.Gw,
		LinkIndex: physical.LinkIndex,
		Table:     BypassTable,
	}); err != nil {
		return nil, fmt.Errorf("wg: bypass default route: %w", err)
	}
	rule := m.rule()
	_ = netlink.RuleDel(rule)
	if err := netlink.RuleAdd(rule); err != nil {
		_ = flushBypassTable()
		return nil, fmt.Errorf("wg: bypass fwmark rule: %w", err)
	}
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

// Close removes the rule and empties the table.
func (m *MarkBypass) Close() error {
	if m == nil {
		return nil
	}
	_ = netlink.RuleDel(m.rule())
	return flushBypassTable()
}

func flushBypassTable() error {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Table: BypassTable}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return err
	}
	for i := range routes {
		_ = netlink.RouteDel(&routes[i])
	}
	return nil
}

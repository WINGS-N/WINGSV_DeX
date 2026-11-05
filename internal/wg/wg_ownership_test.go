//go:build linux

package wg

import (
	"testing"

	"github.com/vishvananda/netlink"
)

func TestOwnedLink(t *testing.T) {
	for _, tc := range []struct {
		name      string
		link      netlink.Link
		awg, want bool
	}{
		{"foreign wireguard", &netlink.Wireguard{LinkAttrs: netlink.LinkAttrs{Name: "wingsv0"}}, false, false},
		{"foreign dummy", &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "wingsv0", Alias: linkAlias}}, false, false},
		{"owned wireguard", &netlink.Wireguard{LinkAttrs: netlink.LinkAttrs{Name: "wingsv0", Alias: linkAlias}}, false, true},
		{"wrong type", &netlink.Wireguard{LinkAttrs: netlink.LinkAttrs{Name: "wingsv0", Alias: linkAlias}}, true, false},
		{"owned awg", &netlink.GenericLink{LinkAttrs: netlink.LinkAttrs{Name: "wingsv0", Alias: linkAlias}, LinkType: "amneziawg"}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ownedLink(tc.link, tc.awg); got != tc.want {
				t.Fatalf("ownedLink() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateConfigRejectsRegularWGKeys(t *testing.T) {
	cfg := Config{PrivateKey: "invalid", PeerPublicKey: "invalid", PeerEndpoint: "127.0.0.1:9000"}
	if err := validateConfig(cfg); err == nil {
		t.Fatal("invalid regular WireGuard keys accepted")
	}
}

func TestDeleteOwnedAWGRefusesForeignReplacement(t *testing.T) {
	created := &netlink.GenericLink{LinkAttrs: netlink.LinkAttrs{Name: "wingsv0", Index: 4, Alias: linkAlias}, LinkType: "amneziawg"}
	foreign := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "wingsv0", Index: 5, Alias: linkAlias}}
	deleted := false
	err := deleteOwnedAWG(created, foreign, func(netlink.Link) error { deleted = true; return nil })
	if err == nil {
		t.Fatal("expected foreign replacement refusal")
	}
	if deleted {
		t.Fatal("foreign replacement was deleted")
	}
	if err := deleteOwnedAWG(created, nil, func(netlink.Link) error { deleted = true; return nil }); err == nil {
		t.Fatal("missing link was not refused")
	}
	if deleted {
		t.Fatal("missing link caused deletion")
	}
}

func TestSameOwnedAWGAfterAliasSet(t *testing.T) {
	created := &netlink.GenericLink{LinkAttrs: netlink.LinkAttrs{Name: "wingsv0", Index: 4}, LinkType: "amneziawg"}
	current := &netlink.GenericLink{LinkAttrs: netlink.LinkAttrs{Name: "wingsv0", Index: 4, Alias: linkAlias}, LinkType: "amneziawg"}
	// LinkSetAlias updates the kernel, not the local link snapshot.
	created.Attrs().Alias = linkAlias
	if !sameOwnedAWG(created, current) {
		t.Fatal("same AWG link rejected after alias was set")
	}
}

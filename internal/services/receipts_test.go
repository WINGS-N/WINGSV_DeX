package services

import "testing"

func TestTunnelGatewayOf(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"ipv4 /32", []string{"10.8.0.5/32"}, "10.8.0.1"},
		{"ipv4 no mask", []string{"10.8.0.42"}, "10.8.0.1"},
		{"skips ipv6, takes ipv4", []string{"fd00::2/128", "172.16.4.9/32"}, "172.16.4.1"},
		{"ipv6 only -> none", []string{"fd00::2/128"}, ""},
		{"empty -> none", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := tunnelGatewayOf(c.in); got != c.want {
				t.Fatalf("tunnelGatewayOf(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

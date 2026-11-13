package middleware

import (
	"net/netip"
	"testing"
)

func TestClientIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("fd00::/8")}

	for _, test := range []struct {
		name         string
		peer         string
		forwardedFor []string
		want         string
	}{
		{"peer", "203.0.113.7:51234", nil, "203.0.113.7"},
		{"untrusted peer's header is ignored", "203.0.113.7:51234", []string{"198.51.100.1"}, "203.0.113.7"},
		{"trusted peer without a header", "10.0.0.2:4000", nil, "10.0.0.2"},
		{"behind a proxy", "10.0.0.2:4000", []string{"198.51.100.1"}, "198.51.100.1"},
		{"through two proxies", "10.0.0.2:4000", []string{"198.51.100.1, 10.0.0.3"}, "198.51.100.1"},
		{"spoofed hops left of the client", "10.0.0.2:4000", []string{"192.0.2.66, 198.51.100.1"}, "198.51.100.1"},
		{"several headers", "10.0.0.2:4000", []string{"192.0.2.66", "198.51.100.1, 10.0.0.3"}, "198.51.100.1"},
		{"garbage stops at the last good hop", "10.0.0.2:4000", []string{"198.51.100.1, nonsense, 10.0.0.3"}, "10.0.0.3"},
		{"only proxies", "10.0.0.2:4000", []string{"10.0.0.4"}, "10.0.0.4"},
		{"IPv6", "[fd00::2]:4000", []string{"2001:db8::1"}, "2001:db8::1"},
		{"IPv4 in IPv6", "[::ffff:10.0.0.2]:4000", []string{"198.51.100.1"}, "198.51.100.1"},
		{"no port", "203.0.113.7", nil, "203.0.113.7"},
		{"unknown peer", "", []string{"198.51.100.1"}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ClientIP(test.peer, test.forwardedFor, trusted); got != test.want {
				t.Errorf("ClientIP(%q, %q) = %q, want %q", test.peer, test.forwardedFor, got, test.want)
			}
		})
	}
}

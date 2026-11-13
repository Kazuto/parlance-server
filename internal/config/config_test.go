package config

import (
	"net/netip"
	"slices"
	"testing"
)

func TestParsePrefixes(t *testing.T) {
	got := parsePrefixes("TRUSTED_PROXIES", "10.0.0.0/8, 172.18.0.5,fd00::/8, ::ffff:192.0.2.1, 192.0.2.77/24, proxy.local, 300.1.1.1")
	want := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("172.18.0.5/32"),
		netip.MustParsePrefix("fd00::/8"),
		netip.MustParsePrefix("192.0.2.1/32"),
		netip.MustParsePrefix("192.0.2.0/24"),
	}

	if !slices.Equal(got, want) {
		t.Errorf("parsePrefixes() = %v, want %v", got, want)
	}

	if got := parsePrefixes("TRUSTED_PROXIES", ""); len(got) != 0 {
		t.Errorf("parsePrefixes(\"\") = %v, want none", got)
	}
}

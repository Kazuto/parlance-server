package middleware

import (
	"net/netip"
	"strings"
)

// ClientIP returns the address a request came from. That's the peer's, unless the
// peer is a trusted proxy: then it's the right-most address in X-Forwarded-For that
// isn't one, since clients can put anything left of it.
func ClientIP(peerAddr string, forwardedFor []string, trusted []netip.Prefix) string {
	ip, ok := parseIP(peerAddr)
	if !ok {
		return peerAddr
	}

	var hops []string
	for _, value := range forwardedFor {
		hops = append(hops, strings.Split(value, ",")...)
	}

	for i := len(hops) - 1; i >= 0 && isTrusted(ip, trusted); i-- {
		hop, ok := parseIP(strings.TrimSpace(hops[i]))
		if !ok {
			break
		}

		ip = hop
	}

	return ip.String()
}

// parseIP reads an address with or without a port.
func parseIP(value string) (netip.Addr, bool) {
	if addrPort, err := netip.ParseAddrPort(value); err == nil {
		return addrPort.Addr().Unmap(), true
	}

	addr, err := netip.ParseAddr(value)

	return addr.Unmap(), err == nil
}

func isTrusted(ip netip.Addr, trusted []netip.Prefix) bool {
	for _, prefix := range trusted {
		if prefix.Contains(ip) {
			return true
		}
	}

	return false
}

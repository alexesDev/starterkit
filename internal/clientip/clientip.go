// Package clientip resolves the caller's address from behind a reverse proxy.
//
// X-Forwarded-For is only consulted when the request itself came from a trusted
// proxy, and the chain is walked from the right past trusted hops. See
// docs/auth-dex.md.
package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func Parse(list string) []netip.Prefix {
	var out []netip.Prefix

	for _, item := range strings.Split(list, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		prefix, err := netip.ParsePrefix(item)
		if err != nil {
			continue
		}

		out = append(out, prefix)
	}

	return out
}

func Resolve(r *http.Request, trusted []netip.Prefix) string {
	remote := hostOnly(r.RemoteAddr)

	addr, err := netip.ParseAddr(remote)
	if err != nil || !isTrusted(addr, trusted) {
		return remote
	}

	forwarded := r.Header.Values("X-Forwarded-For")

	var hops []string
	for _, header := range forwarded {
		hops = append(hops, strings.Split(header, ",")...)
	}

	for i := len(hops) - 1; i >= 0; i-- {
		hop, hopErr := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if hopErr != nil {
			continue
		}

		if isTrusted(hop, trusted) {
			continue
		}

		return hop.String()
	}

	return remote
}

func isTrusted(addr netip.Addr, trusted []netip.Prefix) bool {
	addr = addr.Unmap()

	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}

func hostOnly(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}

	return host
}

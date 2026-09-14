package server

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type proxyTrust struct{ networks []netip.Prefix }

func newProxyTrust(raw string) (*proxyTrust, error) {
	trust := &proxyTrust{}
	if strings.TrimSpace(raw) == "" {
		return trust, nil
	}
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			address, parseErr := netip.ParseAddr(value)
			if parseErr != nil || address.Zone() != "" {
				return nil, fmt.Errorf("invalid trusted proxy address or CIDR: %q", value)
			}
			address = address.Unmap()
			prefix = netip.PrefixFrom(address, address.BitLen())
		}
		trust.networks = append(trust.networks, prefix.Masked())
	}
	return trust, nil
}
func (p *proxyTrust) handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, port, err := net.SplitHostPort(r.RemoteAddr)
		address, parseErr := netip.ParseAddr(peer)
		trusted := false
		if err == nil && parseErr == nil {
			for _, network := range p.networks {
				if network.Contains(address.Unmap()) {
					trusted = true
					break
				}
			}
		}
		// Anonymous local health checks do not participate in device routing.
		if !trusted || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		values := r.Header.Values("X-Real-IP")
		var realIP netip.Addr
		if len(values) == 1 {
			realIP, err = netip.ParseAddr(strings.TrimSpace(values[0]))
		}
		if len(values) != 1 || err != nil || !realIP.IsValid() || realIP.Zone() != "" || realIP.IsUnspecified() {
			writeError(w, 400, "The configured proxy must supply one valid X-Real-IP value.")
			return
		}
		forwarded := r.Clone(r.Context())
		forwarded.RemoteAddr = net.JoinHostPort(realIP.Unmap().String(), port)
		next.ServeHTTP(w, forwarded)
	})
}

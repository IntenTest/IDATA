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
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		values := r.Header.Values("X-Real-IP")
		if !trusted {
			if len(values) > 0 {
				reportedIP := ""
				if len(values) == 1 {
					if parsed, parseReportedErr := netip.ParseAddr(strings.TrimSpace(values[0])); parseReportedErr == nil && parsed.IsValid() && parsed.Zone() == "" && !parsed.IsUnspecified() {
						reportedIP = parsed.Unmap().String()
					}
				}
				proxyIP := ""
				if parseErr == nil && address.IsValid() {
					proxyIP = address.Unmap().String()
				}
				writeJSON(w, http.StatusBadRequest, map[string]string{
					"error":      "The reverse proxy is not trusted. Device matching was blocked to prevent cross-PC access.",
					"error_code": "untrusted_proxy",
					"browser_ip": reportedIP,
					"proxy_ip":   proxyIP,
				})
				return
			}
			next.ServeHTTP(w, r)
			return
		}
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

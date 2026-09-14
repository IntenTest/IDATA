package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

type externalSchemeKey struct{}

func validatePublicOrigin(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return errors.New("public origin must be an http(s) origin without credentials, path, query, or fragment")
	}
	return nil
}

// The operator explicitly configures TLS termination. Never infer trust from
// forwarding protocol headers. The independent proxy-IP adapter normalizes
// the effective PC address before authorization.
func publicOriginHandler(raw string, next http.Handler) http.Handler {
	if raw == "" {
		return next
	}
	origin, _ := url.Parse(raw)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		if !strings.EqualFold(r.Host, origin.Host) {
			http.Error(w, "Unexpected public host", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), externalSchemeKey{}, origin.Scheme)))
	})
}

func requestIsSecure(r *http.Request) bool {
	if scheme, ok := r.Context().Value(externalSchemeKey{}).(string); ok {
		return scheme == "https"
	}
	return r.TLS != nil
}

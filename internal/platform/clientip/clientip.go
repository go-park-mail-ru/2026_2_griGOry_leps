package clientip

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type contextKey struct{}

func FromRequest(r *http.Request) string {
	if ip, ok := r.Context().Value(contextKey{}).(string); ok {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.Unmap().String()
	}
	return host
}

func Middleware(cidrs string, next http.Handler) (http.Handler, error) {
	var trusted []netip.Prefix
	for _, raw := range strings.Split(cidrs, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid TRUSTED_PROXY_CIDRS entry %q: %w", raw, err)
		}
		trusted = append(trusted, prefix)
	}
	isTrusted := func(ip netip.Addr) bool {
		for _, prefix := range trusted {
			if prefix.Contains(ip.Unmap()) {
				return true
			}
		}
		return false
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer := FromRequest(r)
		resolved := peer
		ip, err := netip.ParseAddr(peer)
		if err == nil && isTrusted(ip) {
			values := r.Header.Values("X-Forwarded-For")
			chain := strings.Split(strings.Join(values, ","), ",")
			for i := len(chain) - 1; i >= 0; i-- {
				candidate, err := netip.ParseAddr(strings.TrimSpace(chain[i]))
				if err != nil {
					resolved = peer
					break
				}
				resolved = candidate.Unmap().String()
				if !isTrusted(candidate) {
					break
				}
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, resolved)))
	}), nil
}

package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func TrustedRealIP(trustedProxies []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if len(trustedProxies) == 0 {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			remoteIP, ok := remoteAddrIP(r.RemoteAddr)
			if !ok || !isTrustedProxy(remoteIP, trustedProxies) {
				next.ServeHTTP(w, r)
				return
			}

			if clientIP, ok := forwardedClientIP(r); ok {
				r.RemoteAddr = clientIP.String()
			}

			next.ServeHTTP(w, r)
		})
	}
}

func remoteAddrIP(remoteAddr string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}

	ip, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}

func forwardedClientIP(r *http.Request) (netip.Addr, bool) {
	for _, header := range []string{"True-Client-IP", "X-Real-IP", "X-Forwarded-For"} {
		value := r.Header.Get(header)
		if value == "" {
			continue
		}

		if header == "X-Forwarded-For" {
			value, _, _ = strings.Cut(value, ",")
		}

		ip, err := netip.ParseAddr(strings.TrimSpace(value))
		if err == nil {
			return ip.Unmap(), true
		}
	}

	return netip.Addr{}, false
}

func isTrustedProxy(ip netip.Addr, trustedProxies []netip.Prefix) bool {
	for _, proxy := range trustedProxies {
		if proxy.Contains(ip) {
			return true
		}
	}
	return false
}

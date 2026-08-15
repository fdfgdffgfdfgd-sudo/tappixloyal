package httpapi

import (
	"net"
	"net/http"
	"strings"
)

// clientIP returns the address a request should be attributed to.
//
// The API runs behind Nginx, so RemoteAddr is the proxy for every proxied
// request. Reading it directly made per-client limits shared by the whole
// platform: ten failed sign-ins from one person locked everybody out.
//
// The forwarded headers are only trusted when the request actually arrived
// from a private address, i.e. from our own proxy. A request straight off the
// internet cannot talk its way into somebody else's bucket by sending its own
// X-Forwarded-For.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if !fromTrustedProxy(host) {
		return host
	}
	// Nginx overwrites X-Real-IP with the peer it saw, so a client cannot set it.
	if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); real != "" {
		return real
	}
	// Fall back to the entry our own proxy appended, which is the last one.
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if last := strings.TrimSpace(parts[len(parts)-1]); last != "" {
			return last
		}
	}
	return host
}

func fromTrustedProxy(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified()
}

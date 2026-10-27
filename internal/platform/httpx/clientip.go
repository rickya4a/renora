package httpx

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type clientIPKey struct{}

// ClientIP resolves the caller's address for logs and rate limits.
//
// Each proxy appends the address it received the request from to the right
// of X-Forwarded-For, and anything to the left of our own proxies was written
// by the client and can be forged. So with hops trusted proxies, the client is
// the entry hops positions from the right. With hops == 0 the header is
// ignored and the TCP peer address is used.
func ClientIP(hops int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := peerIP(r.RemoteAddr)
			if hops > 0 {
				if fwd, ok := forwardedIP(r.Header.Values("X-Forwarded-For"), hops); ok {
					ip = fwd
				}
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientIPKey{}, ip)))
		})
	}
}

// ClientIPFrom returns the address resolved by ClientIP, or "".
func ClientIPFrom(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey{}).(string)
	return ip
}

func forwardedIP(headers []string, hops int) (string, bool) {
	var entries []string
	for _, h := range headers {
		for _, e := range strings.Split(h, ",") {
			entries = append(entries, strings.TrimSpace(e))
		}
	}
	if len(entries) < hops {
		return "", false
	}
	addr, err := netip.ParseAddr(entries[len(entries)-hops])
	if err != nil {
		return "", false
	}
	return addr.Unmap().String(), true
}

func peerIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

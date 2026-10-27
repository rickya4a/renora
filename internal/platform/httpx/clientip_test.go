package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name string
		hops int
		xff  []string
		want string
	}{
		{"header ignored without trusted hops", 0, []string{"203.0.113.9"}, "10.0.0.2"},
		{"one load balancer", 1, []string{"203.0.113.9"}, "203.0.113.9"},
		{"client-forged entry on the left is skipped", 1, []string{"1.2.3.4, 203.0.113.9"}, "203.0.113.9"},
		{"two proxies", 2, []string{"203.0.113.9, 10.1.0.7"}, "203.0.113.9"},
		{"multiple header lines", 2, []string{"203.0.113.9", "10.1.0.7"}, "203.0.113.9"},
		{"fewer entries than hops falls back to peer", 2, []string{"203.0.113.9"}, "10.0.0.2"},
		{"garbage falls back to peer", 1, []string{"not-an-ip"}, "10.0.0.2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			h := ClientIP(tt.hops)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = ClientIPFrom(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "10.0.0.2:51234"
			for _, v := range tt.xff {
				req.Header.Add("X-Forwarded-For", v)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != tt.want {
				t.Errorf("client IP = %q, want %q", got, tt.want)
			}
		})
	}
}

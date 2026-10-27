package api

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/renora/renora-api/internal/platform/health"
)

// NewAdminRouter serves /metrics and probes on a separate port that is not
// exposed through the public load balancer.
func NewAdminRouter(reg *prometheus.Registry, checker *health.Checker) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))
	mux.HandleFunc("GET /healthz", health.Live)
	mux.HandleFunc("GET /readyz", checker.Ready)
	return mux
}

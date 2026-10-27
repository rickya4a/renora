// Package api assembles the public HTTP surface: probes and /graphql.
package api

import (
	"log/slog"
	"net/http"

	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/renora/renora-api/internal/config"
	"github.com/renora/renora-api/internal/platform/health"
	"github.com/renora/renora-api/internal/platform/httpx"
)

type Deps struct {
	Config  config.Config
	Logger  *slog.Logger
	Health  *health.Checker
	Metrics *httpx.Metrics
	GraphQL http.Handler
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(
		httpx.ClientIP(d.Config.HTTP.TrustedProxyHops),
		httpx.RequestID,
		httpx.AccessLog(d.Logger),
		httpx.Recover(d.Logger),
		d.Metrics.Middleware,
		httpx.SecurityHeaders,
	)

	r.Get("/healthz", health.Live)
	r.Get("/readyz", d.Health.Ready)

	r.Group(func(r chi.Router) {
		r.Use(
			corsMiddleware(d.Config.HTTP.CORSAllowedOrigins),
			httpx.MaxBody(d.Config.HTTP.MaxBodyBytes),
			middleware.Timeout(d.Config.HTTP.RequestTimeout),
		)
		r.Handle("/graphql", d.GraphQL)
	})

	if *d.Config.GraphQL.Playground {
		r.Get("/", playground.Handler("Renora GraphQL", "/graphql"))
	}

	// otelhttp sits outermost so the span covers the whole request and the
	// request ID middleware already sees the trace context.
	return otelhttp.NewHandler(r, "http.server",
		otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/healthz" && r.URL.Path != "/readyz"
		}),
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	)
}

// corsMiddleware allows browser clients from the given origins. Auth uses a
// bearer token, not cookies, so credentials stay off and CSRF does not apply.
func corsMiddleware(origins []string) func(http.Handler) http.Handler {
	return cors.New(cors.Options{
		AllowedOrigins: origins,
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders: []string{"Authorization", "Content-Type", httpx.RequestIDHeader, "Idempotency-Key"},
		ExposedHeaders: []string{httpx.RequestIDHeader},
		MaxAge:         600,
	}).Handler
}

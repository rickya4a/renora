package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/renora/renora-api/internal/config"
	"github.com/renora/renora-api/internal/graph"
	"github.com/renora/renora-api/internal/platform/health"
	"github.com/renora/renora-api/internal/platform/httpx"
)

func newTestServer(t *testing.T, gqlCfg config.GraphQL) *httptest.Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{
		HTTP: config.HTTP{
			MaxBodyBytes:       1 << 20,
			RequestTimeout:     5 * time.Second,
			CORSAllowedOrigins: []string{"https://app.renora.id"},
		},
		GraphQL: gqlCfg,
	}
	fixed := time.Date(2026, 10, 26, 3, 0, 0, 0, time.UTC)
	gql := graph.NewHandler(&graph.Resolver{Now: func() time.Time { return fixed }}, graph.ServerOptions{
		Config:   gqlCfg,
		Logger:   logger,
		APQCache: graphql.MapCache[string]{},
	})
	h := NewRouter(Deps{
		Config:  cfg,
		Logger:  logger,
		Health:  health.NewChecker(map[string]health.CheckFunc{"postgres": func(context.Context) error { return nil }}),
		Metrics: httpx.NewMetrics(prometheus.NewRegistry()),
		GraphQL: gql,
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func gqlConfig(introspection bool) config.GraphQL {
	off := false
	return config.GraphQL{ComplexityLimit: 100, DepthLimit: 3, Introspection: &introspection, Playground: &off}
}

type gqlResponse struct {
	Data   map[string]any `json:"data"`
	Errors []struct {
		Message    string         `json:"message"`
		Extensions map[string]any `json:"extensions"`
	} `json:"errors"`
}

func post(t *testing.T, srv *httptest.Server, body string) (*http.Response, gqlResponse) {
	t.Helper()
	resp, err := http.Post(srv.URL+"/graphql", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out gqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp, out
}

func TestServerInfo(t *testing.T) {
	srv := newTestServer(t, gqlConfig(true))

	resp, out := post(t, srv, `{"query":"{ serverInfo { version time } }"}`)
	if len(out.Errors) > 0 {
		t.Fatalf("errors: %+v", out.Errors)
	}
	info := out.Data["serverInfo"].(map[string]any)
	if info["time"] != "2026-10-26T03:00:00Z" {
		t.Errorf("time = %v", info["time"])
	}
	if resp.Header.Get(httpx.RequestIDHeader) == "" {
		t.Error("response is missing X-Request-ID")
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("security headers not set")
	}
}

func TestRequestIDIsEchoed(t *testing.T) {
	srv := newTestServer(t, gqlConfig(true))
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/healthz", nil)
	req.Header.Set(httpx.RequestIDHeader, "client-req-12345")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got := resp.Header.Get(httpx.RequestIDHeader); got != "client-req-12345" {
		t.Errorf("X-Request-ID = %q", got)
	}
}

func TestDepthLimitRejectsDeepQuery(t *testing.T) {
	cfg := gqlConfig(true)
	cfg.DepthLimit = 1
	srv := newTestServer(t, cfg)

	_, out := post(t, srv, `{"query":"{ serverInfo { version } }"}`)
	if len(out.Errors) == 0 || out.Errors[0].Extensions["code"] != "DEPTH_LIMIT_EXCEEDED" {
		t.Fatalf("want DEPTH_LIMIT_EXCEEDED, got %+v", out.Errors)
	}
}

func TestIntrospectionDisabled(t *testing.T) {
	srv := newTestServer(t, gqlConfig(false))

	_, out := post(t, srv, `{"query":"{ __schema { queryType { name } } }"}`)
	if len(out.Errors) == 0 {
		t.Fatal("introspection should be rejected when disabled")
	}
}

func TestPersistedQueryRoundTrip(t *testing.T) {
	srv := newTestServer(t, gqlConfig(true))
	// A hash the server has never seen.
	const hash = "c0b8a1ca2bd2c3a7d4c3a0b5c1a8f8e7c1f2b6b1a0c7e7e0e1d4b3c2a1f0e9d8"
	ext := `"extensions":{"persistedQuery":{"version":1,"sha256Hash":"` + hash + `"}}`

	_, out := post(t, srv, `{`+ext+`}`)
	if len(out.Errors) == 0 || out.Errors[0].Message != "PersistedQueryNotFound" {
		t.Fatalf("first call should ask for the full query, got %+v", out.Errors)
	}
}

func TestCORSPreflight(t *testing.T) {
	srv := newTestServer(t, gqlConfig(true))

	for origin, wantAllowed := range map[string]bool{
		"https://app.renora.id": true,
		"https://evil.example":  false,
	} {
		req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/graphql", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
		req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		got := resp.Header.Get("Access-Control-Allow-Origin") == origin
		if got != wantAllowed {
			t.Errorf("origin %s allowed = %v, want %v", origin, got, wantAllowed)
		}
	}
}

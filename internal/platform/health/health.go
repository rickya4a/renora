// Package health serves liveness and readiness probes.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// CheckFunc reports whether one dependency is usable.
type CheckFunc func(ctx context.Context) error

type Checker struct {
	checks   map[string]CheckFunc
	timeout  time.Duration
	draining atomic.Bool
}

// SetDraining makes Ready fail from now on, so the load balancer stops
// sending traffic before the server shuts down.
func (c *Checker) SetDraining() { c.draining.Store(true) }

func NewChecker(checks map[string]CheckFunc) *Checker {
	return &Checker{checks: checks, timeout: 2 * time.Second}
}

// Live answers 200 while the process runs. It never checks dependencies, so
// a database outage doesn't make the orchestrator restart every replica.
func Live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready answers 200 only when every dependency check passes.
func (c *Checker) Ready(w http.ResponseWriter, r *http.Request) {
	if c.draining.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "draining"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), c.timeout)
	defer cancel()

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results = make(map[string]string, len(c.checks))
		healthy = true
	)
	for name, check := range c.checks {
		wg.Go(func() {
			status := "ok"
			if err := check(ctx); err != nil {
				// Error details stay out of the response; they may reveal hosts.
				status = "unavailable"
			}
			mu.Lock()
			defer mu.Unlock()
			results[name] = status
			if status != "ok" {
				healthy = false
			}
		})
	}
	wg.Wait()

	code := http.StatusOK
	status := "ok"
	if !healthy {
		code, status = http.StatusServiceUnavailable, "unavailable"
	}
	writeJSON(w, code, map[string]any{"status": status, "checks": results})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReady(t *testing.T) {
	ok := func(context.Context) error { return nil }
	down := func(context.Context) error { return errors.New("dial tcp 10.0.0.5:5432: refused") }

	tests := []struct {
		name     string
		checks   map[string]CheckFunc
		wantCode int
	}{
		{"all healthy", map[string]CheckFunc{"postgres": ok, "redis": ok}, http.StatusOK},
		{"one down", map[string]CheckFunc{"postgres": down, "redis": ok}, http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			NewChecker(tt.checks).Ready(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

			if rec.Code != tt.wantCode {
				t.Errorf("code = %d, want %d", rec.Code, tt.wantCode)
			}
			if strings.Contains(rec.Body.String(), "10.0.0.5") {
				t.Error("readiness body leaks dependency error details")
			}
		})
	}
}

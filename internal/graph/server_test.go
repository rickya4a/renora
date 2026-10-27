package graph

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/renora/renora-api/internal/apperr"
)

func TestErrorPresenter(t *testing.T) {
	present := errorPresenter(slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	tests := []struct {
		name     string
		err      error
		wantMsg  string
		wantCode any
	}{
		{
			name:     "app error is shown with its code",
			err:      fmt.Errorf("load job: %w", apperr.NotFound("job")),
			wantMsg:  "job not found",
			wantCode: "NOT_FOUND",
		},
		{
			name:     "app error cause stays hidden",
			err:      apperr.Wrap(errors.New("pq: relation app.jobs does not exist"), apperr.CodeConflict, "agreement already signed"),
			wantMsg:  "agreement already signed",
			wantCode: "CONFLICT",
		},
		{
			name:     "unknown error is masked",
			err:      errors.New("dial tcp 10.0.0.5:5432: connection refused"),
			wantMsg:  "internal server error",
			wantCode: "INTERNAL",
		},
		{
			name:    "plain graphql error passes through",
			err:     gqlerror.Errorf("Cannot query field \"foo\""),
			wantMsg: "Cannot query field \"foo\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := present(ctx, tt.err)
			if got.Message != tt.wantMsg {
				t.Errorf("message = %q, want %q", got.Message, tt.wantMsg)
			}
			if tt.wantCode != nil && got.Extensions["code"] != tt.wantCode {
				t.Errorf("code = %v, want %v", got.Extensions["code"], tt.wantCode)
			}
		})
	}
}

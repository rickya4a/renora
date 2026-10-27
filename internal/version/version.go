// Package version holds build metadata, set at link time:
//
//	go build -ldflags "-X github.com/renora/renora-api/internal/version.Version=v1.2.3 -X github.com/renora/renora-api/internal/version.Commit=abc123"
package version

var (
	Version = "dev"
	Commit  = "unknown"
)

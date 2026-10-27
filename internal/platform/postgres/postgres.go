// Package postgres opens the shared pgx connection pool.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/renora/renora-api/internal/config"
)

// Open creates a pool and waits until the database answers, up to 30s,
// so containers that start together don't crash-loop.
func Open(ctx context.Context, cfg config.Database, appName string) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	pc.MaxConns = cfg.MaxConns
	pc.MinConns = cfg.MinConns
	pc.MaxConnLifetime = cfg.MaxConnLifetime
	pc.MaxConnLifetimeJitter = cfg.MaxConnLifetime / 10
	pc.MaxConnIdleTime = cfg.MaxConnIdleTime
	pc.HealthCheckPeriod = 30 * time.Second
	pc.ConnConfig.RuntimeParams["application_name"] = appName
	// Store and compare all timestamps in UTC; each Job carries its own zone.
	pc.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pc.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	deadline, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		pingCtx, pingCancel := context.WithTimeout(deadline, 3*time.Second)
		err = pool.Ping(pingCtx)
		pingCancel()
		if err == nil {
			return pool, nil
		}
		select {
		case <-deadline.Done():
			pool.Close()
			return nil, fmt.Errorf("database not reachable: %w", err)
		case <-time.After(time.Second):
		}
	}
}

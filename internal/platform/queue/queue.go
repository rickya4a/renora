// Package queue wires River, the Postgres-backed job queue. Jobs are inserted
// in the same transaction as the data they act on, so a committed change
// always has its follow-up job (timers, notifications, AI calls).
package queue

import (
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// Schema is the Postgres schema that holds River's tables.
const Schema = "river"

type Client = river.Client[pgx.Tx]

// NewInserter returns a client that can only insert jobs. The API uses it.
func NewInserter(pool *pgxpool.Pool, logger *slog.Logger) (*Client, error) {
	return river.NewClient(riverpgxv5.New(pool), &river.Config{
		Schema: Schema,
		Logger: logger,
	})
}

// NewWorker returns a client that works jobs from the default queue.
func NewWorker(pool *pgxpool.Pool, logger *slog.Logger, workers *river.Workers, periodic []*river.PeriodicJob, concurrency int) (*Client, error) {
	return river.NewClient(riverpgxv5.New(pool), &river.Config{
		Schema:       Schema,
		Logger:       logger,
		Workers:      workers,
		PeriodicJobs: periodic,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: concurrency},
		},
	})
}

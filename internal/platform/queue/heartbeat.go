package queue

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/riverqueue/river"
)

// HeartbeatArgs is a periodic no-op job. If it stops completing, the queue
// is stuck (no live worker, or the database is refusing work); alert on
// time() - max(queue_heartbeat_last_success_timestamp_seconds) > 300.
type HeartbeatArgs struct{}

func (HeartbeatArgs) Kind() string { return "queue.heartbeat" }

type HeartbeatWorker struct {
	river.WorkerDefaults[HeartbeatArgs]
	lastSuccess prometheus.Gauge
}

func NewHeartbeatWorker(reg prometheus.Registerer) *HeartbeatWorker {
	g := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "queue_heartbeat_last_success_timestamp_seconds",
		Help: "Unix time of the last completed queue heartbeat job.",
	})
	reg.MustRegister(g)
	return &HeartbeatWorker{lastSuccess: g}
}

func (w *HeartbeatWorker) Work(context.Context, *river.Job[HeartbeatArgs]) error {
	w.lastSuccess.SetToCurrentTime()
	return nil
}

// HeartbeatPeriodicJob schedules the heartbeat every minute. River elects one
// leader to enqueue periodic jobs, so replicas don't multiply it.
func HeartbeatPeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(time.Minute),
		func() (river.JobArgs, *river.InsertOpts) {
			return HeartbeatArgs{}, &river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: time.Minute}}
		},
		&river.PeriodicJobOpts{RunOnStart: true},
	)
}

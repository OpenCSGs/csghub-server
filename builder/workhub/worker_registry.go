package workhub

import (
	"context"
	"fmt"
	"time"

	"github.com/riverqueue/river"
)

// WorkerOverrides supplies the real workers owned by the current work client.
// Kinds without an override receive a maintenance worker so a schema-wide River
// leader can safely evaluate every registered job kind without consuming its queue.
type WorkerOverrides struct {
	// MirrorRepo is the real repository mirror worker when this client owns that queue.
	MirrorRepo river.Worker[RepoArgs]
	// MirrorLFS is the real Git LFS mirror worker when this client owns that queue.
	MirrorLFS river.Worker[LFSArgs]
}

// NewWorkerRegistry creates the complete worker registry required by River's
// schema-wide maintenance services. Queue configuration still determines which
// of these workers the current client can execute during normal job processing.
func NewWorkerRegistry(overrides WorkerOverrides) *river.Workers {
	workers := river.NewWorkers()
	river.AddWorker(workers, workerForRegistry(overrides.MirrorRepo, MirrorRepoQueue, MirrorRepoJobTimeout))
	river.AddWorker(workers, workerForRegistry(overrides.MirrorLFS, MirrorLFSQueue, MirrorLFSJobTimeout))
	return workers
}

// workerForRegistry returns a real worker override or a maintenance-only
// fallback with the same scheduling policy.
func workerForRegistry[T river.JobArgs](override river.Worker[T], kind string, timeout time.Duration) river.Worker[T] {
	if override != nil {
		return override
	}
	return &maintenanceWorker[T]{
		kind:    kind,
		timeout: timeout,
	}
}

// maintenanceWorker exposes job scheduling policy to River maintenance
// services for a kind that the current client does not consume.
type maintenanceWorker[T river.JobArgs] struct {
	river.WorkerDefaults[T]
	kind    string
	timeout time.Duration
}

// Work rejects accidental execution because maintenance workers must only be
// inspected by River maintenance services.
func (w *maintenanceWorker[T]) Work(context.Context, *river.Job[T]) error {
	return fmt.Errorf("maintenance worker for job kind %q must not execute", w.kind)
}

// Timeout returns the same timeout used by the corresponding real worker.
func (w *maintenanceWorker[T]) Timeout(*river.Job[T]) time.Duration {
	return w.timeout
}

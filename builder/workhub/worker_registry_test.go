package workhub

import (
	"context"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"
)

// registryTestWorker is a real worker substitute used to verify registry overrides.
type registryTestWorker[T river.JobArgs] struct {
	river.WorkerDefaults[T]
}

// Work completes successfully because registry tests only verify worker selection.
func (w *registryTestWorker[T]) Work(context.Context, *river.Job[T]) error {
	return nil
}

// TestWorkerForRegistryUsesOverride verifies a real worker is not replaced by a maintenance worker.
func TestWorkerForRegistryUsesOverride(t *testing.T) {
	override := &registryTestWorker[RepoArgs]{}

	worker := workerForRegistry[RepoArgs](override, MirrorRepoQueue, MirrorRepoJobTimeout)

	require.Same(t, override, worker)
}

// TestWorkerForRegistryUsesKindTimeout verifies each fallback retains its job-specific timeout.
func TestWorkerForRegistryUsesKindTimeout(t *testing.T) {
	repoWorker := workerForRegistry[RepoArgs](nil, MirrorRepoQueue, MirrorRepoJobTimeout)
	lfsWorker := workerForRegistry[LFSArgs](nil, MirrorLFSQueue, MirrorLFSJobTimeout)

	repoMaintenance, ok := repoWorker.(*maintenanceWorker[RepoArgs])
	require.True(t, ok)
	lfsMaintenance, ok := lfsWorker.(*maintenanceWorker[LFSArgs])
	require.True(t, ok)
	require.Equal(t, MirrorRepoJobTimeout, repoMaintenance.Timeout(&river.Job[RepoArgs]{}))
	require.Equal(t, MirrorLFSJobTimeout, lfsMaintenance.Timeout(&river.Job[LFSArgs]{}))
}

// TestWorkClientRescueStuckJobsAfter verifies work clients use the shared rescue threshold.
func TestWorkClientRescueStuckJobsAfter(t *testing.T) {
	require.Equal(t, MirrorLFSJobTimeout+5*time.Minute, workClientRescueStuckJobsAfter)
}

// TestMaintenanceWorkerRejectsExecution verifies a fallback cannot silently complete business work.
func TestMaintenanceWorkerRejectsExecution(t *testing.T) {
	worker := &maintenanceWorker[RepoArgs]{
		kind:    MirrorRepoQueue,
		timeout: MirrorRepoJobTimeout,
	}

	err := worker.Work(context.Background(), &river.Job[RepoArgs]{})

	require.ErrorContains(t, err, "maintenance worker")
	require.ErrorContains(t, err, MirrorRepoQueue)
}

// TestNewWorkerRegistryBuildsAllKnownKinds verifies all known job kinds can be registered together.
func TestNewWorkerRegistryBuildsAllKnownKinds(t *testing.T) {
	require.NotNil(t, NewWorkerRegistry(WorkerOverrides{}))
	require.NotNil(t, NewWorkerRegistry(WorkerOverrides{
		MirrorRepo: &registryTestWorker[RepoArgs]{},
		MirrorLFS:  &registryTestWorker[LFSArgs]{},
	}))
}

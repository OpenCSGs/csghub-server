package component

import (
	"context"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/types"
)

// fakeLFSTaskStore records LFS worker task state transitions.
type fakeLFSTaskStore struct {
	task    *database.MirrorTask
	actions []string
}

// FindByID returns the configured task.
func (s *fakeLFSTaskStore) FindByID(ctx context.Context, id int64) (*database.MirrorTask, error) {
	return s.task, nil
}

// UpdateStatusAndRepoSyncStatus records an LFS task status transition.
func (s *fakeLFSTaskStore) UpdateStatusAndRepoSyncStatus(ctx context.Context, task database.MirrorTask, action string) (database.MirrorTask, error) {
	s.actions = append(s.actions, action)
	switch action {
	case database.MirrorContinue:
		if task.Status == types.MirrorQueued {
			task.Status = types.MirrorRepoSyncStart
		} else {
			task.Status = types.MirrorLfsSyncStart
		}
	case database.MirrorSuccess:
		task.Status = types.MirrorLfsSyncFinished
	case database.MirrorTooLarge:
		task.Status = types.MirrorRepoTooLarge
	case database.MirrorFail:
		task.Status = types.MirrorLfsSyncFailed
	case database.MirrorCancel:
		task.Status = types.MirrorCanceled
	case database.MirrorRetry:
		task.Status = types.MirrorRepoSyncFinished
	}
	s.task = &task
	return task, nil
}

// fakeLFSSyncer records whether LFS sync was executed.
type fakeLFSSyncer struct {
	called bool
	err    error
}

// SyncLFS runs the fake LFS sync result.
func (s *fakeLFSSyncer) SyncLFS(ctx context.Context, task *database.MirrorTask) error {
	s.called = true
	return s.err
}

// fakeLFSRepoFilter returns the configured sync decision.
type fakeLFSRepoFilter struct {
	shouldSync bool
}

// ShouldSync returns whether the repository should sync LFS objects.
func (f fakeLFSRepoFilter) ShouldSync(ctx context.Context, repoID int64) (bool, string, error) {
	if f.shouldSync {
		return true, "", nil
	}
	return false, "too large", nil
}

func TestLFSWorker_WorkCompletesLFSTask(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	store := &fakeLFSTaskStore{task: task}
	syncer := &fakeLFSSyncer{}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		syncer:          syncer,
		repoFilter:      fakeLFSRepoFilter{shouldSync: true},
	}

	err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))
	require.NoError(t, err)
	require.True(t, syncer.called)
	require.Equal(t, []string{database.MirrorContinue, database.MirrorSuccess}, store.actions)
}

func TestLFSWorker_WorkMarksTooLargeWithoutSync(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	store := &fakeLFSTaskStore{task: task}
	syncer := &fakeLFSSyncer{}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		syncer:          syncer,
		repoFilter:      fakeLFSRepoFilter{shouldSync: false},
	}

	err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))
	require.NoError(t, err)
	require.False(t, syncer.called)
	require.Equal(t, []string{database.MirrorContinue, database.MirrorTooLarge}, store.actions)
}

func TestLFSWorker_WorkReturnsCanceledWithoutFailingTask(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	store := &fakeLFSTaskStore{task: task}
	syncer := &fakeLFSSyncer{err: context.Canceled}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		syncer:          syncer,
		repoFilter:      fakeLFSRepoFilter{shouldSync: true},
	}

	err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))
	require.ErrorIs(t, err, context.Canceled)
	require.True(t, syncer.called)
	require.Equal(t, []string{database.MirrorContinue}, store.actions)
}

// TestLFSWorker_WorkSkipsCanceledTaskWithoutStateWrite verifies canceled tasks are terminal for LFS jobs.
func TestLFSWorker_WorkSkipsCanceledTaskWithoutStateWrite(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorCanceled)
	store := &fakeLFSTaskStore{task: task}
	syncer := &fakeLFSSyncer{}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		syncer:          syncer,
		repoFilter:      fakeLFSRepoFilter{shouldSync: true},
	}

	err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))

	require.NoError(t, err)
	require.False(t, syncer.called)
	require.Empty(t, store.actions)
}

// TestLFSWorker_WorkSkipsStaleCurrentTaskWithoutStateWrite verifies old LFS jobs cannot update replaced tasks.
func TestLFSWorker_WorkSkipsStaleCurrentTaskWithoutStateWrite(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	task.Mirror.CurrentTaskID = task.ID + 1
	store := &fakeLFSTaskStore{task: task}
	syncer := &fakeLFSSyncer{}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		syncer:          syncer,
		repoFilter:      fakeLFSRepoFilter{shouldSync: true},
	}

	err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))

	require.NoError(t, err)
	require.False(t, syncer.called)
	require.Empty(t, store.actions)
}

func TestLFSWorker_WorkRetriesFailedLFSTask(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorLfsSyncFailed)
	store := &fakeLFSTaskStore{task: task}
	syncer := &fakeLFSSyncer{}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		syncer:          syncer,
		repoFilter:      fakeLFSRepoFilter{shouldSync: true},
	}

	err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))
	require.NoError(t, err)
	require.True(t, syncer.called)
	require.Equal(t, []string{database.MirrorRetry, database.MirrorContinue, database.MirrorSuccess}, store.actions)
}

func TestLFSWorker_WorkSnoozesWhenContextDeadlineStopsSync(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	store := &fakeLFSTaskStore{task: task}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		syncer:          &fakeLFSSyncer{err: context.DeadlineExceeded},
		repoFilter:      fakeLFSRepoFilter{shouldSync: true},
	}

	err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Equal(t, time.Duration(0), snoozeErr.Duration)
}

func TestNewLFSWorkClientRejectsMissingDependencies(t *testing.T) {
	ctx := context.TODO()

	_, err := NewLFSWorkClient(ctx, "", LFSWorkDeps{})
	require.ErrorContains(t, err, "mirror task store is required")

	_, err = NewLFSWorkClient(ctx, "", LFSWorkDeps{
		MirrorTaskStore: &fakeLFSTaskStore{},
	})
	require.ErrorContains(t, err, "LFS syncer is required")

	_, err = NewLFSWorkClient(ctx, "", LFSWorkDeps{
		MirrorTaskStore: &fakeLFSTaskStore{},
		Syncer:          &fakeLFSSyncer{},
	})
	require.ErrorContains(t, err, "LFS repo filter is required")
}

// TestNewLFSRiverConfigUsesConfiguredMaxWorkers verifies LFS clients only consume the LFS queue.
func TestNewLFSRiverConfigUsesConfiguredMaxWorkers(t *testing.T) {
	config := newLFSRiverConfig(LFSWorkDeps{
		MirrorTaskStore: &fakeLFSTaskStore{},
		Syncer:          &fakeLFSSyncer{},
		RepoFilter:      fakeLFSRepoFilter{shouldSync: true},
		MaxWorkers:      3,
	})

	require.Equal(t, 3, config.Queues[workhub.MirrorLFSQueue].MaxWorkers)
	require.Len(t, config.Queues, 1)
	_, consumesRepo := config.Queues[workhub.MirrorRepoQueue]
	require.False(t, consumesRepo)

	config = newLFSRiverConfig(LFSWorkDeps{
		MirrorTaskStore: &fakeLFSTaskStore{},
		Syncer:          &fakeLFSSyncer{},
		RepoFilter:      fakeLFSRepoFilter{shouldSync: true},
	})

	require.Equal(t, 1, config.Queues[workhub.MirrorLFSQueue].MaxWorkers)
}

// TestLFSWorkerTimeout verifies the real LFS worker uses the shared workhub timeout contract.
func TestLFSWorkerTimeout(t *testing.T) {
	require.Equal(t, workhub.MirrorLFSJobTimeout, (&lfsWorker{}).Timeout(&river.Job[workhub.LFSArgs]{}))
}

func TestShouldSkipLFSJobAllowsCurrentTask(t *testing.T) {
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	args := lfsArgsFromTask(task)

	skip, reason := shouldSkipLFSJob(task, args)

	require.False(t, skip)
	require.Empty(t, reason)
}

func TestShouldSkipLFSJobReportsReason(t *testing.T) {
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	args := lfsArgsFromTask(task)
	args.RepositoryID = 99

	skip, reason := shouldSkipLFSJob(task, args)

	require.True(t, skip)
	require.Equal(t, "repository_id_mismatch", reason)
}

func lfsArgsFromTask(task *database.MirrorTask) workhub.LFSArgs {
	return workhub.LFSArgs{
		MirrorID:     task.MirrorID,
		RepositoryID: task.Mirror.RepositoryID,
		MirrorTaskID: task.ID,
	}
}

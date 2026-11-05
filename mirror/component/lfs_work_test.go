package component

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// fakeLFSTaskStore records LFS worker task state transitions.
type fakeLFSTaskStore struct {
	task          *database.MirrorTask
	actions       []string
	failUpdateErr error
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
	case database.MirrorFail:
		if s.failUpdateErr != nil {
			return task, s.failUpdateErr
		}
		task.Status = types.MirrorLfsSyncFailed
	case database.MirrorCancel:
		task.Status = types.MirrorCanceled
	case database.MirrorRetry:
		task.Status = types.MirrorRepoSyncFinished
	case database.MirrorPause:
		task.Status = types.MirrorRepoSyncFinished
	}
	s.task = &task
	return task, nil
}

// fakeLFSSyncer records whether LFS sync was executed.
type fakeLFSSyncer struct {
	called                 bool
	err                    error
	started                chan context.Context
	returnCauseAfterCancel bool
	beforeReturn           func()
}

// SyncLFS runs the fake LFS sync result.
func (s *fakeLFSSyncer) SyncLFS(ctx context.Context, task *database.MirrorTask) error {
	s.called = true
	if s.started != nil {
		s.started <- ctx
		<-ctx.Done()
		if s.returnCauseAfterCancel {
			return context.Cause(ctx)
		}
		return ctx.Err()
	}
	if s.beforeReturn != nil {
		s.beforeReturn()
	}
	return s.err
}

// TestLFSWorker_WorkPreservesSyncErrorWhenFailureUpdateFails prevents persistence errors from hiding real sync failures.
func TestLFSWorker_WorkPreservesSyncErrorWhenFailureUpdateFails(t *testing.T) {
	for _, paused := range []bool{false, true} {
		name := "database failure"
		if paused {
			name = "queue pause during sync failure"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			syncErr := errors.New("LFS download failed")
			updateErr := errors.New("database unavailable")
			var beforeReturn func()
			if paused {
				updateErr = context.Canceled
				beforeReturn = func() { cancel(river.ErrQueuePaused) }
			}
			task := repoWorkerTask(types.MirrorRepoSyncFinished)
			store := &fakeLFSTaskStore{task: task, failUpdateErr: updateErr}
			worker := &lfsWorker{
				mirrorTaskStore: store,
				syncer:          &fakeLFSSyncer{err: syncErr, beforeReturn: beforeReturn},
			}

			err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))

			require.ErrorIs(t, err, syncErr)
			require.ErrorIs(t, err, updateErr)
			require.ErrorContains(t, err, "mark LFS sync failed")
			require.False(t, isQueuePauseCancellation(ctx, err))
			require.Equal(t, []string{database.MirrorContinue, database.MirrorFail}, store.actions)
			require.Equal(t, types.MirrorLfsSyncStart, store.task.Status)
		})
	}
}

// TestLFSWorker_GRPCPause distinguishes pure RPC cancellation from joined sync failures through the full Work path.
func TestLFSWorker_GRPCPause(t *testing.T) {
	rpcCanceled := errorx.ErrGitGetLfsPointersFailed(status.Error(codes.Canceled, "scan canceled"), errorx.Ctx())
	checkpointErr := errors.New("checkpoint failed")
	for _, tc := range []struct {
		name      string
		syncErr   error
		wantPause bool
	}{
		{name: "pure RPC cancellation", syncErr: rpcCanceled, wantPause: true},
		{name: "joined business failure", syncErr: errors.Join(rpcCanceled, checkpointErr)},
		{name: "RPC unavailable", syncErr: status.Error(codes.Unavailable, "Gitaly unavailable")},
		{name: "RPC timeout", syncErr: status.Error(codes.DeadlineExceeded, "scan timed out")},
		{name: "RPC permission denied", syncErr: status.Error(codes.PermissionDenied, "access denied")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			task := repoWorkerTask(types.MirrorRepoSyncFinished)
			store := &fakeLFSTaskStore{task: task, failUpdateErr: context.Canceled}
			worker := &lfsWorker{mirrorTaskStore: store, syncer: &fakeLFSSyncer{
				err: tc.syncErr, beforeReturn: func() { cancel(river.ErrQueuePaused) },
			}}

			err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))

			if tc.wantPause {
				require.ErrorIs(t, err, river.ErrQueuePaused)
				require.Equal(t, []string{database.MirrorContinue, database.MirrorPause}, store.actions)
				require.Equal(t, types.MirrorRepoSyncFinished, store.task.Status)
			} else {
				require.ErrorIs(t, err, tc.syncErr)
				require.ErrorIs(t, err, context.Canceled)
				require.False(t, isQueuePauseCancellation(ctx, err))
				require.Equal(t, []string{database.MirrorContinue, database.MirrorFail}, store.actions)
			}
		})
	}
}

func TestLFSWorker_WorkCompletesLFSTask(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	store := &fakeLFSTaskStore{task: task}
	syncer := &fakeLFSSyncer{}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		urgentManager:   newWorkerTestManagerForQueue(t, workhub.MirrorLFSQueue),
		syncer:          syncer,
	}

	err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))
	require.NoError(t, err)
	require.True(t, syncer.called)
	require.Equal(t, []string{database.MirrorContinue, database.MirrorSuccess}, store.actions)
}

// TestLFSWorker_WorkMarksUnexpectedCancellationFailed verifies a live work context does not hide a sync error.
func TestLFSWorker_WorkMarksUnexpectedCancellationFailed(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	store := &fakeLFSTaskStore{task: task}
	syncer := &fakeLFSSyncer{err: context.Canceled}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		urgentManager:   newWorkerTestManagerForQueue(t, workhub.MirrorLFSQueue),
		syncer:          syncer,
	}

	err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))
	require.ErrorIs(t, err, context.Canceled)
	require.True(t, syncer.called)
	require.Equal(t, []string{database.MirrorContinue, database.MirrorFail}, store.actions)
	require.Equal(t, types.MirrorLfsSyncFailed, store.task.Status)
}

// TestLFSWorker_WorkSkipsCanceledTask verifies canceled tasks complete stale LFS jobs without sync work.
func TestLFSWorker_WorkSkipsCanceledTask(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorCanceled)
	store := &fakeLFSTaskStore{task: task}
	syncer := &fakeLFSSyncer{}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		urgentManager:   newWorkerTestManagerForQueue(t, workhub.MirrorLFSQueue),
		syncer:          syncer,
	}

	err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))

	require.NoError(t, err)
	require.False(t, syncer.called)
	require.Empty(t, store.actions)
}

// TestLFSWorker_workReturnsSkippedTask verifies skipped LFS tasks are logged and returned unchanged.
func TestLFSWorker_workReturnsSkippedTask(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	task := repoWorkerTask(types.MirrorCanceled)
	store := &fakeLFSTaskStore{task: task}
	syncer := &fakeLFSSyncer{}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		syncer:          syncer,
	}

	returnedTask, err := worker.work(context.Background(), lfsArgsFromTask(task), 0, task.LFSJobID)

	require.NoError(t, err)
	require.Same(t, task, returnedTask)
	require.False(t, syncer.called)
	require.Empty(t, store.actions)
	require.Contains(t, output.String(), `"msg":"skip mirror LFS task"`)
	require.Contains(t, output.String(), `"task_status":"cancelled"`)
}

// TestLFSWorker_workLogsUnexpectedStatus verifies invalid phase states return an observable retryable error.
func TestLFSWorker_workLogsUnexpectedStatus(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeLFSTaskStore{task: task}
	syncer := &fakeLFSSyncer{}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		syncer:          syncer,
	}

	returnedTask, err := worker.work(context.Background(), lfsArgsFromTask(task), 0, task.LFSJobID)

	require.ErrorContains(t, err, "status queued is not executable")
	require.Same(t, task, returnedTask)
	require.False(t, syncer.called)
	require.Empty(t, store.actions)
	require.Contains(t, output.String(), `"msg":"invalid mirror LFS task information"`)
	require.Contains(t, output.String(), `"task_status":"queued"`)
}

// TestLFSWorker_WorkRejectsStaleCurrentTask verifies old LFS jobs cannot update replaced tasks.
func TestLFSWorker_WorkRejectsStaleCurrentTask(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	task.Mirror.CurrentTaskID = task.ID + 1
	store := &fakeLFSTaskStore{task: task}
	syncer := &fakeLFSSyncer{}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		urgentManager:   newWorkerTestManagerForQueue(t, workhub.MirrorLFSQueue),
		syncer:          syncer,
	}

	err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))

	require.ErrorContains(t, err, "mirror current task mismatch")
	require.False(t, syncer.called)
	require.Empty(t, store.actions)
}

func TestLFSWorker_WorkRetriesFailedLFSTask(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorLfsSyncFailed)
	task.RetryCount = 3
	store := &fakeLFSTaskStore{task: task}
	syncer := &fakeLFSSyncer{}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		urgentManager:   newWorkerTestManagerForQueue(t, workhub.MirrorLFSQueue),
		syncer:          syncer,
	}
	job := riverJob(lfsArgsFromTask(task))
	job.Attempt = 2

	err := worker.Work(ctx, job)
	require.NoError(t, err)
	require.True(t, syncer.called)
	require.Equal(t, []string{database.MirrorRetry, database.MirrorContinue, database.MirrorSuccess}, store.actions)
	require.Equal(t, job.Attempt, store.task.RetryCount)
}

// TestLFSWorkerFirstAttemptUsesRiverAttempt verifies retry count follows River's one-based attempt value.
func TestLFSWorkerFirstAttemptUsesRiverAttempt(t *testing.T) {
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	task.RetryCount = 3
	store := &fakeLFSTaskStore{task: task}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		urgentManager:   newWorkerTestManagerForQueue(t, workhub.MirrorLFSQueue),
		syncer:          &fakeLFSSyncer{},
	}
	job := riverJob(lfsArgsFromTask(task))
	job.Attempt = 1

	err := worker.Work(context.Background(), job)

	require.NoError(t, err)
	require.Equal(t, job.Attempt, store.task.RetryCount)
}

// TestLFSWorker_WorkReturnsContextDeadline verifies timeouts remain ordinary retryable errors.
func TestLFSWorker_WorkReturnsContextDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	store := &fakeLFSTaskStore{task: task}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		urgentManager:   newWorkerTestManagerForQueue(t, workhub.MirrorLFSQueue),
		syncer:          &fakeLFSSyncer{err: context.DeadlineExceeded},
	}

	err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, []string{database.MirrorContinue}, store.actions)
	require.Equal(t, types.MirrorLfsSyncStart, store.task.Status)
}

func TestNewLFSWorkClientRejectsMissingDependencies(t *testing.T) {
	ctx := context.TODO()

	_, err := NewLFSWorkClient(ctx, "", LFSWorkDeps{})
	require.ErrorContains(t, err, "mirror task store is required")

	_, err = NewLFSWorkClient(ctx, "", LFSWorkDeps{
		MirrorTaskStore: &fakeLFSTaskStore{},
	})
	require.ErrorContains(t, err, "LFS syncer is required")

}

// TestNewLFSRiverConfigUsesConfiguredMaxWorkers verifies LFS clients consume normal and urgent LFS queues.
func TestNewLFSRiverConfigUsesConfiguredMaxWorkers(t *testing.T) {
	config := newLFSRiverConfig(LFSWorkDeps{
		MirrorTaskStore: &fakeLFSTaskStore{},
		Syncer:          &fakeLFSSyncer{},
		MaxWorkers:      3,
	})

	require.Equal(t, 3, config.Queues[workhub.MirrorLFSQueue].MaxWorkers)
	require.Equal(t, 1, config.Queues[workhub.MirrorLFSUrgentQueue].MaxWorkers)
	require.IsType(t, &mirrorJobErrorHandler{}, config.ErrorHandler)
	require.NotNil(t, config.JobLease)
	require.Len(t, config.Queues, 2)
	_, consumesRepo := config.Queues[workhub.MirrorRepoQueue]
	require.False(t, consumesRepo)

	config = newLFSRiverConfig(LFSWorkDeps{
		MirrorTaskStore: &fakeLFSTaskStore{},
		Syncer:          &fakeLFSSyncer{},
	})

	require.Equal(t, 1, config.Queues[workhub.MirrorLFSQueue].MaxWorkers)
	require.Equal(t, 1, config.Queues[workhub.MirrorLFSUrgentQueue].MaxWorkers)
}

// TestLFSWorker_StaleUrgentJobPreemptsBeforeTaskCheck verifies urgent admission precedes stale task detection.
func TestLFSWorker_StaleUrgentJobPreemptsBeforeTaskCheck(t *testing.T) {
	task := repoWorkerTask(types.MirrorCanceled)
	controller := &recordingWorkerQueueController{}
	manager := workhub.NewUrgentManager(workhub.UrgentManagerConfig{
		QueueController: controller,
		NormalQueue:     workhub.MirrorLFSQueue,
		NormalQueueConfig: river.QueueConfig{
			MaxWorkers: 1,
		},
		UrgentIdleDelay: time.Hour,
	})
	defer manager.Close(workhub.ErrWorkerShutdown)
	store := &fakeLFSTaskStore{task: task}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		urgentManager:   manager,
		syncer:          &fakeLFSSyncer{},
	}
	args := lfsArgsFromTask(task)
	args.Urgent = true
	job := riverJob(args)
	job.Queue = workhub.MirrorLFSUrgentQueue

	require.NoError(t, worker.Work(context.Background(), job))
	require.Equal(t, 1, controller.removeCalls)
}

// TestLFSWorker_NormalJobReturnsPauseCancellation verifies LFS interruption returns River's pause marker.
func TestLFSWorker_NormalJobReturnsPauseCancellation(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	manager := newWorkerTestManagerForQueue(t, workhub.MirrorLFSQueue)
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	started := make(chan context.Context, 1)
	store := &fakeLFSTaskStore{task: task}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		urgentManager:   manager,
		syncer:          &fakeLFSSyncer{started: started},
	}
	job := riverJob(lfsArgsFromTask(task))
	job.Queue = workhub.MirrorLFSQueue

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	workResult := make(chan error, 1)
	go func() { workResult <- worker.Work(ctx, job) }()
	workCtx := <-started

	cancel(river.ErrQueuePaused)
	<-workCtx.Done()
	err := <-workResult
	require.True(t, isQueuePauseCancellation(ctx, err))
	require.Equal(t, []string{database.MirrorContinue, database.MirrorPause}, store.actions)
	require.Equal(t, types.MirrorRepoSyncFinished, store.task.Status)
	exitLog := requireSingleWorkerExitLog(t, output.String(), "[lfs] work exit", "INFO")
	require.Contains(t, exitLog, `"success":false`)
	require.Contains(t, exitLog, `"error":"queue paused"`)
}

// TestLFSWorker_ExplicitPauseCauseDoesNotFailTask verifies explicit River pause causes bypass failure handling.
func TestLFSWorker_ExplicitPauseCauseDoesNotFailTask(t *testing.T) {
	manager := newWorkerTestManagerForQueue(t, workhub.MirrorLFSQueue)
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	store := &fakeLFSTaskStore{task: task}
	started := make(chan context.Context, 1)
	worker := &lfsWorker{
		mirrorTaskStore: store,
		urgentManager:   manager,
		syncer: &fakeLFSSyncer{
			started:                started,
			returnCauseAfterCancel: true,
		},
	}
	job := riverJob(lfsArgsFromTask(task))
	job.Queue = workhub.MirrorLFSQueue

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	workResult := make(chan error, 1)
	go func() { workResult <- worker.Work(ctx, job) }()
	workCtx := <-started

	cancel(river.ErrQueuePaused)
	<-workCtx.Done()
	err := <-workResult
	require.ErrorIs(t, err, river.ErrQueuePaused)
	require.Equal(t, []string{database.MirrorContinue, database.MirrorPause}, store.actions)
	require.Equal(t, types.MirrorRepoSyncFinished, store.task.Status)
}

// TestLFSWorkerTimeout verifies the real LFS worker allows up to five days per attempt.
func TestLFSWorkerTimeout(t *testing.T) {
	require.Equal(t, 120*time.Hour, (&lfsWorker{}).Timeout(&river.Job[workhub.LFSArgs]{}))
}

// TestPrepareLFSTaskHandlesExecutableStatuses verifies each accepted state produces a running LFS task.
func TestPrepareLFSTaskHandlesExecutableStatuses(t *testing.T) {
	tests := []struct {
		status      types.MirrorTaskStatus
		wantActions []string
	}{
		{status: types.MirrorRepoSyncFinished, wantActions: []string{database.MirrorContinue}},
		{status: types.MirrorLfsSyncFailed, wantActions: []string{database.MirrorRetry, database.MirrorContinue}},
		{status: types.MirrorLfsSyncStart},
	}

	for _, test := range tests {
		t.Run(string(test.status), func(t *testing.T) {
			task := repoWorkerTask(test.status)
			store := &fakeLFSTaskStore{task: task}
			worker := &lfsWorker{mirrorTaskStore: store}

			prepared, err := worker.prepareLFSTask(context.Background(), *task)

			require.NoError(t, err)
			require.Equal(t, types.MirrorLfsSyncStart, prepared.Status)
			require.Equal(t, test.wantActions, store.actions)
		})
	}
}

// TestPrepareLFSTaskRejectsUnexpectedStatus verifies callers cannot silently complete an invalid LFS state.
func TestPrepareLFSTaskRejectsUnexpectedStatus(t *testing.T) {
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeLFSTaskStore{task: task}
	worker := &lfsWorker{mirrorTaskStore: store}

	prepared, err := worker.prepareLFSTask(context.Background(), *task)

	require.ErrorContains(t, err, "cannot prepare mirror LFS task with status queued")
	require.Equal(t, types.MirrorQueued, prepared.Status)
	require.Empty(t, store.actions)
}

// TestCheckLFSTaskInfoAllowsExecutableStatuses verifies each LFS running or retry state remains executable.
func TestCheckLFSTaskInfoAllowsExecutableStatuses(t *testing.T) {
	for _, status := range []types.MirrorTaskStatus{
		types.MirrorRepoSyncFinished,
		types.MirrorLfsSyncStart,
		types.MirrorLfsSyncFailed,
	} {
		t.Run(string(status), func(t *testing.T) {
			task := repoWorkerTask(status)

			skip, err := checkLFSTaskInfo(task, lfsArgsFromTask(task), task.LFSJobID)

			require.NoError(t, err)
			require.False(t, skip)
		})
	}
}

// TestCheckLFSTaskInfoSkipsTerminalStatuses verifies the accepted terminal and special states complete LFS jobs.
func TestCheckLFSTaskInfoSkipsTerminalStatuses(t *testing.T) {
	for _, status := range []types.MirrorTaskStatus{
		types.MirrorCanceled,
		types.MirrorRepoTooLarge,
		types.MirrorLfsIncomplete,
		types.MirrorLfsSyncFatal,
		types.MirrorLfsSyncFinished,
	} {
		t.Run(string(status), func(t *testing.T) {
			task := repoWorkerTask(status)

			skip, err := checkLFSTaskInfo(task, lfsArgsFromTask(task), task.LFSJobID)

			require.NoError(t, err)
			require.True(t, skip)
		})
	}
}

// TestCheckLFSTaskInfoRejectsInvalidTask verifies invalid identity, ownership, and phase states remain errors.
func TestCheckLFSTaskInfoRejectsInvalidTask(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*database.MirrorTask, *workhub.LFSArgs) int64
		wantError string
	}{
		{
			name: "repository mismatch",
			configure: func(task *database.MirrorTask, args *workhub.LFSArgs) int64 {
				args.RepositoryID = 99
				return task.LFSJobID
			},
			wantError: "repository ID mismatch",
		},
		{
			name: "job mismatch",
			configure: func(task *database.MirrorTask, args *workhub.LFSArgs) int64 {
				return task.LFSJobID + 1
			},
			wantError: "LFS job ID mismatch",
		},
		{
			name: "queued status",
			configure: func(task *database.MirrorTask, args *workhub.LFSArgs) int64 {
				task.Status = types.MirrorQueued
				return task.LFSJobID
			},
			wantError: "status queued is not executable",
		},
		{
			name: "repo running status",
			configure: func(task *database.MirrorTask, args *workhub.LFSArgs) int64 {
				task.Status = types.MirrorRepoSyncStart
				return task.LFSJobID
			},
			wantError: "status running is not executable",
		},
		{
			name: "repo failed status",
			configure: func(task *database.MirrorTask, args *workhub.LFSArgs) int64 {
				task.Status = types.MirrorRepoSyncFailed
				return task.LFSJobID
			},
			wantError: "status repo_failed is not executable",
		},
		{
			name: "repo fatal status",
			configure: func(task *database.MirrorTask, args *workhub.LFSArgs) int64 {
				task.Status = types.MirrorRepoSyncFatal
				return task.LFSJobID
			},
			wantError: "status repo_fatal is not executable",
		},
		{
			name: "unknown status",
			configure: func(task *database.MirrorTask, args *workhub.LFSArgs) int64 {
				task.Status = types.MirrorTaskStatus("unknown")
				return task.LFSJobID
			},
			wantError: "status unknown is not executable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			task := repoWorkerTask(types.MirrorRepoSyncFinished)
			args := lfsArgsFromTask(task)
			jobID := test.configure(task, &args)

			skip, err := checkLFSTaskInfo(task, args, jobID)

			require.ErrorContains(t, err, test.wantError)
			require.False(t, skip)
		})
	}
}

func lfsArgsFromTask(task *database.MirrorTask) workhub.LFSArgs {
	return workhub.LFSArgs{
		MirrorArgs: workhub.MirrorArgs{
			MirrorID:     task.MirrorID,
			RepositoryID: task.Mirror.RepositoryID,
			MirrorTaskID: task.ID,
		},
	}
}

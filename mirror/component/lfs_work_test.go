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
	case database.MirrorFail:
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
	return s.err
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

func TestLFSWorker_WorkSnoozesWhenContextDeadlineStopsSync(t *testing.T) {
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

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Equal(t, time.Duration(0), snoozeErr.Duration)
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

// TestLFSWorker_ManagerClosedUsesNormalRetryDelay verifies rejected normal jobs use the shared retry delay.
func TestLFSWorker_ManagerClosedUsesNormalRetryDelay(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	manager := newWorkerTestManagerForQueue(t, workhub.MirrorLFSQueue)
	manager.Close(workhub.ErrWorkerShutdown)
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	worker := &lfsWorker{
		mirrorTaskStore: &fakeLFSTaskStore{task: task},
		urgentManager:   manager,
		syncer:          &fakeLFSSyncer{},
	}
	job := riverJob(lfsArgsFromTask(task))
	job.Queue = workhub.MirrorLFSQueue

	err := worker.Work(context.Background(), job)

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Equal(t, urgentJobDelay, snoozeErr.Duration)
	logs := output.String()
	requireWorkerJobLogPair(t, logs, "[lfs] work start", "[lfs] work exit")
	exitLog := requireSingleWorkerExitLog(t, logs, "[lfs] work exit", "INFO")
	require.Contains(t, exitLog, `"success":false`)
	require.Contains(t, exitLog, `"error":"JobSnoozeError: 1m0s"`)
}

func TestLFSWorker_NormalJobSnoozesWhenPreempted(t *testing.T) {
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

	workResult := make(chan error, 1)
	go func() { workResult <- worker.Work(context.Background(), job) }()
	workCtx := <-started

	urgentResult := make(chan error, 1)
	var urgentDone func()
	go func() {
		var err error
		urgentDone, err = manager.BeginUrgent(context.Background())
		urgentResult <- err
	}()

	<-workCtx.Done()
	err := <-workResult
	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Equal(t, time.Minute, snoozeErr.Duration)
	require.Equal(t, []string{database.MirrorContinue, database.MirrorPause}, store.actions)
	require.Equal(t, types.MirrorRepoSyncFinished, store.task.Status)
	require.NoError(t, <-urgentResult)
	urgentDone()
	exitLog := requireSingleWorkerExitLog(t, output.String(), "[lfs] work exit", "INFO")
	require.Contains(t, exitLog, `"success":false`)
	require.Contains(t, exitLog, `"error":"JobSnoozeError: 1m0s"`)
}

// TestLFSWorker_NormalJobLogsWhenUrgentWorkBlocksExecution verifies delayed normal jobs remain observable.
func TestLFSWorker_NormalJobLogsWhenUrgentWorkBlocksExecution(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	manager := newWorkerTestManagerForQueue(t, workhub.MirrorLFSQueue)
	urgentDone, err := manager.BeginUrgent(context.Background())
	require.NoError(t, err)
	defer urgentDone()

	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	worker := &lfsWorker{
		mirrorTaskStore: &fakeLFSTaskStore{task: task},
		urgentManager:   manager,
		syncer:          &fakeLFSSyncer{},
	}
	job := riverJob(lfsArgsFromTask(task))
	job.Queue = workhub.MirrorLFSQueue

	err = worker.Work(context.Background(), job)

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	exitLog := requireSingleWorkerExitLog(t, output.String(), "[lfs] work exit", "INFO")
	require.Contains(t, exitLog, `"success":false`)
	require.Contains(t, exitLog, `"error":"JobSnoozeError: 1m0s"`)
}

// TestLFSWorker_ExplicitPreemptionCauseSnoozesWithoutFailingTask verifies explicit cancellation causes bypass failure handling.
func TestLFSWorker_ExplicitPreemptionCauseSnoozesWithoutFailingTask(t *testing.T) {
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

	workResult := make(chan error, 1)
	go func() { workResult <- worker.Work(context.Background(), job) }()
	workCtx := <-started

	urgentResult := make(chan error, 1)
	var urgentDone func()
	go func() {
		var err error
		urgentDone, err = manager.BeginUrgent(context.Background())
		urgentResult <- err
	}()

	<-workCtx.Done()
	err := <-workResult
	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Equal(t, time.Minute, snoozeErr.Duration)
	require.Equal(t, []string{database.MirrorContinue, database.MirrorPause}, store.actions)
	require.Equal(t, types.MirrorRepoSyncFinished, store.task.Status)
	require.NoError(t, <-urgentResult)
	urgentDone()
}

// TestLFSWorkerTimeout verifies the real LFS worker uses the shared workhub timeout contract.
func TestLFSWorkerTimeout(t *testing.T) {
	require.Equal(t, workhub.MirrorLFSJobTimeout, (&lfsWorker{}).Timeout(&river.Job[workhub.LFSArgs]{}))
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

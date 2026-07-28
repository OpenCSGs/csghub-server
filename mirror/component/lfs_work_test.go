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
		syncer:          syncer,
	}

	err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))
	require.NoError(t, err)
	require.True(t, syncer.called)
	require.Equal(t, []string{database.MirrorContinue, database.MirrorSuccess}, store.actions)
}

func TestLFSWorker_WorkReturnsCanceledWithoutFailingTask(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	store := &fakeLFSTaskStore{task: task}
	syncer := &fakeLFSSyncer{err: context.Canceled}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		syncer:          syncer,
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
	}

	err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))

	require.NoError(t, err)
	require.False(t, syncer.called)
	require.Empty(t, store.actions)
}

// TestLFSWorker_workReturnsSkippedTask verifies the business method exposes the task used by exit logging.
func TestLFSWorker_workReturnsSkippedTask(t *testing.T) {
	task := repoWorkerTask(types.MirrorCanceled)
	store := &fakeLFSTaskStore{task: task}
	syncer := &fakeLFSSyncer{}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		syncer:          syncer,
	}

	returnedTask, err := worker.work(context.Background(), lfsArgsFromTask(task), 0)

	require.NoError(t, err)
	require.Same(t, task, returnedTask)
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
	}

	err := worker.Work(ctx, riverJob(lfsArgsFromTask(task)))

	require.NoError(t, err)
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
		syncer:          syncer,
	}
	job := riverJob(lfsArgsFromTask(task))
	job.Attempt = 2

	err := worker.Work(ctx, job)
	require.NoError(t, err)
	require.True(t, syncer.called)
	require.Equal(t, []string{database.MirrorRetry, database.MirrorContinue, database.MirrorSuccess}, store.actions)
	require.Equal(t, 1, store.task.RetryCount)
}

// TestLFSWorkerFirstAttemptResetsRepoRetryCount verifies retry count follows the active LFS River job.
func TestLFSWorkerFirstAttemptResetsRepoRetryCount(t *testing.T) {
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	task.RetryCount = 3
	store := &fakeLFSTaskStore{task: task}
	worker := &lfsWorker{mirrorTaskStore: store, syncer: &fakeLFSSyncer{}}
	job := riverJob(lfsArgsFromTask(task))
	job.Attempt = 1

	err := worker.Work(context.Background(), job)

	require.NoError(t, err)
	require.Zero(t, store.task.RetryCount)
}

func TestLFSWorker_WorkSnoozesWhenContextDeadlineStopsSync(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	store := &fakeLFSTaskStore{task: task}
	worker := &lfsWorker{
		mirrorTaskStore: store,
		syncer:          &fakeLFSSyncer{err: context.DeadlineExceeded},
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

func TestLFSWorker_StaleUrgentJobDoesNotPreemptNormalWork(t *testing.T) {
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
	worker := &lfsWorker{
		mirrorTaskStore: &fakeLFSTaskStore{task: task},
		syncer:          &fakeLFSSyncer{},
		urgentManager:   manager,
	}
	args := lfsArgsFromTask(task)
	args.Urgent = true
	job := riverJob(args)
	job.Queue = workhub.MirrorLFSUrgentQueue

	require.NoError(t, worker.Work(context.Background(), job))
	require.Zero(t, controller.removeCalls)
}

// TestLFSWorker_ManagerClosedSnoozesWithShutdownDelay verifies closed managers briefly delay normal jobs.
func TestLFSWorker_ManagerClosedSnoozesWithShutdownDelay(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	manager := newWorkerTestManagerForQueue(t, workhub.MirrorLFSQueue)
	manager.Close(workhub.ErrWorkerShutdown)
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	worker := &lfsWorker{
		mirrorTaskStore: &fakeLFSTaskStore{task: task},
		syncer:          &fakeLFSSyncer{},
		urgentManager:   manager,
	}
	job := riverJob(lfsArgsFromTask(task))
	job.Queue = workhub.MirrorLFSQueue

	err := worker.Work(context.Background(), job)

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Equal(t, 5*time.Second, snoozeErr.Duration)
	logs := output.String()
	requireWorkerJobLogPair(t, logs, "working on LFS job", "LFS job work exited")
	exitLog := requireSingleWorkerExitLog(t, logs, "LFS job work exited", "INFO")
	require.Contains(t, exitLog, `"snooze":true`)
	require.Contains(t, exitLog, `"reason":"worker_shutdown"`)
	require.Contains(t, exitLog, `"state":"CLOSED"`)
}

// TestLFSWorker_WorkRejectsQueueMismatch verifies invalid jobs are cancelled and logged.
func TestLFSWorker_WorkRejectsQueueMismatch(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	worker := &lfsWorker{
		mirrorTaskStore: &fakeLFSTaskStore{task: task},
		syncer:          &fakeLFSSyncer{},
		urgentManager:   newWorkerTestManagerForQueue(t, workhub.MirrorLFSQueue),
	}
	args := lfsArgsFromTask(task)
	args.Urgent = true
	job := riverJob(args)
	job.Queue = workhub.MirrorLFSQueue

	err := worker.Work(context.Background(), job)

	var cancelErr *river.JobCancelError
	require.ErrorAs(t, err, &cancelErr)
	require.ErrorContains(t, err, "queue mismatch")
	logs := output.String()
	requireWorkerJobLogPair(t, logs, "working on LFS job", "LFS job work exited")
	require.Contains(t, logs, `"msg":"canceling LFS job with queue mismatch"`)
	exitLog := requireSingleWorkerExitLog(t, logs, "LFS job work exited", "ERROR")
	require.Contains(t, exitLog, `"job_id":1`)
	require.Contains(t, exitLog, `"urgent":true`)
	require.Contains(t, exitLog, `"expected_queue":"mirror_lfs_urgent"`)
	require.Contains(t, exitLog, `"actual_queue":"mirror_lfs"`)
}

func TestLFSWorker_NormalJobSnoozesWhenPreempted(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	manager := newWorkerTestManagerForQueue(t, workhub.MirrorLFSQueue)
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	started := make(chan context.Context, 1)
	worker := &lfsWorker{
		mirrorTaskStore: &fakeLFSTaskStore{task: task},
		syncer:          &fakeLFSSyncer{started: started},
		urgentManager:   manager,
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
	require.NoError(t, <-urgentResult)
	urgentDone()
	exitLog := requireSingleWorkerExitLog(t, output.String(), "LFS job work exited", "INFO")
	require.Contains(t, exitLog, `"reason":"urgent_preemption"`)
	require.Contains(t, exitLog, `"state":"PREEMPTING"`)
	require.Contains(t, exitLog, `"snooze":true`)
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
		syncer:          &fakeLFSSyncer{},
		urgentManager:   manager,
	}
	job := riverJob(lfsArgsFromTask(task))
	job.Queue = workhub.MirrorLFSQueue

	err = worker.Work(context.Background(), job)

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	exitLog := requireSingleWorkerExitLog(t, output.String(), "LFS job work exited", "INFO")
	require.Contains(t, exitLog, `"reason":"urgent_work_blocks_execution"`)
	require.Contains(t, exitLog, `"state":"URGENT"`)
	require.Contains(t, exitLog, `"snooze":true`)
}

// TestLFSWorker_ExplicitPreemptionCauseSnoozesWithoutFailingTask verifies explicit cancellation causes bypass failure handling.
func TestLFSWorker_ExplicitPreemptionCauseSnoozesWithoutFailingTask(t *testing.T) {
	manager := newWorkerTestManagerForQueue(t, workhub.MirrorLFSQueue)
	task := repoWorkerTask(types.MirrorRepoSyncFinished)
	store := &fakeLFSTaskStore{task: task}
	started := make(chan context.Context, 1)
	worker := &lfsWorker{
		mirrorTaskStore: store,
		syncer: &fakeLFSSyncer{
			started:                started,
			returnCauseAfterCancel: true,
		},
		urgentManager: manager,
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
	require.Equal(t, []string{database.MirrorContinue}, store.actions)
	require.NoError(t, <-urgentResult)
	urgentDone()
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

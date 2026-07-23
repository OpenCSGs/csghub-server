package component

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/types"
)

// fakeRepoTaskStore records repo worker task state transitions.
type fakeRepoTaskStore struct {
	task        *database.MirrorTask
	actions     []string
	lfsInputs   []database.MirrorLFSJobInput
	insertedLFS bool
}

// FindByID returns the configured task.
func (s *fakeRepoTaskStore) FindByID(ctx context.Context, id int64) (*database.MirrorTask, error) {
	return s.task, nil
}

// UpdateStatusAndRepoSyncStatus records a plain task status transition.
func (s *fakeRepoTaskStore) UpdateStatusAndRepoSyncStatus(ctx context.Context, task database.MirrorTask, action string) (database.MirrorTask, error) {
	s.actions = append(s.actions, action)
	switch action {
	case database.MirrorContinue:
		task.Status = types.MirrorRepoSyncStart
	case database.MirrorFail:
		task.Status = types.MirrorRepoSyncFailed
	}
	s.task = &task
	return task, nil
}

// CompleteRepoSyncAndInsertLFSJob records the repo completion transaction.
func (s *fakeRepoTaskStore) CompleteRepoSyncAndInsertLFSJob(ctx context.Context, input database.CompleteRepoSyncInput) (database.MirrorTask, error) {
	s.actions = append(s.actions, database.MirrorSuccess)
	s.lfsInputs = append(s.lfsInputs, input.JobInput)
	s.insertedLFS = true
	input.Task.Status = types.MirrorRepoSyncFinished
	if input.Task.Mirror != nil && input.Task.Mirror.Repository != nil {
		input.Task.Mirror.Repository.DefaultBranch = input.DefaultBranch
	}
	s.task = &input.Task
	return input.Task, nil
}

// fakeRepoSyncer returns the configured mirror task after repo sync.
type fakeRepoSyncer struct {
	task                     *database.MirrorTask
	err                      error
	started                  chan context.Context
	returnSuccessAfterCancel bool
	returnCauseAfterCancel   bool
}

// SyncRepo returns the configured sync result.
func (s fakeRepoSyncer) SyncRepo(ctx context.Context, mirror *database.Mirror, mt *database.MirrorTask) (*database.MirrorTask, error) {
	if s.started != nil {
		s.started <- ctx
		<-ctx.Done()
		if s.returnSuccessAfterCancel {
			if s.task != nil {
				return s.task, nil
			}
			return mt, nil
		}
		if s.returnCauseAfterCancel {
			return s.task, context.Cause(ctx)
		}
		return s.task, ctx.Err()
	}
	if s.err != nil {
		return s.task, s.err
	}
	if s.task != nil {
		return s.task, nil
	}
	return mt, nil
}

// failRepoSyncer fails the test if a skipped repo job reaches the syncer.
type failRepoSyncer struct {
	t *testing.T
}

// SyncRepo fails because stale or terminal jobs must stop before repo sync.
func (s failRepoSyncer) SyncRepo(ctx context.Context, mirror *database.Mirror, mt *database.MirrorTask) (*database.MirrorTask, error) {
	s.t.Fatalf("SyncRepo should not be called")
	return nil, nil
}

// fakeMirrorLFSJobClient satisfies the LFS job client dependency for config tests.
type fakeMirrorLFSJobClient struct{}

// InsertMirrorLFSJobTx records no job because config tests never enqueue work.
func (fakeMirrorLFSJobClient) InsertMirrorLFSJobTx(ctx context.Context, tx *sql.Tx, input database.MirrorLFSJobInput) (int64, error) {
	return 0, nil
}

// synchronizedLogBuffer safely captures logs written by asynchronous workers.
type synchronizedLogBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

// Write appends one log record while holding the buffer lock.
func (b *synchronizedLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

// String returns the captured logs while holding the buffer lock.
func (b *synchronizedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

// captureMirrorWorkerLogs installs a synchronized JSON logger for one test.
func captureMirrorWorkerLogs(t *testing.T) *synchronizedLogBuffer {
	t.Helper()
	var output synchronizedLogBuffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(originalLogger) })
	return &output
}

// requireWorkerJobLogPair verifies each started worker job has exactly one exit log.
func requireWorkerJobLogPair(t *testing.T, logs, workingMessage, exitedMessage string) {
	t.Helper()
	working := `"msg":"` + workingMessage + `"`
	exited := `"msg":"` + exitedMessage + `"`
	require.Equal(t, 1, strings.Count(logs, working))
	require.Equal(t, 1, strings.Count(logs, exited))
	require.Less(t, strings.Index(logs, working), strings.Index(logs, exited))
}

// requireSingleWorkerExitLog returns the only matching exit record at the expected level.
func requireSingleWorkerExitLog(t *testing.T, logs, message, level string) string {
	t.Helper()
	var matches []string
	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(line, `"msg":"`+message+`"`) {
			matches = append(matches, line)
		}
	}
	require.Len(t, matches, 1)
	require.Contains(t, matches[0], `"level":"`+level+`"`)
	return matches[0]
}

func TestRepoWorker_WorkAlwaysEnqueuesLFSJobAfterRepoSync(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorQueued)
	syncedTask := *task
	syncedTask.Status = types.MirrorRepoSyncStart
	syncedTask.Progress = 100
	syncedTask.BeforeLastCommitID = "before"
	syncedTask.AfterLastCommitID = "after"
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          fakeRepoSyncer{task: &syncedTask},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))
	require.NoError(t, err)
	require.Equal(t, []string{database.MirrorContinue, database.MirrorSuccess}, store.actions)
	require.True(t, store.insertedLFS)
	require.Len(t, store.lfsInputs, 1)
	require.Equal(t, task.ID, store.lfsInputs[0].MirrorTaskID)
}

func TestRepoWorker_WorkEnqueuesLFSJobWhenRepoHasLFS(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorQueued)
	syncedTask := *task
	syncedTask.Status = types.MirrorRepoSyncStart
	syncedTask.Progress = 0
	syncedTask.BeforeLastCommitID = "before"
	syncedTask.AfterLastCommitID = "after"
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          fakeRepoSyncer{task: &syncedTask},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))
	require.NoError(t, err)
	require.Equal(t, []string{database.MirrorContinue, database.MirrorSuccess}, store.actions)
	require.True(t, store.insertedLFS)
	require.Len(t, store.lfsInputs, 1)
	require.Equal(t, task.ID, store.lfsInputs[0].MirrorTaskID)
	require.Equal(t, task.MirrorID, store.lfsInputs[0].MirrorID)
	require.Equal(t, task.Mirror.RepositoryID, store.lfsInputs[0].RepositoryID)
	require.Equal(t, task.Mirror.SourceUrl, store.lfsInputs[0].SourceURL)
}

func TestRepoWorker_WorkMarksOriginalTaskFailedWhenSyncReturnsNilTask(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          fakeRepoSyncer{err: errors.New("sync failed")},
	}
	job := riverJob(repoArgsFromTask(task))
	job.Attempt = 3

	err := worker.Work(ctx, job)
	require.ErrorContains(t, err, "sync failed")
	require.Equal(t, []string{database.MirrorContinue, database.MirrorFail}, store.actions)
	require.Equal(t, "sync failed", store.task.ErrorMessage)
	require.Equal(t, 2, store.task.RetryCount)
}

func TestRepoWorker_WorkReturnsCanceledWithoutFailingTask(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          fakeRepoSyncer{err: context.Canceled},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, []string{database.MirrorContinue}, store.actions)
}

// TestRepoWorker_WorkSkipsCanceledTaskWithoutStateWrite verifies canceled tasks are terminal for repo jobs.
func TestRepoWorker_WorkSkipsCanceledTaskWithoutStateWrite(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorCanceled)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          failRepoSyncer{t: t},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))

	require.NoError(t, err)
	require.Empty(t, store.actions)
	require.False(t, store.insertedLFS)
}

// TestRepoWorker_workReturnsSkippedTask verifies the business method exposes the task used by exit logging.
func TestRepoWorker_workReturnsSkippedTask(t *testing.T) {
	task := repoWorkerTask(types.MirrorCanceled)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          failRepoSyncer{t: t},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}

	returnedTask, err := worker.work(context.Background(), repoArgsFromTask(task), 0)

	require.NoError(t, err)
	require.Same(t, task, returnedTask)
	require.Empty(t, store.actions)
	require.False(t, store.insertedLFS)
}

// TestRepoWorker_WorkSkipsStaleCurrentTaskWithoutStateWrite verifies old repo jobs cannot update replaced tasks.
func TestRepoWorker_WorkSkipsStaleCurrentTaskWithoutStateWrite(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorQueued)
	task.Mirror.CurrentTaskID = task.ID + 1
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          failRepoSyncer{t: t},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))

	require.NoError(t, err)
	require.Empty(t, store.actions)
	require.False(t, store.insertedLFS)
}

func TestRepoWorker_WorkSnoozesWhenContextDeadlineStopsSync(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          fakeRepoSyncer{err: context.DeadlineExceeded},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Equal(t, time.Duration(0), snoozeErr.Duration)
}

func TestNewRepoWorkClientRejectsMissingDependencies(t *testing.T) {
	ctx := context.TODO()

	_, err := NewRepoWorkClient(ctx, "", RepoWorkDeps{})
	require.ErrorContains(t, err, "mirror task store is required")

	_, err = NewRepoWorkClient(ctx, "", RepoWorkDeps{
		MirrorTaskStore: &fakeRepoTaskStore{},
	})
	require.ErrorContains(t, err, "repo syncer is required")

	_, err = NewRepoWorkClient(ctx, "", RepoWorkDeps{
		MirrorTaskStore: &fakeRepoTaskStore{},
		Syncer:          fakeRepoSyncer{},
	})
	require.ErrorContains(t, err, "LFS job client is required")
}

// TestNewRepoRiverConfigUsesConfiguredMaxWorkers verifies repo clients consume normal and urgent repo queues.
func TestNewRepoRiverConfigUsesConfiguredMaxWorkers(t *testing.T) {
	config := newRepoRiverConfig(RepoWorkDeps{
		MirrorTaskStore: &fakeRepoTaskStore{},
		Syncer:          fakeRepoSyncer{},
		LFSJobClient:    fakeMirrorLFSJobClient{},
		MaxWorkers:      7,
	})

	require.Equal(t, 7, config.Queues[workhub.MirrorRepoQueue].MaxWorkers)
	require.Equal(t, 3, config.Queues[workhub.MirrorRepoUrgentQueue].MaxWorkers)
	require.IsType(t, &mirrorJobErrorHandler{}, config.ErrorHandler)
	require.Len(t, config.Queues, 2)
	_, consumesLFS := config.Queues[workhub.MirrorLFSQueue]
	require.False(t, consumesLFS)
}

func TestNewRepoRiverConfigDefaultsMaxWorkers(t *testing.T) {
	config := newRepoRiverConfig(RepoWorkDeps{
		MirrorTaskStore: &fakeRepoTaskStore{},
		Syncer:          fakeRepoSyncer{},
		LFSJobClient:    fakeMirrorLFSJobClient{},
	})

	require.Equal(t, 1, config.Queues[workhub.MirrorRepoQueue].MaxWorkers)
	require.Equal(t, 1, config.Queues[workhub.MirrorRepoUrgentQueue].MaxWorkers)
}

// TestBeginUrgentWorkSnoozesManagerShutdownWithDelay verifies shutdown avoids immediate job reacquisition.
func TestBeginUrgentWorkSnoozesManagerShutdownWithDelay(t *testing.T) {
	manager := newWorkerTestManager(t)
	manager.Close(workhub.ErrWorkerShutdown)

	done, err := beginUrgentWork(manager, context.Background())

	require.Nil(t, done)
	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Equal(t, 5*time.Second, snoozeErr.Duration)
}

func TestIsUrgentPreemptionRequiresActiveRiverContext(t *testing.T) {
	riverCtx, cancelRiver := context.WithCancel(context.Background())
	ctx, cancelWork := context.WithCancelCause(riverCtx)
	cancelWork(workhub.ErrUrgentPreempt)

	require.True(t, isUrgentPreemption(riverCtx, ctx, context.Canceled))
	require.True(t, isUrgentPreemption(riverCtx, ctx, workhub.ErrUrgentPreempt))

	cancelRiver()
	require.False(t, isUrgentPreemption(riverCtx, ctx, context.Canceled))
	require.False(t, isUrgentPreemption(context.Background(), ctx, errors.New("business error")))
}

func newWorkerTestManager(t *testing.T) *workhub.UrgentManager {
	return newWorkerTestManagerForQueue(t, workhub.MirrorRepoQueue)
}

func newWorkerTestManagerForQueue(t *testing.T, normalQueue string) *workhub.UrgentManager {
	t.Helper()
	manager := workhub.NewUrgentManager(workhub.UrgentManagerConfig{
		QueueController: &workerQueueController{},
		NormalQueue:     normalQueue,
		NormalQueueConfig: river.QueueConfig{
			MaxWorkers: 1,
		},
		UrgentIdleDelay: time.Hour,
	})
	t.Cleanup(func() { manager.Close(workhub.ErrWorkerShutdown) })
	return manager
}

type workerQueueController struct{}

func (workerQueueController) RemoveQueue(ctx context.Context, queue string) error   { return nil }
func (workerQueueController) AddQueue(queue string, config river.QueueConfig) error { return nil }

type recordingWorkerQueueController struct {
	removeCalls int
}

func (c *recordingWorkerQueueController) RemoveQueue(ctx context.Context, queue string) error {
	c.removeCalls++
	return nil
}

func (c *recordingWorkerQueueController) AddQueue(queue string, config river.QueueConfig) error {
	return nil
}

func TestRepoWorker_StaleUrgentJobDoesNotPreemptNormalWork(t *testing.T) {
	task := repoWorkerTask(types.MirrorCanceled)
	controller := &recordingWorkerQueueController{}
	manager := workhub.NewUrgentManager(workhub.UrgentManagerConfig{
		QueueController: controller,
		NormalQueue:     workhub.MirrorRepoQueue,
		NormalQueueConfig: river.QueueConfig{
			MaxWorkers: 1,
		},
		UrgentIdleDelay: time.Hour,
	})
	defer manager.Close(workhub.ErrWorkerShutdown)
	worker := &repoWorker{
		mirrorTaskStore: &fakeRepoTaskStore{task: task},
		syncer:          failRepoSyncer{t: t},
		lfsJobClient:    fakeMirrorLFSJobClient{},
		urgentManager:   manager,
	}
	args := repoArgsFromTask(task)
	args.Urgent = true
	job := riverJob(args)
	job.Queue = workhub.MirrorRepoUrgentQueue

	require.NoError(t, worker.Work(context.Background(), job))
	require.Zero(t, controller.removeCalls)
}

// TestRepoWorker_ManagerClosedSnoozesWithShutdownDelay verifies closed managers briefly delay normal jobs.
func TestRepoWorker_ManagerClosedSnoozesWithShutdownDelay(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	manager := newWorkerTestManager(t)
	manager.Close(workhub.ErrWorkerShutdown)
	task := repoWorkerTask(types.MirrorQueued)
	worker := &repoWorker{
		mirrorTaskStore: &fakeRepoTaskStore{task: task},
		syncer:          failRepoSyncer{t: t},
		lfsJobClient:    fakeMirrorLFSJobClient{},
		urgentManager:   manager,
	}
	job := riverJob(repoArgsFromTask(task))
	job.Queue = workhub.MirrorRepoQueue

	err := worker.Work(context.Background(), job)

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Equal(t, 5*time.Second, snoozeErr.Duration)
	logs := output.String()
	requireWorkerJobLogPair(t, logs, "working on repo job", "repo job work exited")
	exitLog := requireSingleWorkerExitLog(t, logs, "repo job work exited", "INFO")
	require.Contains(t, exitLog, `"snooze":true`)
	require.Contains(t, exitLog, `"reason":"worker_shutdown"`)
	require.Contains(t, exitLog, `"state":"CLOSED"`)
}

// TestRepoWorker_WorkRejectsQueueMismatch verifies invalid jobs are cancelled and logged.
func TestRepoWorker_WorkRejectsQueueMismatch(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	task := repoWorkerTask(types.MirrorQueued)
	worker := &repoWorker{
		mirrorTaskStore: &fakeRepoTaskStore{task: task},
		syncer:          failRepoSyncer{t: t},
		lfsJobClient:    fakeMirrorLFSJobClient{},
		urgentManager:   newWorkerTestManager(t),
	}
	args := repoArgsFromTask(task)
	args.Urgent = true
	job := riverJob(args)
	job.Queue = workhub.MirrorRepoQueue

	err := worker.Work(context.Background(), job)

	var cancelErr *river.JobCancelError
	require.ErrorAs(t, err, &cancelErr)
	require.ErrorContains(t, err, "queue mismatch")
	logs := output.String()
	requireWorkerJobLogPair(t, logs, "working on repo job", "repo job work exited")
	require.Contains(t, logs, `"msg":"canceling repo job with queue mismatch"`)
	exitLog := requireSingleWorkerExitLog(t, logs, "repo job work exited", "ERROR")
	require.Contains(t, exitLog, `"job_id":1`)
	require.Contains(t, exitLog, `"urgent":true`)
	require.Contains(t, exitLog, `"expected_queue":"mirror_repo_urgent"`)
	require.Contains(t, exitLog, `"actual_queue":"mirror_repo"`)
}

func TestRepoWorker_UrgentJobPropagatesUrgentToLFS(t *testing.T) {
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          fakeRepoSyncer{},
		lfsJobClient:    fakeMirrorLFSJobClient{},
		urgentManager:   newWorkerTestManager(t),
	}
	args := repoArgsFromTask(task)
	args.Urgent = true
	job := riverJob(args)
	job.Queue = workhub.MirrorRepoUrgentQueue

	require.NoError(t, worker.Work(context.Background(), job))
	require.Len(t, store.lfsInputs, 1)
	require.True(t, store.lfsInputs[0].Urgent)
}

func TestRepoWorker_PreemptionBeforeSuccessCommitSnoozesWithoutEnqueueingLFS(t *testing.T) {
	manager := newWorkerTestManager(t)
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeRepoTaskStore{task: task}
	started := make(chan context.Context, 1)
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer: fakeRepoSyncer{
			started:                  started,
			returnSuccessAfterCancel: true,
		},
		lfsJobClient:  fakeMirrorLFSJobClient{},
		urgentManager: manager,
	}
	job := riverJob(repoArgsFromTask(task))
	job.Queue = workhub.MirrorRepoQueue

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
	require.False(t, store.insertedLFS)
	require.NoError(t, <-urgentResult)
	urgentDone()
}

func TestRepoWorker_NormalJobSnoozesWhenPreempted(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	manager := newWorkerTestManager(t)
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeRepoTaskStore{task: task}
	started := make(chan context.Context, 1)
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer: fakeRepoSyncer{
			started:                started,
			returnCauseAfterCancel: true,
		},
		lfsJobClient:  fakeMirrorLFSJobClient{},
		urgentManager: manager,
	}
	job := riverJob(repoArgsFromTask(task))
	job.Queue = workhub.MirrorRepoQueue

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
	exitLog := requireSingleWorkerExitLog(t, output.String(), "repo job work exited", "INFO")
	require.Contains(t, exitLog, `"reason":"urgent_preemption"`)
	require.Contains(t, exitLog, `"state":"PREEMPTING"`)
	require.Contains(t, exitLog, `"snooze":true`)
}

// TestRepoWorker_NormalJobLogsWhenUrgentWorkBlocksExecution verifies delayed normal jobs remain observable.
func TestRepoWorker_NormalJobLogsWhenUrgentWorkBlocksExecution(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	manager := newWorkerTestManager(t)
	urgentDone, err := manager.BeginUrgent(context.Background())
	require.NoError(t, err)
	defer urgentDone()

	task := repoWorkerTask(types.MirrorQueued)
	worker := &repoWorker{
		mirrorTaskStore: &fakeRepoTaskStore{task: task},
		syncer:          failRepoSyncer{t: t},
		lfsJobClient:    fakeMirrorLFSJobClient{},
		urgentManager:   manager,
	}
	job := riverJob(repoArgsFromTask(task))
	job.Queue = workhub.MirrorRepoQueue

	err = worker.Work(context.Background(), job)

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	exitLog := requireSingleWorkerExitLog(t, output.String(), "repo job work exited", "INFO")
	require.Contains(t, exitLog, `"reason":"urgent_work_blocks_execution"`)
	require.Contains(t, exitLog, `"state":"URGENT"`)
	require.Contains(t, exitLog, `"snooze":true`)
}

// TestRepoWorkerTimeout verifies the real repo worker uses the shared workhub timeout contract.
func TestRepoWorkerTimeout(t *testing.T) {
	require.Equal(t, workhub.MirrorRepoJobTimeout, (&repoWorker{}).Timeout(&river.Job[workhub.RepoArgs]{}))
}

func TestShouldSkipRepoJobReportsReason(t *testing.T) {
	task := repoWorkerTask(types.MirrorQueued)
	args := repoArgsFromTask(task)
	args.RepositoryID = 99

	skip, reason := shouldSkipRepoJob(task, args)

	require.True(t, skip)
	require.Equal(t, "repository_id_mismatch", reason)
}

func repoWorkerTask(status types.MirrorTaskStatus) *database.MirrorTask {
	repo := &database.Repository{
		ID:             11,
		Path:           "ns/repo",
		RepositoryType: types.ModelRepo,
		DefaultBranch:  types.MainBranch,
	}
	mirror := &database.Mirror{
		ID:            7,
		RepositoryID:  repo.ID,
		Repository:    repo,
		SourceUrl:     "https://github.com/upstream/repo.git",
		CurrentTaskID: 3,
		Priority:      types.ASAPMirrorPriority,
	}
	return &database.MirrorTask{
		ID:       3,
		MirrorID: mirror.ID,
		Mirror:   mirror,
		Status:   status,
		Priority: types.ASAPMirrorPriority,
	}
}

func repoArgsFromTask(task *database.MirrorTask) workhub.RepoArgs {
	return workhub.RepoArgs{
		MirrorID:     task.MirrorID,
		RepositoryID: task.Mirror.RepositoryID,
		MirrorTaskID: task.ID,
	}
}

func riverJob[T river.JobArgs](args T) *river.Job[T] {
	return &river.Job[T]{
		JobRow: &rivertype.JobRow{
			ID:          1,
			Attempt:     1,
			Kind:        args.Kind(),
			MaxAttempts: 3,
			Priority:    1,
		},
		Args: args,
	}
}

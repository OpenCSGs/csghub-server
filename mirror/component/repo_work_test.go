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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mirror/reposyncer"
)

// fakeRepoTaskStore records repo worker task state transitions.
type fakeRepoTaskStore struct {
	task          *database.MirrorTask
	actions       []string
	lfsInputs     []database.MirrorLFSJobInput
	insertedLFS   bool
	failUpdateErr error
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
		if task.StartedAt.IsZero() {
			task.StartedAt = time.Now()
		}
	case database.MirrorRetry:
		task.Status = types.MirrorQueued
	case database.MirrorFail:
		if s.failUpdateErr != nil {
			return task, s.failUpdateErr
		}
		task.Status = types.MirrorRepoSyncFailed
	case database.MirrorPause:
		task.Status = types.MirrorQueued
	}
	s.task = &task
	return task, nil
}

// CompleteRepoSyncAndInsertLFSJob records the repo completion transaction.
func (s *fakeRepoTaskStore) CompleteRepoSyncAndInsertLFSJob(ctx context.Context, input database.CompleteRepoSyncInput) (database.MirrorTask, error) {
	s.actions = append(s.actions, database.MirrorSuccess)
	if input.SkipLFSJob {
		input.Task.Status = types.MirrorLfsSyncFinished
	} else {
		s.lfsInputs = append(s.lfsInputs, input.JobInput)
		s.insertedLFS = true
		input.Task.Status = types.MirrorRepoSyncFinished
	}
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
	noLFS                    bool
	beforeReturn             func()
}

// SyncRepo returns the configured sync result.
func (s fakeRepoSyncer) SyncRepo(ctx context.Context, mirror *database.Mirror, mt *database.MirrorTask) (reposyncer.RepoSyncResult, error) {
	if s.started != nil {
		s.started <- ctx
		<-ctx.Done()
		if s.returnSuccessAfterCancel {
			if s.task != nil {
				return reposyncer.RepoSyncResult{Task: s.task, NoLFS: s.noLFS}, nil
			}
			return reposyncer.RepoSyncResult{Task: mt, NoLFS: s.noLFS}, nil
		}
		if s.returnCauseAfterCancel {
			return reposyncer.RepoSyncResult{Task: s.task, NoLFS: s.noLFS}, context.Cause(ctx)
		}
		return reposyncer.RepoSyncResult{Task: s.task, NoLFS: s.noLFS}, ctx.Err()
	}
	if s.beforeReturn != nil {
		s.beforeReturn()
	}
	if s.err != nil {
		return reposyncer.RepoSyncResult{Task: s.task, NoLFS: s.noLFS}, s.err
	}
	if s.task != nil {
		return reposyncer.RepoSyncResult{Task: s.task, NoLFS: s.noLFS}, nil
	}
	return reposyncer.RepoSyncResult{Task: mt, NoLFS: s.noLFS}, nil
}

// failRepoSyncer fails the test if a skipped repo job reaches the syncer.
type failRepoSyncer struct {
	t *testing.T
}

// SyncRepo fails because stale or terminal jobs must stop before repo sync.
func (s failRepoSyncer) SyncRepo(ctx context.Context, mirror *database.Mirror, mt *database.MirrorTask) (reposyncer.RepoSyncResult, error) {
	s.t.Fatalf("SyncRepo should not be called")
	return reposyncer.RepoSyncResult{}, nil
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
		urgentManager:   newWorkerTestManager(t),
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
		urgentManager:   newWorkerTestManager(t),
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

// TestRepoWorker_WorkSkipsLFSJobWhenRepoHasNoLFS verifies that a repo sync which
// reports no LFS objects finishes the task straight to the terminal state
// without enqueuing an LFS job.
func TestRepoWorker_WorkSkipsLFSJobWhenRepoHasNoLFS(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorQueued)
	syncedTask := *task
	syncedTask.Status = types.MirrorRepoSyncStart
	syncedTask.BeforeLastCommitID = "before"
	syncedTask.AfterLastCommitID = "after"
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		urgentManager:   newWorkerTestManager(t),
		syncer:          fakeRepoSyncer{task: &syncedTask, noLFS: true},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))
	require.NoError(t, err)
	require.Equal(t, []string{database.MirrorContinue, database.MirrorSuccess}, store.actions)
	require.False(t, store.insertedLFS)
	require.Empty(t, store.lfsInputs)
}

func TestRepoWorker_WorkMarksOriginalTaskFailedWhenSyncReturnsNilTask(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		urgentManager:   newWorkerTestManager(t),
		syncer:          fakeRepoSyncer{err: errors.New("sync failed")},
	}
	job := riverJob(repoArgsFromTask(task))
	job.Attempt = 3

	err := worker.Work(ctx, job)
	require.ErrorContains(t, err, "sync failed")
	require.Equal(t, []string{database.MirrorContinue, database.MirrorFail}, store.actions)
	require.Equal(t, "sync failed", store.task.ErrorMessage)
	require.Equal(t, job.Attempt, store.task.RetryCount)
}

// TestRepoWorker_WorkPreservesSyncErrorWhenFailureUpdateFails prevents persistence errors from hiding real sync failures.
func TestRepoWorker_WorkPreservesSyncErrorWhenFailureUpdateFails(t *testing.T) {
	for _, paused := range []bool{false, true} {
		name := "database failure"
		if paused {
			name = "queue pause during sync failure"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			syncErr := errors.New("repo fetch failed")
			updateErr := errors.New("database unavailable")
			var beforeReturn func()
			if paused {
				updateErr = context.Canceled
				beforeReturn = func() { cancel(river.ErrQueuePaused) }
			}
			task := repoWorkerTask(types.MirrorQueued)
			store := &fakeRepoTaskStore{task: task, failUpdateErr: updateErr}
			worker := &repoWorker{
				mirrorTaskStore: store,
				syncer:          fakeRepoSyncer{err: syncErr, beforeReturn: beforeReturn},
			}

			err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))

			require.ErrorIs(t, err, syncErr)
			require.ErrorIs(t, err, updateErr)
			require.ErrorContains(t, err, "mark repo sync failed")
			require.False(t, isQueuePauseCancellation(ctx, err))
			require.Equal(t, []string{database.MirrorContinue, database.MirrorFail}, store.actions)
			require.Equal(t, types.MirrorRepoSyncStart, store.task.Status)
			require.False(t, store.insertedLFS)
		})
	}
}

// TestRepoWorker_GRPCPause distinguishes pure RPC cancellation from joined sync failures through the full Work path.
func TestRepoWorker_GRPCPause(t *testing.T) {
	rpcCanceled := errorx.ErrGitMirrorSyncFailed(status.Error(codes.Canceled, "fetch canceled"), errorx.Ctx())
	checkpointErr := errors.New("checkpoint failed")
	for _, tc := range []struct {
		name      string
		syncErr   error
		wantPause bool
	}{
		{name: "pure RPC cancellation", syncErr: rpcCanceled, wantPause: true},
		{name: "joined business failure", syncErr: errors.Join(rpcCanceled, checkpointErr)},
		{name: "RPC unavailable", syncErr: status.Error(codes.Unavailable, "Gitaly unavailable")},
		{name: "RPC timeout", syncErr: status.Error(codes.DeadlineExceeded, "fetch timed out")},
		{name: "RPC permission denied", syncErr: status.Error(codes.PermissionDenied, "access denied")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			task := repoWorkerTask(types.MirrorQueued)
			store := &fakeRepoTaskStore{task: task, failUpdateErr: context.Canceled}
			worker := &repoWorker{mirrorTaskStore: store, syncer: fakeRepoSyncer{
				err: tc.syncErr, beforeReturn: func() { cancel(river.ErrQueuePaused) },
			}}

			err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))

			if tc.wantPause {
				require.ErrorIs(t, err, river.ErrQueuePaused)
				require.Equal(t, []string{database.MirrorContinue, database.MirrorPause}, store.actions)
				require.Equal(t, types.MirrorQueued, store.task.Status)
			} else {
				require.ErrorIs(t, err, tc.syncErr)
				require.ErrorIs(t, err, context.Canceled)
				require.False(t, isQueuePauseCancellation(ctx, err))
				require.Equal(t, []string{database.MirrorContinue, database.MirrorFail}, store.actions)
			}
			require.False(t, store.insertedLFS)
		})
	}
}

// TestRepoWorker_WorkMarksUnexpectedCancellationFailed verifies a live work context does not hide a sync error.
func TestRepoWorker_WorkMarksUnexpectedCancellationFailed(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		urgentManager:   newWorkerTestManager(t),
		syncer:          fakeRepoSyncer{err: context.Canceled},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, []string{database.MirrorContinue, database.MirrorFail}, store.actions)
	require.Equal(t, types.MirrorRepoSyncFailed, store.task.Status)
}

// TestRepoWorker_WorkSkipsCanceledTask verifies canceled tasks complete stale repo jobs without sync work.
func TestRepoWorker_WorkSkipsCanceledTask(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorCanceled)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		urgentManager:   newWorkerTestManager(t),
		syncer:          failRepoSyncer{t: t},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))

	require.NoError(t, err)
	require.Empty(t, store.actions)
	require.False(t, store.insertedLFS)
}

// TestRepoWorker_workReturnsSkippedTask verifies skipped repo tasks are logged and returned unchanged.
func TestRepoWorker_workReturnsSkippedTask(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	task := repoWorkerTask(types.MirrorCanceled)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		syncer:          failRepoSyncer{t: t},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}

	returnedTask, err := worker.work(context.Background(), repoArgsFromTask(task), 0, task.RepoJobID)

	require.NoError(t, err)
	require.Same(t, task, returnedTask)
	require.Empty(t, store.actions)
	require.False(t, store.insertedLFS)
	require.Contains(t, output.String(), `"msg":"skip mirror repo task"`)
	require.Contains(t, output.String(), `"task_status":"cancelled"`)
}

// TestRepoWorker_WorkRejectsStaleCurrentTask verifies old repo jobs cannot update replaced tasks.
func TestRepoWorker_WorkRejectsStaleCurrentTask(t *testing.T) {
	ctx := context.TODO()
	task := repoWorkerTask(types.MirrorQueued)
	task.Mirror.CurrentTaskID = task.ID + 1
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		urgentManager:   newWorkerTestManager(t),
		syncer:          failRepoSyncer{t: t},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))

	require.ErrorContains(t, err, "mirror current task mismatch")
	require.Empty(t, store.actions)
	require.False(t, store.insertedLFS)
}

// TestRepoWorker_WorkReturnsContextDeadline verifies timeouts remain ordinary retryable errors.
func TestRepoWorker_WorkReturnsContextDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		urgentManager:   newWorkerTestManager(t),
		syncer:          fakeRepoSyncer{err: context.DeadlineExceeded},
	}

	err := worker.Work(ctx, riverJob(repoArgsFromTask(task)))

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, []string{database.MirrorContinue}, store.actions)
	require.Equal(t, types.MirrorRepoSyncStart, store.task.Status)
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
	require.NotNil(t, config.JobLease)
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

// TestRunWorkReturnsUrgentManagerShutdown verifies closure errors propagate without entering business execution.
func TestRunWorkReturnsUrgentManagerShutdown(t *testing.T) {
	manager := newWorkerTestManager(t)
	manager.Close(workhub.ErrWorkerShutdown)

	executed := false
	err := runWork(context.Background(), &river.Job[workhub.RepoArgs]{
		JobRow: &rivertype.JobRow{ID: 1, Attempt: 1, MaxAttempts: 1},
	}, mirrorWorkConfig[workhub.RepoArgs]{
		manager: manager,
		urgent:  true,
		work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
			executed = true
			return nil, nil
		},
	})

	require.Equal(t, workhub.ErrWorkerShutdown, err)
	require.False(t, executed)
	require.Zero(t, manager.UrgentReservations())
}

// TestIsQueuePauseCancellationRequiresRiverCause verifies only pure pause cancellation is eligible.
func TestIsQueuePauseCancellationRequiresRiverCause(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(river.ErrQueuePaused)
	require.True(t, isQueuePauseCancellation(ctx, context.Canceled))
	require.True(t, isQueuePauseCancellation(ctx, river.ErrQueuePaused))
	require.False(t, isQueuePauseCancellation(context.Background(), context.Canceled))
	require.False(t, isQueuePauseCancellation(ctx, errors.New("business error")))
	require.False(t, isQueuePauseCancellation(ctx, errors.Join(context.Canceled, errors.New("checkpoint failed"))))
	require.False(t, isQueuePauseCancellation(ctx, river.JobCancel(context.Canceled)))
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

// TestRepoWorker_StaleUrgentJobPreemptsBeforeTaskCheck verifies urgent admission precedes stale task detection.
func TestRepoWorker_StaleUrgentJobPreemptsBeforeTaskCheck(t *testing.T) {
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
		urgentManager:   manager,
		syncer:          failRepoSyncer{t: t},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}
	args := repoArgsFromTask(task)
	args.Urgent = true
	job := riverJob(args)
	job.Queue = workhub.MirrorRepoUrgentQueue

	require.NoError(t, worker.Work(context.Background(), job))
	require.Equal(t, 1, controller.removeCalls)
}

func TestRepoWorker_UrgentJobPropagatesUrgentToLFS(t *testing.T) {
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{
		mirrorTaskStore: store,
		urgentManager:   newWorkerTestManager(t),
		syncer:          fakeRepoSyncer{},
		lfsJobClient:    fakeMirrorLFSJobClient{},
	}
	args := repoArgsFromTask(task)
	args.Urgent = true
	job := riverJob(args)
	job.Queue = workhub.MirrorRepoUrgentQueue

	require.NoError(t, worker.Work(context.Background(), job))
	require.Len(t, store.lfsInputs, 1)
	require.True(t, store.lfsInputs[0].Urgent)
}

// TestRepoWorker_PauseBeforeSuccessCommitDoesNotEnqueueLFS verifies River pause cancellation preserves the business waiting state.
func TestRepoWorker_PauseBeforeSuccessCommitDoesNotEnqueueLFS(t *testing.T) {
	manager := newWorkerTestManager(t)
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeRepoTaskStore{task: task}
	started := make(chan context.Context, 1)
	worker := &repoWorker{
		mirrorTaskStore: store,
		urgentManager:   manager,
		syncer: fakeRepoSyncer{
			started:                  started,
			returnSuccessAfterCancel: true,
		},
		lfsJobClient: fakeMirrorLFSJobClient{},
	}
	job := riverJob(repoArgsFromTask(task))
	job.Queue = workhub.MirrorRepoQueue

	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	workResult := make(chan error, 1)
	go func() { workResult <- worker.Work(ctx, job) }()
	workCtx := <-started
	cancel(river.ErrQueuePaused)
	<-workCtx.Done()

	err := <-workResult
	require.True(t, isQueuePauseCancellation(ctx, err))
	require.False(t, store.insertedLFS)
	require.Equal(t, []string{database.MirrorContinue, database.MirrorPause}, store.actions)
	require.Equal(t, types.MirrorQueued, store.task.Status)
}

// TestRepoWorker_NormalJobReturnsPauseCancellation verifies River pause cancellation preserves the business waiting state.
func TestRepoWorker_NormalJobReturnsPauseCancellation(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	manager := newWorkerTestManager(t)
	task := repoWorkerTask(types.MirrorQueued)
	store := &fakeRepoTaskStore{task: task}
	started := make(chan context.Context, 1)
	worker := &repoWorker{
		mirrorTaskStore: store,
		urgentManager:   manager,
		syncer: fakeRepoSyncer{
			started:                started,
			returnCauseAfterCancel: true,
		},
		lfsJobClient: fakeMirrorLFSJobClient{},
	}
	job := riverJob(repoArgsFromTask(task))
	job.Queue = workhub.MirrorRepoQueue

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
	require.Equal(t, types.MirrorQueued, store.task.Status)
	exitLog := requireSingleWorkerExitLog(t, output.String(), "[repo] work exit", "INFO")
	require.Contains(t, exitLog, `"success":false`)
	require.Contains(t, exitLog, `"error":"queue paused"`)
}

// TestRepoWorkerTimeout verifies the real repo worker uses the shared workhub timeout contract.
func TestRepoWorkerTimeout(t *testing.T) {
	require.Equal(t, 30*time.Minute, (&repoWorker{}).Timeout(&river.Job[workhub.RepoArgs]{}))
}

// TestPrepareRepoTaskHandlesExecutableStatuses verifies each accepted state produces a running repo task.
func TestPrepareRepoTaskHandlesExecutableStatuses(t *testing.T) {
	tests := []struct {
		status      types.MirrorTaskStatus
		wantActions []string
	}{
		{status: types.MirrorQueued, wantActions: []string{database.MirrorContinue}},
		{status: types.MirrorRepoSyncFailed, wantActions: []string{database.MirrorRetry, database.MirrorContinue}},
		{status: types.MirrorRepoSyncStart},
	}

	for _, test := range tests {
		t.Run(string(test.status), func(t *testing.T) {
			task := repoWorkerTask(test.status)
			store := &fakeRepoTaskStore{task: task}
			worker := &repoWorker{mirrorTaskStore: store}

			prepared, err := worker.prepareRepoTask(context.Background(), *task)

			require.NoError(t, err)
			require.Equal(t, types.MirrorRepoSyncStart, prepared.Status)
			require.Equal(t, test.wantActions, store.actions)
		})
	}
}

// TestPrepareRepoTaskRejectsUnexpectedStatus verifies callers cannot silently complete an invalid repo state.
func TestPrepareRepoTaskRejectsUnexpectedStatus(t *testing.T) {
	task := repoWorkerTask(types.MirrorCanceled)
	store := &fakeRepoTaskStore{task: task}
	worker := &repoWorker{mirrorTaskStore: store}

	prepared, err := worker.prepareRepoTask(context.Background(), *task)

	require.ErrorContains(t, err, "cannot prepare mirror repo task with status cancelled")
	require.Equal(t, types.MirrorCanceled, prepared.Status)
	require.Empty(t, store.actions)
}

// TestCheckRepoTaskInfoAllowsExecutableStatuses verifies each repo running or retry state remains executable.
func TestCheckRepoTaskInfoAllowsExecutableStatuses(t *testing.T) {
	for _, status := range []types.MirrorTaskStatus{
		types.MirrorQueued,
		types.MirrorRepoSyncStart,
		types.MirrorRepoSyncFailed,
	} {
		t.Run(string(status), func(t *testing.T) {
			task := repoWorkerTask(status)

			skip, err := checkRepoTaskInfo(task, repoArgsFromTask(task), task.RepoJobID)

			require.NoError(t, err)
			require.False(t, skip)
		})
	}
}

// TestCheckRepoTaskInfoSkipsOtherStatuses verifies every non-repo phase completes the stale repo job.
func TestCheckRepoTaskInfoSkipsOtherStatuses(t *testing.T) {
	for _, status := range []types.MirrorTaskStatus{
		types.MirrorRepoSyncFinished,
		types.MirrorRepoSyncFatal,
		types.MirrorLfsSyncStart,
		types.MirrorLfsSyncFailed,
		types.MirrorLfsSyncFinished,
		types.MirrorLfsSyncFatal,
		types.MirrorLfsIncomplete,
		types.MirrorRepoTooLarge,
		types.MirrorCanceled,
		types.MirrorTaskStatus("unknown"),
	} {
		t.Run(string(status), func(t *testing.T) {
			task := repoWorkerTask(status)

			skip, err := checkRepoTaskInfo(task, repoArgsFromTask(task), task.RepoJobID)

			require.NoError(t, err)
			require.True(t, skip)
		})
	}
}

// TestCheckRepoTaskInfoRejectsInvalidTask verifies identity and River ownership failures remain retryable errors.
func TestCheckRepoTaskInfoRejectsInvalidTask(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*database.MirrorTask, *workhub.RepoArgs) int64
		wantError string
	}{
		{
			name: "repository mismatch",
			configure: func(task *database.MirrorTask, args *workhub.RepoArgs) int64 {
				args.RepositoryID = 99
				return task.RepoJobID
			},
			wantError: "repository ID mismatch",
		},
		{
			name: "job mismatch",
			configure: func(task *database.MirrorTask, args *workhub.RepoArgs) int64 {
				return task.RepoJobID + 1
			},
			wantError: "repo job ID mismatch",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			task := repoWorkerTask(types.MirrorQueued)
			args := repoArgsFromTask(task)
			jobID := test.configure(task, &args)

			skip, err := checkRepoTaskInfo(task, args, jobID)

			require.ErrorContains(t, err, test.wantError)
			require.False(t, skip)
		})
	}
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
		ID:        3,
		MirrorID:  mirror.ID,
		Mirror:    mirror,
		Status:    status,
		Priority:  types.ASAPMirrorPriority,
		RepoJobID: 1,
		LFSJobID:  1,
	}
}

func repoArgsFromTask(task *database.MirrorTask) workhub.RepoArgs {
	return workhub.RepoArgs{
		MirrorArgs: workhub.MirrorArgs{
			MirrorID:     task.MirrorID,
			RepositoryID: task.Mirror.RepositoryID,
			MirrorTaskID: task.ID,
		},
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

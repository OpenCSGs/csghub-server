package component

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mirror/reposyncer"
)

// preemptionTaskStore isolates business persistence while real River clients coordinate claims in PostgreSQL.
type preemptionTaskStore struct {
	mu         sync.Mutex
	tasks      map[int64]database.MirrorTask
	pauseDelay time.Duration
}

// clonePreemptionTask prevents worker-owned values from racing with persisted test snapshots.
func clonePreemptionTask(task database.MirrorTask) database.MirrorTask {
	mirror := *task.Mirror
	repo := *mirror.Repository
	mirror.Repository = &repo
	task.Mirror = &mirror
	return task
}

// FindByID returns an independent business task snapshot.
func (s *preemptionTaskStore) FindByID(ctx context.Context, id int64) (*database.MirrorTask, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	task := clonePreemptionTask(s.tasks[id])
	return &task, nil
}

// UpdateStatusAndRepoSyncStatus applies the task FSM and persists the execution fields used by worker assertions.
func (s *preemptionTaskStore) UpdateStatusAndRepoSyncStatus(ctx context.Context, task database.MirrorTask, action string) (database.MirrorTask, error) {
	if action == database.MirrorPause && s.pauseDelay > 0 {
		// Simulate bounded business-state lock contention after the syncer exits.
		timer := time.NewTimer(s.pauseDelay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return task, ctx.Err()
		case <-timer.C:
		}
	}
	if err := ctx.Err(); err != nil {
		return task, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current := clonePreemptionTask(s.tasks[task.ID])
	fsm := database.NewMirrorTaskWithFSM(&current)
	if !fsm.SubmitEvent(ctx, action) {
		return current, fmt.Errorf("invalid test task transition: %s, %s", current.Status, action)
	}
	current.Status = types.MirrorTaskStatus(fsm.Current())
	current.Progress = task.Progress
	current.RetryCount = task.RetryCount
	current.BeforeLastCommitID = task.BeforeLastCommitID
	current.AfterLastCommitID = task.AfterLastCommitID
	current.ErrorMessage = task.ErrorMessage
	// Match production error clearing for successful, resumed, and paused execution.
	switch action {
	case database.MirrorContinue, database.MirrorRetry, database.MirrorPause, database.MirrorSuccess, database.MirrorNoLfsToSync:
		current.ErrorMessage = ""
	}
	s.tasks[task.ID] = clonePreemptionTask(current)
	return current, nil
}

// CompleteRepoSyncAndInsertLFSJob completes the no-LFS fixture without introducing an unrelated stage.
func (s *preemptionTaskStore) CompleteRepoSyncAndInsertLFSJob(ctx context.Context, input database.CompleteRepoSyncInput) (database.MirrorTask, error) {
	if !input.SkipLFSJob {
		return input.Task, fmt.Errorf("integration fixture expects no LFS objects")
	}
	return s.UpdateStatusAndRepoSyncStatus(ctx, input.Task, database.MirrorNoLfsToSync)
}

// TestPreemptionTaskStoreExecutionFields prevents the fixture from hiding lost checkpoints or stale errors.
func TestPreemptionTaskStoreExecutionFields(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    types.MirrorTaskStatus
		action    string
		wantError string
	}{
		{name: "pause", status: types.MirrorRepoSyncStart, action: database.MirrorPause},
		{name: "continue", status: types.MirrorQueued, action: database.MirrorContinue},
		{name: "failure", status: types.MirrorRepoSyncStart, action: database.MirrorFail, wantError: "sync failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := repoWorkerTask(tc.status)
			original.Progress = 10
			original.ErrorMessage = "previous failure"
			store := &preemptionTaskStore{tasks: map[int64]database.MirrorTask{original.ID: clonePreemptionTask(*original)}}
			input := clonePreemptionTask(*original)
			input.Progress, input.RetryCount = 42, 2
			input.BeforeLastCommitID, input.AfterLastCommitID = "before", "saved-commit"
			input.ErrorMessage = "sync failed"

			updated, err := store.UpdateStatusAndRepoSyncStatus(t.Context(), input, tc.action)
			require.NoError(t, err)
			persisted, err := store.FindByID(t.Context(), input.ID)
			require.NoError(t, err)
			require.Equal(t, updated, *persisted)
			require.Equal(t, 42, persisted.Progress)
			require.Equal(t, 2, persisted.RetryCount)
			require.Equal(t, "before", persisted.BeforeLastCommitID)
			require.Equal(t, "saved-commit", persisted.AfterLastCommitID)
			require.Equal(t, tc.wantError, persisted.ErrorMessage)
		})
	}
}

// preemptionSyncer holds A until River cancels it, and holds M until the test permits completion.
type preemptionSyncer struct {
	mu        sync.Mutex
	attempts  map[int64]int
	starts    chan int64
	releaseM  chan struct{}
	store     *preemptionTaskStore
	exitDelay time.Duration
	canceledA chan time.Time
	cancelErr error
	// releaseCanceledA optionally holds compensation until a canceled urgent waiter has exited.
	releaseCanceledA <-chan struct{}
}

// sync records business execution order and saves a checkpoint before A's first cancellation.
func (s *preemptionSyncer) sync(ctx context.Context, task *database.MirrorTask) error {
	s.mu.Lock()
	s.attempts[task.ID]++
	firstA := task.ID == 1 && s.attempts[task.ID] == 1
	s.mu.Unlock()
	if firstA {
		s.store.mu.Lock()
		checkpoint := s.store.tasks[task.ID]
		checkpoint.Progress = 42
		checkpoint.AfterLastCommitID = "saved-commit"
		s.store.tasks[task.ID] = checkpoint
		s.store.mu.Unlock()
	}
	s.starts <- task.ID
	if firstA {
		<-ctx.Done()
		s.canceledA <- time.Now()
		if s.releaseCanceledA != nil {
			<-s.releaseCanceledA
		}
		// Compensation may continue after cancellation, independently of the business context.
		time.Sleep(s.exitDelay)
		if s.cancelErr != nil {
			return s.cancelErr
		}
		return ctx.Err()
	}
	if task.ID == 5 {
		select {
		case <-s.releaseM:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// SyncRepo exercises the real repo Work lifecycle around a controllable external operation.
func (s *preemptionSyncer) SyncRepo(ctx context.Context, _ *database.Mirror, task *database.MirrorTask) (reposyncer.RepoSyncResult, error) {
	err := s.sync(ctx, task)
	return reposyncer.RepoSyncResult{Task: task, NoLFS: true}, err
}

// SyncLFS exercises the real LFS Work lifecycle using the same synchronization gates.
func (s *preemptionSyncer) SyncLFS(ctx context.Context, task *database.MirrorTask) error {
	return s.sync(ctx, task)
}

// preemptionIdleDelay exercises real timer-based restoration without waiting for the production debounce.
// TestMirrorUrgentIdleDelay verifies the full production debounce using virtual time.
const preemptionIdleDelay = 100 * time.Millisecond

// preemptionWaitTimeout leaves headroom for slow cleanup, River settlement, and CI scheduling.
const preemptionWaitTimeout = 30 * time.Second

// preemptionScenarioSlots bounds database scenarios across parallel test groups, including worker and pool cleanup.
var preemptionScenarioSlots = make(chan struct{}, 4)

// parallelPreemptionScenario runs an isolated scenario in parallel with at most three peers.
func parallelPreemptionScenario(t *testing.T) {
	t.Helper()
	t.Parallel()
	preemptionScenarioSlots <- struct{}{}
	// Register first so this slot is released after later worker and database cleanups.
	t.Cleanup(func() { <-preemptionScenarioSlots })
}

// preemptionFixture owns one isolated River database and the application worker dependencies.
type preemptionFixture struct {
	t           *testing.T
	ctx         context.Context
	dsn         string
	stage       string
	queue       string
	urgent      string
	producer    *river.Client[pgx.Tx]
	store       *preemptionTaskStore
	syncer      *preemptionSyncer
	maxAttempts int
	managers    []*workhub.UrgentManager
	jobIDs      []int64
}

// newPreemptionFixture clones an isolated database through the shared PostgreSQL test helper.
// Worker cleanup is registered later and runs before the database's pool is closed.
func newPreemptionFixture(t *testing.T, stage string) *preemptionFixture {
	t.Helper()
	db := tests.InitTransactionTestDBWithHooks()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	pool, ok := db.GetPGXPool()
	require.True(t, ok, "preemption tests require a PostgreSQL connection pool")
	dsn := pool.Config().ConnString()

	// Keep River's schema current independently of the application's migration snapshot.
	setupCtx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	driver := riverpgxv5.New(pool)
	migrator, err := rivermigrate.New(driver, nil)
	require.NoError(t, err)
	_, err = migrator.Migrate(setupCtx, rivermigrate.DirectionUp, nil)
	require.NoError(t, err)
	producer, err := river.NewClient(driver, &river.Config{SkipUnknownJobCheck: true})
	require.NoError(t, err)
	store := &preemptionTaskStore{tasks: make(map[int64]database.MirrorTask)}
	f := &preemptionFixture{t: t, ctx: t.Context(), dsn: dsn, stage: stage, producer: producer, store: store, maxAttempts: 1,
		syncer: &preemptionSyncer{store: store, attempts: make(map[int64]int), starts: make(chan int64, 16), releaseM: make(chan struct{}), canceledA: make(chan time.Time, 1)}}
	f.queue, f.urgent = workhub.MirrorRepoQueue, workhub.MirrorRepoUrgentQueue
	if stage == "lfs" {
		f.queue, f.urgent = workhub.MirrorLFSQueue, workhub.MirrorLFSUrgentQueue
	}
	return f
}

// insert creates an associated business task before any worker can claim its River job.
func (f *preemptionFixture) insert(id int64) *rivertype.JobRow {
	f.t.Helper()
	status := types.MirrorQueued
	if f.stage == "lfs" {
		status = types.MirrorRepoSyncFinished
	}
	task := repoWorkerTask(status)
	task.ID, task.Mirror.CurrentTaskID = id, id
	args := repoArgsFromTask(task).MirrorArgs
	args.Urgent = id == 5
	queue := f.queue
	if args.Urgent {
		queue = f.urgent
	}
	var jobArgs river.JobArgs = workhub.RepoArgs{MirrorArgs: args}
	if f.stage == "lfs" {
		jobArgs = workhub.LFSArgs{MirrorArgs: args}
	}
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	opts := &river.InsertOpts{Queue: queue, Priority: 3, MaxAttempts: f.maxAttempts}
	if !args.Urgent {
		// Distinct normal-job timestamps verify claim order survives preemption.
		opts.ScheduledAt = time.Now().Add(-time.Hour + time.Duration(id)*time.Minute)
	}
	// Urgent jobs are immediately available instead of waiting for River's scheduler.
	result, err := f.producer.Insert(f.ctx, jobArgs, opts)
	require.NoError(f.t, err)
	task.RepoJobID, task.LFSJobID = result.Job.ID, result.Job.ID
	f.store.tasks[id] = clonePreemptionTask(*task)
	f.jobIDs = append(f.jobIDs, result.Job.ID)
	return result.Job
}

// startClient uses the real workhub adapter and mirror worker; a second client can consume only the normal queue.
func (f *preemptionFixture) startClient(normalOnly bool) *workhub.UrgentManager {
	f.t.Helper()
	var cfg *river.Config
	var bind func(*workhub.UrgentManager)
	if f.stage == "repo" {
		deps := RepoWorkDeps{MirrorTaskStore: f.store, Syncer: f.syncer, LFSJobClient: fakeMirrorLFSJobClient{}, MaxWorkers: 1}
		worker := newRepoWorker(deps)
		cfg = newRepoRiverConfigForWorker(deps, worker)
		bind = func(manager *workhub.UrgentManager) { worker.urgentManager = manager }
	} else {
		deps := LFSWorkDeps{MirrorTaskStore: f.store, Syncer: f.syncer, MaxWorkers: 1}
		worker := newLFSWorker(deps)
		cfg = newLFSRiverConfigForWorker(deps, worker)
		bind = func(manager *workhub.UrgentManager) { worker.urgentManager = manager }
	}
	if normalOnly {
		delete(cfg.Queues, f.urgent)
	}
	cfg.FetchCooldown, cfg.FetchPollInterval = 10*time.Millisecond, 20*time.Millisecond
	// Disable maintenance startup jitter while retaining real queue and lease behavior.
	cfg.TestOnly = true
	client, err := workhub.NewWorkClient(f.ctx, f.dsn, cfg)
	require.NoError(f.t, err)
	manager := client.ConfigureUrgentManager(workhub.UrgentManagerConfig{
		NormalQueue: f.queue, NormalQueueConfig: cfg.Queues[f.queue], UrgentIdleDelay: preemptionIdleDelay,
	})
	bind(manager)
	f.managers = append(f.managers, manager)
	f.t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), preemptionWaitTimeout)
		defer cancel()
		require.NoError(f.t, client.Stop(ctx))
	})
	require.NoError(f.t, client.Start(f.ctx))
	return manager
}

// waitStart verifies execution order using business events, with a bounded wait for slow cancellation cleanup.
func (f *preemptionFixture) waitStart(want int64) {
	f.t.Helper()
	timer := time.NewTimer(preemptionWaitTimeout)
	defer timer.Stop()
	select {
	case id := <-f.syncer.starts:
		if id != want {
			f.failWait(fmt.Sprintf("expected task %d to start, got %d", want, id))
		}
	case <-f.ctx.Done():
		f.failWait(fmt.Sprintf("context ended waiting for task %d to start", want))
	case <-timer.C:
		f.failWait(fmt.Sprintf("task %d did not start", want))
	}
}

// waitJobState waits for River's persisted state rather than inferring completion from a syncer event.
func (f *preemptionFixture) waitJobState(jobID int64, want rivertype.JobState) *rivertype.JobRow {
	f.t.Helper()
	var row *rivertype.JobRow
	require.Eventually(f.t, func() bool {
		var err error
		row, err = f.producer.JobGet(f.ctx, jobID)
		return err == nil && row.State == want
	}, preemptionWaitTimeout, 10*time.Millisecond, "job %d did not reach %s", jobID, want)
	return row
}

// requireCompleted verifies every expected success was persisted without retries or execution errors.
func (f *preemptionFixture) requireCompleted(jobIDs ...int64) {
	f.t.Helper()
	for _, jobID := range jobIDs {
		row := f.waitJobState(jobID, rivertype.JobStateCompleted)
		require.Equal(f.t, 1, row.Attempt, "job %d", jobID)
		require.Empty(f.t, row.Errors, "job %d", jobID)
	}
}

// waitCancellation waits for River's cancellation signal before inspecting or releasing compensation.
func (f *preemptionFixture) waitCancellation() time.Time {
	f.t.Helper()
	timer := time.NewTimer(preemptionWaitTimeout)
	defer timer.Stop()
	select {
	case canceledAt := <-f.syncer.canceledA:
		return canceledAt
	case <-f.ctx.Done():
		f.failWait("context ended waiting for normal work cancellation")
	case <-timer.C:
		f.failWait("River did not request normal work cancellation")
	}
	return time.Time{}
}

// failWait captures bounded diagnostics even if the worker context has already been canceled.
func (f *preemptionFixture) failWait(reason string) {
	f.t.Helper()
	for i, manager := range f.managers {
		f.t.Logf("client %d: state=%s urgent_reservations=%d", i, manager.State(), manager.UrgentReservations())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, id := range f.jobIDs {
		job, err := f.producer.JobGet(ctx, id)
		if err != nil {
			f.t.Logf("job %d: %v", id, err)
			continue
		}
		f.t.Logf("job %d: queue=%s state=%s attempt=%d claim_version=%d errors=%v", id,
			job.Queue, job.State, job.Attempt, job.ClaimVersion, job.Errors)
	}
	f.t.Fatalf("%s (stage=%s, context=%v)", reason, f.stage, f.ctx.Err())
}

// preemptionIdleQueueController exposes queue restoration without external I/O in a synctest bubble.
type preemptionIdleQueueController struct {
	workerQueueController
	restored chan struct{}
}

// AddQueue signals the actual queue restoration so state updates alone cannot satisfy the delay assertion.
func (c *preemptionIdleQueueController) AddQueue(_ string, _ river.QueueConfig) error {
	close(c.restored)
	return nil
}

// TestMirrorUrgentIdleDelay preserves the production debounce boundary without real ten-second waits.
func TestMirrorUrgentIdleDelay(t *testing.T) {
	for _, queue := range []string{workhub.MirrorRepoQueue, workhub.MirrorLFSQueue} {
		t.Run(queue, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				controller := &preemptionIdleQueueController{restored: make(chan struct{})}
				manager := workhub.NewUrgentManager(workhub.UrgentManagerConfig{
					QueueController: controller, NormalQueue: queue,
					NormalQueueConfig: river.QueueConfig{MaxWorkers: 1}, UrgentIdleDelay: urgentIdleDelay,
				})
				defer manager.Close(workhub.ErrWorkerShutdown)
				done, err := manager.BeginUrgent(t.Context())
				require.NoError(t, err)
				done()
				time.Sleep(urgentIdleDelay - time.Nanosecond)
				synctest.Wait()
				require.Equal(t, workhub.UrgentStateUrgentIdle, manager.State())
				select {
				case <-controller.restored:
					t.Fatal("normal queue restored before the production idle delay elapsed")
				default:
				}
				time.Sleep(time.Nanosecond)
				synctest.Wait()
				select {
				case <-controller.restored:
				default:
					t.Fatal("normal queue did not restore when the production idle delay elapsed")
				}
				require.Equal(t, workhub.UrgentStateNormal, manager.State())
			})
		})
	}
}

// TestMirrorWorkerShutdownReturnsCancellation verifies River settles shutdown without application Snooze delays.
func TestMirrorWorkerShutdownReturnsCancellation(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"repo", "lfs"} {
		for _, tc := range []struct {
			name string
			err  error
		}{
			{name: "Go cancellation"},
			{name: "RPC cancellation", err: status.Error(codes.Canceled, "worker stopped during fetch")},
		} {
			t.Run(stage+"/"+tc.name, func(t *testing.T) {
				parallelPreemptionScenario(t)
				f := newPreemptionFixture(t, stage)
				f.syncer.cancelErr = tc.err
				job := f.insert(1)
				var client workhub.WorkClient
				var err error
				if stage == "repo" {
					client, err = NewRepoWorkClient(f.ctx, f.dsn, RepoWorkDeps{
						MirrorTaskStore: f.store, Syncer: f.syncer, LFSJobClient: fakeMirrorLFSJobClient{}, MaxWorkers: 1,
					})
				} else {
					client, err = NewLFSWorkClient(f.ctx, f.dsn, LFSWorkDeps{
						MirrorTaskStore: f.store, Syncer: f.syncer, MaxWorkers: 1,
					})
				}
				require.NoError(t, err)
				stopped := false
				t.Cleanup(func() {
					if !stopped {
						ctx, cancel := context.WithTimeout(context.Background(), preemptionWaitTimeout)
						defer cancel()
						require.NoError(t, client.Stop(ctx))
					}
				})
				require.NoError(t, client.Start(f.ctx))
				f.waitStart(1)

				stopCtx, cancel := context.WithTimeout(f.ctx, preemptionWaitTimeout)
				defer cancel()
				err = client.Stop(stopCtx)
				stopped = err == nil
				require.NoError(t, err)
				interrupted, err := f.producer.JobGet(f.ctx, job.ID)
				require.NoError(t, err)
				require.Equal(t, rivertype.JobStateAvailable, interrupted.State)
				require.Zero(t, interrupted.Attempt)
				require.Empty(t, interrupted.Errors)
				require.NotContains(t, string(interrupted.Metadata), "snoozes")
				task, err := f.store.FindByID(f.ctx, 1)
				require.NoError(t, err)
				wantStatus := types.MirrorRepoSyncStart
				if stage == "lfs" {
					wantStatus = types.MirrorLfsSyncStart
				}
				require.Equal(t, wantStatus, task.Status)
				require.Equal(t, "saved-commit", task.AfterLastCommitID)

				f.startClient(true)
				f.waitStart(1)
				f.requireCompleted(job.ID)
			})
		}
	}
}

// TestMirrorUrgentLocalPreemption verifies order, attempt refunds, checkpoint preservation and client-local scope.
func TestMirrorUrgentLocalPreemption(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		stage            string
		otherClient      bool
		grpcCancellation bool
	}{
		{stage: "repo"}, {stage: "lfs"}, {stage: "repo", otherClient: true},
		{stage: "repo", grpcCancellation: true}, {stage: "lfs", grpcCancellation: true},
	} {
		t.Run(fmt.Sprintf("%s/other_client=%t/grpc=%t", tc.stage, tc.otherClient, tc.grpcCancellation), func(t *testing.T) {
			parallelPreemptionScenario(t)
			f := newPreemptionFixture(t, tc.stage)
			if tc.grpcCancellation {
				// Gitaly preserves RPC cancellation inside an operation-specific error wrapper.
				f.syncer.cancelErr = errorx.ErrGitMirrorSyncFailed(status.Error(codes.Canceled, "RPC interrupted"), errorx.Ctx())
			}
			a := f.insert(1)
			for id := int64(2); id <= 4; id++ {
				f.insert(id)
			}
			manager := f.startClient(false)
			f.waitStart(1)
			f.insert(5)
			f.waitStart(5)
			paused, err := f.producer.JobGet(f.ctx, a.ID)
			require.NoError(t, err)
			require.Equal(t, rivertype.JobStateAvailable, paused.State)
			require.True(t, a.ScheduledAt.Equal(paused.ScheduledAt))
			require.Equal(t, a.Priority, paused.Priority)
			require.Zero(t, paused.Attempt)
			require.Empty(t, paused.Errors)
			require.NotContains(t, string(paused.Metadata), "snoozes")
			task, err := f.store.FindByID(f.ctx, 1)
			require.NoError(t, err)
			require.Equal(t, 42, task.Progress)
			require.Equal(t, "saved-commit", task.AfterLastCommitID)
			if tc.stage == "repo" {
				require.Equal(t, types.MirrorQueued, task.Status)
			} else {
				require.Equal(t, types.MirrorRepoSyncFinished, task.Status)
			}
			queue, err := f.producer.QueueGet(f.ctx, f.queue)
			require.NoError(t, err)
			require.Nil(t, queue.PausedAt)
			if tc.otherClient {
				f.startClient(true)
				f.waitStart(1)
				// Keep M running until the other client has started all normal work.
				// This proves local isolation without racing two normal producers after restoration.
				for id := int64(2); id <= 4; id++ {
					f.waitStart(id)
				}
				require.Equal(t, workhub.UrgentStateUrgent, manager.State())
			}
			released := time.Now()
			close(f.syncer.releaseM)
			if !tc.otherClient {
				f.waitStart(1)
				require.GreaterOrEqual(t, time.Since(released), preemptionIdleDelay)
				for id := int64(2); id <= 4; id++ {
					f.waitStart(id)
				}
			}
			require.Eventually(t, func() bool { return manager.State() == workhub.UrgentStateNormal }, preemptionWaitTimeout, 10*time.Millisecond)
			f.requireCompleted(f.jobIDs...)
			require.Eventually(t, func() bool {
				row, err := f.producer.JobGet(f.ctx, a.ID)
				return err == nil && row.State == rivertype.JobStateCompleted && row.Attempt == 1 && row.ClaimVersion > paused.ClaimVersion
			}, preemptionWaitTimeout, 10*time.Millisecond)
		})
	}
}

// TestMirrorUrgentCanceledWaiterKeepsPreemption verifies canceling M does not abandon A's interruption or its delayed restoration.
func TestMirrorUrgentCanceledWaiterKeepsPreemption(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"repo", "lfs"} {
		t.Run(stage, func(t *testing.T) {
			parallelPreemptionScenario(t)
			f := newPreemptionFixture(t, stage)
			releaseA := make(chan struct{})
			resumeA := sync.OnceFunc(func() { close(releaseA) })
			defer resumeA()
			f.syncer.releaseCanceledA = releaseA
			a := f.insert(1)
			for id := int64(2); id <= 4; id++ {
				f.insert(id)
			}
			manager := f.startClient(false)
			f.waitStart(1)
			m := f.insert(5)
			f.waitCancellation()
			_, err := f.producer.JobCancel(f.ctx, m.ID)
			require.NoError(t, err)
			require.Eventually(t, func() bool { return manager.UrgentReservations() == 0 }, preemptionWaitTimeout, time.Millisecond)
			require.Equal(t, workhub.UrgentStatePreempting, manager.State())
			running, err := f.producer.JobGet(f.ctx, a.ID)
			require.NoError(t, err)
			require.Equal(t, rivertype.JobStateRunning, running.State)

			released := time.Now()
			resumeA()
			// M never reaches business execution; A resumes only after removal settles and the idle delay elapses.
			f.waitStart(1)
			require.GreaterOrEqual(t, time.Since(released), preemptionIdleDelay)
			for id := int64(2); id <= 4; id++ {
				f.waitStart(id)
			}
			f.waitJobState(m.ID, rivertype.JobStateCancelled)
			for _, jobID := range f.jobIDs {
				if jobID != m.ID {
					f.requireCompleted(jobID)
				}
			}
			require.Eventually(t, func() bool {
				row, err := f.producer.JobGet(f.ctx, a.ID)
				return err == nil && row.State == rivertype.JobStateCompleted && row.Attempt == 1 &&
					len(row.Errors) == 0 && row.ClaimVersion > running.ClaimVersion && manager.State() == workhub.UrgentStateNormal
			}, preemptionWaitTimeout, time.Millisecond)
		})
	}
}

// TestMirrorUrgentGRPCBusinessFailure verifies joined business failures retain River retry semantics after preemption.
func TestMirrorUrgentGRPCBusinessFailure(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"repo", "lfs"} {
		t.Run(stage, func(t *testing.T) {
			parallelPreemptionScenario(t)
			f := newPreemptionFixture(t, stage)
			f.maxAttempts = 3
			businessErr := errors.New("checkpoint persistence failed")
			f.syncer.cancelErr = errors.Join(status.Error(codes.Canceled, "RPC interrupted"), businessErr)
			a := f.insert(1)
			for id := int64(2); id <= 4; id++ {
				f.insert(id)
			}
			manager := f.startClient(false)
			f.waitStart(1)
			f.insert(5)
			f.waitStart(5)

			failed, err := f.producer.JobGet(f.ctx, a.ID)
			require.NoError(t, err)
			require.Equal(t, rivertype.JobStateRetryable, failed.State)
			require.Equal(t, 1, failed.Attempt)
			require.Greater(t, failed.MaxAttempts, failed.Attempt)
			require.Len(t, failed.Errors, 1)
			require.Contains(t, failed.Errors[0].Error, businessErr.Error())
			require.True(t, failed.ScheduledAt.After(time.Now().Add(4*time.Minute)))
			require.NotContains(t, string(failed.Metadata), "snoozes")

			released := time.Now()
			close(f.syncer.releaseM)
			// Only real failures defer A behind B/C/D; pure cancellation must resume A first.
			for id := int64(2); id <= 4; id++ {
				f.waitStart(id)
			}
			require.GreaterOrEqual(t, time.Since(released), preemptionIdleDelay)
			require.Eventually(t, func() bool { return manager.State() == workhub.UrgentStateNormal }, preemptionWaitTimeout, time.Millisecond)
			for _, jobID := range f.jobIDs {
				if jobID != a.ID {
					f.requireCompleted(jobID)
				}
			}
			f.waitJobState(a.ID, rivertype.JobStateRetryable)
		})
	}
}

// TestMirrorUrgentPreemptionCancellationBudget verifies slow cooperative cleanup settles without lease rescue.
func TestMirrorUrgentPreemptionCancellationBudget(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"repo", "lfs"} {
		t.Run(stage, func(t *testing.T) {
			parallelPreemptionScenario(t)
			f := newPreemptionFixture(t, stage)
			f.syncer.exitDelay = 4 * time.Second
			f.store.pauseDelay = 7 * time.Second
			a := f.insert(1)
			for id := int64(2); id <= 4; id++ {
				f.insert(id)
			}
			manager := f.startClient(false)
			f.waitStart(1)
			f.insert(5)
			canceledAt := f.waitCancellation()
			f.waitStart(5)
			require.GreaterOrEqual(t, time.Since(canceledAt), f.syncer.exitDelay+f.store.pauseDelay)
			paused, err := f.producer.JobGet(f.ctx, a.ID)
			require.NoError(t, err)
			require.Equal(t, rivertype.JobStateAvailable, paused.State)
			require.True(t, a.ScheduledAt.Equal(paused.ScheduledAt))
			require.Zero(t, paused.Attempt)
			require.Empty(t, paused.Errors)
			require.NotContains(t, string(paused.Metadata), "river:rescue_count")
			require.NotContains(t, string(paused.Metadata), "snoozes")
			released := time.Now()
			close(f.syncer.releaseM)
			f.waitStart(1)
			require.GreaterOrEqual(t, time.Since(released), preemptionIdleDelay)
			for id := int64(2); id <= 4; id++ {
				f.waitStart(id)
			}
			f.requireCompleted(f.jobIDs...)
			require.Eventually(t, func() bool {
				row, err := f.producer.JobGet(f.ctx, a.ID)
				return err == nil && row.State == rivertype.JobStateCompleted && row.Attempt == 1 &&
					row.ClaimVersion > paused.ClaimVersion && manager.State() == workhub.UrgentStateNormal
			}, preemptionWaitTimeout, 10*time.Millisecond)
		})
	}
}

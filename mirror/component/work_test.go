package component

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	sharedcomponent "opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/mirror/reposyncer"
)

// TestMirrorJobLeaseConfig verifies both worker factories budget cancellation and pass River's timing constraints.
func TestMirrorJobLeaseConfig(t *testing.T) {
	for stage, cfg := range map[string]*river.Config{
		"repo": newRepoRiverConfig(RepoWorkDeps{}),
		"lfs":  newLFSRiverConfig(LFSWorkDeps{}),
	} {
		t.Run(stage, func(t *testing.T) {
			require.NotNil(t, cfg.JobLease)
			lease := cfg.JobLease.WithDefaults()
			require.NoError(t, lease.Validate())
			// Repo compensation can take 30s before runWork starts its separate pause cleanup.
			cleanupAndCompletion := 30*time.Second + mirrorPauseCleanupTimeout + lease.FinalizeTimeout
			require.GreaterOrEqual(t, lease.CancelGracePeriod, cleanupAndCompletion+20*time.Second)
			require.Greater(t, lease.LocalSafetyMargin, lease.CancelGracePeriod)
			require.Greater(t, lease.LeaseDuration, lease.LocalSafetyMargin+lease.HeartbeatInterval+lease.RenewTimeout)
			require.Equal(t, 30*time.Second, lease.FinalizeTimeout)
			require.Equal(t, 10*time.Second, mirrorPauseCleanupTimeout)
			require.Equal(t, 10*time.Second, urgentIdleDelay)
		})
	}
}

// TestRunWorkExecutesBusinessWork verifies the shared lifecycle delegates typed business work.
func TestRunWorkExecutesBusinessWork(t *testing.T) {
	task := repoWorkerTask(types.MirrorQueued)
	args := repoArgsFromTask(task)
	job := riverJob(args)
	job.Queue = workhub.MirrorRepoQueue
	called := false

	err := runWorkWithLog(context.Background(), job, mirrorWorkConfig[workhub.RepoArgs]{
		args:    args.MirrorArgs,
		urgent:  args.Urgent,
		stage:   "repo",
		manager: newWorkerTestManager(t),
		work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
			called = true
			return task, nil
		},
	})

	require.NoError(t, err)
	require.True(t, called)
}

// TestRunWorkReturnsTimedOutWorkAsFailure verifies deadlines use ordinary retry and exhaustion handling.
func TestRunWorkReturnsTimedOutWorkAsFailure(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	for _, finalAttempt := range []bool{false, true} {
		t.Run(fmt.Sprintf("final attempt %t", finalAttempt), func(t *testing.T) {
			task := repoWorkerTask(types.MirrorRepoSyncStart)
			store := &fakeRepoTaskStore{task: task}
			job := riverJob(repoArgsFromTask(task))
			if finalAttempt {
				job.Attempt = job.MaxAttempts
			}
			encodedArgs, err := json.Marshal(job.Args)
			require.NoError(t, err)
			job.EncodedArgs = encodedArgs

			err = runWork(ctx, job, mirrorWorkConfig[workhub.RepoArgs]{
				args: job.Args.MirrorArgs, stage: "repo", taskStore: store,
				work: func(ctx context.Context, _ workhub.RepoArgs, _ int) (*database.MirrorTask, error) {
					return task, fmt.Errorf("sync timed out: %w", ctx.Err())
				},
			})

			require.ErrorIs(t, err, context.DeadlineExceeded)
			if finalAttempt {
				require.Equal(t, []string{database.MirrorFatal}, store.actions)
			} else {
				require.Empty(t, store.actions)
			}
		})
	}
}

// TestRunWorkReturnsCancelledWork leaves execution cancellation for River to settle.
func TestRunWorkReturnsCancelledWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	task := repoWorkerTask(types.MirrorRepoSyncStart)
	args := repoArgsFromTask(task)
	job := riverJob(args)

	err := runWorkWithLog(ctx, job, mirrorWorkConfig[workhub.RepoArgs]{
		args:    args.MirrorArgs,
		stage:   "repo",
		manager: newWorkerTestManager(t),
		work: func(ctx context.Context, args workhub.RepoArgs, retryCount int) (*database.MirrorTask, error) {
			return task, ctx.Err()
		},
	})

	require.Same(t, context.Canceled, err)
}

// TestRunWorkCancellation preserves business failures and explicit controls under a canceled context.
func TestRunWorkCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rpcCanceled := status.Error(codes.Canceled, "fetch interrupted")
	businessErr := errors.New("checkpoint failed")
	for _, tc := range []struct {
		name             string
		workErr          error
		wantCancellation bool
		control          bool
	}{
		{name: "Go cancellation", workErr: context.Canceled, wantCancellation: true},
		{name: "wrapped cancellation", workErr: fmt.Errorf("sync: %w", context.Canceled), wantCancellation: true},
		{name: "RPC cancellation", workErr: rpcCanceled, wantCancellation: true},
		{name: "Git wrapper", workErr: errorx.ErrGitMirrorSyncFailed(rpcCanceled, errorx.Ctx()), wantCancellation: true},
		{name: "nested cancellations", workErr: errors.Join(rpcCanceled, errors.Join(context.Canceled, rpcCanceled)), wantCancellation: true},
		{name: "joined RPC cancellations", workErr: errors.Join(rpcCanceled, fmt.Errorf("fetch: %w", rpcCanceled)), wantCancellation: true},
		{name: "Go then business", workErr: errors.Join(context.Canceled, businessErr)},
		{name: "business then Go", workErr: errors.Join(businessErr, context.Canceled)},
		{name: "RPC then business", workErr: errors.Join(rpcCanceled, businessErr)},
		{name: "business then RPC", workErr: errors.Join(businessErr, rpcCanceled)},
		{name: "wrapped mixed errors", workErr: fmt.Errorf("sync: %w", errors.Join(context.Canceled, businessErr))},
		{name: "nested mixed errors", workErr: errors.Join(rpcCanceled, errors.Join(context.Canceled, businessErr))},
		{name: "business only", workErr: businessErr},
		{name: "cancellation text only", workErr: errors.New(context.Canceled.Error())},
		{name: "deadline", workErr: context.DeadlineExceeded},
		{name: "RPC unavailable", workErr: status.Error(codes.Unavailable, "server unavailable")},
		{name: "pause error without pause cause", workErr: river.ErrQueuePaused},
		{name: "explicit cancellation", workErr: river.JobCancel(context.Canceled), control: true},
		{name: "wrapped explicit cancellation", workErr: fmt.Errorf("stop: %w", river.JobCancel(rpcCanceled)), control: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := repoWorkerTask(types.MirrorRepoSyncStart)
			store := &fakeRepoTaskStore{task: task}
			job := riverJob(repoArgsFromTask(task))
			job.Attempt = job.MaxAttempts
			encodedArgs, err := json.Marshal(job.Args)
			require.NoError(t, err)
			job.EncodedArgs = encodedArgs

			err = runWork(ctx, job, mirrorWorkConfig[workhub.RepoArgs]{
				args: job.Args.MirrorArgs, stage: "repo", taskStore: store,
				work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
					return task, tc.workErr
				},
			})

			switch {
			case tc.wantCancellation:
				if errors.Is(tc.workErr, context.Canceled) {
					require.Same(t, tc.workErr, err)
				} else {
					require.Same(t, context.Canceled, err)
				}
				require.Empty(t, store.actions)
			case tc.control:
				require.Same(t, tc.workErr, err)
				require.Empty(t, store.actions)
			default:
				require.ErrorIs(t, err, tc.workErr)
				require.Equal(t, []string{database.MirrorFatal}, store.actions)
				require.Contains(t, store.task.ErrorMessage, tc.workErr.Error())
			}
		})
	}
}

// TestIsWorkCancellationContext requires a canceled execution context and keeps pauses separate.
func TestIsWorkCancellationContext(t *testing.T) {
	deadlineCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	require.False(t, isWorkCancellation(deadlineCtx, context.Canceled))
	for _, cause := range []error{nil, context.Canceled, workhub.ErrWorkerShutdown, river.ErrJobCancelledRemotely, river.ErrQueuePaused} {
		t.Run(fmt.Sprint(cause), func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			if cause != nil {
				cancel(cause)
			}
			want := cause != nil && cause != river.ErrQueuePaused
			require.Equal(t, want, isWorkCancellation(ctx, context.Canceled))
			require.Equal(t, want, isWorkCancellation(ctx, status.Error(codes.Canceled, "fetch interrupted")))
			require.False(t, isWorkCancellation(ctx, nil))
		})
	}
}

// TestRunWorkLogsAndPropagatesPanic verifies lifecycle logging does not swallow worker panics.
func TestRunWorkLogsAndPropagatesPanic(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	task := repoWorkerTask(types.MirrorRepoSyncStart)
	args := repoArgsFromTask(task)
	job := riverJob(args)

	require.PanicsWithValue(t, "sync panic", func() {
		_ = runWorkWithLog(context.Background(), job, mirrorWorkConfig[workhub.RepoArgs]{
			args:    args.MirrorArgs,
			stage:   "repo",
			manager: newWorkerTestManager(t),
			work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
				panic("sync panic")
			},
		})
	})

	require.Contains(t, output.String(), `"msg":"[repo] work panic"`)
}

// TestRunWorkReturnsOrdinaryErrors verifies retryable business failures remain ordinary River errors.
func TestRunWorkReturnsOrdinaryErrors(t *testing.T) {
	task := repoWorkerTask(types.MirrorRepoSyncStart)
	args := repoArgsFromTask(task)
	job := riverJob(args)
	wantErr := errors.New("sync failed")

	err := runWorkWithLog(context.Background(), job, mirrorWorkConfig[workhub.RepoArgs]{
		args:    args.MirrorArgs,
		stage:   "repo",
		manager: newWorkerTestManager(t),
		work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
			return task, wantErr
		},
	})

	require.ErrorIs(t, err, wantErr)
}

// TestIsWorkContextTermination distinguishes lifecycle termination from sync errors with similar values.
func TestIsWorkContextTermination(t *testing.T) {
	t.Run("active context", func(t *testing.T) {
		require.False(t, isWorkContextTermination(context.Background(), context.Canceled))
	})

	t.Run("worker cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.True(t, isWorkContextTermination(ctx, context.Canceled))
	})

	t.Run("remote job cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(river.ErrJobCancelledRemotely)
		require.True(t, isWorkContextTermination(ctx, context.Canceled))
		require.True(t, isWorkContextTermination(ctx, river.ErrJobCancelledRemotely))
	})

	t.Run("worker timeout", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()
		require.True(t, isWorkContextTermination(ctx, context.DeadlineExceeded))
	})

	t.Run("urgent preemption", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(river.ErrQueuePaused)
		require.True(t, isWorkContextTermination(ctx, context.Canceled))
		require.False(t, isWorkContextTermination(ctx, context.DeadlineExceeded))
		require.True(t, isWorkContextTermination(ctx, river.ErrQueuePaused))
	})

	t.Run("unrelated error after cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.False(t, isWorkContextTermination(ctx, errors.New("sync failed")))
	})
}

// TestQueuePauseCancellationGRPC checks every error branch and requires the local pause cause.
func TestQueuePauseCancellationGRPC(t *testing.T) {
	rpcCanceled := status.Error(codes.Canceled, "context canceled")
	businessErr := errors.New("checkpoint failed")
	tests := []struct {
		name string
		err  error
		pure bool
	}{
		{name: "nil"},
		{name: "Go cancellation", err: context.Canceled, pure: true},
		{name: "River cancellation", err: river.ErrQueuePaused, pure: true},
		{name: "wrapped RPC cancellation", err: fmt.Errorf("fetch: %w", rpcCanceled), pure: true},
		{name: "Git error wrapper", err: errorx.ErrGitMirrorSyncFailed(rpcCanceled, errorx.Ctx()), pure: true},
		{name: "nested cancellations", err: errors.Join(rpcCanceled, errors.Join(context.Canceled, river.ErrQueuePaused)), pure: true},
		{name: "RPC then business error", err: errors.Join(rpcCanceled, businessErr)},
		{name: "business error then RPC", err: errors.Join(businessErr, rpcCanceled)},
		{name: "wrapped mixed errors", err: fmt.Errorf("sync: %w", errors.Join(rpcCanceled, businessErr))},
		{name: "nested mixed errors", err: errors.Join(rpcCanceled, errors.Join(context.Canceled, businessErr))},
		{name: "explicit cancellation", err: river.JobCancel(rpcCanceled)},
		{name: "Go deadline", err: context.DeadlineExceeded},
		{name: "cancellation text only", err: errors.New(rpcCanceled.Error())},
	}
	// Only Canceled is a pause-compatible RPC status; OK produces no error.
	for code := codes.OK; code <= codes.Unauthenticated; code++ {
		tests = append(tests, struct {
			name string
			err  error
			pure bool
		}{name: "RPC " + code.String(), err: status.Error(code, "RPC result"), pure: code == codes.Canceled})
	}
	for _, cause := range []error{nil, river.ErrQueuePaused, context.Canceled, river.ErrJobCancelledRemotely, context.DeadlineExceeded} {
		name := "active"
		if cause != nil {
			name = cause.Error()
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			if cause != nil {
				cancel(cause)
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					require.Equal(t, cause == river.ErrQueuePaused && tc.pure, isQueuePauseCancellation(ctx, tc.err))
					if cause == river.ErrQueuePaused {
						require.Equal(t, tc.pure, isWorkContextTermination(ctx, tc.err))
					}
				})
			}
		})
	}
}

// pauseCleanupStore verifies cancellation cleanup uses a live, bounded context and preserves errors.
type pauseCleanupStore struct {
	fakeRepoTaskStore
	pauseErr error
}

// FindByID rejects canceled or unbounded cleanup contexts.
func (s *pauseCleanupStore) FindByID(ctx context.Context, id int64) (*database.MirrorTask, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, ok := ctx.Deadline(); !ok {
		return nil, errors.New("cleanup has no deadline")
	}
	return s.fakeRepoTaskStore.FindByID(ctx, id)
}

// UpdateStatusAndRepoSyncStatus injects a business persistence failure after cancellation.
func (s *pauseCleanupStore) UpdateStatusAndRepoSyncStatus(ctx context.Context, task database.MirrorTask, action string) (database.MirrorTask, error) {
	if err := ctx.Err(); err != nil {
		return task, err
	}
	if s.pauseErr != nil {
		return task, s.pauseErr
	}
	return s.fakeRepoTaskStore.UpdateStatusAndRepoSyncStatus(ctx, task, action)
}

// TestRunWorkQueuePause preserves cancellation, business failures and successful completion independently.
func TestRunWorkQueuePause(t *testing.T) {
	checkpointErr := errors.New("checkpoint failed")
	rpcCanceled := status.Error(codes.Canceled, "fetch interrupted")
	for _, tc := range []struct {
		name       string
		workErr    error
		pauseErr   error
		wantPause  bool
		wantAction bool
	}{
		{name: "cancellation", workErr: context.Canceled, wantPause: true, wantAction: true},
		{name: "wrapped cancellation", workErr: fmt.Errorf("sync stopped: %w", context.Canceled), wantPause: true, wantAction: true},
		{name: "joined cancellations", workErr: errors.Join(context.Canceled, river.ErrQueuePaused), wantPause: true, wantAction: true},
		{name: "RPC cancellation", workErr: rpcCanceled, wantPause: true, wantAction: true},
		{name: "wrapped RPC cancellation", workErr: errorx.ErrGitMirrorSyncFailed(rpcCanceled, errorx.Ctx()), wantPause: true, wantAction: true},
		{name: "RPC with business failure", workErr: errors.Join(rpcCanceled, checkpointErr)},
		{name: "RPC pause persistence failure", workErr: rpcCanceled, pauseErr: checkpointErr},
		{name: "completed work"},
		{name: "checkpoint failure", workErr: errors.Join(context.Canceled, checkpointErr)},
		{name: "pause persistence failure", workErr: context.Canceled, pauseErr: checkpointErr},
		{name: "explicit cancellation", workErr: river.JobCancel(context.Canceled)},
		{name: "unrelated timeout", workErr: context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := captureMirrorWorkerLogs(t)
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			task := repoWorkerTask(types.MirrorRepoSyncStart)
			store := &pauseCleanupStore{fakeRepoTaskStore: fakeRepoTaskStore{task: task}, pauseErr: tc.pauseErr}
			job := riverJob(repoArgsFromTask(task))
			if tc.wantPause {
				job.Attempt = job.MaxAttempts
			}
			err := runWork(ctx, job, mirrorWorkConfig[workhub.RepoArgs]{
				args: job.Args.MirrorArgs, stage: "repo", taskStore: store,
				work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
					cancel(river.ErrQueuePaused)
					return task, tc.workErr
				},
			})
			require.Equal(t, tc.wantPause, isQueuePauseCancellation(ctx, err))
			if tc.wantPause {
				require.ErrorIs(t, err, river.ErrQueuePaused)
				if tc.name == "RPC cancellation" {
					require.Contains(t, output.String(), "fetch interrupted")
				}
			} else if tc.workErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.workErr)
			}
			if tc.pauseErr != nil {
				require.ErrorIs(t, err, tc.pauseErr)
			}
			if tc.wantAction {
				require.Equal(t, []string{database.MirrorPause}, store.actions)
			} else {
				require.Empty(t, store.actions)
			}
		})
	}
}

// TestRunWorkPauseRejectsStaleTask prevents pause cleanup from updating a replacement task or job.
func TestRunWorkPauseRejectsStaleTask(t *testing.T) {
	for _, change := range []string{"current task", "stage job"} {
		t.Run(change, func(t *testing.T) {
			task := repoWorkerTask(types.MirrorRepoSyncStart)
			job := riverJob(repoArgsFromTask(task))
			if change == "current task" {
				task.Mirror.CurrentTaskID = task.ID + 1
			} else {
				task.RepoJobID++
			}
			store := &pauseCleanupStore{fakeRepoTaskStore: fakeRepoTaskStore{task: task}}
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			err := runWork(ctx, job, mirrorWorkConfig[workhub.RepoArgs]{
				args: job.Args.MirrorArgs, stage: "repo", taskStore: store,
				work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
					cancel(river.ErrQueuePaused)
					return task, ctx.Err()
				},
			})
			require.Error(t, err)
			require.False(t, isQueuePauseCancellation(ctx, err))
			require.Empty(t, store.actions)
		})
	}
}

// metricsGateSyncer keeps a real worker inside business work until the test releases it.
type metricsGateSyncer struct {
	started chan struct{}
	release chan struct{}
}

// SyncRepo completes a no-LFS repository, or propagates the worker cancellation.
func (s metricsGateSyncer) SyncRepo(ctx context.Context, _ *database.Mirror, task *database.MirrorTask) (reposyncer.RepoSyncResult, error) {
	close(s.started)
	select {
	case <-s.release:
		return reposyncer.RepoSyncResult{Task: task, NoLFS: true}, nil
	case <-ctx.Done():
		return reposyncer.RepoSyncResult{}, ctx.Err()
	}
}

// workerMetrics starts a reporter against a disposable Redis and provides its independent reader.
func workerMetrics(t *testing.T) (*sharedcomponent.MirrorMetricsReporter, *cache.MirrorMetricsStore) {
	t.Helper()
	server := miniredis.RunT(t)
	cfg := &config.Config{}
	cfg.Redis.Endpoint = server.Addr()
	cfg.Mirror.Metrics.Enabled = true
	cfg.Mirror.WorkerNumber = 2
	reporter := sharedcomponent.NewMirrorMetricsReporter(cfg, "repo")
	reporter.SetState("ready")
	store := cache.NewMirrorMetricsStoreFromConfig(cfg)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		reporter.Stop(ctx)
		_ = store.Close()
	})
	return reporter, store
}

// requireMetricExecution waits for the next periodic snapshot.
func requireMetricExecution(t *testing.T, store *cache.MirrorMetricsStore) {
	t.Helper()
	require.Eventually(t, func() bool {
		snapshot, err := store.Read(context.Background())
		return err == nil && len(snapshot.Instances) == 1 && snapshot.Instances[0].Instance.ExecutingTasks == 1 && len(snapshot.Instances[0].Executions) == 1
	}, 2500*time.Millisecond, 10*time.Millisecond)
}

// stopWorkerMetrics forces the normal shutdown flush and returns its final persisted snapshot.
func stopWorkerMetrics(t *testing.T, reporter *sharedcomponent.MirrorMetricsReporter, store *cache.MirrorMetricsStore) *cache.MirrorMetricsReadSnapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	reporter.Stop(ctx)
	snapshot, err := store.Read(context.Background())
	require.NoError(t, err)
	require.Len(t, snapshot.Instances, 1)
	require.Empty(t, snapshot.Instances[0].Executions)
	require.Zero(t, snapshot.Instances[0].Instance.ExecutingTasks)
	return snapshot
}

// TestRepoWorkerMetricsLifecycle covers current work and cleanup after skip, success and cancellation.
func TestRepoWorkerMetricsLifecycle(t *testing.T) {
	t.Run("terminal skip is not success", func(t *testing.T) {
		reporter, store := workerMetrics(t)
		task := repoWorkerTask(types.MirrorLfsSyncFinished)
		worker := &repoWorker{mirrorTaskStore: &fakeRepoTaskStore{task: task}, urgentManager: newWorkerTestManager(t), syncer: failRepoSyncer{t: t}, reporter: reporter}
		require.NoError(t, worker.Work(context.Background(), riverJob(repoArgsFromTask(task))))
		stopWorkerMetrics(t, reporter, store)
	})
	t.Run("committed no LFS success", func(t *testing.T) {
		reporter, store := workerMetrics(t)
		task := repoWorkerTask(types.MirrorQueued)
		syncer := metricsGateSyncer{started: make(chan struct{}), release: make(chan struct{})}
		worker := &repoWorker{mirrorTaskStore: &fakeRepoTaskStore{task: task}, urgentManager: newWorkerTestManager(t), syncer: syncer, reporter: reporter, lfsJobClient: fakeMirrorLFSJobClient{}}
		result := make(chan error, 1)
		go func() { result <- worker.Work(context.Background(), riverJob(repoArgsFromTask(task))) }()
		select {
		case <-syncer.started:
		case <-time.After(time.Second):
			t.Fatal("worker did not begin sync")
		}
		requireMetricExecution(t, store)
		close(syncer.release)
		require.NoError(t, <-result)
		stopWorkerMetrics(t, reporter, store)
	})
	t.Run("shutdown cancellation releases without outcome", func(t *testing.T) {
		reporter, store := workerMetrics(t)
		task := repoWorkerTask(types.MirrorQueued)
		syncer := metricsGateSyncer{started: make(chan struct{}), release: make(chan struct{})}
		worker := &repoWorker{mirrorTaskStore: &fakeRepoTaskStore{task: task}, urgentManager: newWorkerTestManager(t), syncer: syncer, reporter: reporter, lfsJobClient: fakeMirrorLFSJobClient{}}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		result := make(chan error, 1)
		go func() { result <- worker.Work(ctx, riverJob(repoArgsFromTask(task))) }()
		select {
		case <-syncer.started:
		case <-time.After(time.Second):
			t.Fatal("worker did not begin sync")
		}
		requireMetricExecution(t, store)
		cancel()
		err := <-result
		require.ErrorIs(t, err, context.Canceled)
		stopWorkerMetrics(t, reporter, store)
	})
}

// TestMetricTaskDetailsRepositoryRoutes keeps immediate worker rows consistent with collected UI links.
func TestMetricTaskDetailsRepositoryRoutes(t *testing.T) {
	for _, kind := range []types.RepositoryType{"model", "dataset", "code", "space", "mcp", "skill"} {
		t.Run(string(kind), func(t *testing.T) {
			task := &database.MirrorTask{Mirror: &database.Mirror{Repository: &database.Repository{RepositoryType: kind, Path: "team/repository"}}}
			require.Equal(t, string(kind)+"s/team/repository", metricTaskDetails(task).RepoPath)
		})
	}
}

// TestRepoWorkerMetricsFirstStart publishes the prepared timestamp and preserves resumed work's original start.
func TestRepoWorkerMetricsFirstStart(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  types.MirrorTaskStatus
		started time.Time
	}{
		{"first execution", types.MirrorQueued, time.Time{}},
		{"resumed execution", types.MirrorRepoSyncStart, time.Now().Add(-time.Hour)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reporter, store := workerMetrics(t)
			task := repoWorkerTask(tc.status)
			task.StartedAt = tc.started
			taskStore := &fakeRepoTaskStore{task: task}
			syncer := metricsGateSyncer{started: make(chan struct{}), release: make(chan struct{})}
			worker := &repoWorker{mirrorTaskStore: taskStore, urgentManager: newWorkerTestManager(t), syncer: syncer, reporter: reporter, lfsJobClient: fakeMirrorLFSJobClient{}}
			result := make(chan error, 1)
			go func() { result <- worker.Work(context.Background(), riverJob(repoArgsFromTask(task))) }()
			defer func() {
				close(syncer.release)
				select {
				case err := <-result:
					require.NoError(t, err)
				case <-time.After(time.Second):
					t.Error("worker did not stop")
				}
			}()
			select {
			case <-syncer.started:
			case <-time.After(time.Second):
				t.Fatal("worker did not begin sync")
			}
			requireMetricExecution(t, store)
			snapshot, err := store.Read(context.Background())
			require.NoError(t, err)
			execution := snapshot.Instances[0].Executions[0]
			require.NotNil(t, execution.StartedAtMS)
			require.False(t, taskStore.task.StartedAt.IsZero())
			require.Equal(t, taskStore.task.StartedAt.UnixMilli(), *execution.StartedAtMS)
			if !tc.started.IsZero() {
				require.Equal(t, tc.started.UnixMilli(), *execution.StartedAtMS)
			}
		})
	}
}

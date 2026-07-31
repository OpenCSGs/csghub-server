package component

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/types"
)

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

// TestRunWorkPreservesExplicitSnooze verifies business work controls its own River delay.
func TestRunWorkPreservesExplicitSnooze(t *testing.T) {
	task := repoWorkerTask(types.MirrorQueued)
	args := repoArgsFromTask(task)
	job := riverJob(args)
	wantDelay := 2 * time.Minute

	err := runWorkWithLog(context.Background(), job, mirrorWorkConfig[workhub.RepoArgs]{
		args:      args.MirrorArgs,
		stage:     "repo",
		manager:   newWorkerTestManager(t),
		taskStore: &fakeRepoTaskStore{task: task},
		work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
			return task, river.JobSnooze(wantDelay)
		},
	})

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Equal(t, wantDelay, snoozeErr.Duration)
}

// TestRunWorkSnoozesTimedOutWorkImmediately verifies timeout recovery retains the current business status.
func TestRunWorkSnoozesTimedOutWorkImmediately(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
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

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Zero(t, snoozeErr.Duration)
}

// TestRunWorkSnoozesCancelledWork verifies worker shutdown retains the job for another worker.
func TestRunWorkSnoozesCancelledWork(t *testing.T) {
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

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Equal(t, workerShutdownSnoozeDelay, snoozeErr.Duration)
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
		cancel(workhub.ErrUrgentPreempt)
		require.True(t, isWorkContextTermination(ctx, context.Canceled))
		require.True(t, isWorkContextTermination(ctx, context.DeadlineExceeded))
		require.True(t, isWorkContextTermination(ctx, workhub.ErrUrgentPreempt))
	})

	t.Run("unrelated error after cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.False(t, isWorkContextTermination(ctx, errors.New("sync failed")))
	})
}

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

// TestRunMirrorWorkExecutesBusinessWork verifies the shared lifecycle delegates typed business work.
func TestRunMirrorWorkExecutesBusinessWork(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	task := repoWorkerTask(types.MirrorCanceled)
	args := repoArgsFromTask(task)
	job := riverJob(args)
	job.Queue = workhub.MirrorRepoQueue
	called := false

	err := runMirrorWork(context.Background(), job, mirrorWorkConfig[workhub.RepoArgs]{
		name:            "repo",
		preemptionDelay: time.Minute,
		isUrgent:        func(args workhub.RepoArgs) bool { return args.Urgent },
		expectedQueue:   workhub.RepoQueue,
		validateQueue:   workhub.ValidateRepoQueue,
		logArgs:         repoSlogArgs,
		work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
			called = true
			return task, nil
		},
	})

	require.NoError(t, err)
	require.True(t, called)
	logs := output.String()
	requireWorkerJobLogPair(t, logs, "working on repo job", "repo job work exited")
	requireSingleWorkerExitLog(t, logs, "repo job work exited", "INFO")
}

// TestRunMirrorWorkLogsPanicAsFailure verifies lifecycle logging preserves panic semantics.
func TestRunMirrorWorkLogsPanicAsFailure(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	task := repoWorkerTask(types.MirrorCanceled)
	args := repoArgsFromTask(task)
	job := riverJob(args)
	job.Queue = workhub.MirrorRepoQueue

	require.PanicsWithValue(t, "sync panic", func() {
		_ = runMirrorWork(context.Background(), job, mirrorWorkConfig[workhub.RepoArgs]{
			name:            "repo",
			preemptionDelay: time.Minute,
			isUrgent:        func(args workhub.RepoArgs) bool { return args.Urgent },
			expectedQueue:   workhub.RepoQueue,
			validateQueue:   workhub.ValidateRepoQueue,
			logArgs:         repoSlogArgs,
			work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
				panic("sync panic")
			},
		})
	})

	logs := output.String()
	requireWorkerJobLogPair(t, logs, "working on repo job", "repo job work exited")
	exitLog := requireSingleWorkerExitLog(t, logs, "repo job work exited", "ERROR")
	require.Contains(t, exitLog, `"panic":"sync panic"`)
	require.Contains(t, exitLog, `"success":false`)
	require.Contains(t, exitLog, `"snooze":false`)
}

// TestRunMirrorWorkFinalAttemptSnoozesWhenFatalStateCannotPersist verifies River cannot discard before task finalization succeeds.
func TestRunMirrorWorkFinalAttemptSnoozesWhenFatalStateCannotPersist(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	task := repoWorkerTask(types.MirrorRepoSyncFailed)
	task.RepoJobID = 1
	store := &fakeMirrorJobErrorTaskStore{task: task, updateErr: errors.New("database unavailable")}
	args := repoArgsFromTask(task)
	job := riverJob(args)
	job.Queue = workhub.MirrorRepoQueue
	job.Attempt = job.MaxAttempts
	workErr := errors.New("sync failed")

	err := runMirrorWork(context.Background(), job, mirrorWorkConfig[workhub.RepoArgs]{
		name:            "repo",
		preemptionDelay: time.Minute,
		isUrgent:        func(args workhub.RepoArgs) bool { return args.Urgent },
		expectedQueue:   workhub.RepoQueue,
		validateQueue:   workhub.ValidateRepoQueue,
		logArgs:         repoSlogArgs,
		work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
			return task, workErr
		},
		failureTarget:    mirrorRepoJobFailureTarget,
		failureFinalizer: newMirrorJobFailureFinalizer(store),
	})

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Equal(t, time.Minute, snoozeErr.Duration)
	require.Equal(t, 2, store.findCalls)
	require.Equal(t, []string{database.MirrorFatal}, store.actions)
	require.Contains(t, output.String(), `"reason":"terminal_state_persistence"`)
}

// TestRunMirrorWorkFinalAttemptReturnsOriginalErrorAfterFatalStatePersists verifies River may discard after task finalization.
func TestRunMirrorWorkFinalAttemptReturnsOriginalErrorAfterFatalStatePersists(t *testing.T) {
	task := repoWorkerTask(types.MirrorRepoSyncFailed)
	task.RepoJobID = 1
	store := &fakeMirrorJobErrorTaskStore{task: task}
	args := repoArgsFromTask(task)
	job := riverJob(args)
	job.Queue = workhub.MirrorRepoQueue
	job.Attempt = job.MaxAttempts
	workErr := errors.New("sync failed")

	err := runMirrorWork(context.Background(), job, mirrorWorkConfig[workhub.RepoArgs]{
		name:            "repo",
		preemptionDelay: time.Minute,
		isUrgent:        func(args workhub.RepoArgs) bool { return args.Urgent },
		expectedQueue:   workhub.RepoQueue,
		validateQueue:   workhub.ValidateRepoQueue,
		logArgs:         repoSlogArgs,
		work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
			return task, workErr
		},
		failureTarget:    mirrorRepoJobFailureTarget,
		failureFinalizer: newMirrorJobFailureFinalizer(store),
	})

	require.ErrorIs(t, err, workErr)
	require.Equal(t, types.MirrorRepoSyncFatal, store.task.Status)
	require.Equal(t, job.MaxAttempts-1, store.task.RetryCount)
}

// TestRunMirrorWorkRetryableAttemptSkipsFatalState verifies ordinary River retries retain their business state.
func TestRunMirrorWorkRetryableAttemptSkipsFatalState(t *testing.T) {
	task := repoWorkerTask(types.MirrorRepoSyncFailed)
	task.RepoJobID = 1
	store := &fakeMirrorJobErrorTaskStore{task: task}
	args := repoArgsFromTask(task)
	job := riverJob(args)
	job.Queue = workhub.MirrorRepoQueue
	job.Attempt = job.MaxAttempts - 1
	workErr := errors.New("sync failed")

	err := runMirrorWork(context.Background(), job, mirrorWorkConfig[workhub.RepoArgs]{
		name:            "repo",
		preemptionDelay: time.Minute,
		isUrgent:        func(args workhub.RepoArgs) bool { return args.Urgent },
		expectedQueue:   workhub.RepoQueue,
		validateQueue:   workhub.ValidateRepoQueue,
		logArgs:         repoSlogArgs,
		work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
			return task, workErr
		},
		failureTarget:    mirrorRepoJobFailureTarget,
		failureFinalizer: newMirrorJobFailureFinalizer(store),
	})

	require.ErrorIs(t, err, workErr)
	require.Zero(t, store.findCalls)
	require.Empty(t, store.actions)
}

// TestRunMirrorWorkFinalPanicSnoozesWhenFatalStateCannotPersist verifies recoverable panics retain a River execution opportunity.
func TestRunMirrorWorkFinalPanicSnoozesWhenFatalStateCannotPersist(t *testing.T) {
	task := repoWorkerTask(types.MirrorRepoSyncStart)
	task.RepoJobID = 1
	store := &fakeMirrorJobErrorTaskStore{task: task, updateErr: errors.New("database unavailable")}
	args := repoArgsFromTask(task)
	job := riverJob(args)
	job.Queue = workhub.MirrorRepoQueue
	job.Attempt = job.MaxAttempts

	err := runMirrorWork(context.Background(), job, mirrorWorkConfig[workhub.RepoArgs]{
		name:            "repo",
		preemptionDelay: time.Minute,
		isUrgent:        func(args workhub.RepoArgs) bool { return args.Urgent },
		expectedQueue:   workhub.RepoQueue,
		validateQueue:   workhub.ValidateRepoQueue,
		logArgs:         repoSlogArgs,
		work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
			panic("sync panic")
		},
		failureTarget:    mirrorRepoJobFailureTarget,
		failureFinalizer: newMirrorJobFailureFinalizer(store),
	})

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Equal(t, []string{database.MirrorFail}, store.actions)
}

// TestRunMirrorWorkFinalPanicRepanicsAfterFatalStatePersists verifies River records panic exhaustion after business finalization.
func TestRunMirrorWorkFinalPanicRepanicsAfterFatalStatePersists(t *testing.T) {
	task := repoWorkerTask(types.MirrorRepoSyncStart)
	task.RepoJobID = 1
	store := &fakeMirrorJobErrorTaskStore{task: task}
	args := repoArgsFromTask(task)
	job := riverJob(args)
	job.Queue = workhub.MirrorRepoQueue
	job.Attempt = job.MaxAttempts

	require.PanicsWithValue(t, "sync panic", func() {
		_ = runMirrorWork(context.Background(), job, mirrorWorkConfig[workhub.RepoArgs]{
			name:            "repo",
			preemptionDelay: time.Minute,
			isUrgent:        func(args workhub.RepoArgs) bool { return args.Urgent },
			expectedQueue:   workhub.RepoQueue,
			validateQueue:   workhub.ValidateRepoQueue,
			logArgs:         repoSlogArgs,
			work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
				panic("sync panic")
			},
			failureTarget:    mirrorRepoJobFailureTarget,
			failureFinalizer: newMirrorJobFailureFinalizer(store),
		})
	})

	require.Equal(t, types.MirrorRepoSyncFatal, store.task.Status)
	require.Equal(t, []string{database.MirrorFail, database.MirrorFatal}, store.actions)
}

// TestRunMirrorWorkFinalAttemptDoesNotFinalizeJobCancel verifies explicit cancellation remains outside retry exhaustion handling.
func TestRunMirrorWorkFinalAttemptDoesNotFinalizeJobCancel(t *testing.T) {
	task := repoWorkerTask(types.MirrorRepoSyncStart)
	task.RepoJobID = 1
	store := &fakeMirrorJobErrorTaskStore{task: task}
	args := repoArgsFromTask(task)
	job := riverJob(args)
	job.Queue = workhub.MirrorRepoQueue
	job.Attempt = job.MaxAttempts
	cancelCause := errors.New("queue mismatch")

	err := runMirrorWork(context.Background(), job, mirrorWorkConfig[workhub.RepoArgs]{
		name:            "repo",
		preemptionDelay: time.Minute,
		isUrgent:        func(args workhub.RepoArgs) bool { return args.Urgent },
		expectedQueue:   workhub.RepoQueue,
		validateQueue:   workhub.ValidateRepoQueue,
		logArgs:         repoSlogArgs,
		work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
			return task, river.JobCancel(cancelCause)
		},
		failureTarget:    mirrorRepoJobFailureTarget,
		failureFinalizer: newMirrorJobFailureFinalizer(store),
	})

	var cancelErr *river.JobCancelError
	require.ErrorAs(t, err, &cancelErr)
	require.Zero(t, store.findCalls)
	require.Empty(t, store.actions)
}

// TestRunMirrorWorkFinalAttemptSnoozesWhenTaskCannotLoad verifies finalization read failures also retain a River execution opportunity.
func TestRunMirrorWorkFinalAttemptSnoozesWhenTaskCannotLoad(t *testing.T) {
	task := repoWorkerTask(types.MirrorRepoSyncFailed)
	task.RepoJobID = 1
	store := &fakeMirrorJobErrorTaskStore{findErr: errors.New("database unavailable")}
	args := repoArgsFromTask(task)
	job := riverJob(args)
	job.Queue = workhub.MirrorRepoQueue
	job.Attempt = job.MaxAttempts

	err := runMirrorWork(context.Background(), job, mirrorWorkConfig[workhub.RepoArgs]{
		name:            "repo",
		preemptionDelay: time.Minute,
		isUrgent:        func(args workhub.RepoArgs) bool { return args.Urgent },
		expectedQueue:   workhub.RepoQueue,
		validateQueue:   workhub.ValidateRepoQueue,
		logArgs:         repoSlogArgs,
		work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
			return task, errors.New("sync failed")
		},
		failureTarget:    mirrorRepoJobFailureTarget,
		failureFinalizer: newMirrorJobFailureFinalizer(store),
	})

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Equal(t, 1, store.findCalls)
}

// TestRunMirrorWorkFinalAttemptSnoozesRiverCancellationWithoutFinalizing verifies worker shutdown cannot become a permanent task failure.
func TestRunMirrorWorkFinalAttemptSnoozesRiverCancellationWithoutFinalizing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	task := repoWorkerTask(types.MirrorRepoSyncStart)
	task.RepoJobID = 1
	store := &fakeMirrorJobErrorTaskStore{task: task}
	args := repoArgsFromTask(task)
	job := riverJob(args)
	job.Queue = workhub.MirrorRepoQueue
	job.Attempt = job.MaxAttempts

	err := runMirrorWork(ctx, job, mirrorWorkConfig[workhub.RepoArgs]{
		name:            "repo",
		preemptionDelay: time.Minute,
		isUrgent:        func(args workhub.RepoArgs) bool { return args.Urgent },
		expectedQueue:   workhub.RepoQueue,
		validateQueue:   workhub.ValidateRepoQueue,
		logArgs:         repoSlogArgs,
		work: func(context.Context, workhub.RepoArgs, int) (*database.MirrorTask, error) {
			return task, context.Canceled
		},
		failureTarget:    mirrorRepoJobFailureTarget,
		failureFinalizer: newMirrorJobFailureFinalizer(store),
	})

	var snoozeErr *river.JobSnoozeError
	require.ErrorAs(t, err, &snoozeErr)
	require.Equal(t, workerShutdownSnoozeDelay, snoozeErr.Duration)
	require.Zero(t, store.findCalls)
	require.Empty(t, store.actions)
}

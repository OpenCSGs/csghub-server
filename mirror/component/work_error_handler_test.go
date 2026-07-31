package component

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/types"
)

// fakeMirrorJobErrorTaskStore records exhausted-job task transitions.
type fakeMirrorJobErrorTaskStore struct {
	task        *database.MirrorTask
	findErr     error
	updateErr   error
	findCalls   int
	findIDs     []int64
	updateCalls int
	actions     []string
}

// FindByID returns the configured task.
func (s *fakeMirrorJobErrorTaskStore) FindByID(_ context.Context, ID int64) (*database.MirrorTask, error) {
	s.findCalls++
	s.findIDs = append(s.findIDs, ID)
	if s.findErr != nil {
		return nil, s.findErr
	}
	return s.task, nil
}

// UpdateStatusAndRepoSyncStatus records and applies supported FSM transitions.
func (s *fakeMirrorJobErrorTaskStore) UpdateStatusAndRepoSyncStatus(
	_ context.Context,
	task database.MirrorTask,
	action string,
) (database.MirrorTask, error) {
	s.updateCalls++
	s.actions = append(s.actions, action)
	if s.updateErr != nil {
		return task, s.updateErr
	}
	switch action {
	case database.MirrorContinue:
		switch task.Status {
		case types.MirrorQueued:
			task.Status = types.MirrorRepoSyncStart
		case types.MirrorRepoSyncFinished:
			task.Status = types.MirrorLfsSyncStart
		}
	case database.MirrorFail:
		switch task.Status {
		case types.MirrorRepoSyncStart:
			task.Status = types.MirrorRepoSyncFailed
		case types.MirrorLfsSyncStart:
			task.Status = types.MirrorLfsSyncFailed
		}
	case database.MirrorFatal:
		switch task.Status {
		case types.MirrorQueued, types.MirrorRepoSyncStart, types.MirrorRepoSyncFailed:
			task.Status = types.MirrorRepoSyncFatal
		case types.MirrorRepoSyncFinished, types.MirrorLfsSyncStart, types.MirrorLfsSyncFailed:
			task.Status = types.MirrorLfsSyncFatal
		}
	}
	s.task = &task
	return task, nil
}

// TestMirrorJobErrorHandlerWaitsForLastAttempt verifies retryable errors remain nonfatal.
func TestMirrorJobErrorHandlerWaitsForLastAttempt(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	store := &fakeMirrorJobErrorTaskStore{}
	job := mirrorErrorHandlerJob(t, 101, repoErrorHandlerArgs(11))
	job.Attempt = job.MaxAttempts - 1

	result := newMirrorJobErrorHandler(store).HandleError(context.Background(), job, errors.New("sync failed"))

	require.Nil(t, result)
	require.Zero(t, store.findCalls)
	require.Zero(t, store.updateCalls)
	require.Contains(t, output.String(), `"msg":"mirror job error"`)
}

// TestMirrorJobErrorHandlerMarksFailedTaskFatal verifies exhausted Repo and LFS errors submit MirrorFatal.
func TestMirrorJobErrorHandlerMarksFailedTaskFatal(t *testing.T) {
	tests := []struct {
		name       string
		job        *rivertype.JobRow
		task       *database.MirrorTask
		wantStatus types.MirrorTaskStatus
	}{
		{
			name: "repo",
			job:  mirrorErrorHandlerJob(t, 101, repoErrorHandlerArgs(11)),
			task: &database.MirrorTask{
				ID: 11, RepoJobID: 101, Status: types.MirrorRepoSyncFailed,
			},
			wantStatus: types.MirrorRepoSyncFatal,
		},
		{
			name: "lfs",
			job:  mirrorErrorHandlerJob(t, 202, lfsErrorHandlerArgs(22)),
			task: &database.MirrorTask{
				ID: 22, LFSJobID: 202, Status: types.MirrorLfsSyncFailed,
			},
			wantStatus: types.MirrorLfsSyncFatal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeMirrorJobErrorTaskStore{task: test.task}

			result := newMirrorJobErrorHandler(store).HandleError(
				context.Background(), test.job, errors.New("sync failed"),
			)

			require.Nil(t, result)
			require.Equal(t, []string{database.MirrorFatal}, store.actions)
			require.Equal(t, test.wantStatus, store.task.Status)
			require.Equal(t, "sync failed", store.task.ErrorMessage)
			require.Equal(t, test.job.Attempt, store.task.RetryCount)
			require.Equal(t, []int64{test.task.ID}, store.findIDs)
		})
	}
}

// TestMirrorJobErrorHandlerFinalizesActiveStatuses verifies infrastructure errors cannot strand active stages.
func TestMirrorJobErrorHandlerFinalizesActiveStatuses(t *testing.T) {
	tests := []struct {
		name        string
		job         *rivertype.JobRow
		task        *database.MirrorTask
		wantActions []string
		wantStatus  types.MirrorTaskStatus
	}{
		{
			name:        "queued repo",
			job:         mirrorErrorHandlerJob(t, 101, repoErrorHandlerArgs(11)),
			task:        &database.MirrorTask{ID: 11, RepoJobID: 101, Status: types.MirrorQueued},
			wantActions: []string{database.MirrorFatal},
			wantStatus:  types.MirrorRepoSyncFatal,
		},
		{
			name:        "running repo",
			job:         mirrorErrorHandlerJob(t, 101, repoErrorHandlerArgs(11)),
			task:        &database.MirrorTask{ID: 11, RepoJobID: 101, Status: types.MirrorRepoSyncStart},
			wantActions: []string{database.MirrorFatal},
			wantStatus:  types.MirrorRepoSyncFatal,
		},
		{
			name:        "waiting lfs",
			job:         mirrorErrorHandlerJob(t, 202, lfsErrorHandlerArgs(22)),
			task:        &database.MirrorTask{ID: 22, LFSJobID: 202, Status: types.MirrorRepoSyncFinished},
			wantActions: []string{database.MirrorFatal},
			wantStatus:  types.MirrorLfsSyncFatal,
		},
		{
			name:        "running lfs",
			job:         mirrorErrorHandlerJob(t, 202, lfsErrorHandlerArgs(22)),
			task:        &database.MirrorTask{ID: 22, LFSJobID: 202, Status: types.MirrorLfsSyncStart},
			wantActions: []string{database.MirrorFatal},
			wantStatus:  types.MirrorLfsSyncFatal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeMirrorJobErrorTaskStore{task: test.task}

			newMirrorJobErrorHandler(store).HandleError(
				context.Background(), test.job, errors.New("infrastructure failure"),
			)

			require.Equal(t, test.wantActions, store.actions)
			require.Equal(t, test.wantStatus, store.task.Status)
			require.Equal(t, "infrastructure failure", store.task.ErrorMessage)
		})
	}
}

// TestMirrorJobErrorHandlerHandlesFinalPanic verifies panic exhaustion uses the same fatal transition.
func TestMirrorJobErrorHandlerHandlesFinalPanic(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	store := &fakeMirrorJobErrorTaskStore{task: &database.MirrorTask{
		ID: 22, LFSJobID: 202, Status: types.MirrorLfsSyncStart,
	}}
	job := mirrorErrorHandlerJob(t, 202, lfsErrorHandlerArgs(22))

	result := newMirrorJobErrorHandler(store).HandlePanic(
		context.Background(), job, "sync panic", "panic trace",
	)

	require.Nil(t, result)
	require.Equal(t, []string{database.MirrorFatal}, store.actions)
	require.Equal(t, types.MirrorLfsSyncFatal, store.task.Status)
	require.Equal(t, "work panic", store.task.ErrorMessage)
	require.Equal(t, job.Attempt, store.task.RetryCount)
	require.Contains(t, output.String(), `"msg":"mirror job panic"`)
	require.Contains(t, output.String(), `"panic_trace":"panic trace"`)
}

// TestMirrorJobErrorHandlerSkipsStaleJob verifies exhausted old Repo and LFS jobs cannot finalize replacements.
func TestMirrorJobErrorHandlerSkipsStaleJob(t *testing.T) {
	tests := []struct {
		name string
		job  *rivertype.JobRow
		task *database.MirrorTask
	}{
		{
			name: "repo",
			job:  mirrorErrorHandlerJob(t, 101, repoErrorHandlerArgs(11)),
			task: &database.MirrorTask{ID: 11, RepoJobID: 999, Status: types.MirrorRepoSyncFailed},
		},
		{
			name: "lfs",
			job:  mirrorErrorHandlerJob(t, 202, lfsErrorHandlerArgs(22)),
			task: &database.MirrorTask{ID: 22, LFSJobID: 999, Status: types.MirrorLfsSyncFailed},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeMirrorJobErrorTaskStore{task: test.task}

			newMirrorJobErrorHandler(store).HandleError(
				context.Background(), test.job, errors.New("sync failed"),
			)

			require.Equal(t, 1, store.findCalls)
			require.Zero(t, store.updateCalls)
		})
	}
}

// TestMirrorJobErrorHandlerLogsFatalPersistenceFailure verifies handler failures remain observable.
func TestMirrorJobErrorHandlerLogsFatalPersistenceFailure(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	store := &fakeMirrorJobErrorTaskStore{
		task: &database.MirrorTask{
			ID: 11, RepoJobID: 101, Status: types.MirrorRepoSyncFailed,
		},
		updateErr: errors.New("database unavailable"),
	}

	newMirrorJobErrorHandler(store).HandleError(
		context.Background(),
		mirrorErrorHandlerJob(t, 101, repoErrorHandlerArgs(11)),
		errors.New("sync failed"),
	)

	require.Contains(t, output.String(), `"msg":"mirror job error"`)
	require.Contains(t, output.String(), `"handle_error":"update mirror job error: database unavailable"`)
	require.Contains(t, output.String(), "database unavailable")
}

// TestMirrorJobErrorHandlerIgnoresCancellation verifies shutdown and remote cancellation do not finalize tasks.
func TestMirrorJobErrorHandlerIgnoresCancellation(t *testing.T) {
	tests := []struct {
		name      string
		ctx       context.Context
		workError error
	}{
		{
			name: "worker shutdown",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			workError: context.Canceled,
		},
		{
			name:      "remote cancellation",
			ctx:       context.Background(),
			workError: river.ErrJobCancelledRemotely,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeMirrorJobErrorTaskStore{task: &database.MirrorTask{
				ID: 11, RepoJobID: 101, Status: types.MirrorRepoSyncStart,
			}}

			result := newMirrorJobErrorHandler(store).HandleError(
				test.ctx,
				mirrorErrorHandlerJob(t, 101, repoErrorHandlerArgs(11)),
				test.workError,
			)

			require.Nil(t, result)
			require.Zero(t, store.findCalls)
			require.Zero(t, store.updateCalls)
		})
	}
}

// repoErrorHandlerArgs creates Repo job arguments for error-handler tests.
func repoErrorHandlerArgs(taskID int64) workhub.RepoArgs {
	return workhub.RepoArgs{MirrorArgs: workhub.MirrorArgs{MirrorTaskID: taskID}}
}

// lfsErrorHandlerArgs creates LFS job arguments for error-handler tests.
func lfsErrorHandlerArgs(taskID int64) workhub.LFSArgs {
	return workhub.LFSArgs{MirrorArgs: workhub.MirrorArgs{MirrorTaskID: taskID}}
}

// mirrorErrorHandlerJob creates a final-attempt River row for one mirror job payload.
func mirrorErrorHandlerJob(t *testing.T, jobID int64, args workhub.JobArgs) *rivertype.JobRow {
	t.Helper()
	encodedArgs, err := json.Marshal(args)
	require.NoError(t, err)
	return &rivertype.JobRow{
		ID:          jobID,
		Attempt:     4,
		EncodedArgs: encodedArgs,
		Kind:        args.Kind(),
		MaxAttempts: 4,
	}
}

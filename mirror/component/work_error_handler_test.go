package component

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/types"
)

// fakeMirrorJobErrorTaskStore records final retry task transitions.
type fakeMirrorJobErrorTaskStore struct {
	task         *database.MirrorTask
	findErr      error
	updateResult *database.MirrorTask
	updateErr    error
	findCalls    int
	updateCalls  int
	actions      []string
}

// FindByID returns the configured mirror task.
func (s *fakeMirrorJobErrorTaskStore) FindByID(ctx context.Context, ID int64) (*database.MirrorTask, error) {
	s.findCalls++
	if s.findErr != nil {
		return nil, s.findErr
	}
	return s.task, nil
}

// UpdateStatusAndRepoSyncStatus records the fatal transition.
func (s *fakeMirrorJobErrorTaskStore) UpdateStatusAndRepoSyncStatus(
	ctx context.Context,
	task database.MirrorTask,
	statusAction string,
) (database.MirrorTask, error) {
	s.updateCalls++
	s.actions = append(s.actions, statusAction)
	if s.updateErr != nil {
		if s.updateResult != nil {
			return *s.updateResult, s.updateErr
		}
		return task, s.updateErr
	}
	switch statusAction {
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
		case types.MirrorRepoSyncFailed:
			task.Status = types.MirrorRepoSyncFatal
		case types.MirrorLfsSyncFailed:
			task.Status = types.MirrorLfsSyncFatal
		}
	}
	s.task = &task
	return task, nil
}

// TestMirrorJobErrorHandlerLogsTrustedStatusOnUpdateError verifies failed updates ignore their task result.
func TestMirrorJobErrorHandlerLogsTrustedStatusOnUpdateError(t *testing.T) {
	output := captureMirrorWorkerLogs(t)
	store := &fakeMirrorJobErrorTaskStore{
		task: &database.MirrorTask{
			ID:        11,
			RepoJobID: 101,
			Status:    types.MirrorRepoSyncFailed,
		},
		updateResult: &database.MirrorTask{Status: types.MirrorRepoSyncFatal},
		updateErr:    errors.New("update failed"),
	}
	handler := newMirrorJobErrorHandler(store)

	handler.HandleError(
		context.Background(),
		mirrorErrorHandlerJob(t, 101, workhub.RepoArgs{MirrorTaskID: 11}),
		errors.New("sync failed"),
	)

	require.Contains(t, output.String(), `"task_status":"repo_failed"`)
	require.NotContains(t, output.String(), `"task_status":"repo_fatal"`)
}

// TestMirrorJobFailureFinalizerTreatsMissingTaskAsFinished verifies deleted tasks cannot create an infinite snooze loop.
func TestMirrorJobFailureFinalizerTreatsMissingTaskAsFinished(t *testing.T) {
	store := &fakeMirrorJobErrorTaskStore{}
	finalizer := newMirrorJobFailureFinalizer(store)
	target := mirrorRepoJobFailureTarget(workhub.RepoArgs{MirrorTaskID: 11})

	err := finalizer.finalize(context.Background(), 101, workhub.MirrorRepoQueue, 4, target, "sync failed")

	require.NoError(t, err)
	require.Equal(t, 1, store.findCalls)
	require.Zero(t, store.updateCalls)
}

// TestMirrorJobErrorHandlerMarksFailedTaskFatal verifies both mirror stages finalize after their last attempt.
func TestMirrorJobErrorHandlerMarksFailedTaskFatal(t *testing.T) {
	tests := []struct {
		name        string
		job         *rivertype.JobRow
		task        *database.MirrorTask
		fatalStatus types.MirrorTaskStatus
	}{
		{
			name: "repo",
			job: mirrorErrorHandlerJob(t, 101, workhub.RepoArgs{
				MirrorTaskID: 11,
			}),
			task: &database.MirrorTask{
				ID:        11,
				RepoJobID: 101,
				Status:    types.MirrorRepoSyncFailed,
			},
			fatalStatus: types.MirrorRepoSyncFatal,
		},
		{
			name: "lfs",
			job: mirrorErrorHandlerJob(t, 202, workhub.LFSArgs{
				MirrorTaskID: 22,
			}),
			task: &database.MirrorTask{
				ID:       22,
				LFSJobID: 202,
				Status:   types.MirrorLfsSyncFailed,
			},
			fatalStatus: types.MirrorLfsSyncFatal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeMirrorJobErrorTaskStore{task: test.task}
			handler := newMirrorJobErrorHandler(store)

			result := handler.HandleError(context.Background(), test.job, errors.New("sync failed"))

			require.Nil(t, result)
			require.Equal(t, 1, store.findCalls)
			require.Equal(t, 1, store.updateCalls)
			require.Equal(t, []string{database.MirrorFatal}, store.actions)
			require.Equal(t, test.fatalStatus, store.task.Status)
			require.Equal(t, test.job.MaxAttempts-1, store.task.RetryCount)
		})
	}
}

// TestMirrorJobErrorHandlerWaitsForLastAttempt verifies retryable failures stay nonfatal.
func TestMirrorJobErrorHandlerWaitsForLastAttempt(t *testing.T) {
	job := mirrorErrorHandlerJob(t, 101, workhub.RepoArgs{MirrorTaskID: 11})
	job.Attempt = job.MaxAttempts - 1
	store := &fakeMirrorJobErrorTaskStore{}
	handler := newMirrorJobErrorHandler(store)

	result := handler.HandleError(context.Background(), job, errors.New("sync failed"))

	require.Nil(t, result)
	require.Zero(t, store.findCalls)
	require.Zero(t, store.updateCalls)
}

// TestMirrorJobErrorHandlerFinalizesActiveStageStatuses verifies infrastructure failures cannot strand nonterminal tasks.
func TestMirrorJobErrorHandlerFinalizesActiveStageStatuses(t *testing.T) {
	tests := []struct {
		name        string
		job         *rivertype.JobRow
		task        *database.MirrorTask
		wantActions []string
		wantStatus  types.MirrorTaskStatus
	}{
		{
			name:        "queued repo",
			job:         mirrorErrorHandlerJob(t, 101, workhub.RepoArgs{MirrorTaskID: 11}),
			task:        &database.MirrorTask{ID: 11, RepoJobID: 101, Status: types.MirrorQueued},
			wantActions: []string{database.MirrorContinue, database.MirrorFail, database.MirrorFatal},
			wantStatus:  types.MirrorRepoSyncFatal,
		},
		{
			name:        "running repo",
			job:         mirrorErrorHandlerJob(t, 101, workhub.RepoArgs{MirrorTaskID: 11}),
			task:        &database.MirrorTask{ID: 11, RepoJobID: 101, Status: types.MirrorRepoSyncStart},
			wantActions: []string{database.MirrorFail, database.MirrorFatal},
			wantStatus:  types.MirrorRepoSyncFatal,
		},
		{
			name:        "waiting LFS",
			job:         mirrorErrorHandlerJob(t, 202, workhub.LFSArgs{MirrorTaskID: 22}),
			task:        &database.MirrorTask{ID: 22, LFSJobID: 202, Status: types.MirrorRepoSyncFinished},
			wantActions: []string{database.MirrorContinue, database.MirrorFail, database.MirrorFatal},
			wantStatus:  types.MirrorLfsSyncFatal,
		},
		{
			name:        "running LFS",
			job:         mirrorErrorHandlerJob(t, 202, workhub.LFSArgs{MirrorTaskID: 22}),
			task:        &database.MirrorTask{ID: 22, LFSJobID: 202, Status: types.MirrorLfsSyncStart},
			wantActions: []string{database.MirrorFail, database.MirrorFatal},
			wantStatus:  types.MirrorLfsSyncFatal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeMirrorJobErrorTaskStore{task: test.task}
			handler := newMirrorJobErrorHandler(store)

			handler.HandleError(context.Background(), test.job, errors.New("infrastructure failure"))

			require.Equal(t, test.wantActions, store.actions)
			require.Equal(t, test.wantStatus, store.task.Status)
			require.Equal(t, "infrastructure failure", store.task.ErrorMessage)
		})
	}
}

// TestMirrorJobErrorHandlerRejectsStaleAndUnexpectedTasks verifies finalization only affects its active stage.
func TestMirrorJobErrorHandlerRejectsStaleAndUnexpectedTasks(t *testing.T) {
	tests := []struct {
		name string
		task *database.MirrorTask
	}{
		{
			name: "stale job",
			task: &database.MirrorTask{ID: 11, RepoJobID: 102, Status: types.MirrorRepoSyncFailed},
		},
		{
			name: "unexpected task status",
			task: &database.MirrorTask{ID: 11, RepoJobID: 101, Status: types.MirrorLfsSyncStart},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeMirrorJobErrorTaskStore{task: test.task}
			handler := newMirrorJobErrorHandler(store)

			handler.HandleError(
				context.Background(),
				mirrorErrorHandlerJob(t, 101, workhub.RepoArgs{MirrorTaskID: 11}),
				errors.New("sync failed"),
			)

			require.Equal(t, 1, store.findCalls)
			require.Zero(t, store.updateCalls)
		})
	}
}

// TestMirrorJobErrorHandlerHandlesFinalPanic verifies panic exhaustion finalizes a running task.
func TestMirrorJobErrorHandlerHandlesFinalPanic(t *testing.T) {
	store := &fakeMirrorJobErrorTaskStore{task: &database.MirrorTask{
		ID:       22,
		LFSJobID: 202,
		Status:   types.MirrorLfsSyncStart,
	}}
	handler := newMirrorJobErrorHandler(store)

	result := handler.HandlePanic(
		context.Background(),
		mirrorErrorHandlerJob(t, 202, workhub.LFSArgs{MirrorTaskID: 22}),
		"sync panic",
		"trace",
	)

	require.Nil(t, result)
	require.Equal(t, []string{database.MirrorFail, database.MirrorFatal}, store.actions)
	require.Equal(t, types.MirrorLfsSyncFatal, store.task.Status)
	require.Equal(t, "worker panic: sync panic", store.task.ErrorMessage)
}

// TestMirrorJobErrorHandlerIgnoresCancellation verifies worker shutdown does not create a permanent failure.
func TestMirrorJobErrorHandlerIgnoresCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := &fakeMirrorJobErrorTaskStore{task: &database.MirrorTask{
		ID:        11,
		RepoJobID: 101,
		Status:    types.MirrorRepoSyncStart,
	}}
	handler := newMirrorJobErrorHandler(store)

	result := handler.HandleError(
		ctx,
		mirrorErrorHandlerJob(t, 101, workhub.RepoArgs{MirrorTaskID: 11}),
		context.Canceled,
	)

	require.Nil(t, result)
	require.Zero(t, store.findCalls)
	require.Empty(t, store.actions)
}

// TestMirrorJobErrorHandlerFinalizesCancellationWithActiveContext verifies unexpected cancellation can still terminate.
func TestMirrorJobErrorHandlerFinalizesCancellationWithActiveContext(t *testing.T) {
	store := &fakeMirrorJobErrorTaskStore{task: &database.MirrorTask{
		ID:        11,
		RepoJobID: 101,
		Status:    types.MirrorRepoSyncStart,
	}}
	handler := newMirrorJobErrorHandler(store)

	result := handler.HandleError(
		context.Background(),
		mirrorErrorHandlerJob(t, 101, workhub.RepoArgs{MirrorTaskID: 11}),
		context.Canceled,
	)

	require.Nil(t, result)
	require.Equal(t, []string{database.MirrorFail, database.MirrorFatal}, store.actions)
	require.Equal(t, types.MirrorRepoSyncFatal, store.task.Status)
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

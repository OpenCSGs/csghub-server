package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// TestMirrorComponentListMirrorSyncs verifies database pagination and task-based attempt fields.
func TestMirrorComponentListMirrorSyncs(t *testing.T) {
	ctx := context.Background()
	mc := initializeTestMirrorComponent(ctx, t)
	mc.config.Mirror.MaxRetryCount = 3
	first := mirrorWithCurrentTask(types.MirrorRepoSyncStart, 11, 0)
	first.CurrentTask.Priority = types.HighMirrorPriority
	first.CurrentTask.IsUrgent = true
	second := mirrorWithCurrentTask(types.MirrorRepoSyncFinished, 12, 22)
	second.ID = 6
	second.CurrentTask.MirrorID = second.ID
	second.CurrentTask.ID = 8
	second.CurrentTaskID = second.CurrentTask.ID

	mc.mocks.stores.MirrorMock().EXPECT().IndexSyncWithPagination(ctx, database.MirrorSyncListQuery{
		Page: 1, Per: 2, Search: "source",
	}).Return([]database.Mirror{first, second}, 9, nil)

	result, err := mc.ListMirrorSyncs(ctx, types.MirrorSyncListReq{Page: 1, Per: 2, Search: "source"})

	require.NoError(t, err)
	require.Equal(t, 9, result.Total)
	require.Equal(t, 1, result.Page)
	require.Equal(t, 2, result.Per)
	require.Len(t, result.Items, 2)
	require.Equal(t, types.MirrorSyncOverallRunning, result.Items[0].Status)
	require.Zero(t, result.Items[0].RetryCount)
	require.Equal(t, 3, result.Items[0].MaxRetryCount)
	require.Equal(t, types.HighMirrorPriority, result.Items[0].Priority)
	require.True(t, result.Items[0].IsUrgent)
	require.Equal(t, types.MirrorSyncOverallWaiting, result.Items[1].Status)
	require.Zero(t, result.Items[1].RetryCount)
	require.Equal(t, 3, result.Items[1].MaxRetryCount)
}

// TestMirrorComponentListMirrorSyncsUsesTaskStatusFilter verifies filtering is delegated to the database.
func TestMirrorComponentListMirrorSyncsUsesTaskStatusFilter(t *testing.T) {
	ctx := context.Background()
	mc := initializeTestMirrorComponent(ctx, t)
	mc.config.Mirror.MaxRetryCount = 3
	mirror := mirrorWithCurrentTask(types.MirrorRepoSyncFailed, 11, 0)
	mirror.CurrentTask.RetryCount = 2
	query := database.MirrorSyncListQuery{
		Page: 1, Per: 10,
		Statuses: []types.MirrorTaskStatus{
			types.MirrorRepoSyncStart,
			types.MirrorRepoSyncFailed,
			types.MirrorLfsSyncStart,
			types.MirrorLfsSyncFailed,
		},
	}
	mc.mocks.stores.MirrorMock().EXPECT().IndexSyncWithPagination(ctx, query).
		Return([]database.Mirror{mirror}, 1, nil)

	result, err := mc.ListMirrorSyncs(ctx, types.MirrorSyncListReq{Status: types.MirrorSyncOverallRunning})

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, types.MirrorSyncOverallRunning, result.Items[0].Status)
	require.True(t, result.Items[0].Retrying)
	require.Equal(t, 2, result.Items[0].RetryCount)
	require.Equal(t, 3, result.Items[0].MaxRetryCount)
}

// TestMirrorComponentListMirrorSyncsReturnsEmptyPage verifies empty database pages remain empty.
func TestMirrorComponentListMirrorSyncsReturnsEmptyPage(t *testing.T) {
	ctx := context.Background()
	mc := initializeTestMirrorComponent(ctx, t)
	mc.mocks.stores.MirrorMock().EXPECT().IndexSyncWithPagination(ctx, database.MirrorSyncListQuery{
		Page: 1, Per: 10,
	}).Return(nil, 0, nil)

	result, err := mc.ListMirrorSyncs(ctx, types.MirrorSyncListReq{})

	require.NoError(t, err)
	require.Empty(t, result.Items)
	require.Zero(t, result.Total)
}

// TestMirrorComponentListMirrorSyncsRejectsInvalidStatus verifies unsupported status is rejected before querying stores.
func TestMirrorComponentListMirrorSyncsRejectsInvalidStatus(t *testing.T) {
	mc := &mirrorComponentImpl{}
	_, err := mc.ListMirrorSyncs(context.Background(), types.MirrorSyncListReq{Status: "unknown"})
	require.Error(t, err)
}

// TestResolveMirrorSyncStatus verifies every public state is derived only from mirror task status.
func TestResolveMirrorSyncStatus(t *testing.T) {
	tests := []struct {
		name           string
		mirror         database.Mirror
		wantStatus     types.MirrorSyncOverallStatus
		wantPhase      types.MirrorSyncPhase
		wantResult     types.MirrorSyncResult
		wantRetrying   bool
		wantRepoState  types.MirrorSyncStageState
		wantRepoResult types.MirrorSyncResult
		wantLFSState   types.MirrorSyncStageState
		wantLFSResult  types.MirrorSyncResult
	}{
		{
			name: "queued repo", mirror: mirrorWithCurrentTask(types.MirrorQueued, 11, 0),
			wantStatus: types.MirrorSyncOverallWaiting, wantPhase: types.MirrorSyncPhaseRepo,
			wantRepoState: types.MirrorSyncStageNotStarted, wantLFSState: types.MirrorSyncStageNotStarted,
		},
		{
			name: "running repo", mirror: mirrorWithCurrentTask(types.MirrorRepoSyncStart, 11, 0),
			wantStatus: types.MirrorSyncOverallRunning, wantPhase: types.MirrorSyncPhaseRepo,
			wantRepoState: types.MirrorSyncStageRunning, wantLFSState: types.MirrorSyncStageNotStarted,
		},
		{
			name: "retrying repo", mirror: mirrorWithCurrentTask(types.MirrorRepoSyncFailed, 11, 0),
			wantStatus: types.MirrorSyncOverallRunning, wantPhase: types.MirrorSyncPhaseRepo, wantRetrying: true,
			wantRepoState: types.MirrorSyncStageRunning, wantLFSState: types.MirrorSyncStageNotStarted,
		},
		{
			name: "failed repo", mirror: mirrorWithCurrentTask(types.MirrorRepoSyncFatal, 11, 0),
			wantStatus: types.MirrorSyncOverallFinished, wantPhase: types.MirrorSyncPhaseDone, wantResult: types.MirrorSyncResultFailed,
			wantRepoState: types.MirrorSyncStageFinished, wantRepoResult: types.MirrorSyncResultFailed, wantLFSState: types.MirrorSyncStageNotStarted,
		},
		{
			name: "waiting lfs", mirror: mirrorWithCurrentTask(types.MirrorRepoSyncFinished, 11, 22),
			wantStatus: types.MirrorSyncOverallWaiting, wantPhase: types.MirrorSyncPhaseLFS,
			wantRepoState: types.MirrorSyncStageFinished, wantRepoResult: types.MirrorSyncResultSuccess, wantLFSState: types.MirrorSyncStageNotStarted,
		},
		{
			name: "running lfs", mirror: mirrorWithCurrentTask(types.MirrorLfsSyncStart, 11, 22),
			wantStatus: types.MirrorSyncOverallRunning, wantPhase: types.MirrorSyncPhaseLFS,
			wantRepoState: types.MirrorSyncStageFinished, wantRepoResult: types.MirrorSyncResultSuccess, wantLFSState: types.MirrorSyncStageRunning,
		},
		{
			name: "retrying lfs", mirror: mirrorWithCurrentTask(types.MirrorLfsSyncFailed, 11, 22),
			wantStatus: types.MirrorSyncOverallRunning, wantPhase: types.MirrorSyncPhaseLFS, wantRetrying: true,
			wantRepoState: types.MirrorSyncStageFinished, wantRepoResult: types.MirrorSyncResultSuccess, wantLFSState: types.MirrorSyncStageRunning,
		},
		{
			name: "successful lfs", mirror: mirrorWithCurrentTask(types.MirrorLfsSyncFinished, 11, 22),
			wantStatus: types.MirrorSyncOverallFinished, wantPhase: types.MirrorSyncPhaseDone, wantResult: types.MirrorSyncResultSuccess,
			wantRepoState: types.MirrorSyncStageFinished, wantRepoResult: types.MirrorSyncResultSuccess,
			wantLFSState: types.MirrorSyncStageFinished, wantLFSResult: types.MirrorSyncResultSuccess,
		},
		{
			name: "failed lfs", mirror: mirrorWithCurrentTask(types.MirrorLfsSyncFatal, 11, 22),
			wantStatus: types.MirrorSyncOverallFinished, wantPhase: types.MirrorSyncPhaseDone, wantResult: types.MirrorSyncResultFailed,
			wantRepoState: types.MirrorSyncStageFinished, wantRepoResult: types.MirrorSyncResultSuccess,
			wantLFSState: types.MirrorSyncStageFinished, wantLFSResult: types.MirrorSyncResultFailed,
		},
		{
			name: "incomplete lfs", mirror: mirrorWithCurrentTask(types.MirrorLfsIncomplete, 11, 22),
			wantStatus: types.MirrorSyncOverallFinished, wantPhase: types.MirrorSyncPhaseDone, wantResult: types.MirrorSyncResultIncomplete,
			wantRepoState: types.MirrorSyncStageFinished, wantRepoResult: types.MirrorSyncResultSuccess,
			wantLFSState: types.MirrorSyncStageFinished, wantLFSResult: types.MirrorSyncResultIncomplete,
		},
		{
			name: "too large", mirror: mirrorWithCurrentTask(types.MirrorRepoTooLarge, 11, 22),
			wantStatus: types.MirrorSyncOverallFinished, wantPhase: types.MirrorSyncPhaseDone, wantResult: types.MirrorSyncResultTooLarge,
			wantRepoState: types.MirrorSyncStageFinished, wantRepoResult: types.MirrorSyncResultSuccess,
			wantLFSState: types.MirrorSyncStageFinished, wantLFSResult: types.MirrorSyncResultTooLarge,
		},
		{
			name: "cancel during repo", mirror: mirrorWithCurrentTask(types.MirrorCanceled, 11, 0),
			wantStatus: types.MirrorSyncOverallFinished, wantPhase: types.MirrorSyncPhaseDone, wantResult: types.MirrorSyncResultCancelled,
			wantRepoState: types.MirrorSyncStageFinished, wantRepoResult: types.MirrorSyncResultCancelled, wantLFSState: types.MirrorSyncStageNotStarted,
		},
		{
			name: "cancel during lfs", mirror: mirrorWithCurrentTask(types.MirrorCanceled, 11, 22),
			wantStatus: types.MirrorSyncOverallFinished, wantPhase: types.MirrorSyncPhaseDone, wantResult: types.MirrorSyncResultCancelled,
			wantRepoState: types.MirrorSyncStageFinished, wantRepoResult: types.MirrorSyncResultSuccess,
			wantLFSState: types.MirrorSyncStageFinished, wantLFSResult: types.MirrorSyncResultCancelled,
		},
		{
			name: "no current task", mirror: database.Mirror{ID: 7},
			wantStatus:    types.MirrorSyncOverallNoTask,
			wantRepoState: types.MirrorSyncStageNotStarted, wantLFSState: types.MirrorSyncStageNotStarted,
		},
		{
			name: "missing referenced current task", mirror: database.Mirror{ID: 7, CurrentTaskID: 9},
			wantStatus:    types.MirrorSyncOverallNoTask,
			wantRepoState: types.MirrorSyncStageNotStarted, wantLFSState: types.MirrorSyncStageNotStarted,
		},
		{
			name: "inconsistent loaded task",
			mirror: database.Mirror{
				ID: 7, CurrentTaskID: 9,
				CurrentTask: &database.MirrorTask{ID: 9, MirrorID: 8, Status: types.MirrorQueued},
			},
			wantStatus: types.MirrorSyncOverallFinished, wantPhase: types.MirrorSyncPhaseDone, wantResult: types.MirrorSyncResultStateInvalid,
			wantRepoState: types.MirrorSyncStageFinished, wantRepoResult: types.MirrorSyncResultStateInvalid,
			wantLFSState: types.MirrorSyncStageNotStarted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveMirrorSyncStatus(tt.mirror)
			require.Equal(t, tt.wantStatus, got.Status)
			require.Equal(t, tt.wantPhase, got.Phase)
			require.Equal(t, tt.wantResult, got.Result)
			require.Equal(t, tt.wantRetrying, got.Retrying)
			require.Equal(t, tt.wantRepoState, got.RepoStage.State)
			require.Equal(t, tt.wantRepoResult, got.RepoStage.Result)
			require.Equal(t, tt.wantLFSState, got.LFSStage.State)
			require.Equal(t, tt.wantLFSResult, got.LFSStage.Result)
		})
	}
}

// TestBuildMirrorSyncSummaryWithoutTask verifies task-owned fields remain empty when no task exists.
func TestBuildMirrorSyncSummaryWithoutTask(t *testing.T) {
	mirror := database.Mirror{
		ID: 7, RepositoryID: 5, Status: types.MirrorQueued, Progress: 88,
		Username: "source-user", AccessToken: "source-token",
	}
	summary := buildMirrorSyncSummary(mirror, resolveMirrorSyncStatus(database.Mirror{ID: 7}), 3)

	require.Zero(t, summary.TaskID)
	require.Zero(t, summary.Progress)
	require.Zero(t, summary.RetryCount)
	require.Equal(t, types.MirrorSyncOverallNoTask, summary.Status)
	require.Equal(t, "source-user", summary.Username)
	require.Equal(t, "sour********", summary.AccessToken)
}

// TestBuildMirrorSyncSummary verifies token masking and task retry counts.
func TestBuildMirrorSyncSummary(t *testing.T) {
	mirror := mirrorWithCurrentTask(types.MirrorRepoSyncFailed, 11, 0)
	mirror.SourceUrl = "https://example.com/models/ns/name.git"
	mirror.Username = "source-user"
	mirror.AccessToken = "source-token"
	mirror.CurrentTask.Priority = types.HighMirrorPriority
	mirror.CurrentTask.RetryCount = 2
	status := resolveMirrorSyncStatus(mirror)

	summary := buildMirrorSyncSummary(mirror, status, 3)

	require.Equal(t, "https://example.com/models/ns/name.git", summary.SourceURL)
	require.Equal(t, "sour********", summary.AccessToken)
	require.Equal(t, "models/ns/name", summary.RepoPath)
	require.Equal(t, 2, summary.RetryCount)
	require.Equal(t, 3, summary.MaxRetryCount)
}

// TestBuildMirrorSyncSummaryUsesTerminalTaskRetryCount verifies completed tasks retain final-stage retry counts.
func TestBuildMirrorSyncSummaryUsesTerminalTaskRetryCount(t *testing.T) {
	mirror := mirrorWithCurrentTask(types.MirrorLfsSyncFinished, 11, 22)
	mirror.CurrentTask.RetryCount = 2

	summary := buildMirrorSyncSummary(mirror, resolveMirrorSyncStatus(mirror), 3)

	require.Equal(t, 2, summary.RetryCount)
	require.Equal(t, 3, summary.MaxRetryCount)
}

// mirrorWithCurrentTask creates one relation-consistent mirror fixture.
func mirrorWithCurrentTask(status types.MirrorTaskStatus, repoJobID, lfsJobID int64) database.Mirror {
	task := &database.MirrorTask{
		ID: 9, MirrorID: 7, Status: status, RepoJobID: repoJobID, LFSJobID: lfsJobID,
	}
	return database.Mirror{
		ID: 7, RepositoryID: 5, CurrentTaskID: task.ID, CurrentTask: task,
		Repository: &database.Repository{ID: 5, Path: "ns/name", Name: "name", RepositoryType: types.ModelRepo},
	}
}

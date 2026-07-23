package database_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

// fakeMirrorLFSJobClient records LFS job enqueue attempts made inside a database transaction.
type fakeMirrorLFSJobClient struct {
	err    error
	inputs []database.MirrorLFSJobInput
}

// InsertMirrorLFSJobTx records the transactional LFS job input.
func (c *fakeMirrorLFSJobClient) InsertMirrorLFSJobTx(ctx context.Context, tx *sql.Tx, input database.MirrorLFSJobInput) (int64, error) {
	c.inputs = append(c.inputs, input)
	if c.err != nil {
		return 0, c.err
	}
	return 456, nil
}

// fakeMirrorRepoJobClient records repository job enqueue attempts made inside a database transaction.
type fakeMirrorRepoJobClient struct {
	err    error
	inputs []database.MirrorJobInput
}

// InsertMirrorRepoJobTx records the transactional repo job input.
func (c *fakeMirrorRepoJobClient) InsertMirrorRepoJobTx(ctx context.Context, tx *sql.Tx, input database.MirrorJobInput) (int64, error) {
	c.inputs = append(c.inputs, input)
	if c.err != nil {
		return 0, c.err
	}
	return 789, nil
}

// fakeMirrorJobCancelClient records River job cancellation attempts made inside a database transaction.
type fakeMirrorJobCancelClient struct {
	err    error
	jobIDs []int64
}

// JobCancelTx records the job ID cancelled inside the current transaction.
func (c *fakeMirrorJobCancelClient) JobCancelTx(ctx context.Context, tx *sql.Tx, jobID int64) error {
	c.jobIDs = append(c.jobIDs, jobID)
	return c.err
}

func TestMirrorTaskStore_RequeueMirrorRepoTaskCreatesRepoJobInTx(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)
	jobClient := &fakeMirrorRepoJobClient{}
	cancelClient := &fakeMirrorJobCancelClient{}

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:         1,
		Path:           "test/requeue",
		GitPath:        "models_test/requeue",
		Name:           "requeue",
		Nickname:       "Requeue",
		DefaultBranch:  "main",
		Private:        false,
		RepositoryType: types.ModelRepo,
		SyncStatus:     types.SyncStatusCompleted,
	})
	require.Nil(t, err)

	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:      "https://example.com/test/requeue.git",
		RepositoryID:   repo.ID,
		MirrorSourceID: 1,
		Status:         types.MirrorLfsSyncStart,
		Priority:       types.LowMirrorPriority,
	})
	require.Nil(t, err)
	oldUpdatedAt := time.Now().Add(-time.Hour)
	_, err = db.Core.NewUpdate().
		Model((*database.Mirror)(nil)).
		Set("updated_at = ?", oldUpdatedAt).
		Where("id = ?", mirror.ID).
		Exec(ctx)
	require.Nil(t, err)

	oldTask, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID:  mirror.ID,
		Status:    types.MirrorLfsSyncStart,
		Priority:  types.LowMirrorPriority,
		RepoJobID: 101,
		LFSJobID:  202,
	})
	require.Nil(t, err)
	_, err = db.Core.NewUpdate().
		Model(&database.Mirror{}).
		Set("current_task_id = ?", oldTask.ID).
		Where("id = ?", mirror.ID).
		Exec(ctx)
	require.Nil(t, err)
	username := "new-user"
	accessToken := "new-token"

	task, err := taskStore.RequeueMirrorRepoTask(ctx, database.RequeueMirrorRepoTaskInput{
		MirrorID:        mirror.ID,
		RepositoryID:    repo.ID,
		Username:        &username,
		AccessToken:     &accessToken,
		Priority:        types.ASAPMirrorPriority,
		Urgent:          true,
		JobClient:       jobClient,
		JobCancelClient: cancelClient,
	})

	require.Nil(t, err)
	require.Equal(t, types.MirrorQueued, task.Status)
	require.Equal(t, types.ASAPMirrorPriority, task.Priority)
	require.Equal(t, int64(789), task.RepoJobID)
	require.True(t, task.IsUrgent)
	require.Equal(t, []int64{int64(101), int64(202)}, cancelClient.jobIDs)
	require.Len(t, jobClient.inputs, 1)
	require.Equal(t, database.MirrorJobInput{
		MirrorID:     mirror.ID,
		RepositoryID: repo.ID,
		MirrorTaskID: task.ID,
		RepoType:     types.ModelRepo,
		SourceURL:    "https://example.com/test/requeue.git",
		RepoPath:     "test/requeue",
		Priority:     types.ASAPMirrorPriority,
		Urgent:       true,
	}, jobClient.inputs[0])

	var storedOldTask database.MirrorTask
	err = db.Core.NewSelect().Model(&storedOldTask).Where("id = ?", oldTask.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorCanceled, storedOldTask.Status)

	var storedNewTask database.MirrorTask
	err = db.Core.NewSelect().Model(&storedNewTask).Where("id = ?", task.ID).Scan(ctx)
	require.Nil(t, err)
	require.True(t, storedNewTask.IsUrgent)

	var storedMirror database.Mirror
	err = db.Core.NewSelect().Model(&storedMirror).Where("id = ?", mirror.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorQueued, storedMirror.Status)
	require.Equal(t, task.ID, storedMirror.CurrentTaskID)
	require.Equal(t, types.ASAPMirrorPriority, storedMirror.Priority)
	require.Equal(t, username, storedMirror.Username)
	require.Equal(t, accessToken, storedMirror.AccessToken)
	require.True(t, storedMirror.UpdatedAt.After(oldUpdatedAt))

	var storedRepo database.Repository
	err = db.Core.NewSelect().Model(&storedRepo).Where("id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.SyncStatusPending, storedRepo.SyncStatus)
}

func TestMirrorTaskStore_RequeueMirrorRepoTaskRollsBackWhenJobInsertFails(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)
	jobClient := &fakeMirrorRepoJobClient{err: errors.New("insert job failed")}

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:         1,
		Path:           "test/requeue-rollback",
		GitPath:        "models_test/requeue-rollback",
		Name:           "requeue-rollback",
		Nickname:       "Requeue Rollback",
		DefaultBranch:  "main",
		Private:        false,
		RepositoryType: types.ModelRepo,
		SyncStatus:     types.SyncStatusCompleted,
	})
	require.Nil(t, err)

	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:      "https://example.com/test/requeue-rollback.git",
		Username:       "old-user",
		AccessToken:    "old-token",
		RepositoryID:   repo.ID,
		MirrorSourceID: 1,
		Status:         types.MirrorLfsSyncFinished,
		Priority:       types.LowMirrorPriority,
	})
	require.Nil(t, err)
	username := "new-user"
	accessToken := "new-token"

	_, err = taskStore.RequeueMirrorRepoTask(ctx, database.RequeueMirrorRepoTaskInput{
		MirrorID:     mirror.ID,
		RepositoryID: repo.ID,
		Username:     &username,
		AccessToken:  &accessToken,
		Priority:     types.ASAPMirrorPriority,
		JobClient:    jobClient,
	})

	require.ErrorContains(t, err, "insert job failed")
	var storedMirror database.Mirror
	err = db.Core.NewSelect().Model(&storedMirror).Where("id = ?", mirror.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorLfsSyncFinished, storedMirror.Status)
	require.Equal(t, types.LowMirrorPriority, storedMirror.Priority)
	require.Equal(t, "old-user", storedMirror.Username)
	require.Equal(t, "old-token", storedMirror.AccessToken)

	var taskCount int
	taskCount, err = db.Core.NewSelect().
		Model((*database.MirrorTask)(nil)).
		Where("mirror_id = ?", mirror.ID).
		Count(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, taskCount)
}

func TestMirrorTaskStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorTaskStoreWithDB(db)

	m, err := store.Create(ctx, database.MirrorTask{
		MirrorID: 1,
		Status:   types.MirrorQueued,
		Priority: types.LowMirrorPriority,
	})
	require.Nil(t, err)

	var mt database.MirrorTask
	err = db.Core.NewSelect().Model(&mt).Where("id = ?", m.ID).Scan(ctx)
	require.Nil(t, err)

	require.Equal(t, m.ID, mt.ID)
	require.Equal(t, int64(1), mt.MirrorID)
	require.Equal(t, types.MirrorQueued, mt.Status)
	require.Equal(t, types.LowMirrorPriority, mt.Priority)

	m1, err := store.Update(ctx, database.MirrorTask{
		ID:       m.ID,
		MirrorID: 1,
		Status:   types.MirrorQueued,
		Priority: types.LowMirrorPriority,
	})
	require.Nil(t, err)

	var mt1 database.MirrorTask
	err = db.Core.NewSelect().Model(&mt1).Where("id = ?", m.ID).Scan(ctx)
	require.Nil(t, err)

	require.Equal(t, types.MirrorQueued, m1.Status)
	require.Equal(t, types.LowMirrorPriority, m1.Priority)
	require.Equal(t, types.MirrorQueued, mt1.Status)
	require.Equal(t, types.LowMirrorPriority, mt1.Priority)

	m2, err := store.FindByMirrorID(ctx, 1)
	require.Nil(t, err)
	require.Equal(t, m.ID, m2.ID)
	require.Equal(t, int64(1), m2.MirrorID)
	require.Equal(t, types.MirrorQueued, m2.Status)
	require.Equal(t, types.LowMirrorPriority, m2.Priority)

	err = store.Delete(ctx, m.ID)
	require.Nil(t, err)

	err = db.Core.NewSelect().Model(&mt).Where("id = ?", m.ID).Scan(ctx)
	require.NotNil(t, err)
	require.Equal(t, "sql: no rows in result set", err.Error())
}

func TestMirrorTaskStore_SetMirrorCurrentTaskID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorTaskStoreWithDB(db)
	mstore := database.NewMirrorStoreWithDB(db)

	mirror, err := mstore.Create(ctx, &database.Mirror{
		SourceUrl:      "test",
		RepositoryID:   1,
		MirrorSourceID: 1,
	})
	require.Nil(t, err)

	mt, err := store.Create(ctx, database.MirrorTask{
		MirrorID: mirror.ID,
		Status:   types.MirrorQueued,
		Priority: types.LowMirrorPriority,
	})
	require.Nil(t, err)

	err = store.SetMirrorCurrentTaskID(ctx, mt)
	require.Nil(t, err)

	var m database.Mirror

	err = db.Operator.Core.NewSelect().Model(&m).Where("id = ?", mirror.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, mt.ID, m.CurrentTaskID)
}

func TestMirrorTaskStore_UpdateStatusAndRepoSyncStatus(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:        1,
		Path:          "test/repo",
		GitPath:       "test/repo.git",
		Name:          "repo",
		Nickname:      "Test Repo",
		DefaultBranch: "main",
		Private:       false,
		SyncStatus:    types.SyncStatusPending,
	})
	require.Nil(t, err)

	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:      "https://example.com/test/repo.git",
		RepositoryID:   repo.ID,
		MirrorSourceID: 1,
	})
	require.Nil(t, err)

	task, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID: mirror.ID,
		Status:   types.MirrorRepoSyncStart,
		Priority: types.LowMirrorPriority,
		Mirror: &database.Mirror{
			RepositoryID: repo.ID,
		},
	})
	require.Nil(t, err)

	updatedTask, err := taskStore.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorSuccess)
	require.Nil(t, err)
	require.Equal(t, types.MirrorRepoSyncFinished, updatedTask.Status)

	var updatedRepo database.Repository
	err = db.Core.NewSelect().Model(&updatedRepo).Where("id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.SyncStatusInProgress, updatedRepo.SyncStatus)

	var updatedTaskFromDB database.MirrorTask
	err = db.Core.NewSelect().Model(&updatedTaskFromDB).Where("id = ?", task.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorRepoSyncFinished, updatedTaskFromDB.Status)
}

func TestMirrorTaskStore_UpdateStatusAndRepoSyncStatus_MultipleSyncStatuses(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:        1,
		Path:          "test/repo2",
		GitPath:       "test/repo2.git",
		Name:          "repo2",
		Nickname:      "Test Repo 2",
		DefaultBranch: "main",
		Private:       false,
		SyncStatus:    types.SyncStatusPending,
	})
	require.Nil(t, err)

	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:      "https://example.com/test/repo2.git",
		RepositoryID:   repo.ID,
		MirrorSourceID: 1,
	})
	require.Nil(t, err)

	testCases := []struct {
		name           string
		initialStatus  types.MirrorTaskStatus
		action         string
		expectedStatus types.MirrorTaskStatus
		expectedSync   types.RepositorySyncStatus
	}{
		{"continue from queued", types.MirrorQueued, database.MirrorContinue, types.MirrorRepoSyncStart, types.SyncStatusInProgress},
		{"success from repo_sync_start", types.MirrorRepoSyncStart, database.MirrorSuccess, types.MirrorRepoSyncFinished, types.SyncStatusInProgress},
		{"continue from repo_sync_finished", types.MirrorRepoSyncFinished, database.MirrorContinue, types.MirrorLfsSyncStart, types.SyncStatusInProgress},
		{"retry from lfs_sync_failed", types.MirrorLfsSyncFailed, database.MirrorRetry, types.MirrorRepoSyncFinished, types.SyncStatusInProgress},
		{"success from lfs_sync_start", types.MirrorLfsSyncStart, database.MirrorSuccess, types.MirrorLfsSyncFinished, types.SyncStatusCompleted},
		{"too large from lfs_sync_start", types.MirrorLfsSyncStart, database.MirrorTooLarge, types.MirrorRepoTooLarge, types.SyncStatusFailed},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			task, err := taskStore.Create(ctx, database.MirrorTask{
				MirrorID: mirror.ID,
				Status:   tc.initialStatus,
				Priority: types.HighMirrorPriority,
				Mirror: &database.Mirror{
					RepositoryID: repo.ID,
				},
			})
			require.Nil(t, err)

			_, err = db.Core.NewUpdate().
				Model(&database.Mirror{}).
				Set("current_task_id = ?", task.ID).
				Where("id = ?", mirror.ID).
				Exec(ctx)
			require.Nil(t, err)

			updatedTask, err := taskStore.UpdateStatusAndRepoSyncStatus(ctx, task, tc.action)
			require.Nil(t, err)
			require.Equal(t, tc.expectedStatus, updatedTask.Status)

			var updatedRepo database.Repository
			err = db.Core.NewSelect().Model(&updatedRepo).Where("id = ?", repo.ID).Scan(ctx)
			require.Nil(t, err)
			require.Equal(t, tc.expectedSync, updatedRepo.SyncStatus)
		})
	}
}

func TestMirrorTaskStore_UpdateStatusAndRepoSyncStatus_FailedStatus(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:        1,
		Path:          "test/repo3",
		GitPath:       "test/repo3.git",
		Name:          "repo3",
		Nickname:      "Test Repo 3",
		DefaultBranch: "main",
		Private:       false,
		SyncStatus:    types.SyncStatusInProgress,
	})
	require.Nil(t, err)

	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:      "https://example.com/test/repo3.git",
		RepositoryID:   repo.ID,
		MirrorSourceID: 1,
	})
	require.Nil(t, err)

	task, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID:     mirror.ID,
		Status:       types.MirrorRepoSyncStart,
		Priority:     types.HighMirrorPriority,
		ErrorMessage: "sync failed",
		Mirror: &database.Mirror{
			RepositoryID: repo.ID,
		},
	})
	require.Nil(t, err)

	updatedTask, err := taskStore.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorFail)
	require.Nil(t, err)
	require.Equal(t, types.MirrorRepoSyncFailed, updatedTask.Status)

	var updatedRepo database.Repository
	err = db.Core.NewSelect().Model(&updatedRepo).Where("id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.SyncStatusFailed, updatedRepo.SyncStatus)
}

func TestMirrorTaskStore_UpdateStatusAndRepoSyncStatusRejectsStaleCurrentTask(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:        1,
		Path:          "test/stale-current",
		GitPath:       "test/stale-current.git",
		Name:          "stale-current",
		Nickname:      "Stale Current",
		DefaultBranch: "main",
		Private:       false,
		SyncStatus:    types.SyncStatusInProgress,
	})
	require.Nil(t, err)

	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:      "https://example.com/test/stale-current.git",
		RepositoryID:   repo.ID,
		MirrorSourceID: 1,
		Status:         types.MirrorRepoSyncStart,
	})
	require.Nil(t, err)

	oldTask, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID: mirror.ID,
		Status:   types.MirrorRepoSyncStart,
		Priority: types.HighMirrorPriority,
	})
	require.Nil(t, err)
	newTask, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID: mirror.ID,
		Status:   types.MirrorQueued,
		Priority: types.HighMirrorPriority,
	})
	require.Nil(t, err)

	_, err = db.Core.NewUpdate().
		Model(&database.Mirror{}).
		Set("current_task_id = ?", newTask.ID).
		Where("id = ?", mirror.ID).
		Exec(ctx)
	require.Nil(t, err)

	_, err = taskStore.UpdateStatusAndRepoSyncStatus(ctx, oldTask, database.MirrorSuccess)
	require.ErrorContains(t, err, "current task")

	var storedMirror database.Mirror
	err = db.Core.NewSelect().Model(&storedMirror).Where("id = ?", mirror.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, newTask.ID, storedMirror.CurrentTaskID)

	var storedTask database.MirrorTask
	err = db.Core.NewSelect().Model(&storedTask).Where("id = ?", oldTask.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorRepoSyncStart, storedTask.Status)
}

// TestMirrorTaskStore_UpdateStatusAndRepoSyncStatusRejectsCanceledTask verifies stale workers cannot overwrite a canceled task.
func TestMirrorTaskStore_UpdateStatusAndRepoSyncStatusRejectsCanceledTask(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:        1,
		Path:          "test/canceled-race",
		GitPath:       "test/canceled-race.git",
		Name:          "canceled-race",
		Nickname:      "Canceled Race",
		DefaultBranch: "main",
		Private:       false,
		SyncStatus:    types.SyncStatusInProgress,
	})
	require.Nil(t, err)

	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:      "https://example.com/test/canceled-race.git",
		RepositoryID:   repo.ID,
		MirrorSourceID: 1,
		Status:         types.MirrorRepoSyncStart,
	})
	require.Nil(t, err)

	runningTask, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID: mirror.ID,
		Status:   types.MirrorRepoSyncStart,
		Priority: types.HighMirrorPriority,
	})
	require.Nil(t, err)
	_, err = db.Core.NewUpdate().
		Model(&database.Mirror{}).
		Set("current_task_id = ?", runningTask.ID).
		Where("id = ?", mirror.ID).
		Exec(ctx)
	require.Nil(t, err)

	cancelled, err := taskStore.CancelMirrorTaskByIDWithJobCancel(ctx, runningTask.ID, nil)
	require.Nil(t, err)
	require.True(t, cancelled)

	// Simulate a worker that started before cancellation and now tries to
	// write a successful result with the stale in-memory running task.
	_, err = taskStore.UpdateStatusAndRepoSyncStatus(ctx, runningTask, database.MirrorSuccess)
	require.ErrorContains(t, err, "not allow action")

	var storedTask database.MirrorTask
	err = db.Core.NewSelect().Model(&storedTask).Where("id = ?", runningTask.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorCanceled, storedTask.Status)

	var storedMirror database.Mirror
	err = db.Core.NewSelect().Model(&storedMirror).Where("id = ?", mirror.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorCanceled, storedMirror.Status)

	var storedRepo database.Repository
	err = db.Core.NewSelect().Model(&storedRepo).Where("id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.SyncStatusCanceled, storedRepo.SyncStatus)
}

func TestMirrorTaskStore_CancelMirrorTaskByIDWithJobCancelSynchronizesMirrorAndRepo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:        1,
		Path:          "test/cancel-sync",
		GitPath:       "test/cancel-sync.git",
		Name:          "cancel-sync",
		Nickname:      "Cancel Sync",
		DefaultBranch: "main",
		Private:       false,
		SyncStatus:    types.SyncStatusInProgress,
	})
	require.Nil(t, err)

	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:      "https://example.com/test/cancel-sync.git",
		RepositoryID:   repo.ID,
		MirrorSourceID: 1,
		Status:         types.MirrorRepoSyncStart,
	})
	require.Nil(t, err)

	task, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID: mirror.ID,
		Status:   types.MirrorRepoSyncStart,
		Priority: types.HighMirrorPriority,
	})
	require.Nil(t, err)

	_, err = db.Core.NewUpdate().
		Model(&database.Mirror{}).
		Set("current_task_id = ?", task.ID).
		Where("id = ?", mirror.ID).
		Exec(ctx)
	require.Nil(t, err)

	cancelled, err := taskStore.CancelMirrorTaskByIDWithJobCancel(ctx, task.ID, nil)
	require.Nil(t, err)
	require.True(t, cancelled)

	var storedTask database.MirrorTask
	err = db.Core.NewSelect().Model(&storedTask).Where("id = ?", task.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorCanceled, storedTask.Status)

	var storedMirror database.Mirror
	err = db.Core.NewSelect().Model(&storedMirror).Where("id = ?", mirror.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorCanceled, storedMirror.Status)
	require.Equal(t, task.ID, storedMirror.CurrentTaskID)

	var storedRepo database.Repository
	err = db.Core.NewSelect().Model(&storedRepo).Where("id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.SyncStatusCanceled, storedRepo.SyncStatus)
}

// TestMirrorTaskStore_CancelMirrorTaskByIDWithJobCancelKeepsFinishedTask verifies cancel cannot overwrite completed sync results.
func TestMirrorTaskStore_CancelMirrorTaskByIDWithJobCancelKeepsFinishedTask(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:        1,
		Path:          "test/cancel-finished",
		GitPath:       "test/cancel-finished.git",
		Name:          "cancel-finished",
		Nickname:      "Cancel Finished",
		DefaultBranch: "main",
		Private:       false,
		SyncStatus:    types.SyncStatusCompleted,
	})
	require.Nil(t, err)

	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:      "https://example.com/test/cancel-finished.git",
		RepositoryID:   repo.ID,
		MirrorSourceID: 1,
		Status:         types.MirrorLfsSyncFinished,
	})
	require.Nil(t, err)

	task, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID: mirror.ID,
		Status:   types.MirrorLfsSyncFinished,
		Priority: types.HighMirrorPriority,
	})
	require.Nil(t, err)

	_, err = db.Core.NewUpdate().
		Model(&database.Mirror{}).
		Set("current_task_id = ?", task.ID).
		Where("id = ?", mirror.ID).
		Exec(ctx)
	require.Nil(t, err)

	cancelled, err := taskStore.CancelMirrorTaskByIDWithJobCancel(ctx, task.ID, nil)
	require.Nil(t, err)
	require.False(t, cancelled)

	var storedTask database.MirrorTask
	err = db.Core.NewSelect().Model(&storedTask).Where("id = ?", task.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorLfsSyncFinished, storedTask.Status)

	var storedMirror database.Mirror
	err = db.Core.NewSelect().Model(&storedMirror).Where("id = ?", mirror.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorLfsSyncFinished, storedMirror.Status)
	require.Equal(t, task.ID, storedMirror.CurrentTaskID)

	var storedRepo database.Repository
	err = db.Core.NewSelect().Model(&storedRepo).Where("id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.SyncStatusCompleted, storedRepo.SyncStatus)
}

func TestMirrorTaskStore_CancelMirrorTaskByIDCancelsRiverJobsInTx(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)
	cancelClient := &fakeMirrorJobCancelClient{}

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:        1,
		Path:          "test/cancel-jobs",
		GitPath:       "test/cancel-jobs.git",
		Name:          "cancel-jobs",
		Nickname:      "Cancel Jobs",
		DefaultBranch: "main",
		Private:       false,
		SyncStatus:    types.SyncStatusInProgress,
	})
	require.Nil(t, err)

	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:      "https://example.com/test/cancel-jobs.git",
		RepositoryID:   repo.ID,
		MirrorSourceID: 1,
		Status:         types.MirrorLfsSyncStart,
	})
	require.Nil(t, err)

	task, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID:  mirror.ID,
		Status:    types.MirrorLfsSyncStart,
		Priority:  types.HighMirrorPriority,
		RepoJobID: 101,
		LFSJobID:  202,
	})
	require.Nil(t, err)
	_, err = db.Core.NewUpdate().
		Model(&database.Mirror{}).
		Set("current_task_id = ?", task.ID).
		Where("id = ?", mirror.ID).
		Exec(ctx)
	require.Nil(t, err)

	cancelled, err := taskStore.CancelMirrorTaskByIDWithJobCancel(ctx, task.ID, cancelClient)

	require.Nil(t, err)
	require.True(t, cancelled)
	require.Equal(t, []int64{int64(101), int64(202)}, cancelClient.jobIDs)
}

func TestMirrorTaskStore_CancelMirrorTaskByIDRollsBackWhenRiverCancelFails(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)
	cancelClient := &fakeMirrorJobCancelClient{err: errors.New("river cancel failed")}

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:        1,
		Path:          "test/cancel-rollback",
		GitPath:       "test/cancel-rollback.git",
		Name:          "cancel-rollback",
		Nickname:      "Cancel Rollback",
		DefaultBranch: "main",
		Private:       false,
		SyncStatus:    types.SyncStatusInProgress,
	})
	require.Nil(t, err)

	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:      "https://example.com/test/cancel-rollback.git",
		RepositoryID:   repo.ID,
		MirrorSourceID: 1,
		Status:         types.MirrorRepoSyncStart,
	})
	require.Nil(t, err)

	task, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID:  mirror.ID,
		Status:    types.MirrorRepoSyncStart,
		Priority:  types.HighMirrorPriority,
		RepoJobID: 303,
	})
	require.Nil(t, err)
	_, err = db.Core.NewUpdate().
		Model(&database.Mirror{}).
		Set("current_task_id = ?", task.ID).
		Where("id = ?", mirror.ID).
		Exec(ctx)
	require.Nil(t, err)

	cancelled, err := taskStore.CancelMirrorTaskByIDWithJobCancel(ctx, task.ID, cancelClient)

	require.ErrorContains(t, err, "river cancel failed")
	require.False(t, cancelled)
	var storedTask database.MirrorTask
	err = db.Core.NewSelect().Model(&storedTask).Where("id = ?", task.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorRepoSyncStart, storedTask.Status)
}

func TestMirrorTaskStore_UpdateStatusAndRepoSyncStatus_UpdatesMirrorAndRepoFromStoredRelations(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:        1,
		Path:          "test/repo4",
		GitPath:       "test/repo4.git",
		Name:          "repo4",
		Nickname:      "Test Repo 4",
		DefaultBranch: "main",
		Private:       false,
		SyncStatus:    types.SyncStatusPending,
	})
	require.Nil(t, err)

	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:      "https://example.com/test/repo4.git",
		RepositoryID:   repo.ID,
		MirrorSourceID: 1,
		Status:         types.MirrorQueued,
	})
	require.Nil(t, err)

	task, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID: mirror.ID,
		Status:   types.MirrorQueued,
		Priority: types.HighMirrorPriority,
	})
	require.Nil(t, err)

	updatedTask, err := taskStore.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorContinue)
	require.Nil(t, err)
	require.Equal(t, types.MirrorRepoSyncStart, updatedTask.Status)

	var updatedRepo database.Repository
	err = db.Core.NewSelect().Model(&updatedRepo).Where("id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.SyncStatusInProgress, updatedRepo.SyncStatus)

	var updatedMirror database.Mirror
	err = db.Core.NewSelect().Model(&updatedMirror).Where("id = ?", mirror.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorRepoSyncStart, updatedMirror.Status)
}

func TestMirrorTaskStore_UpdateStatusAndRepoSyncStatusReturnsMirrorRepository(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:        1,
		Path:          "test/repo-relation",
		GitPath:       "test/repo-relation.git",
		Name:          "repo-relation",
		Nickname:      "Test Repo Relation",
		DefaultBranch: "main",
		Private:       false,
		SyncStatus:    types.SyncStatusFailed,
	})
	require.Nil(t, err)

	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:      "https://example.com/test/repo-relation.git",
		RepositoryID:   repo.ID,
		MirrorSourceID: 1,
		Status:         types.MirrorRepoSyncFailed,
		CurrentTaskID:  0,
	})
	require.Nil(t, err)

	task, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID: mirror.ID,
		Status:   types.MirrorRepoSyncFailed,
		Priority: types.ASAPMirrorPriority,
	})
	require.Nil(t, err)

	retriedTask, err := taskStore.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorRetry)
	require.Nil(t, err)
	require.NotNil(t, retriedTask.Mirror)
	require.NotNil(t, retriedTask.Mirror.Repository)
	require.Equal(t, repo.ID, retriedTask.Mirror.Repository.ID)

	startedTask, err := taskStore.UpdateStatusAndRepoSyncStatus(ctx, retriedTask, database.MirrorContinue)
	require.Nil(t, err)
	require.NotNil(t, startedTask.Mirror)
	require.NotNil(t, startedTask.Mirror.Repository)
	require.Equal(t, repo.ID, startedTask.Mirror.Repository.ID)
}

func TestMirrorTaskStore_UpdateStatusAndRepoSyncStatusClearsErrorOnSuccess(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:         1,
		Path:           "test/repo-clear-error",
		GitPath:        "test/repo-clear-error.git",
		Name:           "repo-clear-error",
		Nickname:       "Test Repo Clear Error",
		DefaultBranch:  "main",
		Private:        false,
		RepositoryType: types.ModelRepo,
		SyncStatus:     types.SyncStatusInProgress,
	})
	require.Nil(t, err)

	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:      "https://example.com/test/repo-clear-error.git",
		RepositoryID:   repo.ID,
		MirrorSourceID: 1,
		Status:         types.MirrorLfsSyncStart,
	})
	require.Nil(t, err)

	task, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID:          mirror.ID,
		Status:            types.MirrorLfsSyncStart,
		ErrorMessage:      "stale error",
		Priority:          types.ASAPMirrorPriority,
		Progress:          100,
		Mirror:            mirror,
		FinishedAt:        time.Time{},
		StartedAt:         time.Now(),
		RetryCount:        1,
		AfterLastCommitID: "after",
	})
	require.Nil(t, err)

	_, err = db.Core.NewUpdate().
		Model(&database.Mirror{}).
		Set("current_task_id = ?", task.ID).
		Where("id = ?", mirror.ID).
		Exec(ctx)
	require.Nil(t, err)

	updatedTask, err := taskStore.UpdateStatusAndRepoSyncStatus(ctx, task, database.MirrorSuccess)
	require.Nil(t, err)
	require.Empty(t, updatedTask.ErrorMessage)

	dbTask, err := taskStore.FindByID(ctx, task.ID)
	require.Nil(t, err)
	require.Empty(t, dbTask.ErrorMessage)
}

func TestMirrorTaskStore_CompleteRepoSyncAndInsertLFSJobCommitsRepoResultAndJob(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)
	jobClient := &fakeMirrorLFSJobClient{}

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:        1,
		Path:          "test/repo7",
		GitPath:       "test/repo7.git",
		Name:          "repo7",
		Nickname:      "Test Repo 7",
		DefaultBranch: "main",
		Private:       false,
		SyncStatus:    types.SyncStatusInProgress,
	})
	require.Nil(t, err)

	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:      "https://example.com/test/repo7.git",
		RepositoryID:   repo.ID,
		MirrorSourceID: 1,
		Status:         types.MirrorRepoSyncStart,
	})
	require.Nil(t, err)

	task, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID:           mirror.ID,
		Status:             types.MirrorRepoSyncStart,
		Priority:           types.ASAPMirrorPriority,
		BeforeLastCommitID: "before",
		AfterLastCommitID:  "after",
	})
	require.Nil(t, err)

	updatedTask, err := taskStore.CompleteRepoSyncAndInsertLFSJob(ctx, database.CompleteRepoSyncInput{
		Task:          task,
		DefaultBranch: "master",
		JobClient:     jobClient,
		JobInput: database.MirrorLFSJobInput{
			MirrorID:     mirror.ID,
			RepositoryID: repo.ID,
			MirrorTaskID: task.ID,
			SourceURL:    mirror.SourceUrl,
			Priority:     task.Priority,
		},
	})
	require.Nil(t, err)
	require.Equal(t, types.MirrorRepoSyncFinished, updatedTask.Status)
	require.Equal(t, int64(456), updatedTask.LFSJobID)
	require.Len(t, jobClient.inputs, 1)
	require.Equal(t, task.ID, jobClient.inputs[0].MirrorTaskID)

	storedTaskJob, err := taskStore.FindByID(ctx, task.ID)
	require.Nil(t, err)
	require.Equal(t, int64(456), storedTaskJob.LFSJobID)

	var updatedMirror database.Mirror
	err = db.Core.NewSelect().Model(&updatedMirror).Where("id = ?", mirror.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorRepoSyncFinished, updatedMirror.Status)
	require.Equal(t, task.ID, updatedMirror.CurrentTaskID)
	require.False(t, updatedMirror.LastUpdatedAt.IsZero())

	var updatedRepo database.Repository
	err = db.Core.NewSelect().Model(&updatedRepo).Where("id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.SyncStatusInProgress, updatedRepo.SyncStatus)
	require.Equal(t, "master", updatedRepo.DefaultBranch)
}

func TestMirrorTaskStore_CompleteRepoSyncAndInsertLFSJobRollsBackRepoResultOnJobInsertError(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	mirrorStore := database.NewMirrorStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)
	jobClient := &fakeMirrorLFSJobClient{err: errors.New("insert lfs job failed")}

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:        1,
		Path:          "test/repo8",
		GitPath:       "test/repo8.git",
		Name:          "repo8",
		Nickname:      "Test Repo 8",
		DefaultBranch: "main",
		Private:       false,
		SyncStatus:    types.SyncStatusInProgress,
	})
	require.Nil(t, err)

	mirror, err := mirrorStore.Create(ctx, &database.Mirror{
		SourceUrl:      "https://example.com/test/repo8.git",
		RepositoryID:   repo.ID,
		MirrorSourceID: 1,
		Status:         types.MirrorRepoSyncStart,
	})
	require.Nil(t, err)

	task, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID: mirror.ID,
		Status:   types.MirrorRepoSyncStart,
		Priority: types.ASAPMirrorPriority,
	})
	require.Nil(t, err)

	_, err = taskStore.CompleteRepoSyncAndInsertLFSJob(ctx, database.CompleteRepoSyncInput{
		Task:          task,
		DefaultBranch: "master",
		JobClient:     jobClient,
		JobInput: database.MirrorLFSJobInput{
			MirrorID:     mirror.ID,
			RepositoryID: repo.ID,
			MirrorTaskID: task.ID,
			SourceURL:    mirror.SourceUrl,
			Priority:     task.Priority,
		},
	})
	require.Error(t, err)
	require.Len(t, jobClient.inputs, 1)

	var storedTask database.MirrorTask
	err = db.Core.NewSelect().Model(&storedTask).Where("id = ?", task.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorRepoSyncStart, storedTask.Status)

	var storedMirror database.Mirror
	err = db.Core.NewSelect().Model(&storedMirror).Where("id = ?", mirror.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorRepoSyncStart, storedMirror.Status)
	require.True(t, storedMirror.LastUpdatedAt.IsZero())

	var storedRepo database.Repository
	err = db.Core.NewSelect().Model(&storedRepo).Where("id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "main", storedRepo.DefaultBranch)
}

func TestMirrorTaskStore_UpdateCommitCheckpoint(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskJobStoreWithDB(db)
	task, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID: 1,
		Status:   types.MirrorRepoSyncStart,
		Priority: types.ASAPMirrorPriority,
	})
	require.Nil(t, err)

	updatedTask, err := taskStore.UpdateCommitCheckpoint(ctx, task.ID, "before", "")
	require.Nil(t, err)
	require.Equal(t, "before", updatedTask.BeforeLastCommitID)
	require.Empty(t, updatedTask.AfterLastCommitID)

	updatedTask, err = taskStore.UpdateCommitCheckpoint(ctx, task.ID, "", "after")
	require.Nil(t, err)
	require.Equal(t, "before", updatedTask.BeforeLastCommitID)
	require.Equal(t, "after", updatedTask.AfterLastCommitID)
}

func TestMirrorTaskStore_UpdateProgress(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorTaskStoreWithDB(db)

	task, err := store.Create(ctx, database.MirrorTask{
		MirrorID:     1,
		Status:       types.MirrorLfsSyncStart,
		Priority:     types.HighMirrorPriority,
		Progress:     50,
		ErrorMessage: "previous error",
	})
	require.Nil(t, err)
	require.Greater(t, task.ID, int64(0))

	// Update progress and error message
	task.Progress = 80
	task.ErrorMessage = "new error"
	updatedTask, err := store.UpdateProgress(ctx, task)
	require.Nil(t, err)
	require.Equal(t, 80, updatedTask.Progress)
	require.Equal(t, "new error", updatedTask.ErrorMessage)
	require.Equal(t, types.MirrorLfsSyncStart, updatedTask.Status)

	// Verify in DB: progress and error_message updated, status preserved
	var dbTask database.MirrorTask
	err = db.Core.NewSelect().Model(&dbTask).Where("id = ?", task.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 80, dbTask.Progress)
	require.Equal(t, "new error", dbTask.ErrorMessage)
	require.Equal(t, types.MirrorLfsSyncStart, dbTask.Status,
		"UpdateProgress must not change the status field")
}

func TestMirrorTaskStore_UpdateProgress_DoesNotOverwritePriority(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorTaskStoreWithDB(db)

	task, err := store.Create(ctx, database.MirrorTask{
		MirrorID: 1,
		Status:   types.MirrorLfsSyncStart,
		Priority: types.ASAPMirrorPriority,
		Progress: 0,
	})
	require.Nil(t, err)

	// Simulate: another process changes priority concurrently
	_, err = db.Core.NewUpdate().
		Model(&database.MirrorTask{}).
		Set("priority = ?", types.LowMirrorPriority).
		Where("id = ?", task.ID).
		Exec(ctx)
	require.Nil(t, err)

	// UpdateProgress should NOT overwrite priority
	task.Progress = 100
	task.Priority = types.ASAPMirrorPriority // original value in struct
	_, err = store.UpdateProgress(ctx, task)
	require.Nil(t, err)

	var dbTask database.MirrorTask
	err = db.Core.NewSelect().Model(&dbTask).Where("id = ?", task.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 100, dbTask.Progress)
	require.Equal(t, types.LowMirrorPriority, dbTask.Priority,
		"UpdateProgress must not overwrite priority set by another process")
}

package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

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

	require.Equal(t, int64(1), mt.ID)
	require.Equal(t, int64(1), mt.MirrorID)
	require.Equal(t, types.MirrorQueued, mt.Status)
	require.Equal(t, types.LowMirrorPriority, mt.Priority)

	m1, err := store.Update(ctx, database.MirrorTask{
		ID:       1,
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
	require.Equal(t, int64(1), m2.ID)
	require.Equal(t, int64(1), m2.MirrorID)
	require.Equal(t, types.MirrorQueued, m2.Status)
	require.Equal(t, types.LowMirrorPriority, m2.Priority)

	err = store.Delete(ctx, 1)
	require.Nil(t, err)

	err = db.Core.NewSelect().Model(&mt).Where("id = ?", m.ID).Scan(ctx)
	require.NotNil(t, err)
	require.Equal(t, "sql: no rows in result set", err.Error())
}

func TestMirrorTaskStore_GetHighestPriority(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorTaskStoreWithDB(db)

	_, err := store.Create(ctx, database.MirrorTask{
		MirrorID: 1,
		Status:   types.MirrorQueued,
		Priority: types.LowMirrorPriority,
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.MirrorTask{
		MirrorID: 1,
		Status:   types.MirrorQueued,
		Priority: types.ASAPMirrorPriority,
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.MirrorTask{
		MirrorID: 1,
		Status:   types.MirrorQueued,
		Priority: types.HighMirrorPriority,
	})
	require.Nil(t, err)

	mt, err := store.GetHighestPriorityByTaskStatus(ctx, []types.MirrorTaskStatus{})
	require.Nil(t, err)
	require.Equal(t, int64(1), mt.MirrorID)
	require.Equal(t, types.MirrorRepoSyncStart, mt.Status)
	require.Equal(t, types.ASAPMirrorPriority, mt.Priority)
}

func TestMirrorTaskStore_SetMirrorCurrentTaskID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorTaskStoreWithDB(db)
	mstore := database.NewMirrorStoreWithDB(db)

	mt, err := store.Create(ctx, database.MirrorTask{
		MirrorID: 1,
		Status:   types.MirrorQueued,
		Priority: types.LowMirrorPriority,
	})
	require.Nil(t, err)

	_, err = mstore.Create(ctx, &database.Mirror{
		Interval:       "1",
		SourceUrl:      "test",
		RepositoryID:   1,
		MirrorSourceID: 1,
	})
	require.Nil(t, err)

	err = store.SetMirrorCurrentTaskID(ctx, mt)
	require.Nil(t, err)

	var m database.Mirror

	err = db.Operator.Core.NewSelect().Model(&m).Where("id = ?", 1).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, mt.ID, m.CurrentTaskID)
}

func TestMirrorTaskStore_ResetRunningTasks(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorTaskStoreWithDB(db)

	_, err := store.Create(ctx, database.MirrorTask{
		MirrorID: 1,
		Status:   types.MirrorLfsSyncStart,
		Priority: types.HighMirrorPriority,
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.MirrorTask{
		MirrorID: 2,
		Status:   types.MirrorLfsSyncStart,
		Priority: types.LowMirrorPriority,
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.MirrorTask{
		MirrorID: 3,
		Status:   types.MirrorQueued,
		Priority: types.HighMirrorPriority,
	})
	require.Nil(t, err)

	count, err := store.ResetRunningTasks(ctx, types.MirrorLfsSyncStart, types.MirrorRepoSyncFinished)
	require.Nil(t, err)
	require.Equal(t, 2, count)

	var tasks []database.MirrorTask
	err = db.Core.NewSelect().Model(&tasks).Where("status = ?", types.MirrorRepoSyncFinished).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 2, len(tasks))

	var queuedTask database.MirrorTask
	err = db.Core.NewSelect().Model(&queuedTask).Where("mirror_id = ?", 3).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.MirrorQueued, queuedTask.Status)
}

func TestMirrorTaskStore_UpdateStatusAndRepoSyncStatus(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	taskStore := database.NewMirrorTaskStoreWithDB(db)
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
		Interval:       "1h",
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

	taskStore := database.NewMirrorTaskStoreWithDB(db)
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
		Interval:       "1h",
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
		{"success from lfs_sync_start", types.MirrorLfsSyncStart, database.MirrorSuccess, types.MirrorLfsSyncFinished, types.SyncStatusCompleted},
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

	taskStore := database.NewMirrorTaskStoreWithDB(db)
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
		Interval:       "1h",
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

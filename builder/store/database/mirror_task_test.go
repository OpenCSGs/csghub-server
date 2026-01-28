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
		Status:   types.MirrorQueued,
		Priority: types.LowMirrorPriority,
		Mirror: &database.Mirror{
			RepositoryID: repo.ID,
		},
	})
	require.Nil(t, err)

	task.Status = types.MirrorRepoSyncFinished
	updatedTask, err := taskStore.UpdateStatusAndRepoSyncStatus(ctx, task, types.SyncStatusCompleted)
	require.Nil(t, err)
	require.Equal(t, types.MirrorRepoSyncFinished, updatedTask.Status)

	var updatedRepo database.Repository
	err = db.Core.NewSelect().Model(&updatedRepo).Where("id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.SyncStatusCompleted, updatedRepo.SyncStatus)

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

	task, err := taskStore.Create(ctx, database.MirrorTask{
		MirrorID: mirror.ID,
		Status:   types.MirrorQueued,
		Priority: types.HighMirrorPriority,
		Mirror: &database.Mirror{
			RepositoryID: repo.ID,
		},
	})
	require.Nil(t, err)

	testCases := []struct {
		taskStatus types.MirrorTaskStatus
		syncStatus types.RepositorySyncStatus
	}{
		{types.MirrorRepoSyncStart, types.SyncStatusInProgress},
		{types.MirrorRepoSyncFinished, types.SyncStatusInProgress},
		{types.MirrorLfsSyncStart, types.SyncStatusInProgress},
		{types.MirrorLfsSyncFinished, types.SyncStatusCompleted},
	}

	for _, tc := range testCases {
		task.Status = tc.taskStatus
		updatedTask, err := taskStore.UpdateStatusAndRepoSyncStatus(ctx, task, tc.syncStatus)
		require.Nil(t, err)
		require.Equal(t, tc.taskStatus, updatedTask.Status)

		var updatedRepo database.Repository
		err = db.Core.NewSelect().Model(&updatedRepo).Where("id = ?", repo.ID).Scan(ctx)
		require.Nil(t, err)
		require.Equal(t, tc.syncStatus, updatedRepo.SyncStatus)
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

	task.Status = types.MirrorRepoSyncFailed
	updatedTask, err := taskStore.UpdateStatusAndRepoSyncStatus(ctx, task, types.SyncStatusFailed)
	require.Nil(t, err)
	require.Equal(t, types.MirrorRepoSyncFailed, updatedTask.Status)

	var updatedRepo database.Repository
	err = db.Core.NewSelect().Model(&updatedRepo).Where("id = ?", repo.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, types.SyncStatusFailed, updatedRepo.SyncStatus)
}

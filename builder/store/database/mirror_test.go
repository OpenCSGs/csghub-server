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

type fakeMirrorDeleteJobCancelClient struct {
	cancelled []int64
	err       error
}

func (c *fakeMirrorDeleteJobCancelClient) JobCancelTx(ctx context.Context, tx *sql.Tx, jobID int64) error {
	c.cancelled = append(c.cancelled, jobID)
	return c.err
}

func TestMirrorStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	_, err := store.Create(ctx, &database.Mirror{
		Interval:          "foo",
		RepositoryID:      123,
		PushMirrorCreated: true,
		Status:            types.MirrorLfsSyncFinished,
		Priority:          types.HighMirrorPriority,
	})
	require.Nil(t, err)

	mi := &database.Mirror{}
	err = db.Core.NewSelect().Model(mi).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "foo", mi.Interval)

	mi, err = store.FindByID(ctx, mi.ID)
	require.Nil(t, err)
	require.Equal(t, "foo", mi.Interval)

	mi, err = store.FindByRepoID(ctx, 123)
	require.Nil(t, err)
	require.Equal(t, "foo", mi.Interval)

	exist, err := store.IsExist(ctx, 123)
	require.Nil(t, err)
	require.True(t, exist)
	exist, err = store.IsExist(ctx, 456)
	require.Nil(t, err)
	require.False(t, exist)

	repo := &database.Repository{
		RepositoryType: types.ModelRepo,
		GitPath:        "models_ns/n",
		Name:           "repo",
		Path:           "ns/n",
	}
	err = db.Core.NewInsert().Model(repo).Scan(ctx, repo)
	require.Nil(t, err)

	exist, err = store.IsRepoExist(ctx, types.ModelRepo, "ns", "n")
	require.Nil(t, err)
	require.True(t, exist)
	exist, err = store.IsRepoExist(ctx, types.ModelRepo, "NS", "N")
	require.Nil(t, err)
	require.True(t, exist)

	exist, err = store.IsRepoExist(ctx, types.ModelRepo, "ns", "n2")
	require.Nil(t, err)
	require.False(t, exist)

	mi.RepositoryID = repo.ID
	err = store.Update(ctx, mi)
	require.Nil(t, err)

	err = db.Core.NewSelect().Model(mi).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, repo.ID, mi.RepositoryID)

	mi, err = store.FindByRepoPath(ctx, types.ModelRepo, "ns", "n")
	require.Nil(t, err)
	require.Equal(t, repo.ID, mi.RepositoryID)

	ms, err := store.PushedMirror(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(ms))

	ms, err = store.NoPushMirror(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, len(ms))

	ms, err = store.Finished(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(ms))

	ms, err = store.Unfinished(ctx)
	require.Nil(t, err)
	require.Equal(t, 0, len(ms))

	mi.AccessToken = "abc"
	repo.Nickname = "fooo"
	err = store.UpdateMirrorAndRepository(ctx, mi, repo)
	require.Nil(t, err)
	mi = &database.Mirror{}
	err = db.Core.NewSelect().Model(mi).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "abc", mi.AccessToken)
	repo = &database.Repository{}
	err = db.Core.NewSelect().Model(repo).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "fooo", repo.Nickname)

	err = store.Delete(ctx, mi)
	require.Nil(t, err)
	_, err = store.FindByID(ctx, mi.ID)
	require.NotNil(t, err)

}

func TestMirrorStore_DeleteWithTaskCancelTxCancelsJobsAndDeletesMirror(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	repo := &database.Repository{
		RepositoryType: types.ModelRepo,
		GitPath:        "models_ns/repo",
		Name:           "repo",
		Path:           "ns/repo",
		SyncStatus:     types.SyncStatusInProgress,
	}
	require.NoError(t, db.Core.NewInsert().Model(repo).Scan(ctx, repo))

	mirror, err := store.Create(ctx, &database.Mirror{
		Interval:     "m1",
		SourceUrl:    "https://github.com/a/repo.git",
		RepositoryID: repo.ID,
		Status:       types.MirrorRepoSyncStart,
		Priority:     types.ASAPMirrorPriority,
	})
	require.NoError(t, err)
	tasks := []database.MirrorTask{
		{MirrorID: mirror.ID, Status: types.MirrorRepoSyncStart, Priority: types.ASAPMirrorPriority, RepoJobID: 11, LFSJobID: 12},
		{MirrorID: mirror.ID, Status: types.MirrorQueued, Priority: types.HighMirrorPriority, RepoJobID: 21},
	}
	for i := range tasks {
		require.NoError(t, db.Core.NewInsert().Model(&tasks[i]).Scan(ctx, &tasks[i]))
	}

	cancelClient := &fakeMirrorDeleteJobCancelClient{}
	err = store.DeleteWithTaskCancelTx(ctx, mirror.ID, cancelClient)
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{int64(11), int64(12), int64(21)}, cancelClient.cancelled)

	_, err = store.FindByID(ctx, mirror.ID)
	require.Error(t, err)
	count, err := db.Core.NewSelect().Model((*database.MirrorTask)(nil)).Where("mirror_id = ?", mirror.ID).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)

	var storedRepo database.Repository
	require.NoError(t, db.Core.NewSelect().Model(&storedRepo).Where("id = ?", repo.ID).Scan(ctx))
	require.Equal(t, types.SyncStatusCanceled, storedRepo.SyncStatus)
}

func TestMirrorStore_DeleteWithTaskCancelTxRollsBackWhenJobCancelFails(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	repo := &database.Repository{
		RepositoryType: types.ModelRepo,
		GitPath:        "models_ns/repo",
		Name:           "repo",
		Path:           "ns/repo",
	}
	require.NoError(t, db.Core.NewInsert().Model(repo).Scan(ctx, repo))
	mirror, err := store.Create(ctx, &database.Mirror{
		Interval:     "m1",
		SourceUrl:    "https://github.com/a/repo.git",
		RepositoryID: repo.ID,
		Status:       types.MirrorRepoSyncStart,
		Priority:     types.ASAPMirrorPriority,
	})
	require.NoError(t, err)
	task := &database.MirrorTask{
		MirrorID:  mirror.ID,
		Status:    types.MirrorRepoSyncStart,
		Priority:  types.ASAPMirrorPriority,
		RepoJobID: 11,
	}
	require.NoError(t, db.Core.NewInsert().Model(task).Scan(ctx, task))

	cancelClient := &fakeMirrorDeleteJobCancelClient{err: errors.New("cancel failed")}
	err = store.DeleteWithTaskCancelTx(ctx, mirror.ID, cancelClient)
	require.Error(t, err)

	_, err = store.FindByID(ctx, mirror.ID)
	require.NoError(t, err)
	count, err := db.Core.NewSelect().Model((*database.MirrorTask)(nil)).Where("mirror_id = ?", mirror.ID).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestMirrorStore_DeleteWithTaskCancelTxKeepsCompletedRepoStatus(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	repo := &database.Repository{
		RepositoryType: types.ModelRepo,
		GitPath:        "models_ns/repo",
		Name:           "repo",
		Path:           "ns/repo",
		SyncStatus:     types.SyncStatusCompleted,
	}
	require.NoError(t, db.Core.NewInsert().Model(repo).Scan(ctx, repo))
	mirror, err := store.Create(ctx, &database.Mirror{
		Interval:     "m1",
		SourceUrl:    "https://github.com/a/repo.git",
		RepositoryID: repo.ID,
		Status:       types.MirrorLfsSyncFinished,
		Priority:     types.ASAPMirrorPriority,
	})
	require.NoError(t, err)
	task := &database.MirrorTask{
		MirrorID: mirror.ID,
		Status:   types.MirrorLfsSyncFinished,
		Priority: types.ASAPMirrorPriority,
	}
	require.NoError(t, db.Core.NewInsert().Model(task).Scan(ctx, task))

	err = store.DeleteWithTaskCancelTx(ctx, mirror.ID, &fakeMirrorDeleteJobCancelClient{})
	require.NoError(t, err)

	var storedRepo database.Repository
	require.NoError(t, db.Core.NewSelect().Model(&storedRepo).Where("id = ?", repo.ID).Scan(ctx))
	require.Equal(t, types.SyncStatusCompleted, storedRepo.SyncStatus)
}

func TestMirrorStore_FindWithMapping(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	repos := []*database.Repository{
		{Name: "repo1", RepositoryType: types.ModelRepo, Path: "ns/repo1", HFPath: "hf/repo1"},
		{Name: "repo2", RepositoryType: types.DatasetRepo, Path: "ns/repo2", MSPath: "ms/repo2"},
		{Name: "repo3", RepositoryType: types.PromptRepo, Path: "ns/repo3"},
	}

	for _, repo := range repos {
		repo.GitPath = repo.Path
		err := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
		require.Nil(t, err)
	}

	mi, err := store.FindWithMapping(ctx, types.ModelRepo, "ns", "repo1", types.CSGHubMapping)
	require.Nil(t, err)
	require.Equal(t, "repo1", mi.Name)

	_, err = store.FindWithMapping(ctx, types.ModelRepo, "HF", "REPO1", types.HFMapping)
	require.Nil(t, err)

	_, err = store.FindWithMapping(ctx, types.ModelRepo, "aaa", "repo1", types.HFMapping)
	require.NotNil(t, err)

	mi, err = store.FindWithMapping(ctx, types.DatasetRepo, "MS", "REPO2", types.ModelScopeMapping)
	require.Nil(t, err)
	require.Equal(t, "repo2", mi.Name)

	mi, err = store.FindWithMapping(ctx, types.ModelRepo, "HF", "REPO1", types.AutoMapping)
	require.Nil(t, err)
	require.Equal(t, "repo1", mi.Name)

	mi, err = store.FindWithMapping(ctx, types.PromptRepo, "NS", "REPO3", types.CSGHubMapping)
	require.Nil(t, err)
	require.Equal(t, "repo3", mi.Name)
}

func TestMirrorStore_ToSync(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	dt := time.Now().Add(1 * time.Hour)
	mirrors := []*database.Mirror{
		{NextExecutionTimestamp: dt, Status: types.MirrorRepoSyncFailed, Interval: "m1"},
		{NextExecutionTimestamp: dt, Status: types.MirrorLfsSyncFinished, Interval: "m2"},
		{NextExecutionTimestamp: dt, Status: types.MirrorLfsSyncFailed, Interval: "m3"},
		{NextExecutionTimestamp: dt, Status: types.MirrorRepoSyncFinished, Interval: "m4"},
		{NextExecutionTimestamp: dt, Status: types.MirrorRepoSyncStart, Interval: "m5"},
		{NextExecutionTimestamp: dt.Add(-5 * time.Hour), Status: types.MirrorLfsSyncFinished, Interval: "m7"},
		{NextExecutionTimestamp: dt, Status: types.MirrorRepoSyncFatal, Interval: "m8"},
		{NextExecutionTimestamp: dt, Status: types.MirrorLfsSyncStart, Interval: "m9"},
	}
	for _, m := range mirrors {
		_, err := store.Create(ctx, m)
		require.Nil(t, err)
	}

	ms, err := store.ToSyncRepo(ctx)
	require.Nil(t, err)
	names := []string{}
	for _, m := range ms {
		names = append(names, m.Interval)
	}
	require.ElementsMatch(t, []string{"m1", "m3", "m7"}, names)

	ms, err = store.ToSyncLfs(ctx)
	require.Nil(t, err)
	names = []string{}
	for _, m := range ms {
		names = append(names, m.Interval)
	}
	require.ElementsMatch(t, []string{"m4", "m7"}, names)

}

func TestMirrorStore_IndexWithPagination(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	mirrors := []*database.Mirror{
		{Interval: "m1", LocalRepoPath: "foo", SourceUrl: "bar"},
		{Interval: "m2", LocalRepoPath: "bar", SourceUrl: "foo"},
	}
	for _, m := range mirrors {
		_, err := store.Create(ctx, m)
		require.Nil(t, err)
	}

	ms, count, err := store.IndexWithPagination(ctx, 10, 1, types.MirrorFilter{Search: "foo"}, false)
	require.Nil(t, err)
	require.Equal(t, 2, count)
	// make sure in "DESC" order
	require.Equal(t, "m2", ms[0].Interval)
	require.Equal(t, "m1", ms[1].Interval)

	_, count, err = store.IndexWithPagination(ctx, 10, 1, types.MirrorFilter{Search: "foo"}, true)
	require.Nil(t, err)
	require.Equal(t, 0, count)
}

// TestMirrorStore_IndexWithPaginationStatusFilter verifies current task status filtering and mirror status fallback.
func TestMirrorStore_IndexWithPaginationStatusFilter(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	mirrors := []*database.Mirror{
		{Interval: "mirror-status", Status: types.MirrorRepoSyncFailed},
		{Interval: "current-task-status", Status: types.MirrorQueued},
		{Interval: "current-task-overrides-mirror-status", Status: types.MirrorRepoSyncFailed},
	}
	for _, mirror := range mirrors {
		_, err := store.Create(ctx, mirror)
		require.NoError(t, err)
	}

	tasks := []*database.MirrorTask{
		{MirrorID: mirrors[1].ID, Status: types.MirrorRepoSyncFailed},
		{MirrorID: mirrors[2].ID, Status: types.MirrorQueued},
	}
	for i, task := range tasks {
		require.NoError(t, db.Core.NewInsert().Model(task).Scan(ctx, task))
		_, err := db.Core.NewUpdate().Model(mirrors[i+1]).Set("current_task_id = ?", task.ID).WherePK().Exec(ctx)
		require.NoError(t, err)
	}

	status := types.MirrorRepoSyncFailed
	result, count, err := store.IndexWithPagination(ctx, 10, 1, types.MirrorFilter{Status: &status}, false)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.ElementsMatch(t, []string{"mirror-status", "current-task-status"}, []string{result[0].Interval, result[1].Interval})
}

func TestMirrorStore_StatusCount(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	mirrors := []*database.Mirror{
		{Interval: "m1", Status: types.MirrorRepoSyncFailed},
		{Interval: "m2", Status: types.MirrorRepoSyncFailed},
		{Interval: "m3", Status: types.MirrorRepoSyncFinished},
	}
	for _, m := range mirrors {
		_, err := store.Create(ctx, m)
		require.Nil(t, err)
	}

	cs, err := store.StatusCount(ctx)
	require.Nil(t, err)
	require.Equal(t, 2, len(cs))
	require.ElementsMatch(t, []database.MirrorStatusCount{
		{types.MirrorRepoSyncFailed, 2},
		{types.MirrorRepoSyncFinished, 1},
	}, cs)

}

func TestMirrorStore_FindBySourceURLs(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	mirrors := []*database.Mirror{
		{SourceUrl: "https://opencsg.com/models/repo1", Status: types.MirrorRepoSyncFailed},
		{SourceUrl: "https://opencsg.com/models/repo2", Status: types.MirrorRepoSyncFailed},
		{SourceUrl: "https://opencsg.com/models/repo2", Status: types.MirrorRepoSyncFailed},
		{SourceUrl: "https://opencsg.com/models/repo3", Status: types.MirrorRepoSyncFinished},
	}
	for _, m := range mirrors {
		_, err := store.Create(ctx, m)
		require.Nil(t, err)
	}

	cs, err := store.FindBySourceURLs(ctx, []string{"https://opencsg.com/models/repo2", "https://opencsg.com/models/repo3"})
	require.Nil(t, err)
	require.Equal(t, 3, len(cs))
}

func TestMirrorStore_BatchUpdate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	mirrors := []*database.Mirror{
		{SourceUrl: "https://opencsg.com/models/repo1", Status: types.MirrorRepoSyncFailed, Priority: 1},
		{SourceUrl: "https://opencsg.com/models/repo2", Status: types.MirrorRepoSyncFailed, Priority: 1},
		{SourceUrl: "https://opencsg.com/models/repo3", Status: types.MirrorRepoSyncFailed, Priority: 1},
	}
	createdMirrors := []database.Mirror{}
	date := time.Now()
	for _, m := range mirrors {
		m, err := store.Create(ctx, m)
		require.Nil(t, err)
		m.Priority = 3
		m.RemoteUpdatedAt = date
		createdMirrors = append(createdMirrors, *m)
	}
	err := store.BatchUpdate(ctx, createdMirrors)
	require.Nil(t, err)
	m1, err := store.FindByID(ctx, createdMirrors[0].ID)
	require.Nil(t, err)
	require.Equal(t, createdMirrors[0].Priority, m1.Priority)
	m2, err := store.FindByID(ctx, createdMirrors[1].ID)
	require.Nil(t, err)
	require.Equal(t, createdMirrors[1].Priority, m2.Priority)
	m3, err := store.FindByID(ctx, createdMirrors[2].ID)
	require.Nil(t, err)
	require.Equal(t, createdMirrors[2].Priority, m3.Priority)
}

func TestMirrorStore_BatchCreate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	mirrors := []database.Mirror{
		{SourceUrl: "https://opencsg.com/models/repo1", Status: types.MirrorRepoSyncFailed, Priority: 1},
		{SourceUrl: "https://opencsg.com/models/repo2", Status: types.MirrorRepoSyncFailed, Priority: 1},
		{SourceUrl: "https://opencsg.com/models/repo3", Status: types.MirrorRepoSyncFailed, Priority: 1},
	}
	err := store.BatchCreate(ctx, mirrors)
	require.Nil(t, err)
	ms, count, err := store.IndexWithPagination(ctx, 10, 1, types.MirrorFilter{}, false)
	require.Nil(t, err)
	require.Equal(t, 3, len(ms))
	require.Equal(t, 3, count)
}

// func TestMirrorStore_ToBeScheduled_Status(t *testing.T) {
// 	db := tests.InitTestDB()
// 	defer db.Close()
// 	ctx := context.TODO()

// 	store := database.NewMirrorStoreWithDB(db)

// 	mirrors := []database.Mirror{
// 		{SourceUrl: "https://opencsg.com/models/repo1", Status: types.MirrorFailed, Priority: 1},
// 		{SourceUrl: "https://opencsg.com/models/repo3", Status: types.MirrorFinished, Priority: 1},
// 	}
// 	err := store.BatchCreate(ctx, mirrors)
// 	require.Nil(t, err)
// 	ms, err := store.ToBeScheduled(ctx)
// 	require.Nil(t, err)
// 	require.Equal(t, 2, len(ms))
// 	require.Equal(t, ms[0].SourceUrl, "https://opencsg.com/models/repo2")
// 	require.Equal(t, ms[1].SourceUrl, "https://opencsg.com/models/repo1")
// }

// func TestMirrorStore_ToBeScheduled_NextExecutionTimestamp(t *testing.T) {
// 	db := tests.InitTestDB()
// 	defer db.Close()
// 	ctx := context.TODO()

// 	store := database.NewMirrorStoreWithDB(db)

// 	mirrors := []database.Mirror{
// 		{SourceUrl: "https://opencsg.com/models/repo1", NextExecutionTimestamp: time.Now().Add(-time.Hour), Status: types.MirrorFinished, Priority: 3},
// 		{SourceUrl: "https://opencsg.com/models/repo2", Status: types.MirrorFinished, Priority: 1},
// 		{SourceUrl: "https://opencsg.com/models/repo3", Status: types.MirrorFinished, Priority: 1},
// 	}
// 	err := store.BatchCreate(ctx, mirrors)
// 	require.Nil(t, err)
// 	ms, err := store.ToBeScheduled(ctx)
// 	require.Nil(t, err)
// 	require.Equal(t, 1, len(ms))
// 	require.Equal(t, ms[0].SourceUrl, "https://opencsg.com/models/repo1")
// }

package database_test

import (
	"context"
	"database/sql"
	"encoding/json"
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
	payload, err := json.Marshal(mi)
	require.NoError(t, err)
	require.NotContains(t, string(payload), `"interval"`)

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

	_, err = store.FindWithMapping(ctx, types.ModelRepo, "hf", "repo1", types.HFMapping)
	require.Nil(t, err)

	_, err = store.FindWithMapping(ctx, types.ModelRepo, "aaa", "repo1", types.HFMapping)
	require.NotNil(t, err)

	mi, err = store.FindWithMapping(ctx, types.DatasetRepo, "ms", "repo2", types.ModelScopeMapping)
	require.Nil(t, err)
	require.Equal(t, "repo2", mi.Name)

	mi, err = store.FindWithMapping(ctx, types.PromptRepo, "ns", "repo3", types.CSGHubMapping)
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
		{NextExecutionTimestamp: dt, Status: types.MirrorRepoSyncFailed, SourceUrl: "m1"},
		{NextExecutionTimestamp: dt, Status: types.MirrorLfsSyncFinished, SourceUrl: "m2"},
		{NextExecutionTimestamp: dt, Status: types.MirrorLfsSyncFailed, SourceUrl: "m3"},
		{NextExecutionTimestamp: dt, Status: types.MirrorRepoSyncFinished, SourceUrl: "m4"},
		{NextExecutionTimestamp: dt, Status: types.MirrorRepoSyncStart, SourceUrl: "m5"},
		{NextExecutionTimestamp: dt.Add(-5 * time.Hour), Status: types.MirrorLfsSyncFinished, SourceUrl: "m7"},
		{NextExecutionTimestamp: dt, Status: types.MirrorRepoSyncFatal, SourceUrl: "m8"},
		{NextExecutionTimestamp: dt, Status: types.MirrorLfsSyncStart, SourceUrl: "m9"},
	}
	for _, m := range mirrors {
		_, err := store.Create(ctx, m)
		require.Nil(t, err)
	}

	ms, err := store.ToSyncRepo(ctx)
	require.Nil(t, err)
	names := []string{}
	for _, m := range ms {
		names = append(names, m.SourceUrl)
	}
	require.ElementsMatch(t, []string{"m1", "m3", "m7"}, names)

	ms, err = store.ToSyncLfs(ctx)
	require.Nil(t, err)
	names = []string{}
	for _, m := range ms {
		names = append(names, m.SourceUrl)
	}
	require.ElementsMatch(t, []string{"m4", "m7"}, names)

}

func TestMirrorStore_IndexWithPagination(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	msStore := database.NewMirrorTaskStoreWithDB(db)

	mirrors := []*database.Mirror{
		{LocalRepoPath: "foo", SourceUrl: "bar"},
		{LocalRepoPath: "bar", SourceUrl: "foo"},
	}
	var mStatus types.MirrorTaskStatus
	for i, m := range mirrors {
		mr, err := store.Create(ctx, m)
		require.Nil(t, err)
		if i == 0 {
			mStatus = types.MirrorRepoSyncStart
		} else {
			mStatus = types.MirrorLfsSyncFailed
		}
		ct, err := msStore.Create(ctx, database.MirrorTask{
			MirrorID: mr.ID,
			Status:   mStatus,
		})
		require.Nil(t, err)
		m.CurrentTaskID = ct.ID
		err = store.Update(ctx, m)
		require.Nil(t, err)
	}

	ms, count, err := store.IndexWithPagination(ctx, 10, 1, types.MirrorFilter{Search: "foo"}, false)
	require.Nil(t, err)
	require.Equal(t, 2, count)
	// make sure in "DESC" order
	require.Equal(t, "bar", ms[0].LocalRepoPath)
	require.Equal(t, "foo", ms[1].LocalRepoPath)

	_, count, err = store.IndexWithPagination(ctx, 10, 1, types.MirrorFilter{Search: "foo"}, true)
	require.Nil(t, err)
	require.Equal(t, 0, count)

	status := types.MirrorRepoSyncStart
	ms, count, err = store.IndexWithPagination(ctx, 10, 1, types.MirrorFilter{Status: &status}, false)
	require.Nil(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, "foo", ms[0].LocalRepoPath)
}

// TestMirrorStore_IndexSyncWithPagination verifies static search includes usernames.
func TestMirrorStore_IndexSyncWithPagination(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	_, err := store.Create(ctx, &database.Mirror{SourceUrl: "https://example.com/other.git", Username: "other-user"})
	require.NoError(t, err)
	wanted, err := store.Create(ctx, &database.Mirror{SourceUrl: "https://example.com/repo.git", Username: "target-user"})
	require.NoError(t, err)

	mirrors, total, err := store.IndexSyncWithPagination(ctx, database.MirrorSyncListQuery{
		Page: 1, Per: 1, Search: "TARGET-USER",
	})

	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, mirrors, 1)
	require.Equal(t, wanted.ID, mirrors[0].ID)
}

// TestMirrorStore_IndexSyncWithPaginationOrdersByCurrentTaskUpdatedAt verifies recent current tasks sort first and taskless mirrors last.
func TestMirrorStore_IndexSyncWithPaginationOrdersByCurrentTaskUpdatedAt(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	taskStore := database.NewMirrorTaskStoreWithDB(db)
	createMirrorWithTask := func(updatedAt time.Time) *database.Mirror {
		mirror, err := store.Create(ctx, &database.Mirror{SourceUrl: "https://example.com/repo.git"})
		require.NoError(t, err)
		task, err := taskStore.Create(ctx, database.MirrorTask{MirrorID: mirror.ID, Status: types.MirrorQueued})
		require.NoError(t, err)
		_, err = db.Core.NewUpdate().
			Model((*database.MirrorTask)(nil)).
			Set("updated_at = ?", updatedAt).
			Where("id = ?", task.ID).
			Exec(ctx)
		require.NoError(t, err)
		mirror.CurrentTaskID = task.ID
		require.NoError(t, store.Update(ctx, mirror))
		return mirror
	}

	now := time.Now()
	newest := createMirrorWithTask(now)
	tiedUpdatedAt := now.Add(-time.Hour)
	tiedLowerID := createMirrorWithTask(tiedUpdatedAt)
	tiedHigherID := createMirrorWithTask(tiedUpdatedAt)
	taskless, err := store.Create(ctx, &database.Mirror{SourceUrl: "https://example.com/taskless.git"})
	require.NoError(t, err)

	mirrors, total, err := store.IndexSyncWithPagination(ctx, database.MirrorSyncListQuery{Page: 1, Per: 10})

	require.NoError(t, err)
	require.Equal(t, 4, total)
	require.Equal(t, []int64{newest.ID, tiedHigherID.ID, tiedLowerID.ID, taskless.ID}, mirrorIDs(mirrors))
}

// TestMirrorStore_IndexSyncWithPaginationFiltersTaskStatus verifies status conditions apply only to current tasks before pagination.
func TestMirrorStore_IndexSyncWithPaginationFiltersTaskStatus(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	taskStore := database.NewMirrorTaskStoreWithDB(db)
	var wanted *database.Mirror
	for _, status := range []types.MirrorTaskStatus{types.MirrorQueued, types.MirrorRepoSyncFatal} {
		mirror, err := store.Create(ctx, &database.Mirror{SourceUrl: "https://example.com/repo.git"})
		require.NoError(t, err)
		task, err := taskStore.Create(ctx, database.MirrorTask{MirrorID: mirror.ID, Status: status})
		require.NoError(t, err)
		mirror.CurrentTaskID = task.ID
		require.NoError(t, store.Update(ctx, mirror))
		if status == types.MirrorRepoSyncFatal {
			wanted = mirror
		}
	}

	tasklessWaiting, err := store.Create(ctx, &database.Mirror{
		SourceUrl: "https://example.com/taskless-waiting.git",
		Status:    types.MirrorQueued,
	})
	require.NoError(t, err)
	tasklessRunning, err := store.Create(ctx, &database.Mirror{
		SourceUrl: "https://example.com/taskless-running.git",
		Status:    types.MirrorRepoSyncStart,
	})
	require.NoError(t, err)

	allMirrors, allTotal, err := store.IndexSyncWithPagination(ctx, database.MirrorSyncListQuery{Page: 1, Per: 10})
	require.NoError(t, err)
	require.Equal(t, 4, allTotal)
	require.Contains(t, mirrorIDs(allMirrors), tasklessWaiting.ID)
	require.Contains(t, mirrorIDs(allMirrors), tasklessRunning.ID)

	waiting, waitingTotal, err := store.IndexSyncWithPagination(ctx, database.MirrorSyncListQuery{
		Page: 1, Per: 10, Statuses: []types.MirrorTaskStatus{types.MirrorQueued, types.MirrorRepoSyncFinished},
	})
	require.NoError(t, err)
	require.Equal(t, 1, waitingTotal)
	require.NotContains(t, mirrorIDs(waiting), tasklessWaiting.ID)
	require.NotContains(t, mirrorIDs(waiting), tasklessRunning.ID)

	running, runningTotal, err := store.IndexSyncWithPagination(ctx, database.MirrorSyncListQuery{
		Page: 1, Per: 10, Statuses: []types.MirrorTaskStatus{
			types.MirrorRepoSyncStart,
			types.MirrorRepoSyncFailed,
			types.MirrorLfsSyncStart,
			types.MirrorLfsSyncFailed,
		},
	})
	require.NoError(t, err)
	require.Zero(t, runningTotal)
	require.NotContains(t, mirrorIDs(running), tasklessWaiting.ID)
	require.NotContains(t, mirrorIDs(running), tasklessRunning.ID)

	mirrors, total, err := store.IndexSyncWithPagination(ctx, database.MirrorSyncListQuery{
		Page: 1, Per: 10, Statuses: []types.MirrorTaskStatus{types.MirrorRepoSyncFatal},
	})

	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, mirrors, 1)
	require.Equal(t, wanted.ID, mirrors[0].ID)
}

// mirrorIDs returns mirror identifiers for list result assertions.
func mirrorIDs(mirrors []database.Mirror) []int64 {
	ids := make([]int64, 0, len(mirrors))
	for _, mirror := range mirrors {
		ids = append(ids, mirror.ID)
	}
	return ids
}

func TestMirrorStore_StatusCount(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	mirrors := []*database.Mirror{
		{Status: types.MirrorRepoSyncFailed},
		{Status: types.MirrorRepoSyncFailed},
		{Status: types.MirrorRepoSyncFinished},
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
		{SourceUrl: "https://opencsg.com/models/repo1", Username: "old-user-1", AccessToken: "old-token-1", Status: types.MirrorRepoSyncFailed, Priority: 1},
		{SourceUrl: "https://opencsg.com/models/repo2", Username: "old-user-2", AccessToken: "old-token-2", Status: types.MirrorRepoSyncFailed, Priority: 1},
		{SourceUrl: "https://opencsg.com/models/repo3", Username: "old-user-3", AccessToken: "old-token-3", Status: types.MirrorRepoSyncFailed, Priority: 1},
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
	createdMirrors[0].Username = "new-user"
	createdMirrors[0].AccessToken = "new-token"
	createdMirrors[1].Username = ""
	createdMirrors[1].AccessToken = ""
	err := store.BatchUpdate(ctx, createdMirrors)
	require.Nil(t, err)
	m1, err := store.FindByID(ctx, createdMirrors[0].ID)
	require.Nil(t, err)
	require.Equal(t, createdMirrors[0].Priority, m1.Priority)
	require.Equal(t, "new-user", m1.Username)
	require.Equal(t, "new-token", m1.AccessToken)
	m2, err := store.FindByID(ctx, createdMirrors[1].ID)
	require.Nil(t, err)
	require.Equal(t, createdMirrors[1].Priority, m2.Priority)
	require.Empty(t, m2.Username)
	require.Empty(t, m2.AccessToken)
	m3, err := store.FindByID(ctx, createdMirrors[2].ID)
	require.Nil(t, err)
	require.Equal(t, createdMirrors[2].Priority, m3.Priority)
	require.Equal(t, "old-user-3", m3.Username)
	require.Equal(t, "old-token-3", m3.AccessToken)
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
	ms, count, err := store.IndexWithPagination(ctx, 10, 1, types.MirrorFilter{Search: ""}, false)
	require.Nil(t, err)
	require.Equal(t, 3, len(ms))
	require.Equal(t, 3, count)
}

// TestMirrorStoreToBeScheduledOrdersByPriority verifies lower numeric priority values are scheduled first.
func TestMirrorStoreToBeScheduledOrdersByPriority(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	store := database.NewMirrorStoreWithDB(db)

	err := store.BatchCreate(ctx, []database.Mirror{
		{SourceUrl: "https://example.com/low.git", Status: types.MirrorQueued, Priority: types.LowMirrorPriority},
		{SourceUrl: "https://example.com/asap.git", Status: types.MirrorQueued, Priority: types.ASAPMirrorPriority},
		{SourceUrl: "https://example.com/medium.git", Status: types.MirrorQueued, Priority: types.MediumMirrorPriority},
	})
	require.NoError(t, err)

	mirrors, err := store.ToBeScheduled(ctx)
	require.NoError(t, err)
	require.Len(t, mirrors, 3)
	require.Equal(t, []types.MirrorPriority{
		types.ASAPMirrorPriority,
		types.MediumMirrorPriority,
		types.LowMirrorPriority,
	}, []types.MirrorPriority{mirrors[0].Priority, mirrors[1].Priority, mirrors[2].Priority})
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

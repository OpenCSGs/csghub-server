package database_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestMirrorStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)
	_, err := store.Create(ctx, &database.Mirror{
		Interval:          "foo",
		RepositoryID:      123,
		PushMirrorCreated: true,
		Status:            types.MirrorFinished,
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

	ms, err := store.WithPagination(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(ms))

	ms, err = store.WithPaginationWithRepository(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(ms))
	require.Equal(t, "repo", ms[0].Repository.Name)

	ms, err = store.PushedMirror(ctx)
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

func TestMirrorStore_FindWithMapping(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	repos := []*database.Repository{
		{Name: "repo1", RepositoryType: types.ModelRepo, Path: "models_ns/repo1"},
		{Name: "repo2", RepositoryType: types.DatasetRepo, Path: "datasets_ns/repo2"},
		{Name: "repo3", RepositoryType: types.PromptRepo, Path: "prompts_ns/repo3"},
	}

	for _, repo := range repos {
		repo.GitPath = repo.Path
		err := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
		require.Nil(t, err)
		sp := strings.Split(repo.Path, "_")
		_, err = store.Create(ctx, &database.Mirror{
			RepositoryID:   repo.ID,
			SourceRepoPath: strings.ReplaceAll(sp[1], "ns/", "nsn/"),
			Interval:       repo.Name,
		})
		require.Nil(t, err)
	}

	mi, err := store.FindWithMapping(ctx, types.ModelRepo, "ns", "repo1", types.CSGHubMapping)
	require.Nil(t, err)
	require.Equal(t, "repo1", mi.Interval)

	_, err = store.FindWithMapping(ctx, types.ModelRepo, "ns", "repo1", types.HFMapping)
	require.NotNil(t, err)

	mi, err = store.FindWithMapping(ctx, types.ModelRepo, "nsn", "repo1", types.HFMapping)
	require.Nil(t, err)
	require.Equal(t, "repo1", mi.Interval)

	mi, err = store.FindWithMapping(ctx, types.DatasetRepo, "nsn", "repo2", types.HFMapping)
	require.Nil(t, err)
	require.Equal(t, "repo2", mi.Interval)

	mi, err = store.FindWithMapping(ctx, types.PromptRepo, "nsn", "repo3", types.AutoMapping)
	require.Nil(t, err)
	require.Equal(t, "repo3", mi.Interval)
}

func TestMirrorStore_ToSync(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	dt := time.Now().Add(1 * time.Hour)
	mirrors := []*database.Mirror{
		{NextExecutionTimestamp: dt, Status: types.MirrorFailed, Interval: "m1"},
		{NextExecutionTimestamp: dt, Status: types.MirrorFinished, Interval: "m2"},
		{NextExecutionTimestamp: dt, Status: types.MirrorIncomplete, Interval: "m3"},
		{NextExecutionTimestamp: dt, Status: types.MirrorRepoSynced, Interval: "m4"},
		{NextExecutionTimestamp: dt, Status: types.MirrorRunning, Interval: "m5"},
		{NextExecutionTimestamp: dt, Status: types.MirrorWaiting, Interval: "m6"},
		{NextExecutionTimestamp: dt.Add(-5 * time.Hour), Status: types.MirrorFinished, Interval: "m7"},
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
	require.ElementsMatch(t, []string{"m1", "m3", "m5", "m6", "m7"}, names)

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

	ms, count, err := store.IndexWithPagination(ctx, 10, 1, "foo")
	require.Nil(t, err)
	names := []string{}
	for _, m := range ms {
		names = append(names, m.Interval)
	}
	require.Equal(t, 2, count)
	require.ElementsMatch(t, []string{"m1", "m2"}, names)

}

func TestMirrorStore_StatusCount(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMirrorStoreWithDB(db)

	mirrors := []*database.Mirror{
		{Interval: "m1", Status: types.MirrorFailed},
		{Interval: "m2", Status: types.MirrorFailed},
		{Interval: "m3", Status: types.MirrorFinished},
	}
	for _, m := range mirrors {
		_, err := store.Create(ctx, m)
		require.Nil(t, err)
	}

	cs, err := store.StatusCount(ctx)
	require.Nil(t, err)
	require.Equal(t, 2, len(cs))
	require.ElementsMatch(t, []database.MirrorStatusCount{
		{types.MirrorFailed, 2},
		{types.MirrorFinished, 1},
	}, cs)

}

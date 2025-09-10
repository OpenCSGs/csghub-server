package database_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)


func TestSyncVersionStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSyncVersionStoreWithDB(db)

	err := store.Create(ctx, &database.SyncVersion{
		Version:  1,
		SourceID: 123,
		RepoPath: "foo",
		RepoType: types.ModelRepo,
	})
	require.Nil(t, err)

	sv := &database.SyncVersion{}
	err = db.Core.NewSelect().Model(sv).Where("version=?", 1).Scan(ctx, sv)
	require.Nil(t, err)
	require.Equal(t, int64(123), sv.SourceID)

	sv, err = store.FindByPath(ctx, "foo")
	require.Nil(t, err)
	require.Equal(t, int64(1), sv.Version)

	sv, err = store.FindByRepoTypeAndPath(ctx, "foo", types.ModelRepo)
	require.Nil(t, err)
	require.Equal(t, int64(1), sv.Version)

	err = store.BatchCreate(ctx, []database.SyncVersion{
		{Version: 2, RepoPath: "bar"},
	})
	require.Nil(t, err)
	sv, err = store.FindByPath(ctx, "bar")
	require.Nil(t, err)
	require.Equal(t, int64(2), sv.Version)

}

func TestSyncVersionStore_BatchDeleteOthers(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSyncVersionStoreWithDB(db)

	err := store.Create(ctx, &database.SyncVersion{
		Version:  1,
		SourceID: 123,
		RepoPath: "foo/bar",
		RepoType: types.ModelRepo,
	})
	require.Nil(t, err)

	err = store.Create(ctx, &database.SyncVersion{
		Version:  2,
		SourceID: 123,
		RepoPath: "foo/bar1",
		RepoType: types.ModelRepo,
	})
	require.Nil(t, err)

	err = store.BatchDeleteOthers(ctx, types.ModelRepo, []string{"foo/bar1"})

	require.Nil(t, err)

	var svs []database.SyncVersion
	err = db.Operator.Core.NewSelect().Model(&svs).Scan(ctx)

	require.Nil(t, err)

	require.Equal(t, 1, len(svs))
	require.Equal(t, "foo/bar1", svs[0].RepoPath)
}

func TestSyncVersionStore_FindWithBatch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSyncVersionStoreWithDB(db)

	err := store.Create(ctx, &database.SyncVersion{
		Version:  1,
		SourceID: 123,
		RepoPath: "foo/bar",
		RepoType: types.ModelRepo,
	})
	require.Nil(t, err)

	err = store.Create(ctx, &database.SyncVersion{
		Version:  2,
		SourceID: 123,
		RepoPath: "foo/bar1",
		RepoType: types.ModelRepo,
	})
	require.Nil(t, err)

	svs, err := store.FindWithBatch(ctx, types.ModelRepo, 1, 1)

	require.Nil(t, err)

	require.Equal(t, 1, len(svs))
	require.Equal(t, "foo/bar1", svs[0].RepoPath)
}

func TestSyncVersionStore_DeleteOldVersions(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSyncVersionStoreWithDB(db)

	err := store.Create(ctx, &database.SyncVersion{
		Version:  1,
		SourceID: 123,
		RepoPath: "foo/bar1",
		RepoType: types.ModelRepo,
	})
	require.Nil(t, err)

	err = store.Create(ctx, &database.SyncVersion{
		Version:  2,
		SourceID: 123,
		RepoPath: "foo/bar1",
		RepoType: types.ModelRepo,
	})
	require.Nil(t, err)

	err = store.Create(ctx, &database.SyncVersion{
		Version:  3,
		SourceID: 123,
		RepoPath: "foo/bar1",
		RepoType: types.ModelRepo,
	})
	require.Nil(t, err)

	err = store.Create(ctx, &database.SyncVersion{
		Version:  5,
		SourceID: 123,
		RepoPath: "foo/bar2",
		RepoType: types.DatasetRepo,
	})
	require.Nil(t, err)

	err = store.Create(ctx, &database.SyncVersion{
		Version:  6,
		SourceID: 123,
		RepoPath: "foo/bar2",
		RepoType: types.DatasetRepo,
	})
	require.Nil(t, err)

	err = store.Create(ctx, &database.SyncVersion{
		Version:  7,
		SourceID: 123,
		RepoPath: "foo/bar3",
		RepoType: types.DatasetRepo,
	})
	require.Nil(t, err)

	err = store.DeleteOldVersions(ctx)

	require.Nil(t, err)

	var svs []database.SyncVersion
	err = db.Operator.Core.NewSelect().Model(&svs).Order("version DESC").Scan(ctx)

	require.Nil(t, err)

	require.Equal(t, 3, len(svs))
	require.Equal(t, "foo/bar3", svs[0].RepoPath)
	require.Equal(t, int64(7), svs[0].Version)
	require.Equal(t, "foo/bar2", svs[1].RepoPath)
	require.Equal(t, int64(6), svs[1].Version)
	require.Equal(t, "foo/bar1", svs[2].RepoPath)
	require.Equal(t, int64(3), svs[2].Version)
}

func TestSyncVersionStore_Complete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSyncVersionStoreWithDB(db)

	// Create test data with multiple versions for the same repository
	// and different repositories to test the Complete method behavior

	// Repository 1: source_id=1, repo_path="repo1", repo_type=ModelRepo
	err := store.Create(ctx, &database.SyncVersion{
		Version:   100,
		SourceID:  1,
		RepoPath:  "repo1",
		RepoType:  types.ModelRepo,
		Completed: false,
	})
	require.Nil(t, err)

	err = store.Create(ctx, &database.SyncVersion{
		Version:   110,
		SourceID:  1,
		RepoPath:  "repo1",
		RepoType:  types.ModelRepo,
		Completed: false,
	})
	require.Nil(t, err)

	err = store.Create(ctx, &database.SyncVersion{
		Version:   120,
		SourceID:  1,
		RepoPath:  "repo1",
		RepoType:  types.ModelRepo,
		Completed: false,
	})
	require.Nil(t, err)

	// Repository 2: same repo_path and repo_type but different source_id
	err = store.Create(ctx, &database.SyncVersion{
		Version:   105,
		SourceID:  2,
		RepoPath:  "repo1",
		RepoType:  types.ModelRepo,
		Completed: false,
	})
	require.Nil(t, err)

	// Repository 3: same source_id and repo_path but different repo_type
	err = store.Create(ctx, &database.SyncVersion{
		Version:   115,
		SourceID:  1,
		RepoPath:  "repo1",
		RepoType:  types.DatasetRepo,
		Completed: false,
	})
	require.Nil(t, err)

	// Repository 4: different repo_path
	err = store.Create(ctx, &database.SyncVersion{
		Version:   125,
		SourceID:  1,
		RepoPath:  "repo2",
		RepoType:  types.ModelRepo,
		Completed: false,
	})
	require.Nil(t, err)

	// Call Complete with version 110 for source_id=1, repo_path="repo1", repo_type=ModelRepo
	err = store.Complete(ctx, database.SyncVersion{
		Version:  110,
		SourceID: 1,
		RepoPath: "repo1",
		RepoType: types.ModelRepo,
	})
	require.Nil(t, err)

	// Verify that only versions <= 110 for the same source_id, repo_path, and repo_type are marked as completed
	var svs []database.SyncVersion
	err = db.Core.NewSelect().Model(&svs).Order("version").Scan(ctx)
	require.Nil(t, err)

	// Create a map for easier verification
	completedMap := make(map[string]bool)
	for _, sv := range svs {
		key := fmt.Sprintf("%d-%s-%s-%d", sv.SourceID, sv.RepoPath, sv.RepoType, sv.Version)
		completedMap[key] = sv.Completed
	}

	// Verify completion status
	require.True(t, completedMap["1-repo1-model-100"])   // should be completed (version <= 110)
	require.True(t, completedMap["1-repo1-model-110"])   // should be completed (version = 110)
	require.False(t, completedMap["1-repo1-model-120"])  // should NOT be completed (version > 110)
	require.False(t, completedMap["2-repo1-model-105"])  // should NOT be completed (different source_id)
	require.False(t, completedMap["1-repo1-dataset-115"]) // should NOT be completed (different repo_type)
	require.False(t, completedMap["1-repo2-model-125"])  // should NOT be completed (different repo_path)
}


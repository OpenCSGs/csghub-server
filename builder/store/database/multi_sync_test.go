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

func TestMultiSyncStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMultiSyncStoreWithDB(db)

	_, err := store.Create(ctx, database.SyncVersion{
		Version:  123,
		SourceID: 1,
		RepoPath: "a",
		RepoType: types.ModelRepo,
	})
	require.Nil(t, err)

	sv := &database.SyncVersion{}
	err = db.Core.NewSelect().Model(sv).Where("version=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 123, int(sv.Version))

	_, err = store.Create(ctx, database.SyncVersion{
		Version:  103,
		SourceID: 1,
		RepoPath: "a",
		RepoType: types.ModelRepo,
	})
	require.Nil(t, err)
	_, err = store.Create(ctx, database.SyncVersion{
		Version:  143,
		SourceID: 1,
		RepoPath: "a",
		RepoType: types.ModelRepo,
	})
	require.Nil(t, err)
	svs, err := store.GetAfter(ctx, 123, 1)
	require.Nil(t, err)
	require.Equal(t, len(svs), 1)
	require.Equal(t, 143, int(svs[0].Version))

	svv, err := store.GetLatest(ctx)
	require.Nil(t, err)
	require.Equal(t, 143, int(svv.Version))

	svs, err = store.GetAfterDistinct(ctx, 100)
	require.Nil(t, err)
	require.Equal(t, len(svs), 1)
	require.True(t, int(svs[0].Version) > 100)

}

func TestMultiSyncStore_GetNotCompletedDistinct(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMultiSyncStoreWithDB(db)

	// Create test data with different combinations of (source_id, repo_path, repo_type)
	// and different completion status

	// Group 1: source_id=1, repo_path="repo1", repo_type=ModelRepo
	// Create multiple versions, some completed, some not
	_, err := store.Create(ctx, database.SyncVersion{
		Version:   100,
		SourceID:  1,
		RepoPath:  "repo1",
		RepoType:  types.ModelRepo,
		Completed: true, // completed, should not be returned
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.SyncVersion{
		Version:   110,
		SourceID:  1,
		RepoPath:  "repo1",
		RepoType:  types.ModelRepo,
		Completed: false, // not completed, lower version
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.SyncVersion{
		Version:   120,
		SourceID:  1,
		RepoPath:  "repo1",
		RepoType:  types.ModelRepo,
		Completed: false, // not completed, highest version for this group
	})
	require.Nil(t, err)

	// Group 2: source_id=1, repo_path="repo2", repo_type=ModelRepo
	_, err = store.Create(ctx, database.SyncVersion{
		Version:   200,
		SourceID:  1,
		RepoPath:  "repo2",
		RepoType:  types.ModelRepo,
		Completed: false, // not completed, should be returned
	})
	require.Nil(t, err)

	// Group 3: source_id=2, repo_path="repo1", repo_type=ModelRepo (different source_id)
	_, err = store.Create(ctx, database.SyncVersion{
		Version:   150,
		SourceID:  2,
		RepoPath:  "repo1",
		RepoType:  types.ModelRepo,
		Completed: false, // not completed, should be returned
	})
	require.Nil(t, err)

	// Group 4: source_id=1, repo_path="repo1", repo_type=DatasetRepo (different repo_type)
	_, err = store.Create(ctx, database.SyncVersion{
		Version:   130,
		SourceID:  1,
		RepoPath:  "repo1",
		RepoType:  types.DatasetRepo,
		Completed: false, // not completed, should be returned
	})
	require.Nil(t, err)

	// Group 5: All completed, should not be returned
	_, err = store.Create(ctx, database.SyncVersion{
		Version:   300,
		SourceID:  3,
		RepoPath:  "repo3",
		RepoType:  types.ModelRepo,
		Completed: true, // completed, should not be returned
	})
	require.Nil(t, err)

	// Call GetNotCompletedDistinct
	results, err := store.GetNotCompletedDistinct(ctx)
	require.Nil(t, err)

	// Should return 4 records (one for each distinct group that has uncompleted versions)
	require.Equal(t, 4, len(results))

	// Create a map to verify the results
	resultMap := make(map[string]int64)
	for _, result := range results {
		key := fmt.Sprintf("%d-%s-%s", result.SourceID, result.RepoPath, result.RepoType)
		resultMap[key] = result.Version
		// All results should have Completed = false
		require.False(t, result.Completed)
	}

	// Verify that we get the maximum version for each group
	require.Equal(t, int64(120), resultMap["1-repo1-model"])   // highest version for group 1
	require.Equal(t, int64(200), resultMap["1-repo2-model"])   // only version for group 2
	require.Equal(t, int64(150), resultMap["2-repo1-model"])   // only version for group 3
	require.Equal(t, int64(130), resultMap["1-repo1-dataset"]) // only version for group 4
}

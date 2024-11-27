package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestRepoRuntimeFrameworkStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRepositoriesRuntimeFrameworkWithDB(db)
	err := store.Add(ctx, 123, 456, 1)
	require.Nil(t, err)

	rf := &database.RepositoriesRuntimeFramework{}
	err = db.Core.NewSelect().Model(rf).Where("repo_id=?", 456).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, 456, int(rf.RepoID))

	rfs, err := store.GetByIDsAndType(ctx, 123, 456, 1)
	require.Nil(t, err)
	require.Equal(t, 1, len(rfs))
	require.Equal(t, 456, int(rfs[0].RepoID))

	rfs, err = store.ListRepoIDsByType(ctx, 1)
	require.Nil(t, err)
	require.Equal(t, 1, len(rfs))
	require.Equal(t, 456, int(rfs[0].RepoID))
	rfs, err = store.ListRepoIDsByType(ctx, 2)
	require.Nil(t, err)
	require.Equal(t, 0, len(rfs))

	rfs, err = store.GetByRepoIDsAndType(ctx, 456, 1)
	require.Nil(t, err)
	require.Equal(t, 1, len(rfs))
	require.Equal(t, 456, int(rfs[0].RepoID))
	rfs, err = store.GetByRepoIDsAndType(ctx, 456, 2)
	require.Nil(t, err)
	require.Equal(t, 0, len(rfs))

	rfs, err = store.GetByRepoIDs(ctx, 456)
	require.Nil(t, err)
	require.Equal(t, 1, len(rfs))
	require.Equal(t, 456, int(rfs[0].RepoID))

	err = store.Delete(ctx, 123, 456, 1)
	require.Nil(t, err)
	rfs, err = store.GetByIDsAndType(ctx, 123, 456, 1)
	require.Nil(t, err)
	require.Equal(t, 0, len(rfs))

	err = store.Add(ctx, 123, 456, 1)
	require.Nil(t, err)
	err = store.DeleteByRepoID(ctx, 456)
	require.Nil(t, err)
	rfs, err = store.GetByIDsAndType(ctx, 123, 456, 1)
	require.Nil(t, err)
	require.Equal(t, 0, len(rfs))
}

package database_test

import (
	"context"
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

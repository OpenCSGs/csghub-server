package database_test

import (
	"context"
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

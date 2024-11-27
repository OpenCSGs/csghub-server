package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestLfsLockStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewLfsLockStoreWithDB(db)
	_, err := store.Create(ctx, database.LfsLock{
		RepositoryID: 123,
		Path:         "foo/bar",
	})
	require.Nil(t, err)

	lock := &database.LfsLock{}
	err = db.Core.NewSelect().Model(lock).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "foo/bar", lock.Path)

	lock, err = store.FindByID(ctx, lock.ID)
	require.Nil(t, err)
	require.Equal(t, "foo/bar", lock.Path)

	lock, err = store.FindByPath(ctx, 123, "foo/bar")
	require.Nil(t, err)
	require.Equal(t, "foo/bar", lock.Path)

	ls, err := store.FindByRepoID(ctx, 123, 1, 10)
	require.Nil(t, err)
	require.Equal(t, 1, len(ls))
	require.Equal(t, "foo/bar", ls[0].Path)

	err = store.RemoveByID(ctx, lock.ID)
	require.Nil(t, err)
	_, err = store.FindByID(ctx, lock.ID)
	require.NotNil(t, err)

}

package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestSyncClientSettingStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSyncClientSettingStoreWithDB(db)
	err := store.DeleteAll(ctx)
	require.Nil(t, err)

	_, err = store.Create(ctx, &database.SyncClientSetting{
		Token:           "tk",
		ConcurrentCount: 5,
	})
	require.Nil(t, err)

	sc := &database.SyncClientSetting{}
	err = db.Core.NewSelect().Model(sc).Where("token=?", "tk").Scan(ctx, sc)
	require.Nil(t, err)
	require.Equal(t, 5, sc.ConcurrentCount)

	sc, err = store.First(ctx)
	require.Nil(t, err)
	require.Equal(t, 5, sc.ConcurrentCount)

	exist, err := store.SyncClientSettingExists(ctx)
	require.Nil(t, err)
	require.True(t, exist)

	err = store.DeleteAll(ctx)
	require.Nil(t, err)
	exist, err = store.SyncClientSettingExists(ctx)
	require.Nil(t, err)
	require.False(t, exist)

}

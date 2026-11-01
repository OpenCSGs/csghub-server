package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestAccountSyncQuotaStore_CURD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountSyncQuotaStoreWithDB(db)

	err := store.Create(ctx, database.AccountSyncQuota{
		UserID:        123,
		RepoCountUsed: 6,
	})
	require.Nil(t, err)

	d := &database.AccountSyncQuota{}
	err = db.Core.NewSelect().Model(d).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, int64(6), d.RepoCountUsed)

	d, err = store.GetByID(ctx, 123)
	require.Nil(t, err)
	require.Equal(t, int64(6), d.RepoCountUsed)

	_, err = store.Update(ctx, database.AccountSyncQuota{
		UserID:        123,
		RepoCountUsed: 3,
	})
	require.Nil(t, err)
	d = &database.AccountSyncQuota{}
	err = db.Core.NewSelect().Model(d).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, int64(6), d.RepoCountUsed)

	_, err = store.Update(ctx, database.AccountSyncQuota{
		UserID:         123,
		RepoCountLimit: 3,
	})
	require.Nil(t, err)
	d = &database.AccountSyncQuota{}
	err = db.Core.NewSelect().Model(d).Where("user_id=?", 123).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, int64(3), d.RepoCountLimit)

	all, err := store.ListAllByUserID(ctx, 123)
	require.Nil(t, err)
	require.Equal(t, 1, len(all))
	require.Equal(t, int64(3), all[0].RepoCountLimit)

	err = store.Delete(ctx, database.AccountSyncQuota{
		UserID:        123,
		RepoCountUsed: 3,
	})
	require.Nil(t, err)
	d = &database.AccountSyncQuota{}
	err = db.Core.NewSelect().Model(d).Where("user_id=?", 123).Scan(ctx)
	require.NotNil(t, err)
}

func TestAccountSyncQuotaStore_IncreaseRepoLimit(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountSyncQuotaStoreWithDB(db)

	initialQuota := database.AccountSyncQuota{
		UserID:         456,
		RepoCountLimit: 10,
		RepoCountUsed:  0,
		SpeedLimit:     100,
		TrafficLimit:   1000,
		TrafficUsed:    0,
	}
	err := store.Create(ctx, initialQuota)
	require.NoError(t, err)

	increment := int64(5)
	err = store.IncreaseRepoLimit(ctx, initialQuota.UserID, increment)
	require.NoError(t, err)

	updatedQuota, err := store.GetByID(ctx, initialQuota.UserID)
	require.NoError(t, err)
	require.Equal(t, initialQuota.RepoCountLimit+increment, updatedQuota.RepoCountLimit)

	nonExistentUserID := int64(999999)
	err = store.IncreaseRepoLimit(ctx, nonExistentUserID, increment)
	require.NoError(t, err)
}

package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAccountAccessTokenQuotaStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	store := database.NewAccountAccessTokenQuotaStoreWithDB(db)

	quota := &database.AccountAccessTokenQuota{
		APIKey:      "test-api-key-123",
		QuotaType:   types.AccountingQuotaTypeMonthly,
		ValueType:   types.AccountingQuotaValueTypeFee,
		PeriodStart: time.Now().Unix(),
		PeriodEnd:   time.Now().Add(24 * time.Hour).Unix(),
		Usage:       0,
		Quota:      100,
	}

	err := store.Create(ctx, quota)
	require.Nil(t, err)
	require.Greater(t, quota.ID, int64(0))

	stored, err := store.GetByID(ctx, quota.ID)
	require.Nil(t, err)
	require.Equal(t, "test-api-key-123", stored.APIKey)
	require.Equal(t, types.AccountingQuotaTypeMonthly, stored.QuotaType)
}

func TestAccountAccessTokenQuotaStore_Update(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	store := database.NewAccountAccessTokenQuotaStoreWithDB(db)

	quota := &database.AccountAccessTokenQuota{
		APIKey: "test-api-key-update",
		QuotaType: types.AccountingQuotaTypeMonthly,
		ValueType: types.AccountingQuotaValueTypeFee,
		Usage:   0,
		Quota:   1000,
	}
	err := store.Create(ctx, quota)
	require.Nil(t, err)

	quota.Usage = 500
	quota.Quota = 2000
	err = store.Update(ctx, quota)
	require.Nil(t, err)

	updated, err := store.GetByID(ctx, quota.ID)
	require.Nil(t, err)
	require.Equal(t, float64(500), updated.Usage)
	require.Equal(t, float64(2000), updated.Quota)
}

func TestAccountAccessTokenQuotaStore_GetByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	store := database.NewAccountAccessTokenQuotaStoreWithDB(db)

	quota := &database.AccountAccessTokenQuota{
		APIKey:    "test-api-key-get",
		QuotaType: types.AccountingQuotaTypeMonthly,
		ValueType: types.AccountingQuotaValueTypeFee,
		Usage:    100,
		Quota:   500,
	}
	err := store.Create(ctx, quota)
	require.Nil(t, err)

	found, err := store.GetByID(ctx, quota.ID)
	require.Nil(t, err)
	require.Equal(t, "test-api-key-get", found.APIKey)
}

func TestAccountAccessTokenQuotaStore_GetByID_NotFound(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	store := database.NewAccountAccessTokenQuotaStoreWithDB(db)

	_, err := store.GetByID(ctx, 99999)
	require.NotNil(t, err)
}

func TestAccountAccessTokenQuotaStore_FindByAPIKey(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	store := database.NewAccountAccessTokenQuotaStoreWithDB(db)

	apiKey := "test-find-apikey"
	quota1 := &database.AccountAccessTokenQuota{
		APIKey:    apiKey,
		QuotaType: types.AccountingQuotaTypeMonthly,
		ValueType: types.AccountingQuotaValueTypeFee,
		Usage:    10,
		Quota:    50,
	}
	quota2 := &database.AccountAccessTokenQuota{
		APIKey:    apiKey,
		QuotaType: types.AccountingQuotaTotal,
		ValueType: types.AccountingQuotaValueTypeFee,
		Usage:    20,
		Quota:    100,
	}
	err := store.Create(ctx, quota1)
	require.Nil(t, err)
	err = store.Create(ctx, quota2)
	require.Nil(t, err)

	quotas, err := store.FindByAPIKey(ctx, apiKey)
	require.Nil(t, err)
	require.Len(t, quotas, 2)
}

func TestAccountAccessTokenQuotaStore_FindByAPIKey_NotFound(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	store := database.NewAccountAccessTokenQuotaStoreWithDB(db)

	quotas, err := store.FindByAPIKey(ctx, "non-existent-key")
	require.Nil(t, err)
	require.Len(t, quotas, 0)
}

func TestAccountAccessTokenQuotaStore_DeleteByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	store := database.NewAccountAccessTokenQuotaStoreWithDB(db)

	quota := &database.AccountAccessTokenQuota{
		APIKey:    "test-delete-by-id",
		QuotaType: types.AccountingQuotaTypeMonthly,
		ValueType: types.AccountingQuotaValueTypeFee,
		Usage:    0,
		Quota:    10,
	}
	err := store.Create(ctx, quota)
	require.Nil(t, err)

	err = store.DeleteByID(ctx, quota.ID)
	require.Nil(t, err)

	_, err = store.GetByID(ctx, quota.ID)
	require.NotNil(t, err)
}

func TestAccountAccessTokenQuotaStore_DeleteByAPIKey(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	store := database.NewAccountAccessTokenQuotaStoreWithDB(db)

	apiKey := "test-delete-by-apikey"
	quota1 := &database.AccountAccessTokenQuota{
		APIKey:    apiKey,
		QuotaType: types.AccountingQuotaTypeMonthly,
		ValueType: types.AccountingQuotaValueTypeFee,
		Usage:    0,
		Quota:    10,
	}
	quota2 := &database.AccountAccessTokenQuota{
		APIKey:    apiKey,
		QuotaType: types.AccountingQuotaTotal,
		ValueType: types.AccountingQuotaValueTypeFee,
		Usage:    0,
		Quota:    100,
	}
	err := store.Create(ctx, quota1)
	require.Nil(t, err)
	err = store.Create(ctx, quota2)
	require.Nil(t, err)

	err = store.DeleteByAPIKey(ctx, apiKey)
	require.Nil(t, err)

	quotas, err := store.FindByAPIKey(ctx, apiKey)
	require.Nil(t, err)
	require.Len(t, quotas, 0)
}

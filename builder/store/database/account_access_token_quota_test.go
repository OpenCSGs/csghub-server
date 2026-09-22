package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
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
		Quota:       100,
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
		APIKey:    "test-api-key-update",
		QuotaType: types.AccountingQuotaTypeMonthly,
		ValueType: types.AccountingQuotaValueTypeFee,
		Usage:     0,
		Quota:     1000,
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
		Usage:     100,
		Quota:     500,
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

func TestAccountAccessTokenQuotaStore_FindByTokenID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	store := database.NewAccountAccessTokenQuotaStoreWithDB(db)

	tokenID := int64(700)
	quota1 := &database.AccountAccessTokenQuota{
		APIKey:    "token-700-a",
		TokenID:   tokenID,
		QuotaType: types.AccountingQuotaTypeMonthly,
		ValueType: types.AccountingQuotaValueTypeFee,
		Usage:     10,
		Quota:     50,
	}
	quota2 := &database.AccountAccessTokenQuota{
		APIKey:    "token-700-b",
		TokenID:   tokenID,
		QuotaType: types.AccountingQuotaTotal,
		ValueType: types.AccountingQuotaValueTypeFee,
		Usage:     20,
		Quota:     100,
	}
	err := store.Create(ctx, quota1)
	require.Nil(t, err)
	err = store.Create(ctx, quota2)
	require.Nil(t, err)

	quotas, err := store.FindByTokenID(ctx, tokenID)
	require.Nil(t, err)
	require.Len(t, quotas, 2)
}

func TestAccountAccessTokenQuotaStore_FindByTokenID_NotFound(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	store := database.NewAccountAccessTokenQuotaStoreWithDB(db)

	quotas, err := store.FindByTokenID(ctx, 99999)
	require.Nil(t, err)
	require.Len(t, quotas, 0)
}

func TestUpdateAPIKeyUsage_ByTokenID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	atStore := database.NewAccessTokenStoreWithDB(db)
	quotaStore := database.NewAccountAccessTokenQuotaStoreWithDB(db)

	token := &database.AccessToken{
		GitID:       1234,
		Name:        "usage-token",
		Token:       "usage-token-value",
		UserID:      1,
		Application: types.AccessTokenAppAIGateway,
		NsUUID:      "usage-ns-uuid",
		IsActive:    true,
	}
	quotas := []database.AccountAccessTokenQuota{
		{
			APIKey:    token.Token,
			QuotaType: types.AccountingQuotaTypeMonthly,
			ValueType: types.AccountingQuotaValueTypeFee,
			Quota:     100.0,
		},
		{
			APIKey:    token.Token,
			QuotaType: types.AccountingQuotaTotal,
			ValueType: types.AccountingQuotaValueTypeFee,
			Quota:     500.0,
		},
	}
	err := atStore.Create(ctx, token, quotas)
	require.Nil(t, err)
	require.NotZero(t, token.ID)

	// Refresh the key value so the quota rows' api_key no longer matches the
	// statement input; token_id must still route the usage to these rows.
	newKeyValue := "usage-token-value-refreshed"
	token.Token = newKeyValue
	err = atStore.UpdateToken(ctx, token)
	require.Nil(t, err)

	statement := database.AccountStatement{
		APIKey:  "stale-key-value",
		TokenID: token.ID,
		Value:   -12.5,
	}
	err = db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return database.UpdateAPIKeyUsage(ctx, tx, statement)
	})
	require.Nil(t, err)

	savedQuotas, err := quotaStore.FindByTokenID(ctx, token.ID)
	require.Nil(t, err)
	require.Len(t, savedQuotas, 2)
	for _, quota := range savedQuotas {
		require.Equal(t, newKeyValue, quota.APIKey)
		// Statement values for consumption are negative, so Usage accumulates
		// negatively; consumers apply math.Abs before comparing with Quota.
		require.Equal(t, -12.5, quota.Usage)
		require.NotNil(t, quota.LastUsedAt)
	}
}

func TestUpdateAPIKeyUsage_ZeroTokenIDNoOp(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()

	// Legacy-shaped quota row without a token_id association.
	quota := &database.AccountAccessTokenQuota{
		APIKey:    "legacy-key-value",
		QuotaType: types.AccountingQuotaTypeMonthly,
		ValueType: types.AccountingQuotaValueTypeFee,
		Quota:     100.0,
	}
	err := database.NewAccountAccessTokenQuotaStoreWithDB(db).Create(ctx, quota)
	require.Nil(t, err)

	// Statements without a token_id are ignored: token_id is the only join
	// key used to attribute usage to quota rows, so a TokenID=0 statement
	// must not update anything (including by api_key fallback).
	statement := database.AccountStatement{
		APIKey:  "legacy-key-value",
		TokenID: 0,
		Value:   -3.5,
	}
	err = db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return database.UpdateAPIKeyUsage(ctx, tx, statement)
	})
	require.Nil(t, err)

	stored, err := database.NewAccountAccessTokenQuotaStoreWithDB(db).GetByID(ctx, quota.ID)
	require.Nil(t, err)
	require.Equal(t, 0.0, stored.Usage)
	require.Nil(t, stored.LastUsedAt)
}

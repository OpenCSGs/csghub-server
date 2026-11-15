package database_test

import (
	"context"
	"strings"
	"sync"
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
	assertStoredQuotaCredential(t, ctx, db, quota)
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

	quota.APIKey = "replacement-api-key"
	quota.Usage = 500
	quota.Quota = 2000
	err = store.Update(ctx, quota)
	require.Nil(t, err)

	updated, err := store.GetByID(ctx, quota.ID)
	require.Nil(t, err)
	require.Equal(t, float64(500), updated.Usage)
	require.Equal(t, float64(2000), updated.Quota)
	require.Equal(t, "replacement-api-key", updated.APIKey)
	assertStoredQuotaCredential(t, ctx, db, quota)
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
		assertStoredQuotaCredential(t, ctx, db, &quota)
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

// quotaUsageContextKey marks the metering transaction that the test pauses.
type quotaUsageContextKey struct{}

// pauseQuotaUsageHook pauses metering after its read, before acquiring an update lock.
type pauseQuotaUsageHook struct {
	reached chan struct{}
	release chan struct{}
	once    sync.Once
}

// BeforeQuery lets a real key refresh commit before the metering update executes.
func (h *pauseQuotaUsageHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if ctx.Value(quotaUsageContextKey{}) != true || !strings.Contains(event.Query, `UPDATE "account_access_token_quota"`) {
		return ctx
	}
	h.once.Do(func() {
		close(h.reached)
		select {
		case <-h.release:
		case <-ctx.Done():
		}
	})
	return ctx
}

// AfterQuery completes the query-hook contract without changing query results.
func (*pauseQuotaUsageHook) AfterQuery(context.Context, *bun.QueryEvent) {}

// TestUpdateAPIKeyUsage_PreservesConcurrentKeyRefresh fixes the transaction order:
// metering reads the old key, refresh commits, and metering commits last.
func TestUpdateAPIKeyUsage_PreservesConcurrentKeyRefresh(t *testing.T) {
	db := tests.InitTransactionTestDBWithHooks()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tokens := database.NewAccessTokenStoreWithDB(db)
	quotaStore := database.NewAccountAccessTokenQuotaStoreWithDB(db)
	token := &database.AccessToken{
		Name: "usage-concurrent-refresh", Token: "usage-old-key", UserID: 1,
		Application: types.AccessTokenAppAIGateway, TokenType: types.AccessTokenTypeBuiltIn, IsActive: true,
	}
	quotas := []database.AccountAccessTokenQuota{
		{
			APIKey: token.Token, QuotaType: types.AccountingQuotaTypeMonthly,
			ValueType: types.AccountingQuotaValueTypeFee, Usage: -7, Quota: 1000,
			PeriodStart: 1, PeriodEnd: 2, Allocation: "monthly-allocation",
		},
		{
			APIKey: token.Token, QuotaType: types.AccountingQuotaTotal,
			ValueType: types.AccountingQuotaValueTypeToken, Usage: -5, Quota: 2000,
			Allocation: "total-allocation",
		},
	}
	require.NoError(t, tokens.Create(ctx, token, quotas))

	hook := &pauseQuotaUsageHook{reached: make(chan struct{}), release: make(chan struct{})}
	db.BunDB.AddQueryHook(hook)
	var releaseOnce sync.Once
	resume := func() { releaseOnce.Do(func() { close(hook.release) }) }
	usageDone := make(chan error, 1)
	usageExited := make(chan struct{})
	// Release and join the writer even if refresh or an assertion fails.
	defer func() {
		resume()
		cancel()
		select {
		case <-usageExited:
		case <-time.After(5 * time.Second):
		}
	}()
	statement := database.AccountStatement{TokenID: token.ID, APIKey: token.Token, Value: -12.5, Consumption: -17}
	go func() {
		defer close(usageExited)
		usageCtx := context.WithValue(ctx, quotaUsageContextKey{}, true)
		usageDone <- db.Core.RunInTx(usageCtx, nil, func(ctx context.Context, tx bun.Tx) error {
			return database.UpdateAPIKeyUsage(ctx, tx, statement)
		})
	}()
	select {
	case <-hook.reached:
	case err := <-usageDone:
		t.Fatalf("metering ended before refresh: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	// A second real transaction commits both credential copies while metering waits.
	token.Token = "usage-new-key"
	require.NoError(t, tokens.UpdateToken(ctx, token))
	refreshed, err := quotaStore.FindByTokenID(ctx, token.ID)
	require.NoError(t, err)
	require.Len(t, refreshed, 2)
	resume()
	select {
	case err := <-usageDone:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	for _, expected := range refreshed {
		actual, err := quotaStore.GetByID(ctx, expected.ID)
		require.NoError(t, err)
		require.Equal(t, token.Token, actual.APIKey)
		require.Equal(t, expected.EncryptedAPIKey, actual.EncryptedAPIKey)
		require.Equal(t, expected.CredentialVersion, actual.CredentialVersion)
		require.NotNil(t, actual.LastUsedAt)
		require.False(t, actual.UpdatedAt.Before(expected.UpdatedAt))
		if actual.QuotaType == types.AccountingQuotaTypeMonthly {
			require.Equal(t, -12.5, actual.Usage)
			require.LessOrEqual(t, actual.PeriodStart, actual.LastUsedAt.Unix())
			require.GreaterOrEqual(t, actual.PeriodEnd, actual.LastUsedAt.Unix())
		} else {
			require.Equal(t, -22.0, actual.Usage)
			require.Equal(t, expected.PeriodStart, actual.PeriodStart)
			require.Equal(t, expected.PeriodEnd, actual.PeriodEnd)
		}
		// Only metering fields may differ from the post-refresh snapshot.
		preserved := *actual
		preserved.Usage = expected.Usage
		preserved.PeriodStart = expected.PeriodStart
		preserved.PeriodEnd = expected.PeriodEnd
		preserved.LastUsedAt = expected.LastUsedAt
		preserved.UpdatedAt = expected.UpdatedAt
		require.Equal(t, expected, preserved)
	}
	savedToken, err := tokens.GetByID(ctx, token.ID)
	require.NoError(t, err)
	require.Equal(t, token.Token, savedToken.Token)
}

package database_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/migrate"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database/migrations"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAccessTokenStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	atStore := database.NewAccessTokenStoreWithDB(db)

	token := &database.AccessToken{
		GitID:       1234,
		Name:        "test-token",
		Token:       "abcd1234",
		UserID:      1,
		Application: types.AccessTokenApp("test-app"),
		Permission:  "read-write",
		ExpiredAt:   time.Now().Add(24 * time.Hour),
		IsActive:    true,
	}

	err := atStore.Create(ctx, token, nil)
	require.Nil(t, err)

	storedToken, err := atStore.FindByID(ctx, token.ID)
	require.Nil(t, err)
	require.Equal(t, token.Token, storedToken.Token)
	require.True(t, storedToken.IsActive)
}

func TestAccessTokenStore_Refresh(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	atStore := database.NewAccessTokenStoreWithDB(db)

	oldToken := &database.AccessToken{
		GitID:       1234,
		Name:        "test-token",
		Token:       "abcd1234",
		UserID:      1,
		Application: types.AccessTokenApp("test-app"),
		Permission:  "read-write",
		ExpiredAt:   time.Now().Add(24 * time.Hour),
		IsActive:    true,
	}

	err := atStore.Create(ctx, oldToken, nil)
	require.Nil(t, err)

	newTokenValue := "xyz7890"
	newExpiredAt := time.Now().Add(48 * time.Hour)

	newToken, err := atStore.Refresh(ctx, oldToken, newTokenValue, newExpiredAt)
	require.Nil(t, err)
	require.NotNil(t, newToken)
	require.Equal(t, newTokenValue, newToken.Token)
	require.True(t, newToken.IsActive)

	// Verify old token is inactive
	storedOldToken, err := atStore.FindByID(ctx, oldToken.ID)
	require.Nil(t, err)
	require.False(t, storedOldToken.IsActive)
}

func TestAccessTokenStore_Delete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	atStore := database.NewAccessTokenStoreWithDB(db)
	userStore := database.NewUserStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))

	user := &database.User{
		NickName: "nickname",
		Email:    "test-user@example.com",
		Username: "username",
	}
	namespace := &database.Namespace{
		Path: "username",
		User: *user,
	}
	err := userStore.Create(ctx, user, namespace)
	require.Nil(t, err)

	token := &database.AccessToken{
		GitID:       1234,
		Name:        "delete-token",
		Token:       "to-delete",
		UserID:      user.ID,
		Application: types.AccessTokenApp("test-app"),
		Permission:  "read-write",
		IsActive:    true,
	}

	err = atStore.Create(ctx, token, nil)
	require.Nil(t, err)

	err = atStore.Delete(ctx, "username", "delete-token", "test-app")
	require.Nil(t, err)

	_, err = atStore.FindByID(ctx, token.ID)
	require.NotNil(t, err)
}

func TestAccessTokenStore_IsExist(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	atStore := database.NewAccessTokenStoreWithDB(db)
	userStore := database.NewUserStoreWithDBAndJobClients(db, nil, tests.NewFixtureRebacTupleJobClient(db))

	user := &database.User{
		NickName: "nickname1",
		Email:    "test-user1@example.com",
		Username: "username1",
	}
	namespace := &database.Namespace{
		Path: "username1",
		User: *user,
	}
	err := userStore.Create(ctx, user, namespace)
	require.Nil(t, err)

	token := &database.AccessToken{
		GitID:       1234,
		Name:        "exist-token",
		Token:       "exists",
		UserID:      user.ID,
		Application: types.AccessTokenApp("test-app"),
		IsActive:    true,
	}

	err = atStore.Create(ctx, token, nil)
	require.Nil(t, err)

	exists, err := atStore.IsExist(ctx, "username1", "exist-token", "test-app")
	require.Nil(t, err)
	require.True(t, exists)

	exists, err = atStore.IsExist(ctx, "username1", "nonexistent-token", "test-app")
	require.Nil(t, err)
	require.False(t, exists)
}

func TestAccessTokenStore_FindByUID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	atStore := database.NewAccessTokenStoreWithDB(db)

	token := &database.AccessToken{
		GitID:       1234,
		Name:        "uid-token",
		Token:       "uid1234",
		UserID:      1,
		Application: types.AccessTokenApp("git"),
		IsActive:    true,
	}

	err := atStore.Create(ctx, token, nil)
	require.Nil(t, err)

	storedToken, err := atStore.FindByUID(ctx, token.UserID)
	require.Nil(t, err)
	require.Equal(t, token.Token, storedToken.Token)
	require.True(t, storedToken.IsActive)
}

func TestAccessTokenStore_FindByToken(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	atStore := database.NewAccessTokenStoreWithDB(db)

	token := &database.AccessToken{
		GitID:       1234,
		Name:        "find-token",
		Token:       "find-me",
		UserID:      1,
		Application: types.AccessTokenApp("test-app"),
		IsActive:    true,
	}

	err := atStore.Create(ctx, token, nil)
	require.Nil(t, err)

	storedToken, err := atStore.FindByToken(ctx, "find-me", "test-app")
	require.Nil(t, err)
	require.Equal(t, token.Token, storedToken.Token)
}

func TestAccessTokenStore_IsExistByUUID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	atStore := database.NewAccessTokenStoreWithDB(db)

	token := &database.AccessToken{
		GitID:       1234,
		Name:        "uuid-token",
		Token:       "uuid-token-value",
		UserID:      1,
		Application: types.AccessTokenAppAIGateway,
		NsUUID:      "test-ns-uuid",
		IsActive:    true,
	}

	err := atStore.Create(ctx, token, nil)
	require.Nil(t, err)

	exists, err := atStore.IsExistByUUID(ctx, "test-ns-uuid", "uuid-token", string(types.AccessTokenAppAIGateway))
	require.Nil(t, err)
	require.True(t, exists)

	exists, err = atStore.IsExistByUUID(ctx, "test-ns-uuid", "nonexistent-token", string(types.AccessTokenAppAIGateway))
	require.Nil(t, err)
	require.False(t, exists)

	exists, err = atStore.IsExistByUUID(ctx, "nonexistent-uuid", "uuid-token", string(types.AccessTokenAppAIGateway))
	require.Nil(t, err)
	require.False(t, exists)
}

func TestAccessTokenStore_UpdateTokenAndQuota(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	atStore := database.NewAccessTokenStoreWithDB(db)
	quotaStore := database.NewAccountAccessTokenQuotaStoreWithDB(db)

	token := &database.AccessToken{
		GitID:       1234,
		Name:        "update-token",
		Token:       "update-token-value",
		UserID:      1,
		Application: types.AccessTokenAppAIGateway,
		NsUUID:      "update-ns-uuid",
		IsActive:    true,
	}

	quotas := []database.AccountAccessTokenQuota{
		{
			APIKey:    token.Token,
			QuotaType: types.AccountingQuotaTypeMonthly,
			ValueType: types.AccountingQuotaValueTypeFee,
			Quota:     100.0,
		},
	}

	err := atStore.Create(ctx, token, quotas)
	require.Nil(t, err)
	// token.ID is backfilled into each quota inside Create's transaction.
	require.NotZero(t, token.ID)
	require.Equal(t, token.ID, quotas[0].TokenID)

	// Update token name
	newName := "updated-token-name"
	token.Name = newName

	quotas[0].Quota = 200.0

	updatedToken, err := atStore.UpdateTokenAndQuota(ctx, token, &quotas[0])
	require.Nil(t, err)
	require.NotNil(t, updatedToken)
	require.Equal(t, newName, updatedToken.Name)

	// Verify quota updated
	savedQuotas, err := quotaStore.FindByTokenID(ctx, token.ID)
	require.Nil(t, err)
	require.Len(t, savedQuotas, 1)
	require.Equal(t, 200.0, savedQuotas[0].Quota)
	assertEncryptedTokenCredential(t, ctx, db, token)
	assertStoredQuotaCredential(t, ctx, db, &savedQuotas[0])
}

// quotaRefreshContextKey restricts the pause hook to the refresh transaction.
type quotaRefreshContextKey struct{}

// pauseQuotaRefreshHook lets another connection commit after refresh reads the
// quotas but before its first quota update. Channels make the interleaving deterministic.
type pauseQuotaRefreshHook struct {
	reached chan struct{}
	release chan struct{}
	once    sync.Once
}

// BeforeQuery pauses only credential refresh, leaving concurrent quota writes free.
func (h *pauseQuotaRefreshHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if ctx.Value(quotaRefreshContextKey{}) != true || !strings.Contains(event.Query, `UPDATE "account_access_token_quota"`) {
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

// AfterQuery completes the Bun query-hook contract without changing results.
func (*pauseQuotaRefreshHook) AfterQuery(context.Context, *bun.QueryEvent) {}

// TestAccessTokenStore_UpdateToken_PreservesConcurrentQuotaChanges proves that
// refreshing a key cannot overwrite usage or settings committed after its read.
func TestAccessTokenStore_UpdateToken_PreservesConcurrentQuotaChanges(t *testing.T) {
	db := tests.InitTransactionTestDBWithHooks()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := database.NewAccessTokenStoreWithDB(db)
	quotaStore := database.NewAccountAccessTokenQuotaStoreWithDB(db)
	token := &database.AccessToken{
		Name: "concurrent-key-refresh", Token: "concurrent-old-key", UserID: 1,
		Application: types.AccessTokenAppAIGateway, IsActive: true,
	}
	quotas := []database.AccountAccessTokenQuota{{
		APIKey: token.Token, QuotaType: types.AccountingQuotaTypeMonthly,
		ValueType: types.AccountingQuotaValueTypeFee, Usage: 100, Quota: 1000,
		PeriodStart: 100, PeriodEnd: 200, Allocation: "original-allocation",
	}}
	require.NoError(t, store.Create(ctx, token, quotas))
	initial, err := quotaStore.FindByTokenID(ctx, token.ID)
	require.NoError(t, err)
	require.Len(t, initial, 1)

	hook := &pauseQuotaRefreshHook{reached: make(chan struct{}), release: make(chan struct{})}
	db.BunDB.AddQueryHook(hook)
	var releaseOnce sync.Once
	resume := func() { releaseOnce.Do(func() { close(hook.release) }) }
	defer resume()
	refreshDone := make(chan error, 1)
	refreshExited := make(chan struct{})
	token.Token = "concurrent-new-key"
	go func() {
		defer close(refreshExited)
		refreshDone <- store.UpdateToken(context.WithValue(ctx, quotaRefreshContextKey{}, true), token)
	}()
	// Always release and join the writer before closing its database, even if
	// an assertion on the concurrent connection fails.
	defer func() {
		resume()
		cancel()
		select {
		case <-refreshExited:
		case <-time.After(5 * time.Second):
		}
	}()
	select {
	case <-hook.reached:
	case err := <-refreshDone:
		t.Fatalf("refresh ended before the concurrent write: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	// This statement uses another real connection and commits before refresh
	// resumes. Explicit columns model metering and independent quota management.
	lastUsed := time.Unix(1700000000, 0).UTC()
	_, err = db.Core.NewUpdate().Model((*database.AccountAccessTokenQuota)(nil)).
		Set("usage = usage + ?", 10).
		Set("quota = ?", 2000).
		Set("period_start = ?", 300).
		Set("period_end = ?", 400).
		Set("allocation = ?", "concurrent-allocation").
		Set("last_used_at = ?", lastUsed).
		Set("updated_at = ?", lastUsed).
		Where("id = ?", initial[0].ID).Exec(ctx)
	require.NoError(t, err)
	expected, err := quotaStore.FindByTokenID(ctx, token.ID)
	require.NoError(t, err)
	require.Len(t, expected, 1)
	require.Equal(t, float64(110), expected[0].Usage)

	resume()
	select {
	case err := <-refreshDone:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	actual, err := quotaStore.FindByTokenID(ctx, token.ID)
	require.NoError(t, err)
	require.Len(t, actual, 1)
	require.Equal(t, token.Token, actual[0].APIKey)
	require.NotEqual(t, token.Token, actual[0].EncryptedAPIKey)
	require.Equal(t, 1, actual[0].CredentialVersion)
	// Normalize only credential storage fields; every business field, identity
	// and timestamp must still equal the independently committed quota record.
	preserved := actual[0]
	preserved.APIKey = expected[0].APIKey
	preserved.EncryptedAPIKey = expected[0].EncryptedAPIKey
	preserved.CredentialVersion = expected[0].CredentialVersion
	require.Equal(t, expected[0], preserved)
	_, err = store.FindByToken(ctx, token.Token, string(types.AccessTokenAppAIGateway))
	require.NoError(t, err)
}

func TestAccessTokenStore_UpdateToken_SyncsQuotaAPIKey(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	atStore := database.NewAccessTokenStoreWithDB(db)
	quotaStore := database.NewAccountAccessTokenQuotaStoreWithDB(db)

	token := &database.AccessToken{
		GitID:       1234,
		Name:        "refresh-builtin-token",
		Token:       "old-key-value",
		UserID:      1,
		Application: types.AccessTokenAppAIGateway,
		NsUUID:      "refresh-ns-uuid",
		IsActive:    true,
	}

	quotas := []database.AccountAccessTokenQuota{
		{
			APIKey:    token.Token,
			QuotaType: types.AccountingQuotaTypeMonthly,
			ValueType: types.AccountingQuotaValueTypeFee,
			Quota:     100.0,
		},
	}

	err := atStore.Create(ctx, token, quotas)
	require.Nil(t, err)
	require.NotZero(t, token.ID)

	// Refresh the token value, like RefreshToken's builtin branch does.
	newKeyValue := "new-key-value"
	token.Token = newKeyValue
	err = atStore.UpdateToken(ctx, token)
	require.Nil(t, err)

	// The quota rows should now be joined to the new key value via token_id,
	// so lookups by the old token id still find the row with the new api_key.
	newQuotas, err := quotaStore.FindByTokenID(ctx, token.ID)
	require.Nil(t, err)
	require.Len(t, newQuotas, 1)
	require.Equal(t, newKeyValue, newQuotas[0].APIKey)
	require.Equal(t, 100.0, newQuotas[0].Quota)
	assertEncryptedTokenCredential(t, ctx, db, token)
	assertStoredQuotaCredential(t, ctx, db, &newQuotas[0])
	_, err = atStore.FindByToken(ctx, "old-key-value", "aigateway")
	require.Error(t, err)
}

func TestAccessTokenStore_UpdateTokenAndQuotas_InsertsMissingQuotas(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	atStore := database.NewAccessTokenStoreWithDB(db)
	quotaStore := database.NewAccountAccessTokenQuotaStoreWithDB(db)

	token := &database.AccessToken{
		GitID:       1234,
		Name:        "update-missing-quota-token",
		Token:       "update-missing-quota-token-value",
		UserID:      1,
		Application: types.AccessTokenAppAIGateway,
		NsUUID:      "update-missing-quota-ns-uuid",
		IsActive:    true,
	}

	err := atStore.Create(ctx, token, nil)
	require.Nil(t, err)
	require.NotZero(t, token.ID)

	existingQuota := &database.AccountAccessTokenQuota{
		APIKey:    token.Token,
		TokenID:   token.ID,
		QuotaType: types.AccountingQuotaTypeMonthly,
		ValueType: types.AccountingQuotaValueTypeFee,
		Quota:     100.0,
	}
	require.Nil(t, quotaStore.Create(ctx, existingQuota))

	existingQuota.Quota = 150.0
	// The value types must differ because CheckQuotaSet allows at most one
	// record per value type (token allows two) within a token's quota set.
	missingQuota := &database.AccountAccessTokenQuota{
		APIKey:    token.Token,
		TokenID:   token.ID,
		QuotaType: types.AccountingQuotaTotal,
		ValueType: types.AccountingQuotaValueTypeToken,
		Quota:     300.0,
	}
	token.Name = "updated-missing-quota-token"

	updatedToken, err := atStore.UpdateTokenAndQuotas(ctx, token, []*database.AccountAccessTokenQuota{existingQuota, missingQuota})
	require.Nil(t, err)
	require.NotNil(t, updatedToken)
	require.Equal(t, token.Name, updatedToken.Name)
	require.NotZero(t, missingQuota.ID)

	savedQuotas, err := quotaStore.FindByTokenID(ctx, token.ID)
	require.Nil(t, err)
	require.Len(t, savedQuotas, 2)
	require.Equal(t, 150.0, savedQuotas[1].Quota)
	require.Equal(t, 300.0, savedQuotas[0].Quota)
	assertEncryptedTokenCredential(t, ctx, db, token)
	for i := range savedQuotas {
		assertStoredQuotaCredential(t, ctx, db, &savedQuotas[i])
	}
}

func TestAccessTokenStore_DeleteByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	atStore := database.NewAccessTokenStoreWithDB(db)

	token := &database.AccessToken{
		GitID:       1234,
		Name:        "delete-by-id-token",
		Token:       "delete-by-id-value",
		UserID:      1,
		Application: types.AccessTokenApp("test-app"),
		IsActive:    true,
	}

	err := atStore.Create(ctx, token, nil)
	require.Nil(t, err)

	// Verify token exists and is active
	storedToken, err := atStore.FindByID(ctx, token.ID)
	require.Nil(t, err)
	require.True(t, storedToken.IsActive)

	// Delete by ID
	err = atStore.DeleteByID(ctx, token.ID)
	require.Nil(t, err)

	// Verify token is inactive (soft delete)
	deletedToken, err := atStore.GetByID(ctx, token.ID)
	require.Nil(t, err)
	require.False(t, deletedToken.IsActive)
}

func TestAccessTokenStore_FindByNsUUID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	atStore := database.NewAccessTokenStoreWithDB(db)

	nsUUID := "test-ns-uuid-find"

	// Create multiple tokens for the same namespace
	token1 := &database.AccessToken{
		GitID:       1234,
		Name:        "ns-token-1",
		Token:       "ns-token-value-1",
		UserID:      1,
		Application: types.AccessTokenAppAIGateway,
		NsUUID:      nsUUID,
		IsActive:    true,
	}

	token2 := &database.AccessToken{
		GitID:       1235,
		Name:        "ns-token-2",
		Token:       "ns-token-value-2",
		UserID:      1,
		Application: types.AccessTokenAppAIGateway,
		NsUUID:      nsUUID,
		IsActive:    true,
	}

	// A disabled key is kept in the namespace list (so the UI can show its
	// status); only soft-deleted keys are hidden.
	token3 := &database.AccessToken{
		GitID:       1236,
		Name:        "ns-token-3",
		Token:       "ns-token-value-3",
		UserID:      1,
		Application: types.AccessTokenAppAIGateway,
		NsUUID:      nsUUID,
		IsActive:    false,
	}

	err := atStore.Create(ctx, token1, nil)
	require.Nil(t, err)
	err = atStore.Create(ctx, token2, nil)
	require.Nil(t, err)
	err = atStore.Create(ctx, token3, nil)
	require.Nil(t, err)

	// Find tokens by namespace UUID
	tokens, err := atStore.FindByNsUUID(ctx, nsUUID, string(types.AccessTokenAppAIGateway))
	require.Nil(t, err)
	require.Len(t, tokens, 3)

	// Verify tokens are returned in descending order by ID
	require.Equal(t, token3.ID, tokens[0].ID)
	require.Equal(t, token2.ID, tokens[1].ID)
	require.Equal(t, token1.ID, tokens[2].ID)

	// Find with non-existent namespace UUID
	tokens, err = atStore.FindByNsUUID(ctx, "nonexistent-uuid", string(types.AccessTokenAppAIGateway))
	require.Nil(t, err)
	require.Empty(t, tokens)
}

func TestAccessTokenStore_SetActiveByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.TODO()
	atStore := database.NewAccessTokenStoreWithDB(db)

	token := &database.AccessToken{
		GitID:       1234,
		Name:        "set-active-token",
		Token:       "set-active-token-value",
		UserID:      1,
		Application: types.AccessTokenAppAIGateway,
		IsActive:    true,
	}

	err := atStore.Create(ctx, token, nil)
	require.Nil(t, err)
	require.True(t, token.IsActive)

	// Disable the key. The row is kept but auth lookups refuse it.
	err = atStore.SetActiveByID(ctx, token.ID, false)
	require.Nil(t, err)

	storedToken, err := atStore.FindByID(ctx, token.ID)
	require.Nil(t, err)
	require.False(t, storedToken.IsActive)

	_, err = atStore.FindByToken(ctx, "set-active-token-value", string(types.AccessTokenAppAIGateway))
	require.ErrorIs(t, err, errorx.ErrDatabaseNoRows)

	// Re-enable the key so it grants access again.
	err = atStore.SetActiveByID(ctx, token.ID, true)
	require.Nil(t, err)

	storedToken, err = atStore.FindByID(ctx, token.ID)
	require.Nil(t, err)
	require.True(t, storedToken.IsActive)

	foundToken, err := atStore.FindByToken(ctx, "set-active-token-value", string(types.AccessTokenAppAIGateway))
	require.Nil(t, err)
	require.Equal(t, token.ID, foundToken.ID)
}

// credentialStorageMigration returns the actual registered up/down entry points.
func credentialStorageMigration(t *testing.T) migrate.Migration {
	t.Helper()
	for _, migration := range migrations.Migrations.Sorted() {
		if migration.Name == "20261020064423" {
			return migration
		}
	}
	t.Fatal("credential storage migration not registered")
	return migrate.Migration{}
}

// TestCredentialStorageMigrationKeepsExistingBearers verifies that migration,
// metadata updates, and refresh never replace or leak an existing bearer value.
func TestCredentialStorageMigrationKeepsExistingBearers(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAccessTokenStoreWithDB(db)
	apps := []types.AccessTokenApp{types.AccessTokenAppGit, types.AccessTokenAppStarship, types.AccessTokenAppMirror, types.AccessTokenAppAIGateway}
	var tokens []*database.AccessToken
	for i, app := range apps {
		value := fmt.Sprintf("legacy-credential-%d-secret", i)
		token := &database.AccessToken{Name: fmt.Sprintf("migration-%d", i), Token: value, UserID: 1, Application: app, IsActive: true, ExpiredAt: time.Now().Add(24 * time.Hour)}
		require.NoError(t, store.Create(ctx, token, nil))
		var stored string
		require.NoError(t, db.BunDB.NewRaw("SELECT token FROM access_tokens WHERE id = ?", token.ID).Scan(ctx, &stored))
		require.NotEqual(t, value, stored)
		require.True(t, strings.HasPrefix(stored, "v1:"))
		// Simulate the exact pre-upgrade representation without invoking model hooks.
		_, err := db.BunDB.DB.ExecContext(ctx, "UPDATE access_tokens SET token=$1, token_lookup_hash=NULL, credential_version=0, credential_kind='' WHERE id=$2", value, token.ID)
		require.NoError(t, err)
		legacy, err := store.FindByToken(ctx, value, string(app))
		require.NoError(t, err)
		require.Equal(t, value, legacy.Token)
		tokens = append(tokens, token)
	}
	require.NoError(t, credentialStorageMigration(t).Up(ctx, db.BunDB, nil))
	require.NoError(t, credentialStorageMigration(t).Up(ctx, db.BunDB, nil))
	for _, token := range tokens {
		got, err := store.FindByToken(ctx, token.Token, string(token.Application))
		require.NoError(t, err)
		require.Equal(t, token.Token, got.Token)
		require.Equal(t, token.ID, got.ID)
		got.Name += "-renamed"
		updated, err := store.UpdateTokenAndQuotas(ctx, got, nil)
		require.NoError(t, err)
		require.Equal(t, token.Token, updated.Token)
		var stored string
		require.NoError(t, db.BunDB.NewRaw("SELECT token FROM access_tokens WHERE id = ?", token.ID).Scan(ctx, &stored))
		require.NotEqual(t, token.Token, stored)
	}
	current, err := store.FindByID(ctx, tokens[0].ID)
	require.NoError(t, err)
	replacement, err := store.Refresh(ctx, current, "replacement-token-secret", time.Time{})
	require.NoError(t, err)
	require.Equal(t, "replacement-token-secret", replacement.Token)
	_, err = store.FindByToken(ctx, tokens[0].Token, "git")
	require.Error(t, err)
	require.NoError(t, credentialStorageMigration(t).Down(ctx, db.BunDB, nil))
	restored, err := store.FindByToken(ctx, "replacement-token-secret", "git")
	require.NoError(t, err)
	require.Equal(t, "replacement-token-secret", restored.Token)
	// A successful rollback remains upgradeable without changing the bearer.
	require.NoError(t, credentialStorageMigration(t).Up(ctx, db.BunDB, nil))
	_, err = store.FindByToken(ctx, "replacement-token-secret", "git")
	require.NoError(t, err)
}

// TestCredentialStorageSeparatesNamespaceKinds verifies independent personal and
// organization key domains and decryption through LLM-config relation loading.
func TestCredentialStorageSeparatesNamespaceKinds(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAccessTokenStoreWithDB(db)
	for _, kind := range []database.NamespaceType{database.UserNamespace, database.OrgNamespace} {
		ns := &database.Namespace{Path: "credential-" + string(kind), UUID: "credential-" + string(kind), UserID: 1, NamespaceType: kind}
		_, err := db.BunDB.NewInsert().Model(ns).Exec(ctx)
		require.NoError(t, err)
		token := &database.AccessToken{Name: "kind-key", Token: "kind-key-" + string(kind), Application: types.AccessTokenAppAIGateway, NsUUID: ns.UUID, UserID: 1, IsActive: true}
		require.NoError(t, store.Create(ctx, token, nil))
		var scope string
		require.NoError(t, db.BunDB.NewRaw("SELECT credential_kind FROM access_tokens WHERE id=?", token.ID).Scan(ctx, &scope))
		if kind == database.OrgNamespace {
			require.Equal(t, "organization-api-key", scope)
		} else {
			require.Equal(t, "personal-api-key", scope)
		}
		// Recreate the pre-upgrade row to verify the migration's frozen
		// namespace classification produces runtime-compatible ciphertext.
		_, err = db.BunDB.DB.ExecContext(ctx, "UPDATE access_tokens SET token=$1,token_lookup_hash=NULL,credential_kind='',credential_version=0 WHERE id=$2", token.Token, token.ID)
		require.NoError(t, err)
		require.NoError(t, credentialStorageMigration(t).Up(ctx, db.BunDB, nil))
		read, err := store.FindByToken(ctx, token.Token, string(types.AccessTokenAppAIGateway))
		require.NoError(t, err)
		require.Equal(t, scope, read.CredentialKind)
		require.Equal(t, token.Token, read.Token)
	}
	cfg := &config.Config{}
	llms := database.NewLLMConfigStoreWithDB(db, cfg)
	model, err := llms.Create(ctx, database.LLMConfig{ModelName: "credential-relations", Enabled: true})
	require.NoError(t, err)
	upstreams := database.NewUpstreamStoreWithDB(db, cfg)
	auth := `{"Authorization":"Bearer sk-private-original-key"}`
	up := &database.Upstream{LLMConfigID: model.ID, URL: "https://example.com", Enabled: true, AuthHeader: auth}
	require.NoError(t, upstreams.Create(ctx, up))
	read, err := llms.GetByID(ctx, model.ID)
	require.NoError(t, err)
	require.Len(t, read.Upstreams, 1)
	require.Equal(t, auth, read.Upstreams[0].AuthHeader)
	var stored string
	require.NoError(t, db.BunDB.NewRaw("SELECT auth_header FROM ai_gateway_upstreams WHERE id=?", up.ID).Scan(ctx, &stored))
	require.NotContains(t, stored, "sk-private")
	// Explicitly versioned corruption must never silently become a plaintext credential.
	_, err = db.BunDB.DB.ExecContext(ctx, "UPDATE ai_gateway_upstreams SET auth_header='broken' WHERE id=$1", up.ID)
	require.NoError(t, err)
	_, err = llms.GetByID(ctx, model.ID)
	require.Error(t, err)
}

// TestCredentialMigrationProtectsQuotaCopy verifies the API-key copy is migrated
// without changing limits, usage, timestamps or the original bearer value.
func TestCredentialMigrationProtectsQuotaCopy(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	tokens := database.NewAccessTokenStoreWithDB(db)
	token := &database.AccessToken{Name: "quota-copy", Token: "existing-api-key-copy", UserID: 1, Application: types.AccessTokenAppAIGateway, IsActive: true}
	require.NoError(t, tokens.Create(ctx, token, nil))
	quotas := database.NewAccountAccessTokenQuotaStoreWithDB(db)
	quota := &database.AccountAccessTokenQuota{APIKey: token.Token, TokenID: token.ID, QuotaType: types.AccountingQuotaTypeUnlimited, ValueType: types.AccountingQuotaValueTypeFee, Usage: 12, Quota: 100}
	require.NoError(t, quotas.Create(ctx, quota))
	_, err := db.BunDB.DB.ExecContext(ctx, "UPDATE access_tokens SET token=$1,token_lookup_hash=NULL,credential_kind='',credential_version=0 WHERE id=$2", token.Token, token.ID)
	require.NoError(t, err)
	_, err = db.BunDB.DB.ExecContext(ctx, "UPDATE account_access_token_quota SET api_key=$1,credential_version=0 WHERE id=$2", quota.APIKey, quota.ID)
	require.NoError(t, err)
	before, err := quotas.GetByID(ctx, quota.ID)
	require.NoError(t, err)
	migration := credentialStorageMigration(t)
	require.NoError(t, migration.Up(ctx, db.BunDB, nil))
	read, err := quotas.GetByID(ctx, quota.ID)
	require.NoError(t, err)
	require.Equal(t, token.Token, read.APIKey)
	require.NotEqual(t, read.APIKey, read.EncryptedAPIKey)
	read.EncryptedAPIKey, read.CredentialVersion = before.EncryptedAPIKey, before.CredentialVersion
	require.Equal(t, before, read)
	require.NoError(t, migration.Down(ctx, db.BunDB, nil))
	read, err = quotas.GetByID(ctx, quota.ID)
	require.NoError(t, err)
	require.Equal(t, before, read)
}

// credentialMigrationTestTables contains only primary credential storage and
// the existing API-key copy; no unrelated business tables are part of migration.
var credentialMigrationTestTables = []string{"access_tokens", "account_access_token_quota", "ai_gateway_upstreams"}

// seedAtomicCredentialMigration creates more than one token batch followed by
// one upstream row, allowing failures after earlier batches have been processed.
func seedAtomicCredentialMigration(t *testing.T, ctx context.Context, db *database.DB) int64 {
	t.Helper()
	cfg := createTestLLMConfigForUpstream(ctx, t, db, "atomic-credential-model")
	up := &database.Upstream{LLMConfigID: cfg.ID, URL: "https://atomic.example", Enabled: true, AuthHeader: "atomic-upstream-secret"}
	require.NoError(t, database.NewUpstreamStoreWithDB(db, nil).Create(ctx, up))
	_, err := db.BunDB.DB.ExecContext(ctx, "UPDATE ai_gateway_upstreams SET auth_header=$1,credential_version=0 WHERE id=$2", up.AuthHeader, up.ID)
	require.NoError(t, err)
	_, err = db.BunDB.DB.ExecContext(ctx, `INSERT INTO access_tokens (git_id,name,token,user_id,app,is_active)
 SELECT 0,'atomic-token-' || n,'atomic-credential-' || n,1,'git',true FROM generate_series(1,251) n`)
	require.NoError(t, err)
	return up.ID
}

// credentialTableSnapshot records every column so migrations cannot silently
// modify business values, timestamps or rows outside the intended scope.
func credentialTableSnapshot(t *testing.T, ctx context.Context, db *database.DB, tables []string) map[string]string {
	t.Helper()
	result := make(map[string]string, len(tables))
	for _, table := range tables {
		var snapshot string
		require.NoError(t, db.BunDB.NewRaw("SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY t.id,t.created_at),'[]'::jsonb)::text FROM "+table+" t").Scan(ctx, &snapshot))
		result[table] = snapshot
	}
	return result
}

// TestCredentialMigrationLeavesOtherTablesUntouched ensures the migration does
// not change other credentials, audit snapshots, archives or retained legacy JSON.
func TestCredentialMigrationLeavesOtherTablesUntouched(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	cfg := createTestLLMConfigForUpstream(ctx, t, db, "outside-migration")
	_, err := db.BunDB.DB.ExecContext(ctx, "ALTER TABLE llm_configs ADD COLUMN IF NOT EXISTS upstreams JSONB")
	require.NoError(t, err)
	_, err = db.BunDB.DB.ExecContext(ctx, "UPDATE llm_configs SET auth_header='legacy-plain',upstreams='{}'::jsonb WHERE id=$1", cfg.ID)
	require.NoError(t, err)
	_, err = database.NewSyncClientSettingStoreWithDB(db).Create(ctx, &database.SyncClientSetting{Token: "remote-sync-plain"})
	require.NoError(t, err)
	_, err = database.NewMirrorStoreWithDB(db).Create(ctx, &database.Mirror{RepositoryID: 123, AccessToken: "source-plain", PushAccessToken: "push-plain"})
	require.NoError(t, err)
	log := &database.AuditLog{TableName: "credentials", Action: "update", OperatorID: "operator", BearerToken: "audit-plain", Before: json.RawMessage(`{"token":"snapshot-plain"}`)}
	require.NoError(t, database.NewAuditLogStoreWithDB(db).Create(ctx, log))
	tables := []string{"sync_client_settings", "mirrors", "llm_configs", "audit_logs"}
	for _, table := range []string{"audit_logs_partitioned", "audit_logs_back", "audit_logs_failed_partitioned"} {
		_, err = db.BunDB.NewRaw("CREATE TABLE IF NOT EXISTS ? (LIKE audit_logs INCLUDING DEFAULTS)", bun.Ident(table)).Exec(ctx)
		require.NoError(t, err)
		_, err = db.BunDB.NewRaw("INSERT INTO ? SELECT * FROM audit_logs WHERE id=?", bun.Ident(table), log.ID).Exec(ctx)
		require.NoError(t, err)
		tables = append(tables, table)
	}
	before := credentialTableSnapshot(t, ctx, db, tables)
	migration := credentialStorageMigration(t)
	require.NoError(t, migration.Up(ctx, db.BunDB, nil))
	require.Equal(t, before, credentialTableSnapshot(t, ctx, db, tables))
	require.NoError(t, migration.Down(ctx, db.BunDB, nil))
	require.Equal(t, before, credentialTableSnapshot(t, ctx, db, tables))
	var added int
	require.NoError(t, db.BunDB.NewRaw(`SELECT COUNT(*) FROM information_schema.columns
 WHERE table_schema=current_schema() AND table_name IN (?)
 AND column_name IN ('credential_version','bearer_token_lookup_hash','api_key_lookup_hash')`, bun.In(tables)).Scan(ctx, &added))
	require.Zero(t, added, "unrelated tables must receive neither metadata nor data updates")
}

// TestCredentialMigrationUpRollsBackSchemaAndAllBatches forces a late upstream
// write failure and verifies the preceding token batches and DDL are rolled back.
func TestCredentialMigrationUpRollsBackSchemaAndAllBatches(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	upID := seedAtomicCredentialMigration(t, ctx, db)
	_, err := db.BunDB.DB.ExecContext(ctx, "ALTER TABLE ai_gateway_upstreams ADD CONSTRAINT credential_test_plain_only CHECK (auth_header NOT LIKE 'v1:%')")
	require.NoError(t, err)
	for _, table := range credentialMigrationTestTables {
		_, err = db.BunDB.DB.ExecContext(ctx, "ALTER TABLE "+table+" DROP COLUMN credential_version")
		require.NoError(t, err)
	}
	_, err = db.BunDB.DB.ExecContext(ctx, "ALTER TABLE access_tokens DROP COLUMN token_lookup_hash, DROP COLUMN credential_kind")
	require.NoError(t, err)
	before := credentialTableSnapshot(t, ctx, db, credentialMigrationTestTables)
	migration := credentialStorageMigration(t)
	require.ErrorContains(t, migration.Up(ctx, db.BunDB, nil), "credential_test_plain_only")
	require.Equal(t, before, credentialTableSnapshot(t, ctx, db, credentialMigrationTestTables))
	var added int
	require.NoError(t, db.BunDB.NewRaw(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=current_schema()
 AND table_name IN (?) AND column_name IN ('credential_version','token_lookup_hash','credential_kind')`, bun.In(credentialMigrationTestTables)).Scan(ctx, &added))
	require.Zero(t, added)
	_, err = db.BunDB.DB.ExecContext(ctx, "ALTER TABLE ai_gateway_upstreams DROP CONSTRAINT credential_test_plain_only")
	require.NoError(t, err)
	require.NoError(t, migration.Up(ctx, db.BunDB, nil))
	var encrypted int
	require.NoError(t, db.BunDB.NewRaw("SELECT COUNT(*) FROM access_tokens WHERE name LIKE 'atomic-token-%' AND credential_version=1").Scan(ctx, &encrypted))
	require.Equal(t, 251, encrypted)
	_, err = database.NewAccessTokenStoreWithDB(db).FindByToken(ctx, "atomic-credential-1", "git")
	require.NoError(t, err)
	read, err := database.NewUpstreamStoreWithDB(db, nil).GetByID(ctx, upID)
	require.NoError(t, err)
	require.Equal(t, "atomic-upstream-secret", read.AuthHeader)
}

// TestCredentialMigrationDownRollsBackAllBatches verifies that an upstream
// decryption failure cannot leave earlier token batches partially decrypted.
func TestCredentialMigrationDownRollsBackAllBatches(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	upID := seedAtomicCredentialMigration(t, ctx, db)
	migration := credentialStorageMigration(t)
	require.NoError(t, migration.Up(ctx, db.BunDB, nil))
	var ciphertext string
	require.NoError(t, db.BunDB.NewRaw("SELECT auth_header FROM ai_gateway_upstreams WHERE id=?", upID).Scan(ctx, &ciphertext))
	_, err := db.BunDB.DB.ExecContext(ctx, "UPDATE ai_gateway_upstreams SET auth_header='v1:broken' WHERE id=$1", upID)
	require.NoError(t, err)
	before := credentialTableSnapshot(t, ctx, db, credentialMigrationTestTables)
	require.ErrorContains(t, migration.Down(ctx, db.BunDB, nil), "ai_gateway_upstreams")
	require.Equal(t, before, credentialTableSnapshot(t, ctx, db, credentialMigrationTestTables))
	_, err = db.BunDB.DB.ExecContext(ctx, "UPDATE ai_gateway_upstreams SET auth_header=$1 WHERE id=$2", ciphertext, upID)
	require.NoError(t, err)
	require.NoError(t, migration.Down(ctx, db.BunDB, nil))
	var restored int
	require.NoError(t, db.BunDB.NewRaw(`SELECT COUNT(*) FROM access_tokens WHERE name LIKE 'atomic-token-%'
 AND credential_version=0 AND token LIKE 'atomic-credential-%' AND token_lookup_hash IS NULL`).Scan(ctx, &restored))
	require.Equal(t, 251, restored)
	_, err = database.NewAccessTokenStoreWithDB(db).FindByToken(ctx, "atomic-credential-1", "git")
	require.NoError(t, err)
}

// cancelCredentialIndexHook cancels immediately after successful index creation
// so the outer transaction must undo the index along with every preceding update.
type cancelCredentialIndexHook struct {
	cancel context.CancelFunc
	once   sync.Once
}

// BeforeQuery preserves the normal query context.
func (*cancelCredentialIndexHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}

// AfterQuery injects the cancellation before the outer transaction can commit.
func (h *cancelCredentialIndexHook) AfterQuery(_ context.Context, event *bun.QueryEvent) {
	if event.Err == nil && strings.Contains(event.Query, "CREATE UNIQUE INDEX IF NOT EXISTS idx_access_tokens_lookup_hash") {
		h.once.Do(h.cancel)
	}
}

// TestCredentialMigrationIndexFailureRollsBackAllOperations verifies a failure
// at commit rolls back both the newly created index and credential rewrites.
func TestCredentialMigrationIndexFailureRollsBackAllOperations(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	seedAtomicCredentialMigration(t, ctx, db)
	_, err := db.BunDB.DB.ExecContext(ctx, "DROP INDEX idx_access_tokens_lookup_hash")
	require.NoError(t, err)
	before := credentialTableSnapshot(t, ctx, db, credentialMigrationTestTables)
	migrationCtx, cancelMigration := context.WithCancel(ctx)
	defer cancelMigration()
	db.BunDB.AddQueryHook(&cancelCredentialIndexHook{cancel: cancelMigration})
	migration := credentialStorageMigration(t)
	require.Error(t, migration.Up(migrationCtx, db.BunDB, nil))
	require.ErrorIs(t, migrationCtx.Err(), context.Canceled)
	require.Equal(t, before, credentialTableSnapshot(t, ctx, db, credentialMigrationTestTables))
	var exists bool
	require.NoError(t, db.BunDB.NewRaw("SELECT to_regclass('idx_access_tokens_lookup_hash') IS NOT NULL").Scan(ctx, &exists))
	require.False(t, exists)
	require.NoError(t, migration.Up(ctx, db.BunDB, nil))
}

// TestCredentialExplicitTokenReads verifies every token lookup retains the
// original bearer, supports plaintext rows and rejects damaged encrypted rows.
func TestCredentialExplicitTokenReads(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	user := &database.User{Username: "explicit-token-owner", UUID: "explicit-token-owner-uuid", Email: "explicit-token@example.com"}
	_, err := db.Core.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)
	store := database.NewAccessTokenStoreWithDB(db)
	token := &database.AccessToken{Name: "explicit-token", Token: "original-explicit-token", UserID: user.ID, Application: types.AccessTokenAppGit, NsUUID: "explicit-token-ns", TokenType: types.AccessTokenTypeBuiltIn, IsActive: true, ExpiredAt: time.Now().Add(time.Hour)}
	require.NoError(t, store.Create(ctx, token, nil))
	one := func(read func() (*database.AccessToken, error)) func() ([]database.AccessToken, error) {
		return func() ([]database.AccessToken, error) {
			row, err := read()
			if err != nil {
				return nil, err
			}
			return []database.AccessToken{*row}, nil
		}
	}
	queries := []struct {
		name string
		read func() ([]database.AccessToken, error)
	}{
		{"id", one(func() (*database.AccessToken, error) { return store.FindByID(ctx, token.ID) })},
		{"deleted-inclusive-id", one(func() (*database.AccessToken, error) { return store.GetByID(ctx, token.ID) })},
		{"user-id", one(func() (*database.AccessToken, error) { return store.FindByUID(ctx, user.ID) })},
		{"git", one(func() (*database.AccessToken, error) { return store.GetUserGitToken(ctx, user.Username) })},
		{"bearer", one(func() (*database.AccessToken, error) { return store.FindByToken(ctx, token.Token, "git") })},
		{"name", one(func() (*database.AccessToken, error) {
			return store.FindByTokenName(ctx, user.Username, token.Name, "git")
		})},
		{"user", func() ([]database.AccessToken, error) { return store.FindByUser(ctx, user.Username, "git") }},
		{"namespace", func() ([]database.AccessToken, error) { return store.FindByNsUUID(ctx, token.NsUUID, "git") }},
		{"builtin", one(func() (*database.AccessToken, error) { return store.FindBuiltinByNsUUID(ctx, token.NsUUID, "git") })},
		{"ids", func() ([]database.AccessToken, error) {
			rows, err := store.FindByIDs(ctx, []int64{token.ID})
			out := make([]database.AccessToken, 0, len(rows))
			for _, row := range rows {
				out = append(out, row)
			}
			return out, err
		}},
	}
	for _, phase := range []string{"encrypted", "legacy", "corrupt"} {
		t.Run(phase, func(t *testing.T) {
			if phase != "encrypted" {
				value, version := token.Token, 0
				if phase == "corrupt" {
					value, version = "broken", 1
				}
				_, err = db.BunDB.DB.ExecContext(ctx, "UPDATE access_tokens SET token=$1,credential_version=$2,token_lookup_hash=$3 WHERE id=$4", value, version, token.TokenLookupHash, token.ID)
				require.NoError(t, err)
			}
			for _, q := range queries {
				t.Run(q.name, func(t *testing.T) {
					rows, err := q.read()
					if phase == "corrupt" {
						require.Error(t, err)
						return
					}
					require.NoError(t, err)
					require.Len(t, rows, 1)
					require.Equal(t, token.Token, rows[0].Token)
				})
			}
		})
	}
}

// assertEncryptedTokenCredential checks the actual row and the existing authentication path.
func assertEncryptedTokenCredential(t *testing.T, ctx context.Context, db *database.DB, token *database.AccessToken) {
	t.Helper()
	var raw database.AccessToken
	require.NoError(t, db.Core.NewSelect().Model(&raw).Where("id=?", token.ID).Scan(ctx))
	require.Empty(t, raw.Token)
	require.NotEqual(t, token.Token, raw.EncryptedToken)
	require.Contains(t, raw.EncryptedToken, "v1:")
	require.Equal(t, 1, raw.CredentialVersion)
	digest, err := database.CredentialLookupHash(token.Token)
	require.NoError(t, err)
	require.Equal(t, digest, raw.TokenLookupHash)
	read, err := database.NewAccessTokenStoreWithDB(db).FindByToken(ctx, token.Token, string(token.Application))
	require.NoError(t, err)
	require.Equal(t, token.Token, read.Token)
	require.Equal(t, token.ID, read.ID)
}

// assertStoredQuotaCredential verifies each quota copy is encrypted under its token ID.
func assertStoredQuotaCredential(t *testing.T, ctx context.Context, db *database.DB, quota *database.AccountAccessTokenQuota) {
	t.Helper()
	var raw database.AccountAccessTokenQuota
	require.NoError(t, db.Core.NewSelect().Model(&raw).Where("id=?", quota.ID).Scan(ctx))
	require.Empty(t, raw.APIKey)
	require.NotEqual(t, quota.APIKey, raw.EncryptedAPIKey)
	require.Contains(t, raw.EncryptedAPIKey, "v1:")
	require.Equal(t, 1, raw.CredentialVersion)
	read, err := database.NewAccountAccessTokenQuotaStoreWithDB(db).GetByID(ctx, quota.ID)
	require.NoError(t, err)
	require.Equal(t, quota.APIKey, read.APIKey)
}

// TestCredentialExplicitTokenWrites covers all five issued credential kinds,
// batch quota creation after the generated token ID, and token replacement.
func TestCredentialExplicitTokenWrites(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAccessTokenStoreWithDB(db)
	org := &database.Namespace{Path: "explicit-write-org", UUID: "explicit-write-org", NamespaceType: database.OrgNamespace, UserID: 1}
	_, err := db.Core.NewInsert().Model(org).Exec(ctx)
	require.NoError(t, err)
	cases := []struct {
		kind      string
		app       types.AccessTokenApp
		namespace string
	}{
		{"git", types.AccessTokenAppGit, ""},
		{"starship", types.AccessTokenAppStarship, ""},
		{"mirror", types.AccessTokenAppMirror, ""},
		{"personal-api-key", types.AccessTokenAppAIGateway, ""},
		{"organization-api-key", types.AccessTokenAppAIGateway, org.UUID},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			token := &database.AccessToken{Name: tc.kind, Token: "original-" + tc.kind, UserID: 1, Application: tc.app, NsUUID: tc.namespace, IsActive: true}
			var quotas []database.AccountAccessTokenQuota
			if tc.app == types.AccessTokenAppAIGateway {
				quotas = []database.AccountAccessTokenQuota{
					{APIKey: token.Token, QuotaType: types.AccountingQuotaTypeMonthly, ValueType: types.AccountingQuotaValueTypeFee, Quota: 100},
					{APIKey: token.Token, QuotaType: types.AccountingQuotaTotal, ValueType: types.AccountingQuotaValueTypeToken, Quota: 200},
				}
			}
			require.NoError(t, store.Create(ctx, token, quotas))
			require.Equal(t, "original-"+tc.kind, token.Token, "create must still return plaintext once")
			require.Equal(t, tc.kind, token.CredentialKind)
			assertEncryptedTokenCredential(t, ctx, db, token)
			for i := range quotas {
				require.Equal(t, token.ID, quotas[i].TokenID)
				assertStoredQuotaCredential(t, ctx, db, &quotas[i])
			}
			if tc.app == types.AccessTokenAppAIGateway {
				// Personal and organization API keys refresh in place, including
				// every quota copy encrypted under the generated token ID.
				oldValue := token.Token
				token.Token = "refreshed-" + tc.kind
				require.NoError(t, store.UpdateToken(ctx, token))
				assertEncryptedTokenCredential(t, ctx, db, token)
				updatedQuotas, err := database.NewAccountAccessTokenQuotaStoreWithDB(db).FindByTokenID(ctx, token.ID)
				require.NoError(t, err)
				require.Len(t, updatedQuotas, 2)
				for i := range updatedQuotas {
					require.Equal(t, token.Token, updatedQuotas[i].APIKey)
					assertStoredQuotaCredential(t, ctx, db, &updatedQuotas[i])
				}
				_, err = store.FindByToken(ctx, oldValue, string(tc.app))
				require.Error(t, err)
				return
			}
			before := token.EncryptedToken
			fresh, err := store.Refresh(ctx, token, "refreshed-"+tc.kind, time.Now().Add(time.Hour))
			require.NoError(t, err)
			require.Equal(t, "refreshed-"+tc.kind, fresh.Token)
			assertEncryptedTokenCredential(t, ctx, db, fresh)
			_, err = store.FindByToken(ctx, token.Token, string(tc.app))
			require.Error(t, err)
			old, err := store.GetByID(ctx, token.ID)
			require.NoError(t, err)
			require.Equal(t, before, old.EncryptedToken, "disabling must not rewrite credential storage")
		})
	}
}

// TestCredentialWritesAreExplicit proves raw Bun writes no longer prepare any
// credential fields implicitly; store entry points own encryption instead.
func TestCredentialWritesAreExplicit(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	token := &database.AccessToken{Token: "memory-token", EncryptedToken: "stored-token", UserID: 1, Application: types.AccessTokenAppGit, IsActive: true}
	quota := &database.AccountAccessTokenQuota{APIKey: "memory-key", EncryptedAPIKey: "stored-key", QuotaType: types.AccountingQuotaTypeMonthly, ValueType: types.AccountingQuotaValueTypeFee}
	up := &database.Upstream{LLMConfigID: 1, URL: "https://explicit-write.example", AuthHeader: "memory-upstream", EncryptedAuthHeader: "stored-upstream"}
	for _, model := range []any{token, quota, up} {
		_, err := db.Core.NewInsert().Model(model).Exec(ctx)
		require.NoError(t, err)
		_, err = db.Core.NewUpdate().Model(model).WherePK().Exec(ctx)
		require.NoError(t, err)
	}
	require.Equal(t, "stored-token", token.EncryptedToken)
	require.Empty(t, token.TokenLookupHash)
	require.Empty(t, token.CredentialKind)
	require.Zero(t, token.CredentialVersion)
	require.Equal(t, "memory-token", token.Token)
	require.Equal(t, "stored-key", quota.EncryptedAPIKey)
	require.Zero(t, quota.CredentialVersion)
	require.Equal(t, "memory-key", quota.APIKey)
	require.Equal(t, "stored-upstream", up.EncryptedAuthHeader)
	require.Zero(t, up.CredentialVersion)
	require.Equal(t, "memory-upstream", up.AuthHeader)
}

// TestCredentialExplicitLegacyWrites verifies existing plaintext remains usable
// and becomes encrypted when the original store update paths write it again.
func TestCredentialExplicitLegacyWrites(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	tokens := database.NewAccessTokenStoreWithDB(db)
	quotas := database.NewAccountAccessTokenQuotaStoreWithDB(db)
	upstreams := database.NewUpstreamStoreWithDB(db, nil)
	token := &database.AccessToken{EncryptedToken: "legacy-write-token", UserID: 1, Application: types.AccessTokenAppGit, IsActive: true}
	_, err := db.Core.NewInsert().Model(token).Exec(ctx)
	require.NoError(t, err)
	quota := &database.AccountAccessTokenQuota{EncryptedAPIKey: "legacy-write-quota", TokenID: token.ID, QuotaType: types.AccountingQuotaTypeMonthly, ValueType: types.AccountingQuotaValueTypeFee}
	_, err = db.Core.NewInsert().Model(quota).Exec(ctx)
	require.NoError(t, err)
	up := &database.Upstream{LLMConfigID: 1, URL: "https://legacy.example", EncryptedAuthHeader: "legacy-write-upstream"}
	_, err = db.Core.NewInsert().Model(up).Exec(ctx)
	require.NoError(t, err)
	token, err = tokens.FindByToken(ctx, "legacy-write-token", "git")
	require.NoError(t, err)
	quota, err = quotas.GetByID(ctx, quota.ID)
	require.NoError(t, err)
	up, err = upstreams.GetByID(ctx, up.ID)
	require.NoError(t, err)
	require.Equal(t, "legacy-write-token", token.Token)
	require.Equal(t, "legacy-write-quota", quota.APIKey)
	require.Equal(t, "legacy-write-upstream", up.AuthHeader)
	_, err = tokens.UpdateTokenAndQuota(ctx, token, quota)
	require.NoError(t, err)
	require.NoError(t, upstreams.Update(ctx, up))
	assertEncryptedTokenCredential(t, ctx, db, token)
	assertStoredQuotaCredential(t, ctx, db, quota)
	assertStoredUpstreamCredential(t, ctx, db, up)
}

// TestCredentialExplicitWriteRollback forces failures after earlier credential
// writes to verify batch creation and updates retain their transaction boundary.
func TestCredentialExplicitWriteRollback(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := database.NewAccessTokenStoreWithDB(db)
	tables := []string{"access_tokens", "account_access_token_quota"}
	before := credentialTableSnapshot(t, ctx, db, tables)
	failed := &database.AccessToken{Token: "failed-batch-token", UserID: 1, Application: types.AccessTokenAppAIGateway, IsActive: true}
	// Duplicate quota identities fail only after the token insert succeeds.
	duplicates := []database.AccountAccessTokenQuota{
		{APIKey: failed.Token, QuotaType: types.AccountingQuotaTypeMonthly, ValueType: types.AccountingQuotaValueTypeFee},
		{APIKey: failed.Token, QuotaType: types.AccountingQuotaTypeMonthly, ValueType: types.AccountingQuotaValueTypeFee},
	}
	require.Error(t, store.Create(ctx, failed, duplicates))
	require.Equal(t, before, credentialTableSnapshot(t, ctx, db, tables))

	token := &database.AccessToken{Token: "committed-batch-token", UserID: 1, Application: types.AccessTokenAppAIGateway, IsActive: true}
	quotas := []database.AccountAccessTokenQuota{{APIKey: token.Token, QuotaType: types.AccountingQuotaTypeMonthly, ValueType: types.AccountingQuotaValueTypeFee, Quota: 100}}
	require.NoError(t, store.Create(ctx, token, quotas))
	before = credentialTableSnapshot(t, ctx, db, tables)
	token.Token = "uncommitted-replacement"
	quotas[0].APIKey = token.Token
	quotas[0].Quota = 200
	duplicate := quotas[0]
	duplicate.ID = 0
	_, err := store.UpdateTokenAndQuotas(ctx, token, []*database.AccountAccessTokenQuota{&quotas[0], nil, &duplicate})
	require.Error(t, err)
	require.Equal(t, before, credentialTableSnapshot(t, ctx, db, tables))
	_, err = store.FindByToken(ctx, "committed-batch-token", "aigateway")
	require.NoError(t, err)
	_, err = store.FindByToken(ctx, token.Token, "aigateway")
	require.Error(t, err)
}

// TestAccessTokenStoreCreationLimit verifies the namespace lock, counting scope,
// rollback at the boundary, and insertion in the same transaction below it.
func TestAccessTokenStoreCreationLimit(t *testing.T) {
	for _, tc := range []struct {
		name          string
		count         int
		limit         uint
		namespaceType database.NamespaceType
	}{
		{"below limit", 2, 3, database.UserNamespace},
		{"at limit", 3, 3, database.UserNamespace},
		{"organization scope", 2, 3, database.OrgNamespace},
		{"zero limit", 0, 0, database.UserNamespace},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sqlDB, sqlMock, err := sqlmock.New()
			require.NoError(t, err)
			bunDB := bun.NewDB(sqlDB, pgdialect.New())
			t.Cleanup(func() { _ = bunDB.Close() })
			store := database.NewAccessTokenStoreWithDB(&database.DB{Operator: database.Operator{Core: bunDB}, BunDB: bunDB})
			sqlMock.ExpectQuery(`SELECT .* FROM "namespaces" AS "namespace".*uuid = 'ns'.*LIMIT 1$`).
				WillReturnRows(sqlmock.NewRows([]string{"namespace_type"}).AddRow(tc.namespaceType))
			sqlMock.ExpectBegin()
			if tc.limit > 0 {
				sqlMock.ExpectQuery(`SELECT .* FROM "namespaces" AS "namespace".*uuid = 'ns'.*FOR UPDATE$`).
					WillReturnRows(sqlmock.NewRows([]string{"id", "namespace_type", "user_id"}).AddRow(1, tc.namespaceType, 42))
				// No active/expiry predicate; LIMIT applies before COUNT within the listed namespace.
				scope := "ns_uuid = 'ns'"
				countSQL := fmt.Sprintf(`SELECT count(*) FROM (SELECT 1 FROM "access_tokens" AS "access_token" WHERE (app = 'aigateway') AND (token_type != 'builtin') AND (%s) AND "access_token"."deleted_at" IS NULL LIMIT %d) AS limited_keys`, scope, tc.limit)
				sqlMock.ExpectQuery(regexp.QuoteMeta(countSQL) + "$").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.count))
			}
			if uint(tc.count) >= tc.limit {
				sqlMock.ExpectRollback()
			} else {
				sqlMock.ExpectQuery(`INSERT INTO "access_tokens"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
				sqlMock.ExpectCommit()
			}
			err = store.Create(context.Background(), &database.AccessToken{
				Name: "new-key", Token: "secret", Application: types.AccessTokenAppAIGateway,
				NsUUID: "ns", TokenType: types.AccessTokenTypeOwner, CreationLimit: &tc.limit,
			}, nil)
			if uint(tc.count) >= tc.limit {
				require.ErrorIs(t, err, errorx.ErrAPIKeyLimitReached)
				custom, ok := errorx.GetFirstCustomError(err)
				require.True(t, ok)
				require.Equal(t, "USER-ERR-31", custom.(errorx.CustomError).Code())
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, sqlMock.ExpectationsWereMet())
		})
	}
}

// TestAccessTokenStoreCreationLimitExempt verifies builtin exemption and unrestricted persistence when no cap is supplied.
func TestAccessTokenStoreCreationLimitExempt(t *testing.T) {
	for _, tokenType := range []types.AccessTokenType{types.AccessTokenTypeBuiltIn, types.AccessTokenTypeOwner} {
		t.Run(string(tokenType), func(t *testing.T) {
			sqlDB, sqlMock, err := sqlmock.New()
			require.NoError(t, err)
			bunDB := bun.NewDB(sqlDB, pgdialect.New())
			t.Cleanup(func() { _ = bunDB.Close() })
			store := database.NewAccessTokenStoreWithDB(&database.DB{Operator: database.Operator{Core: bunDB}, BunDB: bunDB})
			limit := uint(0)
			var creationLimit *uint
			if tokenType == types.AccessTokenTypeBuiltIn {
				creationLimit = &limit
			}
			sqlMock.ExpectQuery(`SELECT .* FROM "namespaces" AS "namespace".*uuid = 'ns'.*LIMIT 1$`).
				WillReturnRows(sqlmock.NewRows([]string{"namespace_type"}).AddRow(database.UserNamespace))
			sqlMock.ExpectBegin()
			sqlMock.ExpectQuery(`INSERT INTO "access_tokens"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			sqlMock.ExpectCommit()
			require.NoError(t, store.Create(context.Background(), &database.AccessToken{
				Application: types.AccessTokenAppAIGateway, NsUUID: "ns", TokenType: tokenType, CreationLimit: creationLimit,
			}, nil))
			require.NoError(t, sqlMock.ExpectationsWereMet())
		})
	}
}

// TestAPIKeyLimitConcurrentCreation verifies real PostgreSQL serialization,
// namespace counting, builtin exclusion, and slot release after deletion.
// Worker sessions default to Repeatable Read to verify creation explicitly selects Read Committed.
func TestAPIKeyLimitConcurrentCreation(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ns := &database.Namespace{UUID: "key-limit-ns", Path: "key-limit-user", UserID: 1, NamespaceType: database.UserNamespace}
	_, err := db.Core.NewInsert().Model(ns).Exec(ctx)
	require.NoError(t, err)
	// Reject inserts from any other isolation level, including otherwise successful serial requests.
	_, err = db.Core.ExecContext(ctx, `ALTER TABLE access_tokens ADD CONSTRAINT test_read_committed CHECK (current_setting('transaction_isolation') = 'read committed')`)
	require.NoError(t, err)
	store := database.NewAccessTokenStoreWithDB(db)
	limit := uint(3)
	makeKey := func(name string) *database.AccessToken {
		return &database.AccessToken{Name: name, Token: name, NsUUID: ns.UUID, UserID: ns.UserID,
			Application: types.AccessTokenAppAIGateway, TokenType: types.AccessTokenTypeOwner, IsActive: true, CreationLimit: &limit}
	}
	disabled := makeKey("disabled")
	require.NoError(t, store.Create(ctx, disabled, nil))
	require.NoError(t, store.SetActiveByID(ctx, disabled.ID, false))
	expired := makeKey("expired")
	expired.ExpiredAt = time.Now().Add(-time.Hour)
	require.NoError(t, store.Create(ctx, expired, nil))
	deleted := makeKey("deleted")
	require.NoError(t, store.Create(ctx, deleted, nil))
	require.NoError(t, store.DeleteByID(ctx, deleted.ID))
	builtin := makeKey("builtin")
	builtin.TokenType = types.AccessTokenTypeBuiltIn
	require.NoError(t, store.Create(ctx, builtin, nil))
	git := makeKey("git")
	git.Application = types.AccessTokenAppGit
	require.NoError(t, store.Create(ctx, git, nil))
	other := makeKey("other-namespace")
	other.NsUUID = "another-ns"
	other.CreationLimit = nil
	require.NoError(t, store.Create(ctx, other, nil))

	const attempts = 8
	start := make(chan struct{})
	results := make(chan error, attempts)
	for i := range attempts {
		go func(index int) {
			<-start
			conn, err := db.BunDB.Conn(ctx)
			if err != nil {
				results <- err
				return
			}
			defer conn.Close()
			if _, err := conn.ExecContext(ctx, "SET default_transaction_isolation = 'repeatable read'"); err != nil {
				results <- err
				return
			}
			workerStore := database.NewAccessTokenStoreWithDB(&database.DB{Operator: database.Operator{Core: conn}, BunDB: db.BunDB})
			token := makeKey(fmt.Sprintf("concurrent-%d", index))
			err = workerStore.Create(ctx, token, []database.AccountAccessTokenQuota{{
				APIKey: token.Token, QuotaType: types.AccountingQuotaTypeUnlimited, ValueType: types.AccountingQuotaValueTypeFee,
			}})
			// Restore pooled sessions before the test exercises unrelated deletion operations.
			if _, resetErr := conn.ExecContext(ctx, "RESET default_transaction_isolation"); resetErr != nil {
				results <- resetErr
				return
			}
			results <- err
		}(i)
	}
	close(start)
	successes, limited := 0, 0
	for range attempts {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, errorx.ErrAPIKeyLimitReached):
			limited++
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, attempts-1, limited)
	quotaCount, err := db.Core.NewSelect().Model((*database.AccountAccessTokenQuota)(nil)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, quotaCount, "rejected creations must not leave quota rows")

	require.NoError(t, store.DeleteByID(ctx, disabled.ID))
	require.NoError(t, store.Create(ctx, makeKey("replacement"), nil))
	require.ErrorIs(t, store.Create(ctx, makeKey("over-limit"), nil), errorx.ErrAPIKeyLimitReached)
}

// TestAPIKeyLimitNamespaceCounting verifies visible custom keys determine the limit,
// while historical NULL/empty namespaces do not consume slots in personal or organization namespaces.
func TestAPIKeyLimitNamespaceCounting(t *testing.T) {
	db := tests.InitTransactionTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	store := database.NewAccessTokenStoreWithDB(db)
	namespaces := []database.Namespace{
		{UUID: "personal-ns", Path: "personal", UserID: 42, NamespaceType: database.UserNamespace},
		{UUID: "org-ns", Path: "org", UserID: 42, NamespaceType: database.OrgNamespace},
	}
	_, err := db.Core.NewInsert().Model(&namespaces).Exec(ctx)
	require.NoError(t, err)
	// Seed unversioned plaintext in the storage column; Token is only an in-memory field.
	// Two visible custom keys count; historical namespace-less keys remain excluded.
	keys := []database.AccessToken{
		{Name: "visible-disabled", EncryptedToken: "visible-disabled", UserID: 42, NsUUID: "personal-ns", Application: types.AccessTokenAppAIGateway, TokenType: types.AccessTokenTypeOwner, IsActive: false},
		{Name: "visible-expired", EncryptedToken: "visible-expired", UserID: 42, NsUUID: "personal-ns", Application: types.AccessTokenAppAIGateway, TokenType: types.AccessTokenTypeOwner, ExpiredAt: time.Now().Add(-time.Hour)},
		{Name: "legacy-null", EncryptedToken: "legacy-null", UserID: 42, Application: types.AccessTokenAppAIGateway, TokenType: "", IsActive: false},
		{Name: "legacy-empty", EncryptedToken: "legacy-empty", UserID: 42, Application: types.AccessTokenAppAIGateway, TokenType: types.AccessTokenTypeOwner, ExpiredAt: time.Now().Add(-time.Hour)},
		{Name: "deleted", EncryptedToken: "deleted", UserID: 42, NsUUID: "personal-ns", Application: types.AccessTokenAppAIGateway, DeletedAt: time.Now()},
		{Name: "builtin", EncryptedToken: "builtin", UserID: 42, NsUUID: "personal-ns", Application: types.AccessTokenAppAIGateway, TokenType: types.AccessTokenTypeBuiltIn},
		{Name: "git", EncryptedToken: "git", UserID: 42, NsUUID: "personal-ns", Application: types.AccessTokenAppGit},
		{Name: "another-user", EncryptedToken: "another-user", UserID: 7, Application: types.AccessTokenAppAIGateway},
		{Name: "other-namespace", EncryptedToken: "other-namespace", UserID: 42, NsUUID: "elsewhere", Application: types.AccessTokenAppAIGateway},
	}
	_, err = db.Core.NewInsert().Model(&keys).Exec(ctx)
	require.NoError(t, err)
	// Bun writes the zero string as NULL; explicitly seed the other historical representation.
	_, err = db.Core.NewUpdate().Model((*database.AccessToken)(nil)).Set("ns_uuid = ''").Where("id = ?", keys[3].ID).Exec(ctx)
	require.NoError(t, err)
	limit := uint(3)
	newKey := func(name, namespace string) *database.AccessToken {
		// An administrator can create a key for someone else's namespace.
		return &database.AccessToken{Name: name, Token: name, UserID: 99, NsUUID: namespace,
			Application: types.AccessTokenAppAIGateway, TokenType: types.AccessTokenTypeOwner, CreationLimit: &limit}
	}
	// The list and creation count agree even when the owner has hidden historical keys.
	listed, err := store.FindByNsUUID(ctx, "personal-ns", string(types.AccessTokenAppAIGateway))
	require.NoError(t, err)
	var customNames []string
	for _, token := range listed {
		if token.TokenType != types.AccessTokenTypeBuiltIn {
			customNames = append(customNames, token.Name)
		}
	}
	require.ElementsMatch(t, []string{"visible-disabled", "visible-expired"}, customNames)
	third := newKey("personal-third", "personal-ns")
	quotas := []database.AccountAccessTokenQuota{{APIKey: third.Token, QuotaType: types.AccountingQuotaTypeUnlimited, ValueType: types.AccountingQuotaValueTypeFee}}
	require.NoError(t, store.Create(ctx, third, quotas))
	// Admitted keys and their quota copies retain main's encrypted storage and authentication behavior.
	assertEncryptedTokenCredential(t, ctx, db, third)
	assertStoredQuotaCredential(t, ctx, db, &quotas[0])
	listed, err = store.FindByNsUUID(ctx, "personal-ns", string(types.AccessTokenAppAIGateway))
	require.NoError(t, err)
	require.Len(t, listed, 4, "three custom keys plus the exempt builtin key must be listed")
	require.ErrorIs(t, store.Create(ctx, newKey("personal-fourth", "personal-ns"), nil), errorx.ErrAPIKeyLimitReached)
	// Lowering a cap rejects creation even when existing keys already exceed it.
	limit = 1
	require.ErrorIs(t, store.Create(ctx, newKey("personal-lowered", "personal-ns"), nil), errorx.ErrAPIKeyLimitReached)
	orgKey := newKey("org-first", "org-ns")
	require.NoError(t, store.Create(ctx, orgKey, nil))
	require.Equal(t, "organization-api-key", orgKey.CredentialKind)
	assertEncryptedTokenCredential(t, ctx, db, orgKey)
	require.ErrorIs(t, store.Create(ctx, newKey("org-second", "org-ns"), nil), errorx.ErrAPIKeyLimitReached)
	// Deleting a visible custom key frees a slot while historical keys remain untouched.
	limit = 3
	require.NoError(t, store.DeleteByID(ctx, keys[0].ID))
	require.NoError(t, store.Create(ctx, newKey("personal-replacement", "personal-ns"), nil))
}

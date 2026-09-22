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
	userStore := database.NewUserStoreWithDB(db)

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
	userStore := database.NewUserStoreWithDB(db)

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

	err := atStore.Create(ctx, token1, nil)
	require.Nil(t, err)
	err = atStore.Create(ctx, token2, nil)
	require.Nil(t, err)

	// Find tokens by namespace UUID
	tokens, err := atStore.FindByNsUUID(ctx, nsUUID, string(types.AccessTokenAppAIGateway))
	require.Nil(t, err)
	require.Len(t, tokens, 2)

	// Verify tokens are returned in descending order by ID
	require.Equal(t, token2.ID, tokens[0].ID)
	require.Equal(t, token1.ID, tokens[1].ID)

	// Find with non-existent namespace UUID
	tokens, err = atStore.FindByNsUUID(ctx, "nonexistent-uuid", string(types.AccessTokenAppAIGateway))
	require.Nil(t, err)
	require.Empty(t, tokens)
}

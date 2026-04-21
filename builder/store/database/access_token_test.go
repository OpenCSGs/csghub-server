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
		Application: types.AccessTokenAPIKey,
		NsUUID:      "test-ns-uuid",
		IsActive:    true,
	}

	err := atStore.Create(ctx, token, nil)
	require.Nil(t, err)

	exists, err := atStore.IsExistByUUID(ctx, "test-ns-uuid", "uuid-token", string(types.AccessTokenAPIKey))
	require.Nil(t, err)
	require.True(t, exists)

	exists, err = atStore.IsExistByUUID(ctx, "test-ns-uuid", "nonexistent-token", string(types.AccessTokenAPIKey))
	require.Nil(t, err)
	require.False(t, exists)

	exists, err = atStore.IsExistByUUID(ctx, "nonexistent-uuid", "uuid-token", string(types.AccessTokenAPIKey))
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
		Application: types.AccessTokenAPIKey,
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

	// Update token name
	newName := "updated-token-name"
	token.Name = newName

	quotas[0].Quota = 200.0

	updatedToken, err := atStore.UpdateTokenAndQuota(ctx, token, &quotas[0])
	require.Nil(t, err)
	require.NotNil(t, updatedToken)
	require.Equal(t, newName, updatedToken.Name)

	// Verify quota updated
	savedQuotas, err := quotaStore.FindByAPIKey(ctx, token.Token)
	require.Nil(t, err)
	require.Len(t, savedQuotas, 1)
	require.Equal(t, 200.0, savedQuotas[0].Quota)
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
		Application: types.AccessTokenAPIKey,
		NsUUID:      nsUUID,
		IsActive:    true,
	}

	token2 := &database.AccessToken{
		GitID:       1235,
		Name:        "ns-token-2",
		Token:       "ns-token-value-2",
		UserID:      1,
		Application: types.AccessTokenAPIKey,
		NsUUID:      nsUUID,
		IsActive:    true,
	}

	err := atStore.Create(ctx, token1, nil)
	require.Nil(t, err)
	err = atStore.Create(ctx, token2, nil)
	require.Nil(t, err)

	// Find tokens by namespace UUID
	tokens, err := atStore.FindByNsUUID(ctx, nsUUID, string(types.AccessTokenAPIKey))
	require.Nil(t, err)
	require.Len(t, tokens, 2)

	// Verify tokens are returned in descending order by ID
	require.Equal(t, token2.ID, tokens[0].ID)
	require.Equal(t, token1.ID, tokens[1].ID)

	// Find with non-existent namespace UUID
	tokens, err = atStore.FindByNsUUID(ctx, "nonexistent-uuid", string(types.AccessTokenAPIKey))
	require.Nil(t, err)
	require.Empty(t, tokens)
}

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

	err := atStore.Create(ctx, token)
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

	err := atStore.Create(ctx, oldToken)
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

	err = atStore.Create(ctx, token)
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

	err = atStore.Create(ctx, token)
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

	err := atStore.Create(ctx, token)
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

	err := atStore.Create(ctx, token)
	require.Nil(t, err)

	storedToken, err := atStore.FindByToken(ctx, "find-me", "test-app")
	require.Nil(t, err)
	require.Equal(t, token.Token, storedToken.Token)
}

package component

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockgit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestAccessComponent_Create(t *testing.T) {
	t.Run("create duplicate token", func(t *testing.T) {
		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "user1").Return(database.User{
			Username: "user1",
		}, nil).Once()

		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		// token already exist
		mockTokenStore.EXPECT().IsExist(mock.Anything, "user1", "test_token_name", "git").
			Return(true, nil).Once()

		ac := &accessTokenComponentImpl{
			us: mockUserStore,
			ts: mockTokenStore,
		}
		dbtoken, err := ac.Create(context.Background(), &types.CreateUserTokenRequest{
			Username:    "user1",
			TokenName:   "test_token_name",
			Application: "git",
			Permission:  "",
			ExpiredAt:   time.Now().Add(time.Hour),
		})
		require.Error(t, err)
		require.Nil(t, dbtoken)
	})

	t.Run("create git token for user", func(t *testing.T) {
		user := database.User{
			ID:       1,
			Username: "user1",
			UUID:     uuid.NewString(),
		}
		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "user1").Return(user, nil).Once()

		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		mockTokenStore.EXPECT().IsExist(mock.Anything, "user1", "new_token_name", "git").
			Return(false, nil).Once()

		token := &database.AccessToken{
			ID:          1,
			UserID:      1,
			Name:        "new_token_name",
			Application: "git",
			Permission:  "",
			ExpiredAt:   time.Now().Add(time.Hour),
		}
		mockTokenStore.EXPECT().Create(mock.Anything, token).
			Return(nil).Once()

		mockGitServer := mockgit.NewMockGitServer(t)
		mockGitServer.EXPECT().CreateUserToken(&types.CreateUserTokenRequest{
			Username:    "user1",
			TokenName:   token.Name,
			Application: token.Application,
			Permission:  token.Permission,
			ExpiredAt:   token.ExpiredAt,
		}).Return(token, nil)

		ac := &accessTokenComponentImpl{
			us: mockUserStore,
			ts: mockTokenStore,
			gs: mockGitServer,
		}
		dbtoken, err := ac.Create(context.Background(), &types.CreateUserTokenRequest{
			Username:    "user1",
			TokenName:   token.Name,
			Application: token.Application,
			Permission:  token.Permission,
			ExpiredAt:   token.ExpiredAt,
		})
		require.NoError(t, err)
		require.NotNil(t, dbtoken)
		require.Equal(t, "new_token_name", dbtoken.Name)
	})

	//TODO: add ut for starship and mirror token which depends on accounting client
}

func TestAccessTokenComponentImpl_Delete(t *testing.T) {
	t.Run("delete token for non-existent user", func(t *testing.T) {
		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().IsExist(mock.Anything, "user1").Return(false, nil).Once()

		ac := &accessTokenComponentImpl{
			us: mockUserStore,
		}

		err := ac.Delete(context.Background(), &types.DeleteUserTokenRequest{
			Username:    "user1",
			TokenName:   "test_token_name",
			Application: "git",
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "user does not exists")
	})

	t.Run("delete non-existent token", func(t *testing.T) {
		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().IsExist(mock.Anything, "user1").Return(true, nil).Once()

		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		mockTokenStore.EXPECT().IsExist(mock.Anything, "user1", "test_token_name", "git").
			Return(false, nil).Once()

		ac := &accessTokenComponentImpl{
			us: mockUserStore,
			ts: mockTokenStore,
		}

		err := ac.Delete(context.Background(), &types.DeleteUserTokenRequest{
			Username:    "user1",
			TokenName:   "test_token_name",
			Application: "git",
		})

		require.Error(t, err)
		require.Contains(t, err.Error(), "user access token does not exists")
	})

	t.Run("success delete token", func(t *testing.T) {
		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().IsExist(mock.Anything, "user1").Return(true, nil).Once()

		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		mockTokenStore.EXPECT().IsExist(mock.Anything, "user1", "test_token_name", "git").
			Return(true, nil).Once()
		mockTokenStore.EXPECT().Delete(mock.Anything, "user1", "test_token_name", "git").
			Return(nil).Once()

		mockGitServer := mockgit.NewMockGitServer(t)
		mockGitServer.EXPECT().DeleteUserToken(mock.Anything).Return(nil).Once()

		ac := &accessTokenComponentImpl{
			us: mockUserStore,
			ts: mockTokenStore,
			gs: mockGitServer,
		}

		err := ac.Delete(context.Background(), &types.DeleteUserTokenRequest{
			Username:    "user1",
			TokenName:   "test_token_name",
			Application: "git",
		})

		require.NoError(t, err)
	})
}

func TestAccessTokenComponentImpl_Check(t *testing.T) {
	t.Run("token not found", func(t *testing.T) {
		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		mockTokenStore.EXPECT().FindByToken(mock.Anything, "invalid-token", "git").
			Return(nil, sql.ErrNoRows).Once()

		ac := &accessTokenComponentImpl{
			ts: mockTokenStore,
		}

		resp, err := ac.Check(context.Background(), &types.CheckAccessTokenReq{
			Token:       "invalid-token",
			Application: "git",
		})

		require.Error(t, err)
		require.Empty(t, resp.Token)
	})

	t.Run("success check token", func(t *testing.T) {
		mockToken := &database.AccessToken{
			Token:       "valid-token",
			Name:        "test_token_name",
			Application: "git",
			Permission:  "read",
			User:        &database.User{Username: "user1", UUID: "user-uuid"},
			ExpiredAt:   time.Now().Add(time.Hour),
		}

		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		mockTokenStore.EXPECT().FindByToken(mock.Anything, "valid-token", "git").
			Return(mockToken, nil).Once()

		ac := &accessTokenComponentImpl{
			ts: mockTokenStore,
		}

		resp, err := ac.Check(context.Background(), &types.CheckAccessTokenReq{
			Token:       "valid-token",
			Application: "git",
		})

		require.NoError(t, err)
		require.Equal(t, "valid-token", resp.Token)
		require.Equal(t, "test_token_name", resp.TokenName)
		require.Equal(t, "git", string(resp.Application))
		require.Equal(t, "read", resp.Permission)
		require.Equal(t, "user1", resp.Username)
		require.Equal(t, "user-uuid", resp.UserUUID)
	})
}

func TestAccessTokenComponentImpl_GetTokens(t *testing.T) {
	t.Run("no tokens found", func(t *testing.T) {
		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		mockTokenStore.EXPECT().FindByUser(mock.Anything, "user1", "git").
			Return([]database.AccessToken{}, nil).Once()

		ac := &accessTokenComponentImpl{
			ts: mockTokenStore,
		}

		tokens, err := ac.GetTokens(context.Background(), "user1", "git")

		require.NoError(t, err)
		require.Empty(t, tokens)
	})

	t.Run("success get tokens", func(t *testing.T) {
		mockTokens := []database.AccessToken{
			{
				Token:       "token1",
				Name:        "token_name1",
				Application: "git",
				Permission:  "read",
				User:        &database.User{Username: "user1", UUID: "user-uuid1"},
				ExpiredAt:   time.Now().Add(time.Hour),
			},
			{
				Token:       "token2",
				Name:        "token_name2",
				Application: "git",
				Permission:  "write",
				User:        &database.User{Username: "user1", UUID: "user-uuid2"},
				ExpiredAt:   time.Now().Add(time.Hour),
			},
		}

		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		mockTokenStore.EXPECT().FindByUser(mock.Anything, "user1", "git").
			Return(mockTokens, nil).Once()

		ac := &accessTokenComponentImpl{
			ts: mockTokenStore,
		}

		tokens, err := ac.GetTokens(context.Background(), "user1", "git")

		require.NoError(t, err)
		require.Len(t, tokens, 2)
		require.Equal(t, "token1", tokens[0].Token)
		require.Equal(t, "token_name1", tokens[0].TokenName)
		require.Equal(t, "read", tokens[0].Permission)
		require.Equal(t, "token2", tokens[1].Token)
		require.Equal(t, "token_name2", tokens[1].TokenName)
		require.Equal(t, "write", tokens[1].Permission)
	})
}

func TestAccessTokenComponentImpl_RefreshToken(t *testing.T) {
	t.Run("token not found", func(t *testing.T) {
		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		mockTokenStore.EXPECT().FindByTokenName(mock.Anything, "user1", "test_token_name", "git").
			Return(nil, sql.ErrNoRows).Once()

		ac := &accessTokenComponentImpl{
			ts: mockTokenStore,
		}

		resp, err := ac.RefreshToken(context.Background(), "user1", "test_token_name", "git", time.Now().Add(time.Hour))
		require.Error(t, err)
		require.Empty(t, resp)
	})

	t.Run("success refresh token", func(t *testing.T) {
		mockToken := &database.AccessToken{
			Token:       "old-token",
			Name:        "test_token_name",
			Application: "git",
			Permission:  "read",
			User:        &database.User{Username: "user1", UUID: "user-uuid"},
			ExpiredAt:   time.Now(),
		}
		newTokenValue := "new-token"

		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		mockTokenStore.EXPECT().FindByTokenName(mock.Anything, "user1", "test_token_name", "git").
			Return(mockToken, nil).Once()

		newToken := new(database.AccessToken)
		*newToken = *mockToken
		newToken.Token = newTokenValue
		newToken.ExpiredAt = time.Now().Add(time.Hour)
		mockTokenStore.EXPECT().Refresh(mock.Anything, mockToken, newTokenValue, newToken.ExpiredAt).
			Return(newToken, nil).Once()

		mockGitServer := mockgit.NewMockGitServer(t)
		mockGitServer.EXPECT().DeleteUserToken(&types.DeleteUserTokenRequest{
			Username:  "user1",
			TokenName: "test_token_name",
		}).Return(nil).Once()
		mockGitServer.EXPECT().CreateUserToken(&types.CreateUserTokenRequest{
			Username:    "user1",
			TokenName:   "test_token_name",
			Application: types.AccessTokenAppCSGHub,
			Permission:  "read",
		}).Return(&database.AccessToken{
			Token: newTokenValue,
		}, nil)

		ac := &accessTokenComponentImpl{
			ts: mockTokenStore,
			gs: mockGitServer,
		}

		resp, err := ac.RefreshToken(context.Background(), "user1", "test_token_name", "git", newToken.ExpiredAt)
		require.NoError(t, err)
		require.Equal(t, newTokenValue, resp.Token)
		require.Equal(t, "test_token_name", resp.TokenName)
		require.Equal(t, newToken.ExpiredAt, resp.ExpireAt)
	})
}

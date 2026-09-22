package component

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockrebac "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rebac"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
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
		mockTokenStore.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).
			Return(nil).Once()

		ac := &accessTokenComponentImpl{
			us: mockUserStore,
			ts: mockTokenStore,
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
		require.ErrorIs(t, err, errorx.ErrNotFound)
	})

	t.Run("success delete token", func(t *testing.T) {
		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().IsExist(mock.Anything, "user1").Return(true, nil).Once()

		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		mockTokenStore.EXPECT().IsExist(mock.Anything, "user1", "test_token_name", "git").
			Return(true, nil).Once()
		mockTokenStore.EXPECT().Delete(mock.Anything, "user1", "test_token_name", "git").
			Return(nil).Once()

		ac := &accessTokenComponentImpl{
			us: mockUserStore,
			ts: mockTokenStore,
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
			Return(nil, errorx.ErrDatabaseNoRows).Once()

		ac := &accessTokenComponentImpl{
			ts: mockTokenStore,
		}

		resp, err := ac.Check(context.Background(), &types.CheckAccessTokenReq{
			Token:       "invalid-token",
			Application: "git",
		})

		require.Error(t, err)
		require.ErrorIs(t, err, errorx.ErrNotFound)
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

		tokens, err := ac.GetTokens(context.Background(), &types.GetAccessTokenRequest{
			Username:    "user1",
			Application: "git",
		})

		require.NoError(t, err)
		require.Empty(t, tokens)
	})

	t.Run("success get tokens", func(t *testing.T) {
		mockTokens := []database.AccessToken{
			{
				ID:          101,
				Token:       "token1",
				Name:        "token_name1",
				Application: "git",
				Permission:  "read",
				User:        &database.User{Username: "user1", UUID: "user-uuid1"},
				ExpiredAt:   time.Now().Add(time.Hour),
			},
			{
				ID:          102,
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

		mockTokenQuotaStore := mockdb.NewMockAccountAccessTokenQuotaStore(t)
		mockTokenQuotaStore.EXPECT().FindByTokenID(mock.Anything, int64(101)).
			Return([]database.AccountAccessTokenQuota{}, nil).Once()
		mockTokenQuotaStore.EXPECT().FindByTokenID(mock.Anything, int64(102)).
			Return([]database.AccountAccessTokenQuota{}, nil).Once()

		ac := &accessTokenComponentImpl{
			ts:              mockTokenStore,
			tokenQuotaStore: mockTokenQuotaStore,
		}

		tokens, err := ac.GetTokens(context.Background(), &types.GetAccessTokenRequest{
			Username:    "user1",
			Application: "git",
		})

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
			Return(nil, errorx.ErrDatabaseNoRows).Once()

		ac := &accessTokenComponentImpl{
			ts: mockTokenStore,
		}
		req := &types.RefreshTokenReq{
			Username:     "user1",
			TokenName:    "test_token_name",
			App:          "git",
			NewExpiredAt: time.Now().Add(time.Hour),
		}
		resp, err := ac.RefreshToken(context.Background(), req)
		require.Error(t, err)
		require.ErrorIs(t, err, errorx.ErrNotFound)
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
		mockTokenStore.EXPECT().Refresh(mock.Anything, mockToken, mock.Anything, newToken.ExpiredAt).
			Return(newToken, nil).Once()

		ac := &accessTokenComponentImpl{
			ts: mockTokenStore,
		}

		req := &types.RefreshTokenReq{
			Username:     "user1",
			TokenName:    "test_token_name",
			App:          "git",
			NewExpiredAt: newToken.ExpiredAt,
		}

		resp, err := ac.RefreshToken(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, newTokenValue, resp.Token)
		require.Equal(t, "test_token_name", resp.TokenName)
		require.Equal(t, newToken.ExpiredAt, resp.ExpireAt)
	})
}

func TestAccessTokenComponentImpl_GetOrCreateFirstAvaiToken(t *testing.T) {
	t.Run("get existing token", func(t *testing.T) {
		mockTokens := []database.AccessToken{
			{
				ID:          201,
				Token:       "existing-token",
				Name:        "first_token",
				Application: "git",
				Permission:  "read",
				User:        &database.User{Username: "user1", UUID: "user-uuid"},
				ExpiredAt:   time.Now().Add(time.Hour),
			},
		}

		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		mockTokenStore.EXPECT().FindByUser(mock.Anything, "user1", "git").
			Return(mockTokens, nil).Once()

		mockQuotaStore := mockdb.NewMockAccountAccessTokenQuotaStore(t)
		mockQuotaStore.EXPECT().FindByTokenID(mock.Anything, int64(201)).
			Return([]database.AccountAccessTokenQuota{}, nil).Once()

		ac := &accessTokenComponentImpl{
			ts:              mockTokenStore,
			tokenQuotaStore: mockQuotaStore,
		}

		token, err := ac.GetOrCreateFirstAvaiToken(context.Background(), "user1", "git", "first_token")
		require.NoError(t, err)
		require.Equal(t, "existing-token", token)
	})
}

func TestAccessTokenComponentImpl_Update(t *testing.T) {
	t.Run("token not found", func(t *testing.T) {
		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		mockTokenStore.EXPECT().GetByID(mock.Anything, int64(1)).
			Return(nil, errorx.ErrDatabaseNoRows).Once()

		ac := &accessTokenComponentImpl{
			ts: mockTokenStore,
		}

		resp, err := ac.Update(context.Background(), &types.UpdateAPIKeyRequest{
			ID:          1,
			NSUUID:      "test-ns-uuid",
			CurrentUser: "user1",
		})
		require.Error(t, err)
		require.Nil(t, resp)
	})

	t.Run("nsuuid mismatch", func(t *testing.T) {
		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		mockTokenStore.EXPECT().GetByID(mock.Anything, int64(1)).
			Return(&database.AccessToken{
				ID:       1,
				NsUUID:   "other-ns-uuid",
				IsActive: true,
			}, nil).Once()

		ac := &accessTokenComponentImpl{
			ts: mockTokenStore,
		}

		resp, err := ac.Update(context.Background(), &types.UpdateAPIKeyRequest{
			ID:          1,
			NSUUID:      "test-ns-uuid",
			CurrentUser: "user1",
		})
		require.Error(t, err)
		require.ErrorIs(t, err, errorx.ErrNotFound)
		require.Nil(t, resp)
	})

	t.Run("token is inactive", func(t *testing.T) {
		nsUUID := "test-ns-uuid"
		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		mockTokenStore.EXPECT().GetByID(mock.Anything, int64(1)).
			Return(&database.AccessToken{
				ID:       1,
				NsUUID:   nsUUID,
				IsActive: false,
			}, nil).Once()

		mockNsStore := mockdb.NewMockNamespaceStore(t)
		mockNsStore.EXPECT().FindByUUID(mock.Anything, nsUUID).
			Return(database.Namespace{
				UUID:          nsUUID,
				NamespaceType: database.UserNamespace,
				Path:          "user1",
			}, nil).Once()

		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "user1").
			Return(database.User{Username: "user1", UUID: "user1-uuid"}, nil).Once()

		mockAuthorizer := mockrebac.NewMockAuthorizer(t)
		mockAuthorizer.EXPECT().Check(mock.Anything, rebac.CheckRequest{
			Subject:     rebac.UserSubject("user1-uuid"),
			Relation:    rebac.NamespaceCanAdmin,
			Object:      rebac.NamespaceObject(nsUUID),
			Consistency: rebac.ConsistencyHigher,
		}).Return(rebac.Decision{Allowed: true}, nil).Once()

		ac := &accessTokenComponentImpl{
			ts:      mockTokenStore,
			nsStore: mockNsStore,
			us:      mockUserStore,
			rebac:   mockAuthorizer,
		}

		resp, err := ac.Update(context.Background(), &types.UpdateAPIKeyRequest{
			ID:          1,
			NSUUID:      nsUUID,
			CurrentUser: "user1",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "token is inactive")
		require.Nil(t, resp)
	})

	t.Run("success update with user namespace", func(t *testing.T) {
		nsUUID := "test-ns-uuid"
		tokenValue := "test-token-value"
		keyName := "updated-key-name"
		expiredAt := time.Now().Add(48 * time.Hour)
		quotaType := types.AccountingQuotaTypeMonthly
		valueType := types.AccountingQuotaValueTypeFee
		quota := float64(200.0)

		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		mockTokenStore.EXPECT().GetByID(mock.Anything, int64(1)).
			Return(&database.AccessToken{
				ID:          1,
				Token:       tokenValue,
				NsUUID:      nsUUID,
				IsActive:    true,
				Application: types.AccessTokenAppAIGateway,
				Name:        "old-key-name",
			}, nil).Once()

		mockNsStore := mockdb.NewMockNamespaceStore(t)
		mockNsStore.EXPECT().FindByUUID(mock.Anything, nsUUID).
			Return(database.Namespace{
				UUID:          nsUUID,
				NamespaceType: database.UserNamespace,
				Path:          "user1",
			}, nil).Once()

		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "user1").
			Return(database.User{Username: "user1", UUID: "user1-uuid"}, nil).Once()

		mockAuthorizer := mockrebac.NewMockAuthorizer(t)
		mockAuthorizer.EXPECT().Check(mock.Anything, rebac.CheckRequest{
			Subject:     rebac.UserSubject("user1-uuid"),
			Relation:    rebac.NamespaceCanAdmin,
			Object:      rebac.NamespaceObject(nsUUID),
			Consistency: rebac.ConsistencyHigher,
		}).Return(rebac.Decision{Allowed: true}, nil).Once()

		mockQuotaStore := mockdb.NewMockAccountAccessTokenQuotaStore(t)
		// FindByTokenID is called twice: once inside updateAccessTokenQuotas
		// to reconcile the requested quota with existing rows, and once at the
		// end of Update to build the response.
		mockQuotaStore.EXPECT().FindByTokenID(mock.Anything, mock.AnythingOfType("int64")).
			Return([]database.AccountAccessTokenQuota{
				{
					ID:        1,
					APIKey:    tokenValue,
					QuotaType: quotaType,
					ValueType: valueType,
					Quota:     quota,
				},
			}, nil).Twice()

		mockBillStore := mockdb.NewMockAccountBillStore(t)
		mockBillStore.EXPECT().SumValueByAPIKeyBetween(mock.Anything, mock.AnythingOfType("int64"), mock.Anything, mock.Anything).
			Return(float64(50.0), nil).Maybe()

		mockTokenStore.EXPECT().UpdateTokenAndQuotas(mock.Anything, mock.AnythingOfType("*database.AccessToken"), mock.AnythingOfType("[]*database.AccountAccessTokenQuota")).
			Return(&database.AccessToken{
				ID:          1,
				Token:       tokenValue,
				NsUUID:      nsUUID,
				IsActive:    true,
				Application: types.AccessTokenAppAIGateway,
				Name:        keyName,
				ExpiredAt:   expiredAt,
			}, nil).Once()

		ac := &accessTokenComponentImpl{
			ts:               mockTokenStore,
			nsStore:          mockNsStore,
			us:               mockUserStore,
			rebac:            mockAuthorizer,
			tokenQuotaStore:  mockQuotaStore,
			accountBillStore: mockBillStore,
		}

		resp, err := ac.Update(context.Background(), &types.UpdateAPIKeyRequest{
			ID:          1,
			NSUUID:      nsUUID,
			CurrentUser: "user1",
			KeyName:     &keyName,
			ExpiredAt:   &expiredAt,
			Quotas: []types.UpdateAPIKeyQuotaItem{
				{QuotaType: quotaType, ValueType: valueType, Quota: quota},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, keyName, resp.TokenName)
		require.Len(t, resp.Quotas, 1)
		require.Equal(t, quotaType, resp.Quotas[0].QuotaType)
		require.Equal(t, valueType, resp.Quotas[0].QuotaValueType)
		require.Equal(t, quota, resp.Quotas[0].Quota)
	})

	t.Run("success update with org namespace and admin user", func(t *testing.T) {
		nsUUID := "org-ns-uuid"
		tokenValue := "org-token-value"
		keyName := "updated-org-key"

		mockTokenStore := mockdb.NewMockAccessTokenStore(t)
		mockTokenStore.EXPECT().GetByID(mock.Anything, int64(1)).
			Return(&database.AccessToken{
				ID:          1,
				Token:       tokenValue,
				NsUUID:      nsUUID,
				IsActive:    true,
				Application: types.AccessTokenAppAIGateway,
				Name:        "old-org-key",
			}, nil).Once()

		mockNsStore := mockdb.NewMockNamespaceStore(t)
		mockNsStore.EXPECT().FindByUUID(mock.Anything, nsUUID).
			Return(database.Namespace{
				UUID:          nsUUID,
				NamespaceType: database.OrgNamespace,
				Path:          "test-org",
			}, nil).Once()

		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "admin").
			Return(database.User{Username: "admin", UUID: "admin-uuid"}, nil).Once()

		mockAuthorizer := mockrebac.NewMockAuthorizer(t)
		mockAuthorizer.EXPECT().Check(mock.Anything, rebac.CheckRequest{
			Subject:     rebac.UserSubject("admin-uuid"),
			Relation:    rebac.NamespaceCanAdmin,
			Object:      rebac.NamespaceObject(nsUUID),
			Consistency: rebac.ConsistencyHigher,
		}).Return(rebac.Decision{Allowed: true}, nil).Once()

		mockQuotaStore := mockdb.NewMockAccountAccessTokenQuotaStore(t)
		// FindByTokenID is called twice (reconcile + response building).
		mockQuotaStore.EXPECT().FindByTokenID(mock.Anything, mock.AnythingOfType("int64")).
			Return([]database.AccountAccessTokenQuota{}, nil).Twice()

		mockBillStore := mockdb.NewMockAccountBillStore(t)

		mockTokenStore.EXPECT().UpdateTokenAndQuotas(mock.Anything, mock.AnythingOfType("*database.AccessToken"), mock.AnythingOfType("[]*database.AccountAccessTokenQuota")).
			Return(&database.AccessToken{
				ID:          1,
				Token:       tokenValue,
				NsUUID:      nsUUID,
				IsActive:    true,
				Application: types.AccessTokenAppAIGateway,
				Name:        keyName,
			}, nil).Once()

		ac := &accessTokenComponentImpl{
			ts:               mockTokenStore,
			nsStore:          mockNsStore,
			us:               mockUserStore,
			rebac:            mockAuthorizer,
			tokenQuotaStore:  mockQuotaStore,
			accountBillStore: mockBillStore,
		}

		resp, err := ac.Update(context.Background(), &types.UpdateAPIKeyRequest{
			ID:          1,
			NSUUID:      nsUUID,
			CurrentUser: "admin",
			KeyName:     &keyName,
			Quotas: []types.UpdateAPIKeyQuotaItem{
				{QuotaType: types.AccountingQuotaTypeUnlimited, ValueType: types.AccountingQuotaValueTypeFee, Quota: 0},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, keyName, resp.TokenName)
	})
}

func TestAccessTokenComponentImpl_UpdateAccessTokenQuotas_CreateWhenMissing(t *testing.T) {
	mockQuotaStore := mockdb.NewMockAccountAccessTokenQuotaStore(t)
	mockBillStore := mockdb.NewMockAccountBillStore(t)
	mockQuotaStore.EXPECT().FindByTokenID(mock.Anything, int64(42)).
		Return([]database.AccountAccessTokenQuota{}, nil).Once()
	mockBillStore.EXPECT().SumValueByAPIKeyBetween(mock.Anything, int64(42), mock.Anything, mock.Anything).
		Return(float64(0), nil).Maybe()

	ac := &accessTokenComponentImpl{
		tokenQuotaStore:  mockQuotaStore,
		accountBillStore: mockBillStore,
	}

	token := &database.AccessToken{
		ID:    42,
		Token: "test-api-key",
	}

	result, err := ac.updateAccessTokenQuotas(context.Background(), token, []types.UpdateAPIKeyQuotaItem{
		{QuotaType: types.AccountingQuotaTypeMonthly, ValueType: types.AccountingQuotaValueTypeFee, Quota: 100},
	})
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, int64(0), result[0].ID)
	require.Equal(t, int64(42), result[0].TokenID)
	require.Equal(t, "test-api-key", result[0].APIKey)
}

func TestAccessTokenComponentImpl_UpdateWithMultipleQuotas(t *testing.T) {
	nsUUID := "org-ns-uuid"
	tokenValue := "org-token-value"
	tokenID := int64(1)

	mockTokenStore := mockdb.NewMockAccessTokenStore(t)
	mockTokenStore.EXPECT().GetByID(mock.Anything, tokenID).
		Return(&database.AccessToken{
			ID:          tokenID,
			Token:       tokenValue,
			NsUUID:      nsUUID,
			IsActive:    true,
			Application: types.AccessTokenAppAIGateway,
		}, nil).Once()

	mockNsStore := mockdb.NewMockNamespaceStore(t)
	mockNsStore.EXPECT().FindByUUID(mock.Anything, nsUUID).
		Return(database.Namespace{Path: "admin", UUID: nsUUID, NamespaceType: database.UserNamespace}, nil).Maybe()

	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "admin").
		Return(database.User{Username: "admin", UUID: "admin-uuid"}, nil).Once()

	mockAuthorizer := mockrebac.NewMockAuthorizer(t)
	mockAuthorizer.EXPECT().Check(mock.Anything, mock.MatchedBy(func(r rebac.CheckRequest) bool {
		return r.Relation == rebac.NamespaceCanAdmin
	})).Return(rebac.Decision{Allowed: true}, nil).Once()

	mockQuotaStore := mockdb.NewMockAccountAccessTokenQuotaStore(t)
	// FindByTokenID is called twice: the first call reads the pre-update rows
	// for reconciliation, the second re-reads the rows to build the response.
	mockQuotaStore.EXPECT().FindByTokenID(mock.Anything, tokenID).
		Return([]database.AccountAccessTokenQuota{
			{ID: 10, APIKey: tokenValue, TokenID: tokenID, QuotaType: types.AccountingQuotaTypeMonthly, ValueType: types.AccountingQuotaValueTypeFee, Quota: 50},
		}, nil).Once()
	mockQuotaStore.EXPECT().FindByTokenID(mock.Anything, tokenID).
		Return([]database.AccountAccessTokenQuota{
			{ID: 10, APIKey: tokenValue, TokenID: tokenID, QuotaType: types.AccountingQuotaTypeMonthly, ValueType: types.AccountingQuotaValueTypeFee, Quota: 100},
			{ID: 11, APIKey: tokenValue, TokenID: tokenID, QuotaType: types.AccountingQuotaTotal, ValueType: types.AccountingQuotaValueTypeFee, Quota: 500},
		}, nil).Once()

	mockBillStore := mockdb.NewMockAccountBillStore(t)
	mockBillStore.EXPECT().SumValueByAPIKeyBetween(mock.Anything, mock.AnythingOfType("int64"), mock.Anything, mock.Anything).
		Return(float64(10), nil).Maybe()
	mockBillStore.EXPECT().SumValueByAPIKey(mock.Anything, mock.AnythingOfType("int64")).
		Return(float64(10), nil).Maybe()

	// Expect a single UpdateTokenAndQuotas call with a 2-element quota slice.
	mockTokenStore.EXPECT().UpdateTokenAndQuotas(
		mock.Anything,
		mock.AnythingOfType("*database.AccessToken"),
		mock.MatchedBy(func(qs []*database.AccountAccessTokenQuota) bool { return len(qs) == 2 }),
	).Return(&database.AccessToken{
		ID:          tokenID,
		Token:       tokenValue,
		NsUUID:      nsUUID,
		IsActive:    true,
		Application: types.AccessTokenAppAIGateway,
	}, nil).Once()

	ac := &accessTokenComponentImpl{
		ts:               mockTokenStore,
		nsStore:          mockNsStore,
		us:               mockUserStore,
		rebac:            mockAuthorizer,
		tokenQuotaStore:  mockQuotaStore,
		accountBillStore: mockBillStore,
	}

	quotaTypeMonthly := types.AccountingQuotaTypeMonthly
	valueTypeFee := types.AccountingQuotaValueTypeFee
	quotaMonthly := 100.0
	quotaTypeTotal := types.AccountingQuotaTotal
	quotaTotal := 500.0

	resp, err := ac.Update(context.Background(), &types.UpdateAPIKeyRequest{
		ID:          tokenID,
		NSUUID:      nsUUID,
		CurrentUser: "admin",
		Quotas: []types.UpdateAPIKeyQuotaItem{
			// ID 10 matches the pre-existing monthly/fee row, so this item
			// updates that row in place under the ID-matching semantics.
			{ID: 10, QuotaType: quotaTypeMonthly, ValueType: valueTypeFee, Quota: quotaMonthly},
			{QuotaType: quotaTypeTotal, ValueType: valueTypeFee, Quota: quotaTotal},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Quotas, 2)
}

// TestAccessTokenComponentImpl_UpdateAccessTokenQuotas_MatchesByID pins the
// unified quota reconciliation semantics shared by CE and EE: a request item
// matches an existing quota row by ID, so retyping a quota (e.g. monthly/fee
// to daily/fee on the same row) updates that row instead of inserting a
// duplicate that would violate the (token_id, quota_type, value_type) unique
// index at commit time.
func TestAccessTokenComponentImpl_UpdateAccessTokenQuotas_MatchesByID(t *testing.T) {
	mockQuotaStore := mockdb.NewMockAccountAccessTokenQuotaStore(t)
	mockBillStore := mockdb.NewMockAccountBillStore(t)
	mockQuotaStore.EXPECT().FindByTokenID(mock.Anything, int64(42)).
		Return([]database.AccountAccessTokenQuota{
			{ID: 1, APIKey: "test-api-key", TokenID: 42, QuotaType: types.AccountingQuotaTypeMonthly, ValueType: types.AccountingQuotaValueTypeFee, Quota: 100},
		}, nil).Once()
	mockBillStore.EXPECT().SumValueByAPIKeyBetween(mock.Anything, int64(42), mock.Anything, mock.Anything).
		Return(float64(0), nil).Maybe()

	ac := &accessTokenComponentImpl{
		tokenQuotaStore:  mockQuotaStore,
		accountBillStore: mockBillStore,
	}

	token := &database.AccessToken{
		ID:    42,
		Token: "test-api-key",
	}

	result, err := ac.updateAccessTokenQuotas(context.Background(), token, []types.UpdateAPIKeyQuotaItem{
		{ID: 1, QuotaType: types.AccountingQuotaTypeDaily, ValueType: types.AccountingQuotaValueTypeFee, Quota: 50},
	})
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, int64(1), result[0].ID, "must reuse the existing row matched by ID")
	require.Equal(t, types.AccountingQuotaTypeDaily, result[0].QuotaType)
	require.Equal(t, types.AccountingQuotaValueTypeFee, result[0].ValueType)
	require.Equal(t, float64(50), result[0].Quota)
}

// TestAccessTokenComponentImpl_UpdateAccessTokenQuotas_ZeroIDCreatesNewRow
// ensures a request item without an ID (ID 0) is treated as an insertion and
// never silently matches an existing row.
func TestAccessTokenComponentImpl_UpdateAccessTokenQuotas_ZeroIDCreatesNewRow(t *testing.T) {
	mockQuotaStore := mockdb.NewMockAccountAccessTokenQuotaStore(t)
	mockBillStore := mockdb.NewMockAccountBillStore(t)
	mockQuotaStore.EXPECT().FindByTokenID(mock.Anything, int64(42)).
		Return([]database.AccountAccessTokenQuota{
			{ID: 1, APIKey: "test-api-key", TokenID: 42, QuotaType: types.AccountingQuotaTypeMonthly, ValueType: types.AccountingQuotaValueTypeFee, Quota: 100},
		}, nil).Once()
	mockBillStore.EXPECT().SumValueByAPIKey(mock.Anything, int64(42)).
		Return(float64(0), nil).Maybe()

	ac := &accessTokenComponentImpl{
		tokenQuotaStore:  mockQuotaStore,
		accountBillStore: mockBillStore,
	}

	token := &database.AccessToken{
		ID:    42,
		Token: "test-api-key",
	}

	result, err := ac.updateAccessTokenQuotas(context.Background(), token, []types.UpdateAPIKeyQuotaItem{
		{ID: 0, QuotaType: types.AccountingQuotaTotal, ValueType: types.AccountingQuotaValueTypeFee, Quota: 500},
	})
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, int64(0), result[0].ID, "zero ID must insert a new row, not reuse an existing one")
	require.Equal(t, int64(42), result[0].TokenID)
	require.Equal(t, types.AccountingQuotaTotal, result[0].QuotaType)
}

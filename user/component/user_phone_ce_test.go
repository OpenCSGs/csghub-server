//go:build !saas && !ee

package component

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func TestUserPhoneComponent_CanChangePhone_SSOUserPhoneNotExist(t *testing.T) {
	ssomock := mockrpc.NewMockSSOInterface(t)
	ssomock.EXPECT().IsExistByPhone(mock.Anything, "13626487789").Return(false, nil)

	uc := &userPhoneComponentImpl{
		sso: ssomock,
		config: &config.Config{
			SSOType: "casdoor",
		},
	}
	user := &database.User{
		RegProvider: "casdoor",
	}

	can, err := uc.CanChangePhone(context.Background(), user, "13626487789")
	require.NoError(t, err)
	require.True(t, can)
}

func TestUserPhoneComponent_CanChangePhone_SSOUserPhoneExist(t *testing.T) {
	ssomock := mockrpc.NewMockSSOInterface(t)
	ssomock.EXPECT().IsExistByPhone(mock.Anything, "13626487789").Return(true, nil)

	uc := &userPhoneComponentImpl{
		sso: ssomock,
		config: &config.Config{
			SSOType: "casdoor",
		},
	}
	user := &database.User{
		RegProvider: "casdoor",
	}

	can, err := uc.CanChangePhone(context.Background(), user, "13626487789")
	require.ErrorIs(t, err, errorx.ErrPhoneAlreadyExistsInSSO)
	require.False(t, can)
}

func TestUserPhoneComponent_CanChangePhone_SSOCheckError(t *testing.T) {
	ssomock := mockrpc.NewMockSSOInterface(t)
	expectedErr := fmt.Errorf("sso error")
	ssomock.EXPECT().IsExistByPhone(mock.Anything, "13626487789").Return(false, expectedErr)

	uc := &userPhoneComponentImpl{
		sso: ssomock,
		config: &config.Config{
			SSOType: "casdoor",
		},
	}
	user := &database.User{
		RegProvider: "casdoor",
	}

	can, err := uc.CanChangePhone(context.Background(), user, "13626487789")
	require.ErrorIs(t, err, expectedErr)
	require.False(t, can)
}

// TestAccessTokenCreationLimitCE verifies that CE does not select or resolve a namespace limit.
func TestAccessTokenCreationLimitCE(t *testing.T) {
	c := &accessTokenComponentImpl{}
	token := &database.AccessToken{Application: types.AccessTokenAppAIGateway, NsUUID: "ns", TokenType: types.AccessTokenTypeOwner}
	require.NoError(t, c.setAccessTokenCreationLimit(context.Background(), token, database.User{}))
	require.Nil(t, token.CreationLimit)
}

// TestCreateLegacyAPIKeyCE verifies that even a zero configured cap leaves legacy CE creation unrestricted.
func TestCreateLegacyAPIKeyCE(t *testing.T) {
	for _, limit := range []uint{0, 3} {
		t.Run(fmt.Sprintf("limit_%d", limit), func(t *testing.T) {
			user := database.User{ID: 1, Username: "alice"}
			users := mockdb.NewMockUserStore(t)
			users.EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil).Once()
			tokens := mockdb.NewMockAccessTokenStore(t)
			tokens.EXPECT().IsExist(mock.Anything, user.Username, "legacy-key", string(types.AccessTokenAppAIGateway)).Return(false, nil).Once()
			tokens.EXPECT().Create(mock.Anything, mock.MatchedBy(func(token *database.AccessToken) bool {
				return token.CreationLimit == nil && token.NsUUID == "" && token.UserID == user.ID
			}), mock.Anything).Return(nil).Once()
			cfg := &config.Config{}
			cfg.User.PersonalAPIKeyLimit = limit
			c := &accessTokenComponentImpl{config: cfg, us: users, ts: tokens, nsStore: mockdb.NewMockNamespaceStore(t)}
			token, err := c.Create(context.Background(), &types.CreateUserTokenRequest{
				Username: user.Username, TokenName: "legacy-key", Application: types.AccessTokenAppAIGateway,
			})
			require.NoError(t, err)
			require.Nil(t, token.CreationLimit)
		})
	}
}

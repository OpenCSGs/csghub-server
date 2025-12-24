//go:build !saas && !ee

package component

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
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

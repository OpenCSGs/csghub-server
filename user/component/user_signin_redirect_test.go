package component

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
)

func TestUserComponent_BuildSigninRedirect(t *testing.T) {
	t.Run("delegates to sso client", func(t *testing.T) {
		ssomock := mockrpc.NewMockSSOInterface(t)
		uc := &userComponentImpl{
			sso:    ssomock,
			once:   &sync.Once{},
			config: &config.Config{},
		}
		ssomock.EXPECT().BuildAuthorizationURL(mock.Anything, "casdoor").Return("http://portico.example.com/authorize?state=ab", "ab", nil)

		redirectURL, oauthState, err := uc.BuildSigninRedirect(context.Background(), "casdoor")

		require.NoError(t, err)
		assert.Equal(t, "http://portico.example.com/authorize?state=ab", redirectURL)
		assert.Equal(t, "ab", oauthState)
	})

	t.Run("wraps sso client error", func(t *testing.T) {
		ssomock := mockrpc.NewMockSSOInterface(t)
		uc := &userComponentImpl{
			sso:    ssomock,
			once:   &sync.Once{},
			config: &config.Config{},
		}
		ssomock.EXPECT().BuildAuthorizationURL(mock.Anything, "casdoor").Return("", "", errors.New("cache unavailable"))

		_, _, err := uc.BuildSigninRedirect(context.Background(), "casdoor")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cache unavailable")
	})
}

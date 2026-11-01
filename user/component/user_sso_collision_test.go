package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
)

// TestUserComponent_EnsureLocalUserFromSSO_RejectsCrossSourceBind covers the
// case where an SSO subject collides with a local user created by a different
// identity source. The external subject is only unique within its own issuer,
// so the lookup must confirm the stored provider matches the incoming one
// before reusing the account.
func TestUserComponent_EnsureLocalUserFromSSO_RejectsCrossSourceBind(t *testing.T) {
	tests := []struct {
		name              string
		storedRegProvider string
		incomingProvider  string
	}{
		{
			name:              "portico subject collides with casdoor user",
			storedRegProvider: rpc.SSOTypeCasdoor,
			incomingProvider:  rpc.SSOTypePortico,
		},
		{
			name:              "casdoor subject collides with portico user",
			storedRegProvider: rpc.SSOTypePortico,
			incomingProvider:  rpc.SSOTypeCasdoor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserStore := mockdb.NewMockUserStore(t)
			uc := &userComponentImpl{
				userStore: mockUserStore,
			}

			const collidingUUID = "colliding-sub"
			mockUserStore.EXPECT().
				IsExistByUUID(mock.Anything, collidingUUID).
				Return(true, nil)
			mockUserStore.EXPECT().
				FindByUUID(mock.Anything, collidingUUID).
				Return(&database.User{
					UUID:        collidingUUID,
					Username:    "victim",
					RegProvider: tt.storedRegProvider,
				}, nil)

			cu := &rpc.SSOUserInfo{
				UUID:        collidingUUID,
				RegProvider: tt.incomingProvider,
			}

			got, err := uc.ensureLocalUserFromSSO(context.Background(), cu)

			require.Error(t, err)
			assert.Nil(t, got)
			assert.Contains(t, err.Error(), "cross-source bind")
		})
	}
}

// TestUserComponent_EnsureLocalUserFromSSO_AllowsSameProvider guards the
// legitimate path: a returning SSO login whose provider matches the stored
// account must still resolve to the existing user.
func TestUserComponent_EnsureLocalUserFromSSO_AllowsSameProvider(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	uc := &userComponentImpl{
		userStore: mockUserStore,
	}

	const uuidStr = "sub-1"
	mockUserStore.EXPECT().
		IsExistByUUID(mock.Anything, uuidStr).
		Return(true, nil)
	mockUserStore.EXPECT().
		FindByUUID(mock.Anything, uuidStr).
		Return(&database.User{
			UUID:        uuidStr,
			Username:    "alice",
			RegProvider: rpc.SSOTypePortico,
		}, nil)
	// updateUserLastLoginAsync refreshes the login time in a background
	// goroutine, so the Update call is allowed but not required.
	mockUserStore.EXPECT().
		Update(mock.Anything, mock.Anything, "").
		Return(nil).
		Maybe()

	cu := &rpc.SSOUserInfo{
		UUID:        uuidStr,
		RegProvider: rpc.SSOTypePortico,
	}

	got, err := uc.ensureLocalUserFromSSO(context.Background(), cu)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, uuidStr, got.UUID)
}

package component

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestJwtComponent_GenerateToken(t *testing.T) {
	mockus := mockdb.NewMockUserStore(t)
	jwt := &jwtComponentImpl{
		SigningKey: []byte("test-signing-key"),
		us:         mockus,
		ValidTime:  time.Hour,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user := &database.User{
		UUID:     uuid.NewString(),
		Username: "test_user_name",
	}
	mockus.EXPECT().FindByUUID(ctx, mock.Anything, types.UserWithAdminPermissions).Return(user, nil)

	claims, token, err := jwt.GenerateLoginToken(ctx, types.CreateJWTReq{
		UUID: user.UUID,
	})
	require.NoError(t, err)

	require.Equal(t, user.Username, claims.CurrentUser)

	mockus.EXPECT().FindByUsername(ctx, user.Username, types.UserWithAdminPermissions).Return(*user, nil)
	parseUser, err := jwt.ParseToken(ctx, token)
	require.NoError(t, err)
	require.Equal(t, user.UUID, parseUser.UUID)
	require.Equal(t, user.Username, parseUser.Username)
}

func TestJwtComponent_NonAdminRefreshKeepsSlidingExpiration(t *testing.T) {
	mockus := mockdb.NewMockUserStore(t)
	jwtc := &jwtComponentImpl{
		SigningKey: []byte("test-signing-key"),
		us:         mockus,
		ValidTime:  24 * time.Hour,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user := &database.User{
		UUID:     uuid.NewString(),
		Username: "normal_user",
	}
	mockus.EXPECT().FindByUUID(ctx, user.UUID, types.UserWithAdminPermissions).Return(user, nil).Once()

	claims, token, err := jwtc.RefreshToken(ctx, types.RefreshJWTReq{
		UUID:     user.UUID,
		OldToken: "old-refresh-token",
	})
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.WithinDuration(t, time.Now().Add(24*time.Hour), claims.ExpiresAt.Time, 3*time.Second)
}

// TestJwtComponent_ParseToken_RoleAssignments verifies token verification returns current database role facts.
func TestJwtComponent_ParseToken_RoleAssignments(t *testing.T) {
	for _, tt := range userRoleOutputCases() {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			us := mockdb.NewMockUserStore(t)
			us.EXPECT().FindByUsername(ctx, tt.user.Username, types.UserWithAdminPermissions).Return(tt.user, nil).Once()
			jwtc := &jwtComponentImpl{SigningKey: []byte("test-signing-key"), us: us}
			token, err := jwtc.signClaims(jwtc.buildClaims(&tt.user, time.Now().Add(time.Hour)))
			require.NoError(t, err)

			user, err := jwtc.ParseToken(ctx, token)
			require.NoError(t, err)
			require.Equal(t, tt.user.UUID, user.UUID)
			require.Equal(t, tt.user.Username, user.Username)
			require.ElementsMatch(t, tt.wantRoles, user.Roles)
			require.Equal(t, tt.wantAdmin, user.Admin)
			require.Equal(t, tt.wantSuperAdmin, user.SuperAdmin)
		})
	}
}

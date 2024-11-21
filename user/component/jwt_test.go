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
		us:        mockus,
		ValidTime: time.Hour,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user := &database.User{
		UUID:     uuid.NewString(),
		Username: "test_user_name",
	}
	mockus.EXPECT().FindByUUID(ctx, mock.Anything).Return(user, nil)

	claims, token, err := jwt.GenerateToken(ctx, types.CreateJWTReq{
		UUID: user.UUID,
	})
	require.NoError(t, err)

	require.Equal(t, user.Username, claims.CurrentUser)

	mockus.EXPECT().FindByUsername(ctx, user.Username).Return(*user, nil)
	parseUser, err := jwt.ParseToken(ctx, token)
	require.NoError(t, err)
	require.Equal(t, user.UUID, parseUser.UUID)
	require.Equal(t, user.Username, parseUser.Username)
}

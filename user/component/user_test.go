package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
)

func TestUserComponent_CheckIfUserHasOrgs(t *testing.T) {
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().GetUserOwnOrgs(context.TODO(), "user1").Return([]database.Organization{}, 0, nil)
	mockOrgStore.EXPECT().GetUserOwnOrgs(context.TODO(), "user2").Return([]database.Organization{
		{ID: 1},
	}, 1, nil)
	uc := &userComponentImpl{
		orgStore: mockOrgStore,
	}

	has, err := uc.CheckIfUserHasOrgs(context.TODO(), "user1")
	require.Nil(t, err)
	require.False(t, has)

	has, err = uc.CheckIfUserHasOrgs(context.TODO(), "user2")
	require.Nil(t, err)
	require.True(t, has)
}

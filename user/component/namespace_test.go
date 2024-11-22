package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestNamespaceComponent_GetInfo(t *testing.T) {
	t.Run("user namespace", func(t *testing.T) {
		user := database.User{
			ID:       1,
			Username: "user1",
			Avatar:   "user_avatar",
		}
		namespace := database.Namespace{
			ID:            1,
			Path:          user.Username,
			UserID:        user.ID,
			User:          user,
			NamespaceType: database.UserNamespace,
			Mirrored:      false,
		}
		mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
		mockNamespaceStore.EXPECT().FindByPath(mock.Anything, user.Username).Return(namespace, nil).Once()

		mc := &namespaceComponentImpl{
			ns: mockNamespaceStore,
		}
		actual, err := mc.GetInfo(context.Background(), user.Username)
		require.Empty(t, err)

		expected := &types.Namespace{
			Path:   user.Username,
			Type:   "user",
			Avatar: user.Avatar,
		}
		require.EqualValues(t, expected, actual)
	})

	t.Run("org namespace", func(t *testing.T) {
		user := database.User{
			ID:       1,
			Username: "user1",
			Avatar:   "user_avatar",
		}
		org := database.Organization{
			ID:      1,
			Name:    "org1",
			Logo:    "org_logo",
			OrgType: "school",
		}
		namespace := database.Namespace{
			ID:            1,
			Path:          org.Name,
			UserID:        user.ID,
			User:          user,
			NamespaceType: database.OrgNamespace,
			Mirrored:      false,
		}
		mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
		mockNamespaceStore.EXPECT().FindByPath(mock.Anything, org.Name).Return(namespace, nil).Once()

		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockOrgStore.EXPECT().FindByPath(mock.Anything, org.Name).Return(org, nil).Once()

		mc := &namespaceComponentImpl{
			ns: mockNamespaceStore,
			os: mockOrgStore,
		}
		actual, err := mc.GetInfo(context.Background(), org.Name)
		require.Empty(t, err)

		expected := &types.Namespace{
			Path:   org.Name,
			Type:   org.OrgType,
			Avatar: org.Logo,
		}
		require.EqualValues(t, expected, actual)

	})
}

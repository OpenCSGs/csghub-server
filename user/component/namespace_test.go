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
			NSType: "user",
			User:   types.User{ID: user.ID, Username: user.Username},
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
			NSType: "organization",
			User:   types.User{ID: user.ID, Username: user.Username},
		}
		require.EqualValues(t, expected, actual)

	})
}

func TestNamespaceComponent_GetInfoByUUID(t *testing.T) {
	t.Run("user namespace by uuid", func(t *testing.T) {
		namespaceUUID := "test-namespace-uuid"
		user := database.User{
			ID:       1,
			Username: "user1",
			Avatar:   "user_avatar",
		}
		namespace := database.Namespace{
			ID:            1,
			Path:          user.Username,
			UUID:          namespaceUUID,
			UserID:        user.ID,
			User:          user,
			NamespaceType: database.UserNamespace,
			Mirrored:      false,
		}
		mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
		mockNamespaceStore.EXPECT().FindByUUID(mock.Anything, namespaceUUID).Return(namespace, nil).Once()

		mc := &namespaceComponentImpl{
			ns: mockNamespaceStore,
		}
		actual, err := mc.GetInfoByUUID(context.Background(), namespaceUUID)
		require.Empty(t, err)

		expected := &types.Namespace{
			Path:   user.Username,
			Type:   "user",
			UUID:   namespaceUUID,
			Avatar: user.Avatar,
			NSType: "user",
			User:   types.User{ID: user.ID, Username: user.Username},
		}
		require.EqualValues(t, expected, actual)
	})

	t.Run("org namespace by uuid", func(t *testing.T) {
		namespaceUUID := "test-namespace-uuid"
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
			UUID:          namespaceUUID,
			UserID:        user.ID,
			User:          user,
			NamespaceType: database.OrgNamespace,
			Mirrored:      false,
		}
		mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
		mockNamespaceStore.EXPECT().FindByUUID(mock.Anything, namespaceUUID).Return(namespace, nil).Once()

		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockOrgStore.EXPECT().FindByPath(mock.Anything, org.Name).Return(org, nil).Once()

		mc := &namespaceComponentImpl{
			ns: mockNamespaceStore,
			os: mockOrgStore,
		}
		actual, err := mc.GetInfoByUUID(context.Background(), namespaceUUID)
		require.Empty(t, err)

		expected := &types.Namespace{
			Path:   org.Name,
			Type:   org.OrgType,
			UUID:   namespaceUUID,
			Avatar: org.Logo,
			NSType: "organization",
			User:   types.User{ID: user.ID, Username: user.Username},
		}
		require.EqualValues(t, expected, actual)
	})
}

func TestNamespaceComponent_GetInfoByUUIDs(t *testing.T) {
	t.Run("user and org namespaces by uuids", func(t *testing.T) {
		userNamespaceUUID := "11111111-1111-4111-8111-111111111111"
		orgNamespaceUUID := "22222222-2222-4222-8222-222222222222"
		user := database.User{
			ID:       1,
			Username: "user1",
			Avatar:   "user_avatar",
		}
		org := database.Organization{
			ID:      1,
			Name:    "org1",
			Logo:    "org_logo",
			OrgType: "enterprise",
		}
		userNamespace := database.Namespace{
			ID:            1,
			Path:          user.Username,
			UUID:          userNamespaceUUID,
			UserID:        user.ID,
			User:          user,
			NamespaceType: database.UserNamespace,
			Mirrored:      false,
		}
		orgNamespace := database.Namespace{
			ID:            2,
			Path:          org.Name,
			UUID:          orgNamespaceUUID,
			UserID:        user.ID,
			User:          user,
			NamespaceType: database.OrgNamespace,
			Mirrored:      false,
		}

		mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
		mockNamespaceStore.EXPECT().FindByUUIDs(mock.Anything, []string{userNamespaceUUID, orgNamespaceUUID}).
			Return([]database.Namespace{userNamespace, orgNamespace}, nil).Once()
		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockOrgStore.EXPECT().FindByPath(mock.Anything, org.Name).Return(org, nil).Once()

		mc := &namespaceComponentImpl{
			ns: mockNamespaceStore,
			os: mockOrgStore,
		}
		actual, err := mc.GetInfoByUUIDs(context.Background(), []string{userNamespaceUUID, orgNamespaceUUID})
		require.NoError(t, err)

		require.Equal(t, []*types.Namespace{
			{
				Path:   user.Username,
				Type:   string(database.UserNamespace),
				UUID:   userNamespaceUUID,
				Avatar: user.Avatar,
				NSType: string(database.UserNamespace),
				User:   types.User{ID: user.ID, Username: user.Username},
			},
			{
				Path:   org.Name,
				Type:   org.OrgType,
				UUID:   orgNamespaceUUID,
				Avatar: org.Logo,
				NSType: string(database.OrgNamespace),
				User:   types.User{ID: user.ID, Username: user.Username},
			},
		}, actual)
	})
}

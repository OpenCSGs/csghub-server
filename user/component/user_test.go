package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockgit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
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
func TestUserComponent_FindByUUIDs(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)

	uuids := []string{"uuid1", "uuid2"}

	mockUserStore.EXPECT().FindByUUIDs(context.TODO(), uuids).Return([]*database.User{
		{
			ID:       1,
			Username: "user1",
		},
		{
			ID:       2,
			Username: "user2",
		},
	}, nil)

	uc := &userComponentImpl{
		userStore: mockUserStore,
	}

	users, err := uc.FindByUUIDs(context.TODO(), uuids)

	require.Nil(t, err)
	require.Len(t, users, 2)

	require.Equal(t, int64(1), users[0].ID)
	require.Equal(t, "user1", users[0].Username)

	require.Equal(t, int64(2), users[1].ID)
	require.Equal(t, "user2", users[1].Username)
}

func TestUserComponent_SoftDelete(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	mockAuditStore := mockdb.NewMockAuditLogStore(t)
	user := database.User{
		Username: "user1",
	}
	mockAuditStore.EXPECT().Create(context.TODO(), mock.Anything).Return(nil)
	mockUserStore.EXPECT().SoftDeleteUserAndRelations(context.TODO(), user, types.CloseAccountReq{}).Return(nil)
	mockUserStore.EXPECT().FindByUsername(context.TODO(), user.Username).Return(user, nil)
	mockUserStore.EXPECT().FindByUsernameWithDeleted(context.TODO(), user.Username).Return(user, nil)
	uc := &userComponentImpl{
		userStore: mockUserStore,
		audit:     mockAuditStore,
	}

	err := uc.SoftDelete(context.TODO(), "user1", "user2", types.CloseAccountReq{})
	require.NotNil(t, err)

	err = uc.SoftDelete(context.TODO(), "user1", "user1", types.CloseAccountReq{})
	require.Nil(t, err)
}

func TestUserComponent_ResetUserTags(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		Username: "user1",
		UUID:     "uuid1",
	}
	tagIds := []int64{1, 2}
	mockUserStore.EXPECT().FindByUUID(context.TODO(), user.UUID).Return(user, nil)
	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), tagIds).Return(nil)
	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), user.ID, mock.Anything).Return(nil)
	uc := &userComponentImpl{
		userStore: mockUserStore,
		ts:        mockTagStore,
		uts:       mockUserTagStore,
	}

	err := uc.ResetUserTags(context.TODO(), user.UUID, tagIds)
	require.Nil(t, err)
}

func TestUserComponent_Delete(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	mockAuditStore := mockdb.NewMockAuditLogStore(t)
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockPendingDeletionStore := mockdb.NewMockPendingDeletionStore(t)
	mockGitserver := mockgit.NewMockGitServer(t)
	user1 := database.User{
		Username: "user1",
	}
	user2 := database.User{
		Username: "user2",
	}
	mockAuditStore.EXPECT().Create(context.TODO(), mock.Anything).Return(nil)
	mockUserStore.EXPECT().DeleteUserAndRelations(context.TODO(), user2, types.CloseAccountReq{}).Return(nil)
	mockUserStore.EXPECT().FindByUsernameWithDeleted(context.TODO(), user2.Username).Return(user2, nil)
	mockUserStore.EXPECT().FindByUsername(context.TODO(), user1.Username).Return(user1, nil)
	mockRepoStore.EXPECT().ByUser(context.TODO(), user2.ID, 1000, 0).Return([]database.Repository{{
		Path:           "foo/bar",
		RepositoryType: types.ModelRepo,
	}}, nil)
	mockRepoStore.EXPECT().ByUser(context.TODO(), user2.ID, 1000, 1).Return([]database.Repository{}, nil)
	mockPendingDeletionStore.EXPECT().Create(context.TODO(), &database.PendingDeletion{
		TableName: "repositories",
		Value:     "models_foo/bar.git",
	}).Return(nil)
	uc := &userComponentImpl{
		userStore: mockUserStore,
		audit:     mockAuditStore,
		repo:      mockRepoStore,
		gs:        mockGitserver,
		pdStore:   mockPendingDeletionStore,
		config:    &config.Config{},
	}
	uc.config.GitServer.Type = types.GitServerTypeGitaly

	err := uc.Delete(context.TODO(), "user1", "user2")
	require.Nil(t, err)
}

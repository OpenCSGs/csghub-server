package component

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockgit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/membership"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestMemberComponent_InitRoles(t *testing.T) {
	t.Run("init roles for gitea", func(t *testing.T) {
		config := &config.Config{}
		config.GitServer.Type = types.GitServerTypeGitea

		org := &database.Organization{
			Name: "org1",
		}
		roles := []membership.Role{membership.RoleAdmin, membership.RoleRead, membership.RoleWrite}
		mockGitMS := mockgit.NewMockGitMemerShip(t)
		mockGitMS.EXPECT().AddRoles(mock.Anything, org.Name, roles).Return(nil).Once()
		mc := &memberComponentImpl{
			gitMemberShip: mockGitMS,
			config:        config,
		}

		err := mc.InitRoles(context.Background(), org)
		require.Empty(t, err)
	})

	t.Run("init roles for gitaly", func(t *testing.T) {
		config := &config.Config{}
		config.GitServer.Type = types.GitServerTypeGitaly

		org := &database.Organization{
			Name: "org1",
		}

		mc := &memberComponentImpl{
			gitMemberShip: nil,
			config:        config,
		}

		err := mc.InitRoles(context.Background(), org)
		require.Empty(t, err)
	})
}

func TestMemberComponent_SetAdmin(t *testing.T) {
	t.Run("set admin for gitea", func(t *testing.T) {
		config := &config.Config{}
		config.GitServer.Type = types.GitServerTypeGitea

		org := &database.Organization{
			Name: "org1",
		}
		user := &database.User{
			Username: "user1",
		}
		mockms := mockdb.NewMockMemberStore(t)
		mockms.EXPECT().Add(mock.Anything, org.ID, user.ID, string(membership.RoleAdmin)).Return(nil).Once()

		mockGitMS := mockgit.NewMockGitMemerShip(t)
		mockGitMS.EXPECT().AddMember(mock.Anything, org.Name, user.Username, membership.RoleAdmin).Return(nil).Once()
		mc := &memberComponentImpl{
			memberStore:   mockms,
			gitMemberShip: mockGitMS,
			config:        config,
		}

		err := mc.SetAdmin(context.Background(), org, user)
		require.Empty(t, err)
	})

	t.Run("set admin for gitaly", func(t *testing.T) {
		config := &config.Config{}
		config.GitServer.Type = types.GitServerTypeGitaly

		org := &database.Organization{
			ID:   1,
			Name: "org1",
		}
		user := &database.User{
			ID:       1,
			Username: "user1",
		}

		mockms := mockdb.NewMockMemberStore(t)
		mockms.EXPECT().Add(mock.Anything, org.ID, user.ID, string(membership.RoleAdmin)).Return(nil).Once()

		mc := &memberComponentImpl{
			gitMemberShip: nil,
			config:        config,
			memberStore:   mockms,
		}

		err := mc.SetAdmin(context.Background(), org, user)
		require.Empty(t, err)
	})

}

func TestMemberComponent_GetMemberRole(t *testing.T) {
	t.Run("get member role for exsisting member ", func(t *testing.T) {
		config := &config.Config{}
		config.GitServer.Type = types.GitServerTypeGitea

		org := &database.Organization{
			ID:   1,
			Name: "org1",
		}
		user := &database.User{
			ID:       1,
			Username: "user1",
		}

		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockOrgStore.EXPECT().FindByPath(mock.Anything, org.Name).Return(*org, nil).Once()

		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

		mockMemberStore := mockdb.NewMockMemberStore(t)
		// user is not already a member
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, user.ID).Return(&database.Member{
			Role: string(membership.RoleAdmin),
		}, nil).Once()

		mc := &memberComponentImpl{
			orgStore:    mockOrgStore,
			userStore:   mockUserStore,
			memberStore: mockMemberStore,
			config:      config,
		}

		role, err := mc.GetMemberRole(context.Background(), org.Name, user.Username)
		require.Empty(t, err)
		require.Equal(t, membership.RoleAdmin, role)

	})

	t.Run("get member role for non-exsisting member ", func(t *testing.T) {
		config := &config.Config{}
		config.GitServer.Type = types.GitServerTypeGitea

		org := &database.Organization{
			ID:   1,
			Name: "org1",
		}
		user := &database.User{
			ID:       1,
			Username: "user1",
		}

		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockOrgStore.EXPECT().FindByPath(mock.Anything, org.Name).Return(*org, nil).Once()

		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

		mockMemberStore := mockdb.NewMockMemberStore(t)
		// user is not already a member
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, user.ID).Return(nil, nil).Once()

		mc := &memberComponentImpl{
			orgStore:    mockOrgStore,
			userStore:   mockUserStore,
			memberStore: mockMemberStore,
			config:      config,
		}

		role, err := mc.GetMemberRole(context.Background(), org.Name, user.Username)
		require.Empty(t, err)
		require.Equal(t, membership.RoleUnknown, role)

	})
}

func TestMemberComponent_AddMember(t *testing.T) {
	t.Run("add memberfor gitea", func(t *testing.T) {
		config := &config.Config{}
		config.GitServer.Type = types.GitServerTypeGitea
		config.Notification.NotificationRetryCount = 3
		config.APIToken = "test-api-token"
		config.Notification.Host = "localhost"
		config.Notification.Port = 8095

		org := &database.Organization{
			ID:   1,
			Name: "org1",
		}
		user := &database.User{
			ID:       1,
			Username: "user1",
		}

		operator := &database.User{
			ID:       2,
			Username: "op",
		}

		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockOrgStore.EXPECT().FindByPath(mock.Anything, org.Name).Return(*org, nil).Once()

		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, operator.Username).Return(*operator, nil).Once()
		mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

		mockMemberStore := mockdb.NewMockMemberStore(t)
		mockMemberStore.EXPECT().UserUUIDsByOrganizationID(mock.Anything, org.ID).Return([]string{"user0"}, nil).Once()
		// operator is org admin
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, operator.ID).Return(&database.Member{
			Role: string(membership.RoleAdmin),
		}, nil).Once()
		// user is not already a member
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, user.ID).Return(nil, nil).Once()
		// add user to org as member of role admin
		mockMemberStore.EXPECT().Add(mock.Anything, org.ID, user.ID, string(membership.RoleAdmin)).Return(nil).Once()

		mockGitMemberShip := mockgit.NewMockGitMemerShip(t)
		// add git membership
		mockGitMemberShip.EXPECT().AddMember(mock.Anything, org.Name, user.Username, membership.RoleAdmin).Return(nil).Once()
		// add notification rpc mock
		mockNotificationRpc := mockrpc.NewMockNotificationSvcClient(t)
		var wg sync.WaitGroup
		wg.Add(1)
		mockNotificationRpc.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
			defer wg.Done()
			if req.Scenario != types.MessageScenarioInternalNotification || req.Priority != types.MessagePriorityHigh {
				return false
			}

			var msg types.NotificationMessage
			if err := json.Unmarshal([]byte(req.Parameters), &msg); err != nil {
				return false
			}

			return msg.UserUUIDs[0] == "user0" &&
				msg.NotificationType == types.NotificationOrganization &&
				msg.Title == "Organization member change" &&
				msg.Content == fmt.Sprintf("New member %s joined organization %s.", user.Username, org.Name)
		})).Return(nil).Once()
		mc := &memberComponentImpl{
			orgStore:              mockOrgStore,
			userStore:             mockUserStore,
			memberStore:           mockMemberStore,
			gitMemberShip:         mockGitMemberShip,
			config:                config,
			notificationSvcClient: mockNotificationRpc,
		}

		err := mc.AddMember(context.Background(), org.Name, user.Username, operator.Username, string(membership.RoleAdmin))
		require.Empty(t, err)
		wg.Wait()
	})

	t.Run("add member for gitaly", func(t *testing.T) {
		config := &config.Config{}
		config.GitServer.Type = types.GitServerTypeGitaly
		config.Notification.NotificationRetryCount = 3

		org := &database.Organization{
			ID:   1,
			Name: "org1",
		}
		user := &database.User{
			ID:       1,
			Username: "user1",
		}

		operator := &database.User{
			ID:       2,
			Username: "op",
		}

		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockOrgStore.EXPECT().FindByPath(mock.Anything, org.Name).Return(*org, nil).Once()

		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, operator.Username).Return(*operator, nil).Once()
		mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

		mockMemberStore := mockdb.NewMockMemberStore(t)
		mockMemberStore.EXPECT().UserUUIDsByOrganizationID(mock.Anything, org.ID).Return([]string{"user0"}, nil).Once()
		// operator is org admin
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, operator.ID).Return(&database.Member{
			Role: string(membership.RoleAdmin),
		}, nil).Once()
		// user is not already a member
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, user.ID).Return(nil, nil).Once()
		// add user to org as member of role admin
		mockMemberStore.EXPECT().Add(mock.Anything, org.ID, user.ID, string(membership.RoleAdmin)).Return(nil).Once()
		mockNotificationRpc := mockrpc.NewMockNotificationSvcClient(t)
		var wg sync.WaitGroup
		wg.Add(1)
		mockNotificationRpc.EXPECT().
			Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
				defer wg.Done()
				if req.Scenario != types.MessageScenarioInternalNotification || req.Priority != types.MessagePriorityHigh {
					return false
				}

				var msg types.NotificationMessage
				if err := json.Unmarshal([]byte(req.Parameters), &msg); err != nil {
					return false
				}

				res := msg.UserUUIDs[0] == "user0" &&
					msg.NotificationType == types.NotificationOrganization &&
					msg.Title == "Organization member change" &&
					msg.Content == fmt.Sprintf("New member %s joined organization %s.", user.Username, org.Name)
				return res
			})).
			Return(nil).Once()

		mc := &memberComponentImpl{
			orgStore:              mockOrgStore,
			userStore:             mockUserStore,
			memberStore:           mockMemberStore,
			gitMemberShip:         nil,
			config:                config,
			notificationSvcClient: mockNotificationRpc,
		}

		err := mc.AddMember(context.Background(), org.Name, user.Username, operator.Username, string(membership.RoleAdmin))
		require.Empty(t, err)
		wg.Wait()
	})

}

func TestMemberComponent_Delete(t *testing.T) {
	t.Run("delete memberfor gitea", func(t *testing.T) {
		config := &config.Config{}
		config.GitServer.Type = types.GitServerTypeGitea
		config.Notification.NotificationRetryCount = 3

		org := &database.Organization{
			ID:   1,
			Name: "org1",
		}
		user := &database.User{
			ID:       1,
			Username: "user1",
		}

		operator := &database.User{
			ID:       2,
			Username: "op",
		}

		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockOrgStore.EXPECT().FindByPath(mock.Anything, org.Name).Return(*org, nil).Once()

		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, operator.Username).Return(*operator, nil).Once()
		mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

		mockMemberStore := mockdb.NewMockMemberStore(t)
		mockMemberStore.EXPECT().UserUUIDsByOrganizationID(mock.Anything, org.ID).Return([]string{"user0"}, nil).Once()
		// operator is org admin
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, operator.ID).Return(&database.Member{
			Role: string(membership.RoleAdmin),
		}, nil).Once()
		// user is already a member
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, user.ID).Return(&database.Member{
			ID:             1,
			OrganizationID: org.ID,
			UserID:         user.ID,
			Organization:   org,
			User:           user,
			Role:           string(membership.RoleAdmin),
		}, nil).Once()
		//  delete user role
		mockMemberStore.EXPECT().Delete(mock.Anything, org.ID, user.ID, string(membership.RoleAdmin)).Return(nil).Once()

		mockGitMemberShip := mockgit.NewMockGitMemerShip(t)
		// add git membership
		mockGitMemberShip.EXPECT().RemoveMember(mock.Anything, org.Name, user.Username, membership.RoleAdmin).Return(nil).Once()
		mockNotificationRpc := mockrpc.NewMockNotificationSvcClient(t)

		var wg sync.WaitGroup
		wg.Add(1)
		mockNotificationRpc.EXPECT().
			Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
				defer wg.Done()
				if req.Scenario != types.MessageScenarioInternalNotification || req.Priority != types.MessagePriorityHigh {
					return false
				}

				var msg types.NotificationMessage
				if err := json.Unmarshal([]byte(req.Parameters), &msg); err != nil {
					return false
				}

				return msg.NotificationType == types.NotificationOrganization &&
					msg.Title == "Organization member change" &&
					msg.Content == fmt.Sprintf("%s left the organization %s.", user.Username, org.Name)
			})).
			Return(nil).Once()

		mc := &memberComponentImpl{
			orgStore:              mockOrgStore,
			userStore:             mockUserStore,
			memberStore:           mockMemberStore,
			gitMemberShip:         mockGitMemberShip,
			config:                config,
			notificationSvcClient: mockNotificationRpc,
		}

		err := mc.Delete(context.Background(), org.Name, user.Username, operator.Username, string(membership.RoleAdmin))
		require.Empty(t, err)
		wg.Wait()
	})

	t.Run("delete member for gitaly", func(t *testing.T) {
		config := &config.Config{}
		config.GitServer.Type = types.GitServerTypeGitaly
		config.Notification.NotificationRetryCount = 3

		org := &database.Organization{
			ID:   1,
			Name: "org1",
		}
		user := &database.User{
			ID:       1,
			Username: "user1",
		}

		operator := &database.User{
			ID:       2,
			Username: "op",
		}

		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockOrgStore.EXPECT().FindByPath(mock.Anything, org.Name).Return(*org, nil).Once()

		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, operator.Username).Return(*operator, nil).Once()
		mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

		mockMemberStore := mockdb.NewMockMemberStore(t)
		mockMemberStore.EXPECT().UserUUIDsByOrganizationID(mock.Anything, org.ID).Return([]string{"user0"}, nil).Once()
		// operator is org admin
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, operator.ID).Return(&database.Member{
			Role: string(membership.RoleAdmin),
		}, nil).Once()
		// user is already a member
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, user.ID).Return(&database.Member{
			ID:             1,
			OrganizationID: org.ID,
			UserID:         user.ID,
			Organization:   org,
			User:           user,
			Role:           string(membership.RoleAdmin),
		}, nil).Once()
		//  delete user role
		mockMemberStore.EXPECT().Delete(mock.Anything, org.ID, user.ID, string(membership.RoleAdmin)).Return(nil).Once()
		mockNotificationRpc := mockrpc.NewMockNotificationSvcClient(t)
		var wg sync.WaitGroup
		wg.Add(1)
		mockNotificationRpc.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
			defer wg.Done()
			if req.Scenario != types.MessageScenarioInternalNotification || req.Priority != types.MessagePriorityHigh {
				return false
			}

			var msg types.NotificationMessage
			if err := json.Unmarshal([]byte(req.Parameters), &msg); err != nil {
				return false
			}

			return msg.NotificationType == types.NotificationOrganization &&
				msg.Title == "Organization member change" &&
				msg.Content == fmt.Sprintf("%s left the organization %s.", user.Username, org.Name)
		})).Return(nil).Once()

		mc := &memberComponentImpl{
			orgStore:              mockOrgStore,
			userStore:             mockUserStore,
			memberStore:           mockMemberStore,
			gitMemberShip:         nil,
			config:                config,
			notificationSvcClient: mockNotificationRpc,
		}

		err := mc.Delete(context.Background(), org.Name, user.Username, operator.Username, string(membership.RoleAdmin))
		require.Empty(t, err)
		wg.Wait()
	})
}

func TestMemberComponent_OrgMembers(t *testing.T) {
	t.Run("annonymous user without member details", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		orgName := "org1"
		// annonymous user
		userName := ""

		mockorg := mockdb.NewMockOrgStore(t)
		org := database.Organization{
			ID:   1,
			Name: "org1",
		}
		mockorg.EXPECT().FindByPath(ctx, orgName).Return(org, nil)

		mockus := mockdb.NewMockUserStore(t)
		// user not found
		mockus.EXPECT().FindByUsername(ctx, userName).Return(database.User{}, nil)

		mems := mockdb.NewMockMemberStore(t)
		members := []database.Member{}
		members = append(members, database.Member{
			ID:             1,
			OrganizationID: 1,
			UserID:         1,
			Role:           "role_1",
			User: &database.User{
				ID: 1, Username: "user1", NickName: "nick1", Avatar: "avatar1", UUID: "uuid1",
				LastLoginAt: "2020-01-01T00:00:00Z",
			},
		})
		members = append(members, database.Member{
			ID:             2,
			OrganizationID: 2,
			UserID:         2,
			Role:           "role_2",
			User: &database.User{
				ID: 2, Username: "user2", NickName: "nick2", Avatar: "avatar2", UUID: "uuid2",
				LastLoginAt: "2020-01-01T00:00:00Z",
			},
		})
		mems.EXPECT().OrganizationMembers(ctx, org.ID, mock.Anything, mock.Anything).Return(members, len(members), nil)

		mc := &memberComponentImpl{
			memberStore: mems,
			orgStore:    mockorg,
			userStore:   mockus,
		}

		expectedMembers := []types.Member{
			{
				Username: "user1",
				Nickname: "nick1",
				Avatar:   "avatar1",
				UUID:     "uuid1",
				// Role:        "role_1",
				// LastLoginAt: "2020-01-01T00:00:00Z",
			},
			{
				Username: "user2",
				Nickname: "nick2",
				Avatar:   "avatar2",
				UUID:     "uuid2",
				// Role:        "role_2",
				// LastLoginAt: "2020-01-01T00:00:00Z",
			},
		}
		actualMembers, cnt, err := mc.OrgMembers(ctx, orgName, userName, 0, 0)
		require.NoError(t, err)
		require.Equal(t, 2, cnt)
		require.Equal(t, expectedMembers, actualMembers)
	})

	t.Run("org member user with member details", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		orgName := "org1"
		// login user
		userName := "user1"

		mockorg := mockdb.NewMockOrgStore(t)
		org := database.Organization{
			ID:   1,
			Name: "org1",
		}
		mockorg.EXPECT().FindByPath(mock.Anything, orgName).Return(org, nil)

		mockus := mockdb.NewMockUserStore(t)
		user := database.User{
			ID:       1,
			Username: "user1",
		}
		mockus.EXPECT().FindByUsername(mock.Anything, userName).Return(user, nil)

		mems := mockdb.NewMockMemberStore(t)
		members := []database.Member{}
		members = append(members, database.Member{
			ID:             1,
			OrganizationID: 1,
			UserID:         1,
			Role:           "role_1",
			User: &database.User{
				ID: 1, Username: "user1", NickName: "nick1", Avatar: "avatar1", UUID: "uuid1",
				LastLoginAt: "2020-01-01T00:00:00Z",
			},
		})
		members = append(members, database.Member{
			ID:             2,
			OrganizationID: 2,
			UserID:         2,
			Role:           "role_2",
			User: &database.User{
				ID: 2, Username: "user2", NickName: "nick2", Avatar: "avatar2", UUID: "uuid2",
				LastLoginAt: "2020-01-01T00:00:00Z",
			},
		})
		mems.EXPECT().OrganizationMembers(mock.Anything, org.ID, mock.Anything, mock.Anything).Return(members, len(members), nil)
		mems.EXPECT().Find(mock.Anything, org.ID, user.ID).Return(&members[0], nil)

		mc := &memberComponentImpl{
			memberStore: mems,
			orgStore:    mockorg,
			userStore:   mockus,
		}

		expectedMembers := []types.Member{
			{
				Username:    "user1",
				Nickname:    "nick1",
				Avatar:      "avatar1",
				UUID:        "uuid1",
				Role:        "role_1",
				LastLoginAt: "2020-01-01T00:00:00Z",
			},
			{
				Username:    "user2",
				Nickname:    "nick2",
				Avatar:      "avatar2",
				UUID:        "uuid2",
				Role:        "role_2",
				LastLoginAt: "2020-01-01T00:00:00Z",
			},
		}
		actualMembers, cnt, err := mc.OrgMembers(ctx, orgName, userName, 0, 0)
		require.NoError(t, err)
		require.Equal(t, 2, cnt)
		require.Equal(t, expectedMembers, actualMembers)
	})
}

func TestMemberComponent_GetMember(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	orgName := "org1"
	// annonymous user
	userName := ""

	mockorg := mockdb.NewMockOrgStore(t)
	org := database.Organization{
		ID:   1,
		Name: "org1",
	}
	mockorg.EXPECT().FindByPath(ctx, orgName).Return(org, nil)

	user := database.User{
		ID: 1,
	}
	mockus := mockdb.NewMockUserStore(t)
	// user not found
	mockus.EXPECT().FindByUsername(ctx, userName).Return(user, nil)

	mems := mockdb.NewMockMemberStore(t)
	member := &database.Member{
		ID:             1,
		OrganizationID: 1,
		UserID:         1,
		Role:           "role_1",
		User: &database.User{
			ID: 1, Username: "user1", NickName: "nick1", Avatar: "avatar1", UUID: "uuid1",
			LastLoginAt: "2020-01-01T00:00:00Z",
		},
	}
	mems.EXPECT().Find(ctx, org.ID, member.UserID).Return(member, nil)
	mc := &memberComponentImpl{
		memberStore: mems,
		orgStore:    mockorg,
		userStore:   mockus,
	}
	m, err := mc.GetMember(ctx, orgName, userName)
	require.NoError(t, err)
	require.Equal(t, m.UserID, user.ID)
}

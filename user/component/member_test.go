package component

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockrebac "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rebac"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// expectOrganizationMemberReBACReconciliation configures direct role checks and resulting tuple mutations.
func expectOrganizationMemberReBACReconciliation(
	t *testing.T,
	authorizer *mockrebac.MockAuthorizer,
	organizationUUID, userUUID string,
	currentRelations map[rebac.Relation]bool,
	desiredRelation rebac.Relation,
) {
	t.Helper()
	checks := make([]rebac.BatchCheckItem, 0, len(organizationMemberRelations))
	results := make(map[string]rebac.BatchCheckOutcome, len(organizationMemberRelations))
	writes := make([]rebac.Relationship, 0, 1)
	deletes := make([]rebac.Relationship, 0, len(organizationMemberRelations))
	for relationIndex, relation := range organizationMemberRelations {
		correlationID := organizationMemberCorrelationID(0, relationIndex)
		checks = append(checks, rebac.BatchCheckItem{
			CorrelationID: correlationID,
			Check: rebac.CheckRequest{
				Subject:     rebac.UserSubject(userUUID),
				Relation:    relation,
				Object:      rebac.OrganizationObject(organizationUUID),
				Consistency: rebac.ConsistencyHigher,
			},
		})
		allowed := currentRelations[relation]
		results[correlationID] = rebac.BatchCheckOutcome{Decision: rebac.Decision{Allowed: allowed}}
		relationship := rebac.Relationship{
			Subject: rebac.UserSubject(userUUID), Relation: relation, Object: rebac.OrganizationObject(organizationUUID),
		}
		if allowed && relation != desiredRelation {
			deletes = append(deletes, relationship)
		}
		if !allowed && relation == desiredRelation {
			writes = append(writes, relationship)
		}
	}
	authorizer.EXPECT().BatchCheck(mock.Anything, rebac.BatchCheckRequest{Checks: checks}).Return(rebac.BatchCheckResult{Results: results}, nil).Once()
	if len(deletes) > 0 {
		authorizer.EXPECT().Delete(mock.Anything, deletes).Return(nil).Once()
	}
	if len(writes) > 0 {
		authorizer.EXPECT().Write(mock.Anything, writes).Return(nil).Once()
	}
}

func TestMemberComponent_GetMemberRole(t *testing.T) {
	t.Run("get member role for exsisting member", func(t *testing.T) {
		config := &config.Config{}

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
			Role: string(types.UserAdmin),
		}, nil).Once()

		mc := &memberComponentImpl{
			orgStore:    mockOrgStore,
			userStore:   mockUserStore,
			memberStore: mockMemberStore,
			config:      config,
		}

		role, err := mc.GetMemberRole(context.Background(), org.Name, user.Username)
		require.Empty(t, err)
		require.Equal(t, types.UserAdmin, role)

	})

	t.Run("get member role for non-exsisting member", func(t *testing.T) {
		config := &config.Config{}

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
		require.Equal(t, types.UserRole(""), role)

	})
}

func TestMemberComponent_Delete(t *testing.T) {

	t.Run("delete admin member", func(t *testing.T) {
		config := &config.Config{}
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
		mockMemberStore.EXPECT().
			OrganizationMembers(context.Background(), int64(1), "", 1, 1).
			Return(nil, 2, nil)
		// operator is org admin
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, operator.ID).Return(&database.Member{
			Role: string(types.UserAdmin),
		}, nil).Once()
		// user is already a member with admin role
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, user.ID).Return(&database.Member{
			ID:             1,
			OrganizationID: org.ID,
			UserID:         user.ID,
			Organization:   org,
			User:           user,
			Role:           string(types.UserAdmin),
		}, nil).Once()
		// delete user membership
		mockMemberStore.EXPECT().Delete(mock.Anything, org.ID, user.ID).Return(nil).Once()
		mockAuthorizer := mockrebac.NewMockAuthorizer(t)
		expectOrganizationMemberReBACReconciliation(t, mockAuthorizer, org.UUID.String(), user.UUID,
			map[rebac.Relation]bool{rebac.RelationAdmin: true}, "")
		mockNotificationRpc := mockrpc.NewMockNotificationSvcClient(t)
		var wg sync.WaitGroup
		wg.Add(1)
		mockNotificationRpc.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
			defer wg.Done()
			if req.Scenario != types.MessageScenarioOrgMember || req.Priority != types.MessagePriorityHigh {
				return false
			}

			var msg types.NotificationMessage
			if err := json.Unmarshal([]byte(req.Parameters), &msg); err != nil {
				return false
			}

			return msg.NotificationType == types.NotificationOrganization &&
				msg.Template == string(types.MessageScenarioOrgMember)
		})).Return(nil).Once()

		mc := &memberComponentImpl{
			orgStore:              mockOrgStore,
			userStore:             mockUserStore,
			memberStore:           mockMemberStore,
			config:                config,
			notificationSvcClient: mockNotificationRpc,
			rebac:                 mockAuthorizer,
		}

		err := mc.Delete(context.Background(), org.Name, user.Username, operator.Username)
		require.Empty(t, err)
		wg.Wait()
	})

	t.Run("delete non-admin member", func(t *testing.T) {
		config := &config.Config{}
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
		mockMemberStore.EXPECT().
			OrganizationMembers(context.Background(), int64(1), "", 1, 1).
			Return(nil, 2, nil)
		// operator is org admin
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, operator.ID).Return(&database.Member{
			Role: string(types.UserAdmin),
		}, nil).Once()
		// user is already a member with non-admin role (read role)
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, user.ID).Return(&database.Member{
			ID:             1,
			OrganizationID: org.ID,
			UserID:         user.ID,
			Organization:   org,
			User:           user,
			Role:           string(types.UserRead),
		}, nil).Once()
		// delete user membership
		mockMemberStore.EXPECT().Delete(mock.Anything, org.ID, user.ID).Return(nil).Once()
		mockAuthorizer := mockrebac.NewMockAuthorizer(t)
		expectOrganizationMemberReBACReconciliation(t, mockAuthorizer, org.UUID.String(), user.UUID,
			map[rebac.Relation]bool{rebac.RelationReader: true}, "")
		mockNotificationRpc := mockrpc.NewMockNotificationSvcClient(t)
		var wg sync.WaitGroup
		wg.Add(1)
		mockNotificationRpc.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
			defer wg.Done()
			if req.Scenario != types.MessageScenarioOrgMember || req.Priority != types.MessagePriorityHigh {
				return false
			}

			var msg types.NotificationMessage
			if err := json.Unmarshal([]byte(req.Parameters), &msg); err != nil {
				return false
			}

			return msg.NotificationType == types.NotificationOrganization &&
				msg.Template == string(types.MessageScenarioOrgMember)
		})).Return(nil).Once()

		mc := &memberComponentImpl{
			orgStore:              mockOrgStore,
			userStore:             mockUserStore,
			memberStore:           mockMemberStore,
			config:                config,
			notificationSvcClient: mockNotificationRpc,
			rebac:                 mockAuthorizer,
		}

		err := mc.Delete(context.Background(), org.Name, user.Username, operator.Username)
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
		mems.EXPECT().OrganizationMembers(ctx, org.ID, "", mock.Anything, mock.Anything).Return(members, len(members), nil)

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
		mems.EXPECT().OrganizationMembers(mock.Anything, org.ID, "", mock.Anything, mock.Anything).Return(members, len(members), nil)
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

func TestMemberComponent_GetMemberRoleByUUID(t *testing.T) {
	t.Run("get member role by uuid for existing member with admin role", func(t *testing.T) {
		config := &config.Config{}
		orgUUID := "org-uuid-123"

		org := &database.Organization{
			ID:   1,
			Name: "org1",
		}
		user := &database.User{
			ID:       1,
			Username: "user1",
		}

		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockOrgStore.EXPECT().FindByUUID(mock.Anything, orgUUID).Return(org, nil).Once()

		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

		mockMemberStore := mockdb.NewMockMemberStore(t)
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, user.ID).Return(&database.Member{
			Role: string(types.UserAdmin),
		}, nil).Once()

		mc := &memberComponentImpl{
			orgStore:    mockOrgStore,
			userStore:   mockUserStore,
			memberStore: mockMemberStore,
			config:      config,
		}

		role, err := mc.GetMemberRoleByUUID(context.Background(), orgUUID, user.Username)
		require.Empty(t, err)
		require.Equal(t, types.UserAdmin, role)
	})

	t.Run("get member role by uuid for existing member with write role", func(t *testing.T) {
		config := &config.Config{}
		orgUUID := "org-uuid-456"

		org := &database.Organization{
			ID:   1,
			Name: "org1",
		}
		user := &database.User{
			ID:       1,
			Username: "user1",
		}

		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockOrgStore.EXPECT().FindByUUID(mock.Anything, orgUUID).Return(org, nil).Once()

		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

		mockMemberStore := mockdb.NewMockMemberStore(t)
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, user.ID).Return(&database.Member{
			Role: string(types.UserWrite),
		}, nil).Once()

		mc := &memberComponentImpl{
			orgStore:    mockOrgStore,
			userStore:   mockUserStore,
			memberStore: mockMemberStore,
			config:      config,
		}

		role, err := mc.GetMemberRoleByUUID(context.Background(), orgUUID, user.Username)
		require.Empty(t, err)
		require.Equal(t, types.UserWrite, role)
	})

	t.Run("get member role by uuid for existing member with read role", func(t *testing.T) {
		config := &config.Config{}
		orgUUID := "org-uuid-789"

		org := &database.Organization{
			ID:   1,
			Name: "org1",
		}
		user := &database.User{
			ID:       1,
			Username: "user1",
		}

		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockOrgStore.EXPECT().FindByUUID(mock.Anything, orgUUID).Return(org, nil).Once()

		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

		mockMemberStore := mockdb.NewMockMemberStore(t)
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, user.ID).Return(&database.Member{
			Role: string(types.UserRead),
		}, nil).Once()

		mc := &memberComponentImpl{
			orgStore:    mockOrgStore,
			userStore:   mockUserStore,
			memberStore: mockMemberStore,
			config:      config,
		}

		role, err := mc.GetMemberRoleByUUID(context.Background(), orgUUID, user.Username)
		require.Empty(t, err)
		require.Equal(t, types.UserRead, role)
	})

	t.Run("get member role by uuid for non-existing member", func(t *testing.T) {
		config := &config.Config{}
		orgUUID := "org-uuid-999"

		org := &database.Organization{
			ID:   1,
			Name: "org1",
		}
		user := &database.User{
			ID:       1,
			Username: "user1",
		}

		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockOrgStore.EXPECT().FindByUUID(mock.Anything, orgUUID).Return(org, nil).Once()

		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

		mockMemberStore := mockdb.NewMockMemberStore(t)
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, user.ID).Return(nil, nil).Once()

		mc := &memberComponentImpl{
			orgStore:    mockOrgStore,
			userStore:   mockUserStore,
			memberStore: mockMemberStore,
			config:      config,
		}

		role, err := mc.GetMemberRoleByUUID(context.Background(), orgUUID, user.Username)
		require.Empty(t, err)
		require.Equal(t, types.UserRole(""), role)
	})
}

func TestMemberComponent_ChangeMemberRole(t *testing.T) {

	t.Run("update other org member role", func(t *testing.T) {
		config := &config.Config{}
		config.Notification.NotificationRetryCount = 3

		org := &database.Organization{
			ID:   1,
			Name: "org1",
		}
		user := &database.User{
			ID:       1,
			UUID:     "user1",
			Username: "user1",
		}
		operator := &database.User{
			ID:       2,
			UUID:     "op",
			Username: "op",
		}
		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockOrgStore.EXPECT().FindByPath(mock.Anything, org.Name).Return(*org, nil).Once()

		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, operator.Username).Return(*operator, nil).Once()
		mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

		mockMemberStore := mockdb.NewMockMemberStore(t)
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, operator.ID).Return(&database.Member{
			Role: string(types.UserAdmin),
		}, nil).Once()
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, user.ID).Return(&database.Member{
			Role: string(types.UserRead),
		}, nil).Once()
		mockMemberStore.EXPECT().Update(mock.Anything, org.ID, user.ID, string(types.UserWrite)).Return(nil).Once()
		mockMemberStore.EXPECT().UserUUIDsByOrganizationID(mock.Anything, org.ID).Return([]string{"op", "user1"}, nil).Once()
		mockAuthorizer := mockrebac.NewMockAuthorizer(t)
		expectOrganizationMemberReBACReconciliation(t, mockAuthorizer, org.UUID.String(), user.UUID,
			map[rebac.Relation]bool{rebac.RelationReader: true}, rebac.RelationWriter)
		mockNotificationRpc := mockrpc.NewMockNotificationSvcClient(t)

		var wg sync.WaitGroup
		wg.Add(1)
		mockNotificationRpc.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
			defer wg.Done()
			if req.Scenario != types.MessageScenarioOrgMember || req.Priority != types.MessagePriorityHigh {
				return false
			}

			var msg types.NotificationMessage
			if err := json.Unmarshal([]byte(req.Parameters), &msg); err != nil {
				return false
			}

			return msg.UserUUIDs[0] == "op" &&
				msg.NotificationType == types.NotificationOrganization &&
				msg.Template == string(types.MessageScenarioOrgMember)
		})).Return(nil).Once()
		mc := &memberComponentImpl{
			orgStore:              mockOrgStore,
			userStore:             mockUserStore,
			memberStore:           mockMemberStore,
			config:                config,
			notificationSvcClient: mockNotificationRpc,
			rebac:                 mockAuthorizer,
		}

		err := mc.ChangeMemberRole(context.Background(), org.Name, user.Username, operator.Username, string(types.UserRead), string(types.UserWrite))
		require.Empty(t, err)
		wg.Wait()
	})

	t.Run("update self org member role", func(t *testing.T) {
		config := &config.Config{}
		config.Notification.NotificationRetryCount = 3

		org := &database.Organization{
			ID:   1,
			Name: "org1",
		}
		user := &database.User{
			ID:       1,
			UUID:     "user1",
			Username: "user1",
		}
		operator := &database.User{
			ID:       user.ID,
			UUID:     user.UUID,
			Username: user.Username,
		}
		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockOrgStore.EXPECT().FindByPath(mock.Anything, org.Name).Return(*org, nil).Once()

		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, operator.Username).Return(*operator, nil).Once()
		mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

		mockMemberStore := mockdb.NewMockMemberStore(t)
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, operator.ID).Return(&database.Member{
			Role: string(types.UserAdmin),
		}, nil).Once()
		mockMemberStore.EXPECT().OrganizationMembers(mock.Anything, org.ID, string(types.UserAdmin), 1, 1).Return([]database.Member{}, 1, nil).Once()

		mc := &memberComponentImpl{
			orgStore:    mockOrgStore,
			userStore:   mockUserStore,
			memberStore: mockMemberStore,
			config:      config,
		}

		err := mc.ChangeMemberRole(context.Background(), org.Name, user.Username, operator.Username, string(types.UserAdmin), string(types.UserWrite))
		require.Error(t, err)
		require.Equal(t, errorx.LastOrgAdmin(errors.New("cannot revoke the last admin role from organization"), errorx.Ctx().Set("username", user.Username)), err)
	})

	t.Run("forbid promoting self to admin ", func(t *testing.T) {
		config := &config.Config{}
		config.Notification.NotificationRetryCount = 3

		org := &database.Organization{
			ID:   1,
			Name: "org1",
		}
		user := &database.User{
			ID:       1,
			UUID:     "user1",
			Username: "user1",
		}
		operator := &database.User{
			ID:       user.ID,
			UUID:     user.UUID,
			Username: user.Username,
		}
		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockOrgStore.EXPECT().FindByPath(mock.Anything, org.Name).Return(*org, nil).Once()

		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().FindByUsername(mock.Anything, operator.Username).Return(*operator, nil).Once()
		mockUserStore.EXPECT().FindByUsername(mock.Anything, user.Username).Return(*user, nil).Once()

		mockMemberStore := mockdb.NewMockMemberStore(t)
		mockMemberStore.EXPECT().Find(mock.Anything, org.ID, operator.ID).Return(&database.Member{
			Role: string(types.UserAdmin),
		}, nil).Once()
		mockMemberStore.EXPECT().OrganizationMembers(mock.Anything, org.ID, string(types.UserAdmin), 1, 1).Return([]database.Member{}, 1, nil).Once()

		mc := &memberComponentImpl{
			orgStore:    mockOrgStore,
			userStore:   mockUserStore,
			memberStore: mockMemberStore,
			config:      config,
		}

		err := mc.ChangeMemberRole(context.Background(), org.Name, user.Username, operator.Username, string(types.UserRead), string(types.UserAdmin))
		require.Error(t, err)
		require.Equal(t, errorx.CannotPromoteSelfToAdmin(errors.New("cannot promote yourself to admin"), errorx.Ctx().Set("username", user.Username)), err)
	})
}

package component

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockgit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockrebac "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rebac"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockphone "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/user/component"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// superAdminRoleAssignments builds the stored global grant used by administrator mocks.
func superAdminRoleAssignments() []database.UserRole {
	return []database.UserRole{{RoleCode: "super_admin", Role: &database.Role{IsAdmin: true,
		RoleCode: "super_admin", Permissions: []database.AdminRolePermission{{PermissionCode: types.AdminPermissionAll}},
	}}}
}

// userRoleOutputCase describes the authorization facts expected in a full user response.
type userRoleOutputCase struct {
	name           string
	user           database.User
	wantRoles      []string
	wantAdmin      bool
	wantSuperAdmin bool
}

// userRoleOutputCases keeps user and JWT response tests aligned on role assignments rather than legacy role names.
func userRoleOutputCases() []userRoleOutputCase {
	globalRole := &database.Role{IsAdmin: true,
		RoleCode: "owner",
		Permissions: []database.AdminRolePermission{
			{RoleCode: "owner", PermissionCode: types.AdminPermissionAll},
		},
	}
	cases := []userRoleOutputCase{
		{
			name:      "global grant on a custom role",
			user:      database.User{RoleAssignments: []database.UserRole{{RoleCode: "owner", Role: globalRole}}},
			wantRoles: []string{"owner"}, wantAdmin: true, wantSuperAdmin: true,
		},
		{
			name: "scoped administrator and ordinary role",
			user: database.User{RoleAssignments: []database.UserRole{
				{RoleCode: "personal_user"},
				{RoleCode: "content_operator", Role: &database.Role{IsAdmin: true,
					RoleCode: "content_operator",
					Permissions: []database.AdminRolePermission{
						{RoleCode: "content_operator", PermissionCode: types.AdminAssetModel},
					},
				}},
			}},
			wantRoles: []string{"personal_user", "content_operator"}, wantAdmin: true,
		},
		{
			name:      "administrator role name has no implicit grant",
			user:      database.User{RoleAssignments: []database.UserRole{{RoleCode: "super_admin"}}},
			wantRoles: []string{"super_admin"},
		},
		{
			name: "revoked administrator assignment",
			user: database.User{RoleAssignments: []database.UserRole{
				{RoleCode: "owner", Role: globalRole, DeletedAt: time.Unix(1, 0)},
				{RoleCode: "personal_user"},
			}},
			wantRoles: []string{"personal_user"},
		},
		{name: "legacy roles are not a fallback", wantRoles: []string{}},
	}
	for i := range cases {
		cases[i].user.ID = 1
		cases[i].user.UUID = "test-user-uuid"
		cases[i].user.Username = "testuser"
		cases[i].user.RoleMask = "admin,super_user"
	}
	return cases
}

func TestUserComponent_CheckIfUserIsLastOrgAdmin(t *testing.T) {
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().IsLastOrganizationAdmin(context.TODO(), "user1").Return(false, nil)
	mockOrgStore.EXPECT().IsLastOrganizationAdmin(context.TODO(), "user2").Return(true, nil)
	uc := &userComponentImpl{
		orgStore: mockOrgStore,
	}

	has, err := uc.CheckIfUserIsLastOrgAdmin(context.TODO(), "user1")
	require.Nil(t, err)
	require.False(t, has)

	has, err = uc.CheckIfUserIsLastOrgAdmin(context.TODO(), "user2")
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
	require.Empty(t, users[0].Email)

	require.Equal(t, int64(2), users[1].ID)
	require.Equal(t, "user2", users[1].Username)
	require.Empty(t, users[1].Email)
}

func TestUserComponent_GetAdminUserUUIDs(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		expectedUUIDs := []string{"admin-uuid", "super-user-uuid"}
		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().GetAdminUserUUIDs(ctx).Return(expectedUUIDs, nil)
		uc := &userComponentImpl{userStore: mockUserStore}

		adminUUIDs, err := uc.GetAdminUserUUIDs(ctx)

		require.NoError(t, err)
		require.Equal(t, expectedUUIDs, adminUUIDs)
	})

	t.Run("store error", func(t *testing.T) {
		ctx := context.Background()
		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().GetAdminUserUUIDs(ctx).Return(nil, errors.New("database error"))
		uc := &userComponentImpl{userStore: mockUserStore}

		adminUUIDs, err := uc.GetAdminUserUUIDs(ctx)

		require.Nil(t, adminUUIDs)
		require.ErrorContains(t, err, "failed to get admin user uuids")
	})
}

func TestUserComponent_GetAdminEmails(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		expectedEmails := []string{"admin@example.com", "super-user@example.com"}
		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().GetAdminEmails(ctx).Return(expectedEmails, nil)
		uc := &userComponentImpl{userStore: mockUserStore}

		adminEmails, err := uc.GetAdminEmails(ctx)

		require.NoError(t, err)
		require.Equal(t, expectedEmails, adminEmails)
	})

	t.Run("store error", func(t *testing.T) {
		ctx := context.Background()
		mockUserStore := mockdb.NewMockUserStore(t)
		mockUserStore.EXPECT().GetAdminEmails(ctx).Return(nil, errors.New("database error"))
		uc := &userComponentImpl{userStore: mockUserStore}

		adminEmails, err := uc.GetAdminEmails(ctx)

		require.Nil(t, adminEmails)
		require.ErrorContains(t, err, "failed to get admin emails")
	})
}

func TestUserComponent_SoftDelete(t *testing.T) {
	ctx := context.TODO()
	mockUserStore := mockdb.NewMockUserStore(t)
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockAuditStore := mockdb.NewMockAuditLogStore(t)
	mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
	mockAuthorizer := mockrebac.NewMockAuthorizer(t)
	user := database.User{
		ID: 1, Username: "user1", UUID: "user1-uuid",
	}
	namespace := database.Namespace{
		Path: "user1", UUID: "user1-namespace-uuid", NamespaceType: database.UserNamespace,
	}
	relationship := rebac.Relationship{
		Subject: rebac.UserSubject(user.UUID), Relation: rebac.RelationOwner, Object: rebac.NamespaceObject(namespace.UUID),
	}
	mockAuditStore.EXPECT().Create(ctx, mock.Anything).Return(nil)
	mockUserStore.EXPECT().SoftDeleteUserAndRelations(ctx, user, types.CloseAccountReq{}).Return(nil)
	mockUserStore.EXPECT().FindByUsername(ctx, user.Username).Return(user, nil)
	mockUserStore.EXPECT().FindByUsernameWithDeleted(ctx, user.Username).Return(user, nil)
	organizations := []database.Organization{
		{ID: 11, UUID: uuid.MustParse("00000000-0000-0000-0000-000000000011")},
		{ID: 12, UUID: uuid.MustParse("00000000-0000-0000-0000-000000000012")},
	}
	mockOrgStore.EXPECT().GetUserBelongOrgs(mock.Anything, user.ID).Return(organizations, nil).Once()
	mockNamespaceStore.EXPECT().FindByPathWithDeleted(ctx, user.Username).Return(namespace, nil).Once()
	correlationID := rebac.BatchCheckCorrelationID(0)
	mockAuthorizer.EXPECT().BatchCheck(ctx, rebac.BatchCheckRequest{Checks: []rebac.BatchCheckItem{{
		CorrelationID: correlationID,
		Check: rebac.CheckRequest{
			Subject: relationship.Subject, Relation: relationship.Relation, Object: relationship.Object,
			Consistency: rebac.ConsistencyHigher,
		},
	}}}).Return(rebac.BatchCheckResult{Results: map[string]rebac.BatchCheckOutcome{
		correlationID: {Decision: rebac.Decision{Allowed: true}},
	}}, nil).Once()
	mockAuthorizer.EXPECT().Delete(ctx, []rebac.Relationship{relationship}).Return(nil).Once()
	organizationCleanups := make([]types.OrganizationReBACCleanup, 0, len(organizations))
	for _, organization := range organizations {
		organizationCleanups = append(organizationCleanups, types.OrganizationReBACCleanup{
			OrganizationUUID: organization.UUID.String(),
			UserUUIDs:        []string{user.UUID},
		})
	}
	expectOrganizationReBACCleanups(t, mockAuthorizer, organizationCleanups)
	userObjectRelationship := rebac.Relationship{
		Subject: rebac.UserSubject(user.UUID), Relation: rebac.RelationOwner, Object: rebac.UserObject(user.UUID),
	}
	mockAuthorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject: userObjectRelationship.Subject, Relation: userObjectRelationship.Relation, Object: userObjectRelationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: true}, nil).Once()
	mockAuthorizer.EXPECT().Delete(ctx, []rebac.Relationship{userObjectRelationship}).Return(nil).Once()
	mockRepositoryAuthorizations := setupSoftDeleteRepositoryAuthorizationCleanup(t, ctx, mockAuthorizer, user)
	uc := &userComponentImpl{
		userStore:                mockUserStore,
		orgStore:                 mockOrgStore,
		nsStore:                  mockNamespaceStore,
		audit:                    mockAuditStore,
		rebac:                    mockAuthorizer,
		repositoryAuthorizations: mockRepositoryAuthorizations,
	}

	err := uc.SoftDelete(ctx, "user1", "user2", types.CloseAccountReq{})
	require.NotNil(t, err)

	err = uc.SoftDelete(ctx, "user1", "user1", types.CloseAccountReq{})
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
	ctx := context.TODO()
	mockUserStore := mockdb.NewMockUserStore(t)
	mockAuditStore := mockdb.NewMockAuditLogStore(t)
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
	mockGitserver := mockgit.NewMockGitServer(t)
	mockAuthorizer := mockrebac.NewMockAuthorizer(t)
	user1 := database.User{
		Username: "user1",
	}
	user2 := database.User{
		ID: 2, Username: "user2", UUID: "user2-uuid",
	}
	mockOrgStore := mockdb.NewMockOrgStore(t)
	repository := database.Repository{
		ID: 42, Path: "foo/bar", RepositoryType: types.ModelRepo,
	}
	relationship := rebac.Relationship{
		Subject: rebac.UserSubject("foo-user-uuid"), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(repository.ID),
	}
	userNamespace := database.Namespace{
		Path: user2.Username, UUID: "user2-namespace-uuid", NamespaceType: database.UserNamespace,
	}
	userNamespaceRelationship := rebac.Relationship{
		Subject: rebac.UserSubject(user2.UUID), Relation: rebac.RelationOwner, Object: rebac.NamespaceObject(userNamespace.UUID),
	}
	mockAuditStore.EXPECT().Create(ctx, mock.Anything).Return(nil)
	mockUserStore.EXPECT().DeleteUserAndRelations(ctx, user2, types.CloseAccountReq{}).Return(nil)
	mockUserStore.EXPECT().FindByUsernameWithDeleted(ctx, user2.Username).Return(user2, nil)
	mockUserStore.EXPECT().FindByUsername(ctx, user1.Username).Return(user1, nil)
	organizations := []database.Organization{
		{ID: 11, UUID: uuid.MustParse("00000000-0000-0000-0000-000000000011")},
		{ID: 12, UUID: uuid.MustParse("00000000-0000-0000-0000-000000000012")},
	}
	mockOrgStore.EXPECT().GetUserBelongOrgs(mock.Anything, user2.ID).Return(organizations, nil).Once()
	mockRepoStore.EXPECT().ByUser(ctx, user2.ID, 1000, 0).Return([]database.Repository{repository}, nil)
	mockRepoStore.EXPECT().ByUser(ctx, user2.ID, 1000, 1).Return([]database.Repository{}, nil)
	mockNamespaceStore.EXPECT().FindByPathWithDeleted(ctx, user2.Username).Return(userNamespace, nil).Once()
	mockNamespaceStore.EXPECT().FindByPath(ctx, "foo").Return(database.Namespace{
		Path: "foo", NamespaceType: database.UserNamespace, User: database.User{UUID: "foo-user-uuid"},
	}, nil)
	mockAuthorizer.EXPECT().Delete(ctx, []rebac.Relationship{relationship}).Return(nil)
	namespaceCorrelationID := rebac.BatchCheckCorrelationID(0)
	mockAuthorizer.EXPECT().BatchCheck(ctx, rebac.BatchCheckRequest{Checks: []rebac.BatchCheckItem{{
		CorrelationID: namespaceCorrelationID,
		Check: rebac.CheckRequest{
			Subject: userNamespaceRelationship.Subject, Relation: userNamespaceRelationship.Relation, Object: userNamespaceRelationship.Object,
			Consistency: rebac.ConsistencyHigher,
		},
	}}}).Return(rebac.BatchCheckResult{Results: map[string]rebac.BatchCheckOutcome{
		namespaceCorrelationID: {Decision: rebac.Decision{Allowed: true}},
	}}, nil).Once()
	mockAuthorizer.EXPECT().Delete(ctx, []rebac.Relationship{userNamespaceRelationship}).Return(nil).Once()
	organizationCleanups := make([]types.OrganizationReBACCleanup, 0, len(organizations))
	for _, organization := range organizations {
		organizationCleanups = append(organizationCleanups, types.OrganizationReBACCleanup{
			OrganizationUUID: organization.UUID.String(),
			UserUUIDs:        []string{user2.UUID},
		})
	}
	expectOrganizationReBACCleanups(t, mockAuthorizer, organizationCleanups)
	userObjectRelationship := rebac.Relationship{
		Subject: rebac.UserSubject(user2.UUID), Relation: rebac.RelationOwner, Object: rebac.UserObject(user2.UUID),
	}
	mockAuthorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject: userObjectRelationship.Subject, Relation: userObjectRelationship.Relation, Object: userObjectRelationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: true}, nil).Once()
	mockAuthorizer.EXPECT().Delete(ctx, []rebac.Relationship{userObjectRelationship}).Return(nil).Once()
	mockRepositoryAuthorizations := setupDeleteRepositoryAuthorizationCleanup(t, ctx, mockAuthorizer, user2)
	uc := &userComponentImpl{
		userStore:                mockUserStore,
		orgStore:                 mockOrgStore,
		audit:                    mockAuditStore,
		repo:                     mockRepoStore,
		nsStore:                  mockNamespaceStore,
		gs:                       mockGitserver,
		rebac:                    mockAuthorizer,
		repositoryAuthorizations: mockRepositoryAuthorizations,
		config:                   &config.Config{},
	}

	err := uc.Delete(ctx, "user1", "user2")
	require.Nil(t, err)
}

// test update UpdateByUUID
func TestUserComponent_UpdateByUUID_UpdateUserName(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:                1,
		UUID:              "user1",
		Username:          "user1",
		Phone:             "13626487789",
		PhoneArea:         "+86",
		RegProvider:       "casdoor",
		CanChangeUserName: true,
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().FindByUUID(context.TODO(), mock.Anything).Return(user, nil)

	mockUserStore.EXPECT().FindByUsername(mock.Anything, "new_user1").Return(database.User{}, nil)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	ssomock.EXPECT().UpdateUserInfo(mock.Anything, mock.MatchedBy(func(params *rpc.SSOUpdateUserInfo) bool {
		return params.UUID == "user1" && params.Name == "new_user1"
	})).Return(nil)
	ssomock.EXPECT().IsExistByName(mock.Anything, "new_user1").Return(false, nil)

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)
	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	phonec := mockphone.NewMockUserPhoneComponent(t)
	phonec.EXPECT().NeedPhoneChange().Return(false)
	uc := &userComponentImpl{
		userStore:  mockUserStore,
		sso:        ssomock,
		config:     config,
		once:       &once,
		ts:         mockTagStore,
		uts:        mockUserTagStore,
		userPhonec: phonec,
	}
	var userUUID = "user1"
	var newUserName = "new_user1"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:        &userUUID,
		OpUser:      "user1",
		NewUserName: &newUserName,
	})
	require.Nil(t, err)
}

func TestUserComponent_UpdateByUUID_UpdateEmail(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Email:       "old@example.com",
		RegProvider: "casdoor",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().FindByEmail(mock.Anything, "new@example.com").Return(database.User{ID: 0}, sql.ErrNoRows)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	ssomock.EXPECT().IsExistByEmail(mock.Anything, "new@example.com").Return(false, nil)
	ssomock.EXPECT().UpdateUserInfo(mock.Anything, mock.MatchedBy(func(params *rpc.SSOUpdateUserInfo) bool {
		return params.UUID == "user1" && params.Email == "new@example.com"
	})).Return(nil)

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	mockCache := mockcache.NewMockRedisClient(t)
	mockCache.EXPECT().Exists(mock.Anything, mock.Anything).Return(int64(1), nil)
	mockCache.EXPECT().Get(mock.Anything, mock.Anything).Return("123456", nil)
	mockCache.EXPECT().Del(mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	phonec := mockphone.NewMockUserPhoneComponent(t)
	phonec.EXPECT().NeedPhoneChange().Return(false)
	uc := &userComponentImpl{
		userStore:  mockUserStore,
		sso:        ssomock,
		config:     config,
		once:       &once,
		ts:         mockTagStore,
		uts:        mockUserTagStore,
		cache:      mockCache,
		userPhonec: phonec,
	}
	var userUUID = "user1"
	var newEmail = "new@example.com"
	var code = "123456"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:                  &userUUID,
		OpUser:                "user1",
		Email:                 &newEmail,
		EmailVerificationCode: &code,
	})
	require.Nil(t, err)
}

func TestUserComponent_UpdateByUUID_NoSyncWhenUsernameNotChanged(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		RegProvider: "casdoor",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	// SSO sync should NOT be called when username doesn't change

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	phonec := mockphone.NewMockUserPhoneComponent(t)
	phonec.EXPECT().NeedPhoneChange().Return(false)
	uc := &userComponentImpl{
		userStore:  mockUserStore,
		sso:        ssomock,
		config:     config,
		once:       &once,
		ts:         mockTagStore,
		uts:        mockUserTagStore,
		userPhonec: phonec,
	}
	var userUUID = "user1"
	var sameUserName = "user1" // same as current username
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:        &userUUID,
		OpUser:      "user1",
		NewUserName: &sameUserName,
	})
	require.Nil(t, err)
}

func TestUserComponent_UpdateByUUID_NoSyncWhenEmailNotChanged(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Email:       "user1@example.com",
		RegProvider: "casdoor",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	// SSO sync should NOT be called when email doesn't change

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	phonec := mockphone.NewMockUserPhoneComponent(t)
	phonec.EXPECT().NeedPhoneChange().Return(false)
	uc := &userComponentImpl{
		userStore:  mockUserStore,
		sso:        ssomock,
		config:     config,
		once:       &once,
		ts:         mockTagStore,
		uts:        mockUserTagStore,
		userPhonec: phonec,
	}
	var userUUID = "user1"
	var sameEmail = "user1@example.com" // same as current email
	var code = "123456"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:                  &userUUID,
		OpUser:                "user1",
		Email:                 &sameEmail,
		EmailVerificationCode: &code,
	})
	require.Nil(t, err)
}

func TestUserComponent_UpdateByUUID_NoSyncForNonSSOUser(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Email:       "old@example.com",
		RegProvider: "default", // non-SSO user
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().FindByEmail(mock.Anything, "new@example.com").Return(database.User{ID: 0}, sql.ErrNoRows)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	// SSO sync should NOT be called for non-SSO users

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	mockCache := mockcache.NewMockRedisClient(t)
	mockCache.EXPECT().Exists(mock.Anything, mock.Anything).Return(int64(1), nil)
	mockCache.EXPECT().Get(mock.Anything, mock.Anything).Return("123456", nil)
	mockCache.EXPECT().Del(mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	phonec := mockphone.NewMockUserPhoneComponent(t)
	phonec.EXPECT().NeedPhoneChange().Return(false)
	uc := &userComponentImpl{
		userStore:  mockUserStore,
		sso:        ssomock,
		config:     config,
		once:       &once,
		ts:         mockTagStore,
		uts:        mockUserTagStore,
		cache:      mockCache,
		userPhonec: phonec,
	}
	var userUUID = "user1"
	var newEmail = "new@example.com"
	var code = "123456"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:                  &userUUID,
		OpUser:                "user1",
		Email:                 &newEmail,
		EmailVerificationCode: &code,
	})
	require.Nil(t, err)
}

func TestUserComponent_UpdateByUUID_NoSyncWhenOnlyOtherFieldsChanged(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Email:       "user1@example.com",
		RegProvider: "casdoor",
		Avatar:      "https://example.com/avatar.jpg",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	// SSO sync should NOT be called when only other fields (like avatar, bio) change

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	phonec := mockphone.NewMockUserPhoneComponent(t)
	phonec.EXPECT().NeedPhoneChange().Return(false)
	uc := &userComponentImpl{
		userStore:  mockUserStore,
		sso:        ssomock,
		config:     config,
		once:       &once,
		ts:         mockTagStore,
		uts:        mockUserTagStore,
		userPhonec: phonec,
	}
	var userUUID = "user1"
	var avatar = "https://example.com/avatar.jpg"
	var bio = "New bio"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:   &userUUID,
		OpUser: "user1",
		Avatar: &avatar,
		Bio:    &bio,
	})
	require.Nil(t, err)
}

// TestUserComponent_UpdateByUUID_IgnoresLegacyRoles verifies profile updates cannot change stored roles through JSON input.
func TestUserComponent_UpdateByUUID_IgnoresLegacyRoles(t *testing.T) {
	for _, tt := range []struct {
		name         string
		operator     string
		payload      string
		wantNickname string
	}{
		{"self adds roles", "user1", `{"name":"Updated name","roles":["admin","super_admin"]}`, "Updated name"},
		{"self clears roles", "user1", `{"name":"Updated name","roles":[]}`, "Updated name"},
		{"admin adds roles for another user", "admin", `{"name":"Updated name","roles":["admin","super_admin"]}`, "Updated name"},
		{"admin clears roles for another user", "admin", `{"name":"Updated name","roles":[]}`, "Updated name"},
		{"self sends only roles", "user1", `{"roles":["super_admin"]}`, "Original name"},
		{"admin sends only roles for another user", "admin", `{"roles":[]}`, "Original name"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			user := &database.User{
				ID:       1,
				UUID:     "user1",
				Username: "user1",
				NickName: "Original name",
				RoleMask: "personal_user,custom_role",
			}
			var req types.UpdateUserRequest
			require.NoError(t, json.Unmarshal([]byte(tt.payload), &req))
			req.UUID = &user.UUID
			req.OpUser = tt.operator

			mockUserStore := mockdb.NewMockUserStore(t)
			mockUserStore.EXPECT().FindByUUID(ctx, user.UUID).Return(user, nil)
			if tt.operator != user.Username {
				mockUserStore.EXPECT().FindByUsername(ctx, tt.operator, types.UserWithAdminPermissions).Return(database.User{
					ID: 2, Username: tt.operator, RoleAssignments: superAdminRoleAssignments(),
				}, nil).Once()
			}
			expectedUser := *user
			expectedUser.NickName = tt.wantNickname
			mockUserStore.EXPECT().Update(ctx, &expectedUser, user.Username).Return(nil).Once()

			mockTagStore := mockdb.NewMockTagStore(t)
			mockTagStore.EXPECT().CheckTagIDsExist(ctx, req.TagIDs).Return(nil)
			mockUserTagStore := mockdb.NewMockUserTagStore(t)
			mockUserTagStore.EXPECT().ResetUserTags(ctx, user.ID, req.TagIDs).Return(nil)
			phonec := mockphone.NewMockUserPhoneComponent(t)
			phonec.EXPECT().NeedPhoneChange().Return(false)
			uc := &userComponentImpl{
				userStore:  mockUserStore,
				sso:        mockrpc.NewMockSSOInterface(t),
				config:     &config.Config{},
				once:       &sync.Once{},
				ts:         mockTagStore,
				uts:        mockUserTagStore,
				userPhonec: phonec,
			}

			require.NoError(t, uc.UpdateByUUID(ctx, &req))
			require.Equal(t, "personal_user,custom_role", user.RoleMask)
		})
	}
}

func TestUserComponent_UpdateByUUID_RollbackWhenDBUpdateFails(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Email:       "old@example.com",
		RegProvider: "casdoor",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().FindByEmail(mock.Anything, "new@example.com").Return(database.User{ID: 0}, sql.ErrNoRows)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("database error"))

	ssomock := mockrpc.NewMockSSOInterface(t)
	ssomock.EXPECT().IsExistByEmail(mock.Anything, "new@example.com").Return(false, nil)
	// First SSO update should succeed
	ssomock.EXPECT().UpdateUserInfo(mock.Anything, mock.MatchedBy(func(params *rpc.SSOUpdateUserInfo) bool {
		return params.UUID == "user1" && params.Email == "new@example.com"
	})).Return(nil)
	// Rollback should be called when DB update fails
	ssomock.EXPECT().UpdateUserInfo(mock.Anything, mock.MatchedBy(func(params *rpc.SSOUpdateUserInfo) bool {
		return params.UUID == "user1" && params.Email == "old@example.com" && params.Name == "user1"
	})).Return(nil)

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	mockCache := mockcache.NewMockRedisClient(t)
	mockCache.EXPECT().Exists(mock.Anything, mock.Anything).Return(int64(1), nil)
	mockCache.EXPECT().Get(mock.Anything, mock.Anything).Return("123456", nil)
	mockCache.EXPECT().Del(mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	phonec := mockphone.NewMockUserPhoneComponent(t)
	phonec.EXPECT().NeedPhoneChange().Return(false)
	uc := &userComponentImpl{
		userStore:  mockUserStore,
		sso:        ssomock,
		config:     config,
		once:       &once,
		ts:         mockTagStore,
		uts:        mockUserTagStore,
		cache:      mockCache,
		userPhonec: phonec,
	}
	var userUUID = "user1"
	var newEmail = "new@example.com"
	var code = "123456"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:                  &userUUID,
		OpUser:                "user1",
		Email:                 &newEmail,
		EmailVerificationCode: &code,
	})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "failed to update user in db")
}

func TestUserComponent_UpdateByUUID_NoRollbackWhenNoSync(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Email:       "user1@example.com",
		RegProvider: "casdoor",
		Avatar:      "https://example.com/avatar.jpg",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("database error"))

	ssomock := mockrpc.NewMockSSOInterface(t)
	// No SSO sync should be called since no username/email change

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	phonec := mockphone.NewMockUserPhoneComponent(t)
	phonec.EXPECT().NeedPhoneChange().Return(false)
	uc := &userComponentImpl{
		userStore:  mockUserStore,
		sso:        ssomock,
		config:     config,
		once:       &once,
		ts:         mockTagStore,
		uts:        mockUserTagStore,
		userPhonec: phonec,
	}
	var userUUID = "user1"
	var avatar = "https://example.com/avatar.jpg"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:   &userUUID,
		OpUser: "user1",
		Avatar: &avatar,
	})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "failed to update user in db")
}

func TestUserComponent_UpdateByUUID_UpdatePhoneWithPhoneArea(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Phone:       "13626487789",
		PhoneArea:   "+86",
		RegProvider: "casdoor",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	ssomock.EXPECT().UpdateUserInfo(mock.Anything, mock.MatchedBy(func(params *rpc.SSOUpdateUserInfo) bool {
		return params.UUID == "user1" && params.Phone == "13800138000" && params.PhoneArea == "+86"
	})).Return(nil)

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	phonec := mockphone.NewMockUserPhoneComponent(t)
	phonec.EXPECT().NeedPhoneChange().Return(true)
	phonec.EXPECT().CanChangePhone(mock.Anything, mock.Anything, "13800138000").Return(true, nil)
	uc := &userComponentImpl{
		userStore:  mockUserStore,
		sso:        ssomock,
		config:     config,
		once:       &once,
		ts:         mockTagStore,
		uts:        mockUserTagStore,
		userPhonec: phonec,
	}
	var userUUID = "user1"
	var newPhone = "13800138000"
	var phoneArea = "+86"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:      &userUUID,
		OpUser:    "user1",
		Phone:     &newPhone,
		PhoneArea: &phoneArea,
	})
	require.Nil(t, err)
}

func TestUserComponent_UpdateByUUID_UpdatePhoneWithPhoneAreaNormalization(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Phone:       "13626487789",
		PhoneArea:   "+86",
		RegProvider: "casdoor",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	// phoneArea "86" should be normalized to "+86"
	ssomock.EXPECT().UpdateUserInfo(mock.Anything, mock.MatchedBy(func(params *rpc.SSOUpdateUserInfo) bool {
		return params.UUID == "user1" && params.Phone == "13800138000" && params.PhoneArea == "+86"
	})).Return(nil)

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	phonec := mockphone.NewMockUserPhoneComponent(t)
	phonec.EXPECT().NeedPhoneChange().Return(true)
	phonec.EXPECT().CanChangePhone(mock.Anything, mock.Anything, "13800138000").Return(true, nil)
	uc := &userComponentImpl{
		userStore:  mockUserStore,
		sso:        ssomock,
		config:     config,
		once:       &once,
		ts:         mockTagStore,
		uts:        mockUserTagStore,
		userPhonec: phonec,
	}
	var userUUID = "user1"
	var newPhone = "13800138000"
	var phoneArea = "86" // without + prefix, should be normalized
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:      &userUUID,
		OpUser:    "user1",
		Phone:     &newPhone,
		PhoneArea: &phoneArea,
	})
	require.Nil(t, err)
}

func TestUserComponent_UpdateByUUID_UpdatePhoneWithoutPhoneArea(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Phone:       "13626487789",
		PhoneArea:   "+86",
		RegProvider: "casdoor",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	// When phoneArea is not provided, should use existing phoneArea from user
	ssomock.EXPECT().UpdateUserInfo(mock.Anything, mock.MatchedBy(func(params *rpc.SSOUpdateUserInfo) bool {
		return params.UUID == "user1" && params.Phone == "13800138000" && params.PhoneArea == "+86"
	})).Return(nil)

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	phonec := mockphone.NewMockUserPhoneComponent(t)
	phonec.EXPECT().NeedPhoneChange().Return(true)
	phonec.EXPECT().CanChangePhone(mock.Anything, mock.Anything, "13800138000").Return(true, nil)
	uc := &userComponentImpl{
		userStore:  mockUserStore,
		sso:        ssomock,
		config:     config,
		once:       &once,
		ts:         mockTagStore,
		uts:        mockUserTagStore,
		userPhonec: phonec,
	}
	var userUUID = "user1"
	var newPhone = "13800138000"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:   &userUUID,
		OpUser: "user1",
		Phone:  &newPhone,
		// PhoneArea is nil, should use existing user.PhoneArea
	})
	require.Nil(t, err)
}

func TestUserComponent_UpdateByUUID_UpdatePhoneWithEmptyPhoneArea(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Phone:       "13626487789",
		PhoneArea:   "", // empty phoneArea in DB
		RegProvider: "casdoor",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	// When user.PhoneArea is empty and req.PhoneArea is not provided, should send empty phoneArea
	ssomock.EXPECT().UpdateUserInfo(mock.Anything, mock.MatchedBy(func(params *rpc.SSOUpdateUserInfo) bool {
		return params.UUID == "user1" && params.Phone == "13800138000" && params.PhoneArea == ""
	})).Return(nil)

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	phonec := mockphone.NewMockUserPhoneComponent(t)
	phonec.EXPECT().NeedPhoneChange().Return(true)
	phonec.EXPECT().CanChangePhone(mock.Anything, mock.Anything, "13800138000").Return(true, nil)
	uc := &userComponentImpl{
		userStore:  mockUserStore,
		sso:        ssomock,
		config:     config,
		once:       &once,
		ts:         mockTagStore,
		uts:        mockUserTagStore,
		userPhonec: phonec,
	}
	var userUUID = "user1"
	var newPhone = "13800138000"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:   &userUUID,
		OpUser: "user1",
		Phone:  &newPhone,
		// PhoneArea is nil, user.PhoneArea is empty, so phoneArea will be empty
	})
	require.Nil(t, err)
}

func TestUserComponent_UpdateByUUID_UpdatePhoneWithNewPhoneArea(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Phone:       "13626487789",
		PhoneArea:   "+86",
		RegProvider: "casdoor",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ssomock := mockrpc.NewMockSSOInterface(t)
	// When new phoneArea is provided and different from existing, should use new normalized phoneArea
	ssomock.EXPECT().UpdateUserInfo(mock.Anything, mock.MatchedBy(func(params *rpc.SSOUpdateUserInfo) bool {
		return params.UUID == "user1" && params.Phone == "13800138000" && params.PhoneArea == "+1"
	})).Return(nil)

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	phonec := mockphone.NewMockUserPhoneComponent(t)
	phonec.EXPECT().NeedPhoneChange().Return(true)
	phonec.EXPECT().CanChangePhone(mock.Anything, mock.Anything, "13800138000").Return(true, nil)
	uc := &userComponentImpl{
		userStore:  mockUserStore,
		sso:        ssomock,
		config:     config,
		once:       &once,
		ts:         mockTagStore,
		uts:        mockUserTagStore,
		userPhonec: phonec,
	}
	var userUUID = "user1"
	var newPhone = "13800138000"
	var newPhoneArea = "1" // will be normalized to "+1"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:      &userUUID,
		OpUser:    "user1",
		Phone:     &newPhone,
		PhoneArea: &newPhoneArea,
	})
	require.Nil(t, err)
}

func TestUserComponent_UpdateByUUID_RollbackPhoneUpdateWhenDBUpdateFails(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	user := &database.User{
		ID:          1,
		UUID:        "user1",
		Username:    "user1",
		Phone:       "13626487789",
		PhoneArea:   "+86",
		RegProvider: "casdoor",
	}
	mockUserStore.EXPECT().FindByUUID(mock.Anything, "user1").Return(user, nil)
	mockUserStore.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("database error"))

	ssomock := mockrpc.NewMockSSOInterface(t)
	// First SSO update should succeed with new phone and phoneArea
	ssomock.EXPECT().UpdateUserInfo(mock.Anything, mock.MatchedBy(func(params *rpc.SSOUpdateUserInfo) bool {
		return params.UUID == "user1" && params.Phone == "13800138000" && params.PhoneArea == "+86"
	})).Return(nil)
	// Rollback should restore old phone and phoneArea
	ssomock.EXPECT().UpdateUserInfo(mock.Anything, mock.MatchedBy(func(params *rpc.SSOUpdateUserInfo) bool {
		return params.UUID == "user1" && params.Phone == "13626487789" && params.PhoneArea == "+86"
	})).Return(nil)

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExist(context.TODO(), mock.Anything).Return(nil)

	mockUserTagStore := mockdb.NewMockUserTagStore(t)
	mockUserTagStore.EXPECT().ResetUserTags(context.TODO(), mock.Anything, mock.Anything).Return(nil)

	config := &config.Config{}
	config.SSOType = "casdoor"

	once := sync.Once{}
	phonec := mockphone.NewMockUserPhoneComponent(t)
	phonec.EXPECT().NeedPhoneChange().Return(true)
	phonec.EXPECT().CanChangePhone(mock.Anything, mock.Anything, "13800138000").Return(true, nil)
	uc := &userComponentImpl{
		userStore:  mockUserStore,
		sso:        ssomock,
		config:     config,
		once:       &once,
		ts:         mockTagStore,
		uts:        mockUserTagStore,
		userPhonec: phonec,
	}
	var userUUID = "user1"
	var newPhone = "13800138000"
	var phoneArea = "+86"
	err := uc.UpdateByUUID(context.TODO(), &types.UpdateUserRequest{
		UUID:      &userUUID,
		OpUser:    "user1",
		Phone:     &newPhone,
		PhoneArea: &phoneArea,
	})
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "failed to update user in db")
}

func TestUserComponent_checkUserConflictsInDB(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		email         string
		mockSetup     func(*mockdb.MockUserStore)
		expectedError error
		expectError   bool
	}{
		{
			name:     "no conflicts - username and email available",
			username: "newuser",
			email:    "newuser@example.com",
			mockSetup: func(mockUserStore *mockdb.MockUserStore) {
				mockUserStore.EXPECT().IsExist(mock.Anything, "newuser").Return(false, nil)
				mockUserStore.EXPECT().FindByEmail(mock.Anything, "newuser@example.com").Return(database.User{ID: 0}, sql.ErrNoRows)
			},
			expectedError: nil,
			expectError:   false,
		},
		{
			name:     "no conflicts - username available, no email provided",
			username: "newuser",
			email:    "",
			mockSetup: func(mockUserStore *mockdb.MockUserStore) {
				mockUserStore.EXPECT().IsExist(mock.Anything, "newuser").Return(false, nil)
			},
			expectedError: nil,
			expectError:   false,
		},
		{
			name:     "username conflict",
			username: "existinguser",
			email:    "newuser@example.com",
			mockSetup: func(mockUserStore *mockdb.MockUserStore) {
				mockUserStore.EXPECT().IsExist(mock.Anything, "existinguser").Return(true, nil)
			},
			expectedError: errorx.UsernameExists("existinguser"),
			expectError:   true,
		},
		{
			name:     "email conflict",
			username: "newuser",
			email:    "existing@example.com",
			mockSetup: func(mockUserStore *mockdb.MockUserStore) {
				mockUserStore.EXPECT().IsExist(mock.Anything, "newuser").Return(false, nil)
				mockUserStore.EXPECT().FindByEmail(mock.Anything, "existing@example.com").Return(database.User{ID: 123}, nil)
			},
			expectedError: errorx.EmailExists("existing@example.com"),
			expectError:   true,
		},
		{
			name:     "username check database error",
			username: "newuser",
			email:    "newuser@example.com",
			mockSetup: func(mockUserStore *mockdb.MockUserStore) {
				mockUserStore.EXPECT().IsExist(mock.Anything, "newuser").Return(false, errors.New("database connection error"))
			},
			expectedError: nil,
			expectError:   true,
		},
		{
			name:     "email check database error",
			username: "newuser",
			email:    "newuser@example.com",
			mockSetup: func(mockUserStore *mockdb.MockUserStore) {
				mockUserStore.EXPECT().IsExist(mock.Anything, "newuser").Return(false, nil)
				mockUserStore.EXPECT().FindByEmail(mock.Anything, "newuser@example.com").Return(database.User{}, errors.New("database connection error"))
			},
			expectedError: nil,
			expectError:   true,
		},
		{
			name:     "email check returns ErrNoRows - no conflict",
			username: "newuser",
			email:    "newuser@example.com",
			mockSetup: func(mockUserStore *mockdb.MockUserStore) {
				mockUserStore.EXPECT().IsExist(mock.Anything, "newuser").Return(false, nil)
				mockUserStore.EXPECT().FindByEmail(mock.Anything, "newuser@example.com").Return(database.User{ID: 0}, sql.ErrNoRows)
			},
			expectedError: nil,
			expectError:   false,
		},
		{
			name:     "email check returns user with ID 0 - no conflict",
			username: "newuser",
			email:    "newuser@example.com",
			mockSetup: func(mockUserStore *mockdb.MockUserStore) {
				mockUserStore.EXPECT().IsExist(mock.Anything, "newuser").Return(false, nil)
				mockUserStore.EXPECT().FindByEmail(mock.Anything, "newuser@example.com").Return(database.User{ID: 0}, nil)
			},
			expectedError: nil,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserStore := mockdb.NewMockUserStore(t)
			tt.mockSetup(mockUserStore)

			uc := &userComponentImpl{
				userStore: mockUserStore,
			}

			err := uc.checkUserConflictsInDB(context.Background(), tt.username, tt.email)

			if tt.expectError {
				require.Error(t, err)
				if tt.expectedError != nil {
					// Check if the error is the expected custom error type
					var customErr errorx.CustomError
					if errors.As(err, &customErr) {
						require.True(t, customErr.Is(tt.expectedError), "Expected error type %v, got %v", tt.expectedError, err)
					} else {
						require.Contains(t, err.Error(), "failed to check", "Expected database error message")
					}
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestUserComponent_StreamExportUsers checks the expected user query options.
func TestUserComponent_StreamExportUsers(t *testing.T) {
	ctx := context.Background()
	req := types.UserIndexReq{
		Search: "u-foo",
		Per:    10,
	}

	data := make(chan database.Wrapper)
	go func() {
		defer close(data)
		data <- database.Wrapper{
			Users: []database.User{
				{Username: "u-foo"},
			},
		}
	}()
	us := mockdb.NewMockUserStore(t)
	us.EXPECT().IndexWithCursor(mock.Anything, req, types.UserWithAdminPermissions).Return(data, nil)

	uc := &userComponentImpl{
		userStore: us,
	}

	ch, err := uc.StreamExportUsers(ctx, req)
	require.NoError(t, err)

	total := 0
	for resp := range ch {
		require.NoError(t, resp.Error)
		for _, u := range resp.Users {
			require.Equal(t, "u-foo", u.Username)
		}
		total += len(resp.Users)
	}

	require.Equal(t, 1, total)
}

// TestUserComponent_StreamExportUsers_RoleAssignments verifies exports use active assignments and derived administrator flags.
func TestUserComponent_StreamExportUsers_RoleAssignments(t *testing.T) {
	for _, tt := range userRoleOutputCases() {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req := types.UserIndexReq{Per: 10}
			data := make(chan database.Wrapper, 1)
			data <- database.Wrapper{Users: []database.User{tt.user}}
			close(data)
			us := mockdb.NewMockUserStore(t)
			us.EXPECT().IndexWithCursor(ctx, req, types.UserWithAdminPermissions).Return(data, nil).Once()
			uc := &userComponentImpl{userStore: us}

			ch, err := uc.StreamExportUsers(ctx, req)
			require.NoError(t, err)
			var users []*types.User
			for response := range ch {
				require.NoError(t, response.Error)
				users = append(users, response.Users...)
			}
			require.Len(t, users, 1)
			require.ElementsMatch(t, tt.wantRoles, users[0].Roles)
			require.Equal(t, tt.wantAdmin, users[0].Admin)
			require.Equal(t, tt.wantSuperAdmin, users[0].SuperAdmin)
		})
	}
}

// TestUserComponent_Get_RoleAssignments verifies full profiles expose current role facts while basic profiles omit them.
func TestUserComponent_Get_RoleAssignments(t *testing.T) {
	for _, tt := range userRoleOutputCases() {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			us := mockdb.NewMockUserStore(t)
			us.EXPECT().FindByUsername(ctx, tt.user.Username, types.UserWithAdminPermissions).Return(tt.user, nil).Twice()
			us.EXPECT().FindByUsername(ctx, tt.user.Username).Return(tt.user, nil).Once()
			uts := mockdb.NewMockUserTagStore(t)
			uts.EXPECT().GetUserTags(ctx, tt.user.ID).Return(nil, nil)
			orgs := mockdb.NewMockOrgStore(t)
			orgs.EXPECT().GetUserRootOrganizations(ctx, tt.user.ID).Return(nil, nil)
			uc := &userComponentImpl{userStore: us, uts: uts, orgStore: orgs}

			full, err := uc.Get(ctx, tt.user.Username, tt.user.Username, false)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.wantRoles, full.Roles)
			require.Equal(t, tt.wantAdmin, full.Admin)
			require.Equal(t, tt.wantSuperAdmin, full.SuperAdmin)

			internal, err := uc.GetInternal(ctx, tt.user.Username, false)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.wantRoles, internal.Roles)
			require.Equal(t, tt.wantAdmin, internal.Admin)
			require.Equal(t, tt.wantSuperAdmin, internal.SuperAdmin)

			basic, err := uc.Get(ctx, tt.user.Username, "", false)
			require.NoError(t, err)
			require.Empty(t, basic.Roles)
			require.False(t, basic.Admin)
			require.False(t, basic.SuperAdmin)
			encoded, err := json.Marshal(basic)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), `"is_admin"`)
			require.NotContains(t, string(encoded), `"is_super_admin"`)
		})
	}
}

// TestUserComponent_Get_AdminPermissionLoading verifies target grants are loaded only for full profiles.
func TestUserComponent_Get_AdminPermissionLoading(t *testing.T) {
	for _, tt := range []struct {
		name       string
		visitor    string
		useUUID    bool
		superAdmin bool
		sharedUUID bool
		wantFull   bool
	}{
		{name: "anonymous username"},
		{name: "anonymous UUID", useUUID: true},
		{name: "own username", visitor: "alice", wantFull: true},
		{name: "own UUID", visitor: "alice", useUUID: true, wantFull: true},
		{name: "other username", visitor: "bob"},
		{name: "other UUID", visitor: "bob", useUUID: true},
		{name: "scoped administrator", visitor: "operator"},
		{name: "super administrator username", visitor: "bob", superAdmin: true, wantFull: true},
		{name: "super administrator UUID", visitor: "bob", useUUID: true, superAdmin: true, wantFull: true},
		{name: "matching UUID does not imply self", visitor: "bob", useUUID: true, sharedUUID: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			target := database.User{
				ID: 42, Username: "alice", UUID: "alice-uuid", Email: "alice@example.com",
				RoleAssignments: []database.UserRole{{RoleCode: "super_admin"}},
				Namespaces:      []database.Namespace{{Path: "alice", NamespaceType: database.UserNamespace}},
			}
			permissions := database.User{RoleAssignments: superAdminRoleAssignments()}
			us := mockdb.NewMockUserStore(t)
			namedSelf := !tt.useUUID && tt.visitor == target.Username
			identifier := target.Username
			switch {
			case tt.useUUID:
				identifier = target.UUID
				us.EXPECT().FindByUUID(ctx, identifier).Return(&target, nil).Once()
			case namedSelf:
				target.RoleAssignments = permissions.RoleAssignments
				us.EXPECT().FindByUsername(ctx, identifier, types.UserWithAdminPermissions).Return(target, nil).Once()
			default:
				us.EXPECT().FindByUsername(ctx, identifier).Return(target, nil).Once()
			}
			if tt.visitor != "" && tt.visitor != target.Username {
				visitor := database.User{Username: tt.visitor, UUID: "visitor-uuid"}
				if tt.superAdmin {
					visitor.RoleAssignments = superAdminRoleAssignments()
				} else if tt.visitor == "operator" {
					visitor.RoleAssignments = []database.UserRole{{RoleCode: "operator", Role: &database.Role{IsAdmin: true,
						RoleCode: "operator", Permissions: []database.AdminRolePermission{{PermissionCode: types.AdminAccountUser}},
					}}}
				}
				if tt.sharedUUID {
					visitor.UUID = target.UUID
				}
				us.EXPECT().FindByUsername(ctx, tt.visitor, types.UserWithAdminPermissions).Return(visitor, nil).Once()
			}
			if tt.wantFull && !namedSelf {
				us.EXPECT().FindByID(ctx, target.ID, types.UserWithAdminPermissions).Return(permissions, nil).Once()
			}
			uts := mockdb.NewMockUserTagStore(t)
			uts.EXPECT().GetUserTags(ctx, target.ID).Return(nil, nil).Once()
			orgs := mockdb.NewMockOrgStore(t)
			orgs.EXPECT().GetUserRootOrganizations(ctx, target.ID).Return(nil, nil).Once()
			uc := &userComponentImpl{userStore: us, uts: uts, orgStore: orgs}

			user, err := uc.Get(ctx, identifier, tt.visitor, tt.useUUID)
			require.NoError(t, err)
			require.Equal(t, target.Username, user.Username)
			require.Len(t, user.Namespaces, 1)
			require.Equal(t, "alice", user.Namespaces[0].Path)
			if tt.wantFull {
				require.Equal(t, target.ID, user.ID)
				require.Equal(t, target.Email, user.Email)
				require.Equal(t, []string{"super_admin"}, user.Roles)
				require.True(t, user.Admin)
				require.True(t, user.SuperAdmin)
			} else {
				require.Zero(t, user.ID)
				require.Empty(t, user.Email)
				require.Empty(t, user.Roles)
				require.False(t, user.Admin)
				require.False(t, user.SuperAdmin)
			}
		})
	}
}

// TestUserComponent_Get_AdminPermissionErrors verifies failed lookups never produce a partial full profile.
func TestUserComponent_Get_AdminPermissionErrors(t *testing.T) {
	for _, tt := range []struct {
		name    string
		visitor string
		useUUID bool
		stage   string
	}{
		{name: "target username lookup", stage: "target"},
		{name: "target UUID lookup", useUUID: true, stage: "target"},
		{name: "visitor permission lookup", visitor: "bob", stage: "visitor"},
		{name: "target permission lookup", visitor: "alice", useUUID: true, stage: "permissions"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			lookupErr := errors.New("lookup failed")
			target := database.User{ID: 42, Username: "alice", UUID: "alice-uuid"}
			us := mockdb.NewMockUserStore(t)
			var targetErr error
			if tt.stage == "target" {
				targetErr = lookupErr
			}
			identifier := target.Username
			if tt.useUUID {
				identifier = target.UUID
				us.EXPECT().FindByUUID(ctx, identifier).Return(&target, targetErr).Once()
			} else {
				us.EXPECT().FindByUsername(ctx, identifier).Return(target, targetErr).Once()
			}
			switch tt.stage {
			case "visitor":
				us.EXPECT().FindByUsername(ctx, tt.visitor, types.UserWithAdminPermissions).Return(database.User{}, lookupErr).Once()
			case "permissions":
				us.EXPECT().FindByID(ctx, target.ID, types.UserWithAdminPermissions).Return(database.User{}, lookupErr).Once()
			}
			uc := &userComponentImpl{userStore: us}

			user, err := uc.Get(ctx, identifier, tt.visitor, tt.useUUID)
			require.ErrorIs(t, err, lookupErr)
			require.Nil(t, user)
		})
	}
}

// TestUserComponent_Index_RoleAssignments verifies list projections retain the existing full/basic visibility rules.
func TestUserComponent_Index_RoleAssignments(t *testing.T) {
	for _, tt := range userRoleOutputCases() {
		t.Run(tt.name, func(t *testing.T) {
			for _, fullInfo := range []bool{true, false} {
				ctx := context.Background()
				req := types.UserListReq{VisitorName: "visitor", Per: 10, Page: 1}
				visitor := database.User{Username: req.VisitorName}
				if fullInfo {
					visitor.RoleAssignments = superAdminRoleAssignments()
				}
				us := mockdb.NewMockUserStore(t)
				us.EXPECT().FindByUsername(ctx, req.VisitorName, types.UserWithAdminPermissions).Return(visitor, nil).Once()
				if fullInfo {
					us.EXPECT().IndexWithSearch(ctx, req, types.UserWithAdminPermissions).Return([]database.User{tt.user}, 1, nil).Once()
				} else {
					us.EXPECT().IndexWithSearch(ctx, req).Return([]database.User{tt.user}, 1, nil).Once()
				}
				uc := &userComponentImpl{userStore: us}

				users, total, err := uc.Index(ctx, req)
				require.NoError(t, err)
				require.Equal(t, 1, total)
				require.Len(t, users, 1)
				if fullInfo {
					require.Equal(t, tt.user.ID, users[0].ID)
					require.ElementsMatch(t, tt.wantRoles, users[0].Roles)
					require.Equal(t, tt.wantAdmin, users[0].Admin)
					require.Equal(t, tt.wantSuperAdmin, users[0].SuperAdmin)
				} else {
					require.Zero(t, users[0].ID)
					require.Empty(t, users[0].Roles)
					require.False(t, users[0].Admin)
					require.False(t, users[0].SuperAdmin)
				}
			}
		})
	}
}

// TestUserComponent_Get_WithFullInfo checks the expected user query options.
func TestUserComponent_Get_WithFullInfo(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockUserTagStore := mockdb.NewMockUserTagStore(t)

	dbUser := database.User{
		ID:        1,
		UUID:      "user-uuid-123",
		Username:  "testuser",
		NickName:  "Test User",
		Avatar:    "https://example.com/avatar.png",
		Email:     "test@example.com",
		Bio:       "Test bio",
		Homepage:  "https://test.com",
		PhoneArea: "+86",
		Phone:     "13612345678",
		Namespaces: []database.Namespace{
			{
				ID:            1,
				Path:          "testuser",
				NamespaceType: database.UserNamespace,
				UUID:          "ns-uuid-123",
			},
		},
	}

	mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser", types.UserWithAdminPermissions).Return(dbUser, nil)
	mockUserTagStore.EXPECT().GetUserTags(mock.Anything, dbUser.ID).Return([]*database.Tag{}, nil)
	mockOrgStore.EXPECT().GetUserRootOrganizations(mock.Anything, dbUser.ID).Return([]database.Organization{}, nil)

	uc := &userComponentImpl{
		userStore: mockUserStore,
		orgStore:  mockOrgStore,
		uts:       mockUserTagStore,
	}

	user, err := uc.Get(context.Background(), "testuser", "testuser", false)
	require.NoError(t, err)
	require.Equal(t, "testuser", user.Username)
	require.Equal(t, "Test User", user.Nickname)
	require.Equal(t, "https://example.com/avatar.png", user.Avatar)
	require.Equal(t, int64(1), user.ID)
	require.Equal(t, "test@example.com", user.Email)
	require.Equal(t, "user-uuid-123", user.UUID)
	require.Equal(t, "Test bio", user.Bio)
	require.Equal(t, "https://test.com", user.Homepage)
	require.Equal(t, "+86", user.PhoneArea)
	require.Equal(t, "13612345678", user.Phone)
	require.Len(t, user.Namespaces, 1)
	require.Equal(t, "testuser", user.Namespaces[0].Path)
}

// TestUserComponent_Get_WithAnonymousVisitor checks the expected user query options.
func TestUserComponent_Get_WithAnonymousVisitor(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockUserTagStore := mockdb.NewMockUserTagStore(t)

	dbUser := database.User{
		ID:        1,
		UUID:      "user-uuid-123",
		Username:  "testuser",
		NickName:  "Test User",
		Avatar:    "https://example.com/avatar.png",
		Email:     "test@example.com",
		Bio:       "Test bio",
		Homepage:  "https://test.com",
		PhoneArea: "+86",
		Phone:     "13612345678",
		Namespaces: []database.Namespace{
			{
				ID:            1,
				Path:          "testuser",
				NamespaceType: database.UserNamespace,
				UUID:          "ns-uuid-123",
			},
		},
	}

	mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser").Return(dbUser, nil).Once()
	mockUserTagStore.EXPECT().GetUserTags(mock.Anything, dbUser.ID).Return([]*database.Tag{}, nil)
	mockOrgStore.EXPECT().GetUserRootOrganizations(mock.Anything, dbUser.ID).Return([]database.Organization{}, nil)

	uc := &userComponentImpl{
		userStore: mockUserStore,
		orgStore:  mockOrgStore,
		uts:       mockUserTagStore,
	}

	user, err := uc.Get(context.Background(), "testuser", "", false)
	require.NoError(t, err)
	require.Equal(t, "testuser", user.Username)
	require.Equal(t, "Test User", user.Nickname)
	require.Equal(t, "https://example.com/avatar.png", user.Avatar)
	require.Equal(t, int64(0), user.ID)
	require.Empty(t, user.Email)
	require.Empty(t, user.UUID)
	require.Empty(t, user.Bio)
	require.Empty(t, user.Homepage)
	require.Empty(t, user.PhoneArea)
	require.Empty(t, user.Phone)
}

// TestUserComponent_Get_WithOnlyBasicInfo checks the expected user query options.
func TestUserComponent_Get_WithOnlyBasicInfo(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockUserTagStore := mockdb.NewMockUserTagStore(t)

	dbUser := database.User{
		ID:        1,
		UUID:      "user-uuid-123",
		Username:  "testuser",
		NickName:  "Test User",
		Avatar:    "https://example.com/avatar.png",
		Email:     "test@example.com",
		Bio:       "Test bio",
		Homepage:  "https://test.com",
		PhoneArea: "+86",
		Phone:     "13612345678",
		Namespaces: []database.Namespace{
			{
				ID:            1,
				Path:          "testuser",
				NamespaceType: database.UserNamespace,
				UUID:          "ns-uuid-123",
			},
		},
	}

	mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser").Return(dbUser, nil).Once()
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "otheruser", types.UserWithAdminPermissions).Return(database.User{Username: "otheruser"}, nil)
	mockUserTagStore.EXPECT().GetUserTags(mock.Anything, dbUser.ID).Return([]*database.Tag{}, nil)
	mockOrgStore.EXPECT().GetUserRootOrganizations(mock.Anything, dbUser.ID).Return([]database.Organization{}, nil)

	uc := &userComponentImpl{
		userStore: mockUserStore,
		orgStore:  mockOrgStore,
		uts:       mockUserTagStore,
	}

	user, err := uc.Get(context.Background(), "testuser", "otheruser", false)
	require.NoError(t, err)
	require.Equal(t, "testuser", user.Username)
	require.Equal(t, "Test User", user.Nickname)
	require.Equal(t, "https://example.com/avatar.png", user.Avatar)
	require.Equal(t, int64(0), user.ID)
	require.Empty(t, user.Email)
	require.Empty(t, user.UUID)
	require.Empty(t, user.Bio)
	require.Empty(t, user.Homepage)
	require.Empty(t, user.PhoneArea)
	require.Empty(t, user.Phone)
}

// TestUserComponent_Get_WithOrgs checks the expected user query options.
func TestUserComponent_Get_WithOrgs(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockUserTagStore := mockdb.NewMockUserTagStore(t)

	dbUser := database.User{
		ID:       1,
		UUID:     "user-uuid-123",
		Username: "testuser",
		NickName: "Test User",
		Avatar:   "https://example.com/avatar.png",
		Namespaces: []database.Namespace{
			{
				ID:            1,
				Path:          "testuser",
				NamespaceType: database.UserNamespace,
				UUID:          "ns-uuid-123",
			},
		},
	}

	dbOrgs := []database.Organization{
		{
			ID:       1,
			UUID:     uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			Homepage: "https://org1.com",
			Logo:     "https://org1.com/logo.png",
			OrgType:  "company",
			Verified: true,
			Namespace: &database.Namespace{
				ID:            2,
				Path:          "org_path_1",
				NamespaceType: database.OrgNamespace,
				UUID:          "ns-org-1",
			},
		},
		{
			ID:       2,
			UUID:     uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			Homepage: "https://org2.com",
			Logo:     "https://org2.com/logo.png",
			OrgType:  "school",
			Verified: false,
		},
	}

	mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser", types.UserWithAdminPermissions).Return(dbUser, nil)
	mockUserTagStore.EXPECT().GetUserTags(mock.Anything, dbUser.ID).Return([]*database.Tag{}, nil)
	mockOrgStore.EXPECT().GetUserRootOrganizations(mock.Anything, dbUser.ID).Return(dbOrgs, nil)

	uc := &userComponentImpl{
		userStore: mockUserStore,
		orgStore:  mockOrgStore,
		uts:       mockUserTagStore,
	}

	user, err := uc.Get(context.Background(), "testuser", "testuser", false)
	require.NoError(t, err)
	require.Len(t, user.Orgs, 2)

	require.Equal(t, "https://org1.com", user.Orgs[0].Homepage)
	require.Equal(t, "https://org1.com/logo.png", user.Orgs[0].Logo)
	require.Equal(t, "company", user.Orgs[0].OrgType)
	require.True(t, user.Orgs[0].Verified)

	require.Equal(t, "https://org2.com", user.Orgs[1].Homepage)
	require.Equal(t, "school", user.Orgs[1].OrgType)
	require.False(t, user.Orgs[1].Verified)
}

// TestUserComponent_Get_WithTags checks the expected user query options.
func TestUserComponent_Get_WithTags(t *testing.T) {
	mockUserStore := mockdb.NewMockUserStore(t)
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockUserTagStore := mockdb.NewMockUserTagStore(t)

	dbUser := database.User{
		ID:       1,
		Username: "testuser",
		NickName: "Test User",
		Avatar:   "https://example.com/avatar.png",
	}

	tags := []*database.Tag{
		{ID: 1, Name: "vip", Category: "label", Group: "user", BuiltIn: true, Scope: "user", I18nKey: "tag.vip"},
		{ID: 2, Name: "developer", Category: "role", Group: "user", BuiltIn: false, Scope: "user", I18nKey: "tag.developer"},
	}

	mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser", types.UserWithAdminPermissions).Return(dbUser, nil)
	mockUserTagStore.EXPECT().GetUserTags(mock.Anything, dbUser.ID).Return(tags, nil)
	mockOrgStore.EXPECT().GetUserRootOrganizations(mock.Anything, dbUser.ID).Return([]database.Organization{}, nil)

	uc := &userComponentImpl{
		userStore: mockUserStore,
		orgStore:  mockOrgStore,
		uts:       mockUserTagStore,
	}

	user, err := uc.Get(context.Background(), "testuser", "testuser", false)
	require.NoError(t, err)
	require.Len(t, user.Tags, 2)
	require.Equal(t, "vip", user.Tags[0].Name)
	require.Equal(t, "label", user.Tags[0].Category)
	require.Equal(t, "developer", user.Tags[1].Name)
	require.Equal(t, "role", user.Tags[1].Category)
}

// TestUserComponent_Get_RoleVisibility checks the expected user query options.
func TestUserComponent_Get_RoleVisibility(t *testing.T) {
	t.Run("self_user_can_see_role", func(t *testing.T) {
		mockUserStore := mockdb.NewMockUserStore(t)
		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockUserTagStore := mockdb.NewMockUserTagStore(t)

		dbUser := database.User{
			ID:       1,
			UUID:     "user-uuid-123",
			Username: "testuser",
			NickName: "Test User",
			Avatar:   "https://example.com/avatar.png",
			Namespaces: []database.Namespace{
				{
					ID:            1,
					Path:          "testuser",
					NamespaceType: database.UserNamespace,
					UUID:          "ns-uuid-123",
				},
			},
		}

		dbOrgs := []database.Organization{
			{
				ID:       1,
				UUID:     uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				Homepage: "https://org1.com",
				OrgType:  "company",
				Verified: true,
				Role:     string(types.UserAdmin),
				Namespace: &database.Namespace{
					ID:            2,
					Path:          "org_path_1",
					NamespaceType: database.OrgNamespace,
					UUID:          "ns-org-1",
				},
			},
			{
				ID:       2,
				UUID:     uuid.MustParse("22222222-2222-2222-2222-222222222222"),
				Homepage: "https://org2.com",
				OrgType:  "school",
				Verified: false,
				Role:     "member",
			},
		}

		mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser", types.UserWithAdminPermissions).Return(dbUser, nil)
		mockUserTagStore.EXPECT().GetUserTags(mock.Anything, dbUser.ID).Return([]*database.Tag{}, nil)
		mockOrgStore.EXPECT().GetUserRootOrganizations(mock.Anything, dbUser.ID).Return(dbOrgs, nil)

		uc := &userComponentImpl{
			userStore: mockUserStore,
			orgStore:  mockOrgStore,
			uts:       mockUserTagStore,
		}

		user, err := uc.Get(context.Background(), "testuser", "testuser", false)
		require.NoError(t, err)
		require.Len(t, user.Orgs, 2)

		require.Equal(t, "admin", user.Orgs[0].Role)
		require.Equal(t, "member", user.Orgs[1].Role)
	})

	t.Run("admin_can_see_role", func(t *testing.T) {
		mockUserStore := mockdb.NewMockUserStore(t)
		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockUserTagStore := mockdb.NewMockUserTagStore(t)

		dbUser := database.User{
			ID:       1,
			UUID:     "user-uuid-123",
			Username: "testuser",
			NickName: "Test User",
			Avatar:   "https://example.com/avatar.png",
			Namespaces: []database.Namespace{
				{
					ID:            1,
					Path:          "testuser",
					NamespaceType: database.UserNamespace,
					UUID:          "ns-uuid-123",
				},
			},
		}

		adminUser := database.User{
			ID:              2,
			Username:        "adminuser",
			RoleAssignments: superAdminRoleAssignments(),
		}

		dbOrgs := []database.Organization{
			{
				ID:       1,
				UUID:     uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				Homepage: "https://org1.com",
				OrgType:  "company",
				Verified: true,
				Role:     string(types.UserAdmin),
			},
		}

		mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser").Return(dbUser, nil).Once()
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "adminuser", types.UserWithAdminPermissions).Return(adminUser, nil).Once()
		mockUserStore.EXPECT().FindByID(mock.Anything, dbUser.ID, types.UserWithAdminPermissions).Return(dbUser, nil).Once()
		mockUserTagStore.EXPECT().GetUserTags(mock.Anything, dbUser.ID).Return([]*database.Tag{}, nil)
		mockOrgStore.EXPECT().GetUserRootOrganizations(mock.Anything, dbUser.ID).Return(dbOrgs, nil)

		uc := &userComponentImpl{
			userStore: mockUserStore,
			orgStore:  mockOrgStore,
			uts:       mockUserTagStore,
		}

		user, err := uc.Get(context.Background(), "testuser", "adminuser", false)
		require.NoError(t, err)
		require.Len(t, user.Orgs, 1)

		require.Equal(t, "admin", user.Orgs[0].Role)
	})

	t.Run("non_admin_other_user_cannot_see_role", func(t *testing.T) {
		mockUserStore := mockdb.NewMockUserStore(t)
		mockOrgStore := mockdb.NewMockOrgStore(t)
		mockUserTagStore := mockdb.NewMockUserTagStore(t)

		dbUser := database.User{
			ID:       1,
			UUID:     "user-uuid-123",
			Username: "testuser",
			NickName: "Test User",
			Avatar:   "https://example.com/avatar.png",
			Namespaces: []database.Namespace{
				{
					ID:            1,
					Path:          "testuser",
					NamespaceType: database.UserNamespace,
					UUID:          "ns-uuid-123",
				},
			},
		}

		otherUser := database.User{
			ID:       2,
			Username: "otheruser",
			RoleMask: "",
		}

		dbOrgs := []database.Organization{
			{
				ID:       1,
				UUID:     uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				Homepage: "https://org1.com",
				OrgType:  "company",
				Verified: true,
				Role:     string(types.UserAdmin),
			},
		}

		mockUserStore.EXPECT().FindByUsername(mock.Anything, "testuser").Return(dbUser, nil).Once()
		mockUserStore.EXPECT().FindByUsername(mock.Anything, "otheruser", types.UserWithAdminPermissions).Return(otherUser, nil).Once()
		mockUserTagStore.EXPECT().GetUserTags(mock.Anything, dbUser.ID).Return([]*database.Tag{}, nil)
		mockOrgStore.EXPECT().GetUserRootOrganizations(mock.Anything, dbUser.ID).Return(dbOrgs, nil)

		uc := &userComponentImpl{
			userStore: mockUserStore,
			orgStore:  mockOrgStore,
			uts:       mockUserTagStore,
		}

		user, err := uc.Get(context.Background(), "testuser", "otheruser", false)
		require.NoError(t, err)
		require.Len(t, user.Orgs, 1)

		require.Empty(t, user.Orgs[0].Role)
	})
}

// TestUserComponent_AdminAuthorizedProfiles verifies request grants expose the existing management response without changing role facts.
func TestUserComponent_AdminAuthorizedProfiles(t *testing.T) {
	for _, tt := range userRoleOutputCases() {
		t.Run(tt.name, func(t *testing.T) {
			ctx := types.WithAdminAuthorization(context.Background())
			target := tt.user
			target.Email = "target@example.com"
			us := mockdb.NewMockUserStore(t)
			us.EXPECT().FindByUsername(ctx, target.Username).Return(target, nil).Once()
			us.EXPECT().FindByID(ctx, target.ID, types.UserWithAdminPermissions).Return(target, nil).Once()
			orgs := mockdb.NewMockOrgStore(t)
			orgs.EXPECT().GetUserRootOrganizations(ctx, target.ID).Return(nil, nil).Once()
			tags := mockdb.NewMockUserTagStore(t)
			tags.EXPECT().GetUserTags(ctx, target.ID).Return(nil, nil).Once()
			uc := &userComponentImpl{userStore: us, orgStore: orgs, uts: tags}
			result, err := uc.Get(ctx, target.Username, "module-admin", false)
			require.NoError(t, err)
			require.Equal(t, target.UUID, result.UUID)
			require.Equal(t, target.Email, result.Email)
			require.ElementsMatch(t, tt.wantRoles, result.Roles)
			require.Equal(t, tt.wantAdmin, result.Admin)
			require.Equal(t, tt.wantSuperAdmin, result.SuperAdmin)

			req := types.UserListReq{VisitorName: "module-admin", Page: 2, Per: 20, Search: "alice", VerifyStatus: "approved", ExactMatch: true, SortBy: "created_at", SortOrder: "desc"}
			us.EXPECT().IndexWithSearch(ctx, req, types.UserWithAdminPermissions).Return([]database.User{target}, 41, nil).Once()
			list, total, err := uc.Index(ctx, req)
			require.NoError(t, err)
			require.Equal(t, 41, total)
			require.Len(t, list, 1)
			require.Equal(t, target.ID, list[0].ID)
			require.Equal(t, target.UUID, list[0].UUID)
			require.ElementsMatch(t, tt.wantRoles, list[0].Roles)
			require.Equal(t, tt.wantSuperAdmin, list[0].SuperAdmin)
		})
	}
}

// TestUserComponent_UpdateByUUIDAdminAuthorization preserves legacy grants and denies unmarked module administrators.
func TestUserComponent_UpdateByUUIDAdminAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		authorized, superAdmin, allowed bool
	}{
		{name: "authorized module administrator", authorized: true, allowed: true},
		{name: "unmarked module administrator"},
		{name: "legacy global administrator", superAdmin: true, allowed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.authorized {
				ctx = types.WithAdminAuthorization(ctx)
			}
			target := &database.User{ID: 1, UUID: "target-uuid", Username: "target", NickName: "Original", RoleMask: "personal_user"}
			actor := database.User{ID: 2, Username: "operator", RoleAssignments: []database.UserRole{{RoleCode: "operator", Role: &database.Role{IsAdmin: true, Permissions: []database.AdminRolePermission{{PermissionCode: types.AdminAccountUser}}}}}}
			if tc.superAdmin {
				actor.RoleAssignments = superAdminRoleAssignments()
			}
			us := mockdb.NewMockUserStore(t)
			lookups := 1
			if tc.allowed {
				lookups++
			}
			us.EXPECT().FindByUUID(ctx, target.UUID).Return(target, nil).Times(lookups)
			us.EXPECT().FindByUsername(ctx, actor.Username, types.UserWithAdminPermissions).Return(actor, nil).Once()
			tags := mockdb.NewMockTagStore(t)
			userTags := mockdb.NewMockUserTagStore(t)
			phone := mockphone.NewMockUserPhoneComponent(t)
			name := "Updated"
			req := &types.UpdateUserRequest{UUID: &target.UUID, OpUser: actor.Username, Nickname: &name}
			if tc.allowed {
				updated := *target
				updated.NickName = name
				us.EXPECT().Update(ctx, &updated, target.Username).Return(nil).Once()
				tags.EXPECT().CheckTagIDsExist(ctx, req.TagIDs).Return(nil).Once()
				userTags.EXPECT().ResetUserTags(ctx, target.ID, req.TagIDs).Return(nil).Once()
				phone.EXPECT().NeedPhoneChange().Return(false).Once()
			}
			uc := &userComponentImpl{userStore: us, ts: tags, uts: userTags, userPhonec: phone, once: &sync.Once{}, config: &config.Config{}, sso: mockrpc.NewMockSSOInterface(t)}
			err := uc.UpdateByUUID(ctx, req)
			if tc.allowed {
				require.NoError(t, err)
				require.Equal(t, "personal_user", target.RoleMask)
			} else {
				require.ErrorIs(t, err, errorx.ErrForbidden)
			}
		})
	}
}

// TestUserComponent_AdminAuthorizedUpdateKeepsValidation preserves target existence and self-only username/email changes.
func TestUserComponent_AdminAuthorizedUpdateKeepsValidation(t *testing.T) {
	for _, field := range []string{"missing target", "username", "email", "avatar"} {
		t.Run(field, func(t *testing.T) {
			ctx := types.WithAdminAuthorization(context.Background())
			target := &database.User{ID: 1, UUID: "target-uuid", Username: "target", Email: "target@example.com"}
			us := mockdb.NewMockUserStore(t)
			req := &types.UpdateUserRequest{UUID: &target.UUID, OpUser: "operator"}
			uc := &userComponentImpl{userStore: us, once: &sync.Once{}, config: &config.Config{}, sso: mockrpc.NewMockSSOInterface(t)}
			if field == "missing target" {
				us.EXPECT().FindByUUID(ctx, target.UUID).Return(nil, nil).Once()
				require.ErrorIs(t, uc.UpdateByUUID(ctx, req), errorx.ErrUserNotFound)
				return
			}
			us.EXPECT().FindByUUID(ctx, target.UUID).Return(target, nil).Once()
			if field == "avatar" {
				value := "invalid-image-url"
				req.Avatar = &value
				require.ErrorIs(t, uc.UpdateByUUID(ctx, req), errorx.ErrReqParamInvalid)
				return
			}
			us.EXPECT().FindByUsername(ctx, "operator", types.UserWithAdminPermissions).Return(database.User{ID: 2, Username: "operator"}, nil).Once()
			value := "changed"
			if field == "username" {
				req.NewUserName = &value
			} else {
				req.Email = &value
			}
			err := uc.UpdateByUUID(ctx, req)
			require.Error(t, err)
			require.Contains(t, err.Error(), "can only be changed by the user itself")
		})
	}
}

// TestUserComponent_AdminOperatorAuthorization keeps target protection separate from operator grants.
func TestUserComponent_AdminOperatorAuthorization(t *testing.T) {
	for _, authorized := range []bool{false, true} {
		for _, protected := range []bool{false, true} {
			t.Run(fmt.Sprintf("authorized=%v/protected=%v", authorized, protected), func(t *testing.T) {
				ctx := context.Background()
				if authorized {
					ctx = types.WithAdminAuthorization(ctx)
				}
				store := mockdb.NewMockUserStore(t)
				target := database.User{Username: "target"}
				if protected {
					target.RoleAssignments = superAdminRoleAssignments()
				}
				store.EXPECT().FindByUsername(ctx, "operator", types.UserWithAdminPermissions).Return(database.User{Username: "operator"}, nil)
				store.EXPECT().FindByUsernameWithDeleted(ctx, "target", types.UserWithAdminPermissions).Return(target, nil)
				c := &userComponentImpl{userStore: store}
				serverError, err := c.CheckOperatorAndUser(ctx, "operator", "target")
				require.False(t, serverError)
				if authorized && !protected {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			})
		}
	}
}

// TestUserComponent_AdminDeletionProtection rejects module administrators and self-deletion without depending on role names.
func TestUserComponent_AdminDeletionProtection(t *testing.T) {
	for _, tc := range []struct {
		name       string
		permission types.AdminPermission
		isAdmin    bool
		revoked    bool
		self       bool
	}{
		{name: "account administrator", permission: types.AdminAccountAdmin, isAdmin: true},
		{name: "account wildcard", permission: "account/*", isAdmin: true},
		{name: "other module administrator", permission: types.AdminAssetModel, isAdmin: true},
		{name: "administrator without grants", isAdmin: true},
		{name: "self deletion with both account permissions", permission: types.AdminAccountAdmin, isAdmin: true, self: true},
		{name: "revoked administrator", permission: types.AdminAccountAdmin, isAdmin: true, revoked: true},
		{name: "ordinary role with wildcard", permission: types.AdminPermissionAll},
		{name: "ordinary user"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := types.WithAdminAuthorization(context.Background())
			store := mockdb.NewMockUserStore(t)
			target := database.User{Username: "target", RoleAssignments: []database.UserRole{{
				RoleCode: "arbitrary", Role: &database.Role{RoleCode: "arbitrary", IsAdmin: tc.isAdmin},
			}}}
			if tc.permission != "" {
				target.RoleAssignments[0].Role.Permissions = []database.AdminRolePermission{{PermissionCode: tc.permission}}
			}
			if tc.revoked {
				target.RoleAssignments[0].DeletedAt = time.Now()
			}
			operator := database.User{Username: "operator"}
			if tc.self {
				target.RoleAssignments = append(target.RoleAssignments, database.UserRole{RoleCode: "users", Role: &database.Role{IsAdmin: true,
					Permissions: []database.AdminRolePermission{{PermissionCode: types.AdminAccountUser}},
				}})
				operator = target
			}
			store.EXPECT().FindByUsername(ctx, operator.Username, types.UserWithAdminPermissions).Return(operator, nil).Once()
			store.EXPECT().FindByUsernameWithDeleted(ctx, target.Username, types.UserWithAdminPermissions).Return(target, nil).Once()
			c := &userComponentImpl{userStore: store}
			serverError, err := c.CheckOperatorAndUser(ctx, operator.Username, target.Username)
			require.False(t, serverError)
			if tc.isAdmin && !tc.revoked {
				require.ErrorIs(t, err, errorx.ErrAdminUserCannotBeDeleted)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestUserComponent_AdminLabels bypasses only the operator's legacy global grant.
func TestUserComponent_AdminLabels(t *testing.T) {
	for _, authorized := range []bool{false, true} {
		t.Run(fmt.Sprint(authorized), func(t *testing.T) {
			ctx := context.Background()
			if authorized {
				ctx = types.WithAdminAuthorization(ctx)
			}
			store := mockdb.NewMockUserStore(t)
			c := &userComponentImpl{userStore: store}
			req := &types.UserLabelsRequest{OpUser: "operator", UUID: "target", Labels: []string{"vip"}}
			if authorized {
				store.EXPECT().UpdateLabels(ctx, req.UUID, req.Labels).Return(nil)
			} else {
				store.EXPECT().FindByUsername(ctx, "operator", types.UserWithAdminPermissions).Return(database.User{Username: "operator"}, nil)
			}
			err := c.UpdateUserLabels(ctx, req)
			if authorized {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

// TestUserComponent_UserChoices excludes private account fields even under administrator authorization.
func TestUserComponent_UserChoices(t *testing.T) {
	ctx := types.WithAdminAuthorization(context.Background())
	req := types.UserListReq{VisitorName: "model-admin", Search: "alice", Per: 10, Page: 2, ExactMatch: true}
	user := database.User{
		ID: 42, UUID: "alice-uuid", Username: "alice", NickName: "Alice", Avatar: "avatar.png",
		Email: "private@example.com", Phone: "123456", PhoneArea: "+86", Bio: "private bio",
		Homepage: "private homepage", Labels: []string{"private-label"}, LastLoginAt: "yesterday",
		VerifyStatus: types.VerifyStatus("verified"), RoleAssignments: superAdminRoleAssignments(),
	}
	for _, tc := range []struct {
		name  string
		users []database.User
		err   error
	}{
		{name: "populated", users: []database.User{user}},
		{name: "empty", users: []database.User{}},
		{name: "store error", err: errors.New("store unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := mockdb.NewMockUserStore(t)
			// No UserWithAdminPermissions option or visitor permission lookup is needed by selectors.
			store.EXPECT().IndexWithSearch(ctx, req).Return(tc.users, len(tc.users), tc.err).Once()
			choices, total, err := (&userComponentImpl{userStore: store}).UserChoices(ctx, req)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, len(tc.users), total)
			require.NotNil(t, choices)
			if len(tc.users) == 0 {
				require.Empty(t, choices)
				return
			}
			encoded, err := json.Marshal(choices)
			require.NoError(t, err)
			require.JSONEq(t, `[{"id":42,"uuid":"alice-uuid","username":"alice","nickname":"Alice","avatar":"avatar.png"}]`, string(encoded))
		})
	}
}

// TestUserComponent_Index_AdminAuthorization retains full account data for an authorized account-list request.
func TestUserComponent_Index_AdminAuthorization(t *testing.T) {
	ctx := types.WithAdminAuthorization(context.Background())
	req := types.UserListReq{VisitorName: "account-admin", Per: 10, Page: 1}
	user := database.User{ID: 42, Username: "alice", Email: "private@example.com", Phone: "123456", RoleAssignments: superAdminRoleAssignments()}
	store := mockdb.NewMockUserStore(t)
	store.EXPECT().IndexWithSearch(ctx, req, types.UserWithAdminPermissions).Return([]database.User{user}, 1, nil).Once()
	users, total, err := (&userComponentImpl{userStore: store}).Index(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, users, 1)
	require.Equal(t, user.Email, users[0].Email)
	require.Equal(t, user.Phone, users[0].Phone)
	require.Equal(t, user.Roles(), users[0].Roles)
	require.True(t, users[0].Admin)
	require.True(t, users[0].SuperAdmin)
}

// TestUserComponent_UserChoice limits another user's voucher detail to identity and active personal namespaces.
func TestUserComponent_UserChoice(t *testing.T) {
	ctx := types.WithAdminAuthorization(context.Background())
	personal := database.Namespace{Path: "alice", UUID: "personal-ns", NamespaceType: database.UserNamespace}
	deleted := personal
	deleted.UUID, deleted.DeletedAt = "deleted-ns", time.Now()
	user := database.User{ID: 42, UUID: "user-uuid", Username: "alice", NickName: "Alice", Avatar: "avatar.png",
		Email: "private@example.com", Phone: "123456", RoleAssignments: superAdminRoleAssignments(),
		Namespaces: []database.Namespace{personal, {Path: "org", UUID: "org-ns", NamespaceType: database.OrgNamespace}, deleted},
	}
	for _, tc := range []struct {
		name string
		user database.User
		err  error
	}{
		{name: "another user", user: user},
		{name: "no namespace", user: database.User{Username: "alice"}},
		{name: "missing user", err: sql.ErrNoRows},
		{name: "store failure", err: errors.New("store unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := mockdb.NewMockUserStore(t)
			store.EXPECT().FindByUsername(ctx, "alice").Return(tc.user, tc.err).Once()
			choice, err := (&userComponentImpl{userStore: store}).UserChoice(ctx, "alice")
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			encoded, err := json.Marshal(choice)
			require.NoError(t, err)
			if tc.name == "no namespace" {
				require.JSONEq(t, `{"id":0,"uuid":"","username":"alice","nickname":"","avatar":"","namespaces":[]}`, string(encoded))
			} else {
				require.JSONEq(t, `{"id":42,"uuid":"user-uuid","username":"alice","nickname":"Alice","avatar":"avatar.png","namespaces":[{"Path":"alice","Type":"user","UUID":"personal-ns"}]}`, string(encoded))
			}
		})
	}
}

package component

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockrebac "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rebac"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

// TestOrganizationDeletionWorkerReturnsCleanupFailureForRiverRetry verifies the
// durable path reports failures even though the request fast path only logs them.
func TestOrganizationDeletionWorkerReturnsCleanupFailureForRiverRetry(t *testing.T) {
	ctx := context.Background()
	sso := mockrpc.NewMockSSOInterface(t)
	sso.EXPECT().DeleteUser(ctx, "organization-uuid").Return(errors.New("SSO unavailable")).Once()
	worker := NewOrganizationDeletionWorker(sso, mockrebac.NewMockAuthorizer(t), nil)

	err := worker.Work(ctx, &river.Job[workhub.OrganizationDeletionArgs]{Args: workhub.OrganizationDeletionArgs{
		OrganizationUUIDs: []string{"organization-uuid"},
	}})

	require.ErrorContains(t, err, "SSO unavailable")
}

// TestOrganizationDeletionWorkerRejectsEmptySnapshot verifies malformed jobs
// are retried or discarded by River instead of being silently completed.
func TestOrganizationDeletionWorkerRejectsEmptySnapshot(t *testing.T) {
	worker := NewOrganizationDeletionWorker(nil, mockrebac.NewMockAuthorizer(t), nil)
	err := worker.Work(context.Background(), &river.Job[workhub.OrganizationDeletionArgs]{})
	require.ErrorContains(t, err, "requires organization UUIDs")
}

// TestOrganizationComponent_DeleteAlreadyMissing is idempotent when the database row was already removed.
func TestOrganizationComponent_DeleteAlreadyMissing(t *testing.T) {
	ctx := context.Background()
	req := &types.DeleteOrgReq{Name: "already-deleted", CurrentUser: "admin"}
	orgStore := mockdb.NewMockOrgStore(t)
	orgStore.EXPECT().FindForDeletion(ctx, database.OrganizationDeletionLookup{Path: req.Name}).Return(database.Organization{}, sql.ErrNoRows).Once()

	c := &organizationComponentImpl{orgStore: orgStore}
	require.NoError(t, c.Delete(ctx, req))
}

func TestOrganizationComponent_Create(t *testing.T) {
	req := &types.CreateOrgReq{
		Name:        "org1",
		Nickname:    "org_nickname",
		Description: "org_description",
		Username:    "user1",
		Homepage:    "org-homepage.com",
		Logo:        "org-logo.png",
		Verified:    false,
		OrgType:     "school",
	}
	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, req.Username).Return(database.User{
		Username: "user1",
		UUID:     "user-uuid",
	}, nil).Once()

	mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
	mockNamespaceStore.EXPECT().Exists(mock.Anything, req.Name).Return(false, nil).Once()
	mockNamespaceStore.EXPECT().ExistsByUUID(mock.Anything, mock.Anything).Return(false, nil).Once()

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().CreateWithRelations(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	mockSSO := mockrpc.NewMockSSOInterface(t)
	mockSSO.EXPECT().IsExistByName(mock.Anything, req.Name).Return(false, nil).Once()
	mockSSO.EXPECT().CreateUser(mock.Anything, mock.Anything).Return(nil).Once()

	mockTagStore := mockdb.NewMockTagStore(t)
	mockAuthorizer := mockrebac.NewMockAuthorizer(t)
	expectNamespaceReBACWrite(mockAuthorizer, rebac.RelationOrganization, 1)
	expectAnyOrganizationAdminReBACReconciliation(mockAuthorizer, "user-uuid")

	// GetOrganizationTags is called to return tags in the response
	mockOrgStore.EXPECT().GetOrganizationTags(mock.Anything, mock.Anything).Return([]database.Tag{}, nil).Once()

	c := &organizationComponentImpl{
		userStore: mockUserStore,
		nsStore:   mockNamespaceStore,
		orgStore:  mockOrgStore,
		tagStore:  mockTagStore,
		sso:       mockSSO,
		rebac:     mockAuthorizer,
	}
	org, err := c.Create(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, req.Name, org.Name)
	require.Equal(t, req.Nickname, org.Nickname)
	require.Equal(t, req.Homepage, org.Homepage)
	require.Equal(t, req.Logo, org.Logo)
	require.Equal(t, req.OrgType, org.OrgType)
	require.Equal(t, req.Verified, org.Verified)
	require.True(t, org.IsRoot)
	require.NotEqual(t, uuid.Nil, org.UUID)
}

// TestOrganizationComponent_ListCurrentUserWritableNamespaces resolves OpenFGA namespace objects to display data.
func TestOrganizationComponent_ListCurrentUserWritableNamespaces(t *testing.T) {
	ctx := context.Background()
	userUUID := "user-uuid"
	organizationUUID := uuid.New()
	newerOrganizationUUID := uuid.New()
	actor := database.User{ID: 42, Username: "current-user", UUID: userUUID}
	organizationNamespaceUUID := organizationUUID.String()
	newerOrganizationNamespaceUUID := newerOrganizationUUID.String()
	userNamespace := database.Namespace{Path: actor.Username, UUID: userUUID, NamespaceType: database.UserNamespace}
	userNamespace.CreatedAt = time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	organizationNamespace := database.Namespace{Path: "engineering", UUID: organizationNamespaceUUID, NamespaceType: database.OrgNamespace}
	organizationNamespace.CreatedAt = time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	newerOrganizationNamespace := database.Namespace{Path: "research", UUID: newerOrganizationNamespaceUUID, NamespaceType: database.OrgNamespace}
	newerOrganizationNamespace.CreatedAt = time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	organization := database.Organization{ID: 1, Name: "engineering", Nickname: "IT Department", UUID: organizationUUID}
	newerOrganization := database.Organization{ID: 2, Name: "research", Nickname: "Research", UUID: newerOrganizationUUID}

	userStore := mockdb.NewMockUserStore(t)
	userStore.EXPECT().FindByUsername(mock.Anything, actor.Username).Return(actor, nil).Once()
	userStore.EXPECT().FindByUUIDs(mock.Anything, []string{userUUID}).Return([]*database.User{{UUID: userUUID, Username: actor.Username, NickName: "Alice"}}, nil).Once()
	orgStore := mockdb.NewMockOrgStore(t)
	orgStore.EXPECT().FindByUUIDs(mock.Anything, []string{organizationNamespaceUUID, newerOrganizationNamespaceUUID}).Return([]database.Organization{organization, newerOrganization}, nil).Once()
	nsStore := mockdb.NewMockNamespaceStore(t)
	nsStore.EXPECT().FindByUUIDs(mock.Anything, []string{organizationNamespaceUUID, userUUID, newerOrganizationNamespaceUUID}).Return([]database.Namespace{organizationNamespace, userNamespace, newerOrganizationNamespace}, nil).Once()
	authorizer := mockrebac.NewMockAuthorizer(t)
	authorizer.EXPECT().ListObjects(ctx, rebac.ListObjectsRequest{
		Subject: rebac.UserSubject(userUUID), Relation: rebac.NamespaceCanWrite,
		ObjectType: rebac.ObjectTypeNamespace, Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.ListObjectsResult{Objects: []rebac.Object{
		rebac.NamespaceObject(organizationNamespaceUUID), rebac.NamespaceObject(userUUID), rebac.NamespaceObject(newerOrganizationNamespaceUUID),
	}}, nil).Once()

	component := &organizationComponentImpl{userStore: userStore, nsStore: nsStore, orgStore: orgStore, rebac: authorizer}
	result, err := component.ListCurrentUserWritableNamespaces(ctx, actor.Username)

	require.NoError(t, err)
	require.Equal(t, []types.WritableNamespace{
		{Path: actor.Username, Type: "user", Name: "Alice", UUID: userUUID},
		{Path: "engineering", Type: "organization", Name: "IT Department", UUID: organizationNamespaceUUID},
		{Path: "research", Type: "organization", Name: "Research", UUID: newerOrganizationNamespaceUUID},
	}, result)
}

// TestOrganizationComponent_ListCurrentUserWritableNamespacesFallsBackToUsername uses username when a display name is empty.
func TestOrganizationComponent_ListCurrentUserWritableNamespacesFallsBackToUsername(t *testing.T) {
	ctx := context.Background()
	actor := database.User{Username: "current-user", UUID: "user-uuid"}
	userStore := mockdb.NewMockUserStore(t)
	userStore.EXPECT().FindByUsername(mock.Anything, actor.Username).Return(actor, nil).Once()
	userStore.EXPECT().FindByUUIDs(mock.Anything, []string{actor.UUID}).Return([]*database.User{{UUID: actor.UUID, Username: actor.Username}}, nil).Once()
	nsStore := mockdb.NewMockNamespaceStore(t)
	nsStore.EXPECT().FindByUUIDs(mock.Anything, []string{actor.UUID}).Return([]database.Namespace{{Path: actor.Username, UUID: actor.UUID, NamespaceType: database.UserNamespace}}, nil).Once()
	authorizer := mockrebac.NewMockAuthorizer(t)
	authorizer.EXPECT().ListObjects(mock.Anything, mock.Anything).Return(rebac.ListObjectsResult{Objects: []rebac.Object{rebac.NamespaceObject(actor.UUID)}}, nil).Once()

	component := &organizationComponentImpl{userStore: userStore, nsStore: nsStore, orgStore: mockdb.NewMockOrgStore(t), rebac: authorizer}
	result, err := component.ListCurrentUserWritableNamespaces(ctx, actor.Username)

	require.NoError(t, err)
	require.Equal(t, []types.WritableNamespace{{Path: actor.Username, Type: "user", Name: actor.Username, UUID: actor.UUID}}, result)
}

// TestOrganizationComponent_Create_UsesAtomicStore verifies the component creates organization records through the atomic Store path.
func TestOrganizationComponent_Create_UsesAtomicStore(t *testing.T) {
	req := &types.CreateOrgReq{
		Name:     "org-atomic",
		Username: "user1",
		TagIDs:   []int64{7, 7},
	}
	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, req.Username).Return(database.User{
		Username: "user1",
		ID:       42,
		UUID:     "user-uuid",
	}, nil).Once()

	mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
	mockNamespaceStore.EXPECT().Exists(mock.Anything, req.Name).Return(false, nil).Once()
	mockNamespaceStore.EXPECT().ExistsByUUID(mock.Anything, mock.Anything).Return(false, nil).Once()

	mockSSO := mockrpc.NewMockSSOInterface(t)
	mockSSO.EXPECT().IsExistByName(mock.Anything, req.Name).Return(false, nil).Once()
	mockSSO.EXPECT().CreateUser(mock.Anything, mock.Anything).Return(nil).Once()

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExistInScope(mock.Anything, req.TagIDs, types.OrganizationTagScope, string(types.IndustryCategory)).Return(nil).Once()

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().CreateWithRelations(mock.Anything, mock.Anything, mock.Anything, req.TagIDs).Return(nil).Once()
	mockOrgStore.EXPECT().GetOrganizationTags(mock.Anything, mock.Anything).Return([]database.Tag{}, nil).Once()
	mockAuthorizer := mockrebac.NewMockAuthorizer(t)
	expectNamespaceReBACWrite(mockAuthorizer, rebac.RelationOrganization, 1)
	expectAnyOrganizationAdminReBACReconciliation(mockAuthorizer, "user-uuid")

	c := &organizationComponentImpl{
		userStore: mockUserStore,
		nsStore:   mockNamespaceStore,
		orgStore:  mockOrgStore,
		tagStore:  mockTagStore,
		sso:       mockSSO,
		rebac:     mockAuthorizer,
	}
	org, err := c.Create(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, req.Name, org.Name)
}

// expectAnyOrganizationAdminReBACReconciliation verifies the generated organization administrator tuple.
func expectAnyOrganizationAdminReBACReconciliation(authorizer *mockrebac.MockAuthorizer, userUUID string) {
	authorizer.EXPECT().BatchCheck(mock.Anything, mock.MatchedBy(func(request rebac.BatchCheckRequest) bool {
		if len(request.Checks) != len(organizationMemberRelations) {
			return false
		}
		for _, item := range request.Checks {
			if item.Check.Subject != rebac.UserSubject(userUUID) ||
				item.Check.Object.Type != rebac.ObjectTypeOrganization ||
				item.Check.Consistency != rebac.ConsistencyHigher {
				return false
			}
		}
		return true
	})).RunAndReturn(func(_ context.Context, request rebac.BatchCheckRequest) (rebac.BatchCheckResult, error) {
		results := make(map[string]rebac.BatchCheckOutcome, len(request.Checks))
		for _, item := range request.Checks {
			results[item.CorrelationID] = rebac.BatchCheckOutcome{Decision: rebac.Decision{Allowed: false}}
		}
		return rebac.BatchCheckResult{Results: results}, nil
	}).Once()
	authorizer.EXPECT().Write(mock.Anything, mock.MatchedBy(func(relationships []rebac.Relationship) bool {
		return len(relationships) == 1 &&
			relationships[0].Subject == rebac.UserSubject(userUUID) &&
			relationships[0].Relation == rebac.RelationAdmin &&
			relationships[0].Object.Type == rebac.ObjectTypeOrganization
	})).Return(nil).Once()
}

// TestOrganizationComponent_Create_DatabaseFailureCompensatesSSO verifies a failed database write cleans up SSO once.
func TestOrganizationComponent_Create_DatabaseFailureCompensatesSSO(t *testing.T) {
	req := &types.CreateOrgReq{Name: "org-db-failure", Username: "user1"}
	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, req.Username).Return(database.User{Username: req.Username}, nil).Once()
	mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
	mockNamespaceStore.EXPECT().Exists(mock.Anything, req.Name).Return(false, nil).Once()
	mockNamespaceStore.EXPECT().ExistsByUUID(mock.Anything, mock.Anything).Return(false, nil).Once()
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().CreateWithRelations(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("database transaction failed")).Once()
	mockSSO := mockrpc.NewMockSSOInterface(t)
	mockSSO.EXPECT().IsExistByName(mock.Anything, req.Name).Return(false, nil).Once()
	mockSSO.EXPECT().CreateUser(mock.Anything, mock.Anything).Return(nil).Once()
	mockSSO.EXPECT().DeleteUser(mock.Anything, mock.Anything).Return(errors.New("SSO cleanup failed")).Once()

	c := &organizationComponentImpl{
		userStore: mockUserStore,
		nsStore:   mockNamespaceStore,
		orgStore:  mockOrgStore,
		sso:       mockSSO,
	}
	organization, err := c.Create(context.Background(), req)
	require.Nil(t, organization)
	require.ErrorContains(t, err, "database transaction failed")
}

// TestOrganizationComponent_DeleteDefersSSOFailure verifies a durable job owns
// eventual cleanup when the synchronous SSO attempt fails.
func TestOrganizationComponent_DeleteDefersSSOFailure(t *testing.T) {
	ctx := context.Background()
	organizationUUID := uuid.New()
	req := &types.DeleteOrgReq{Name: "org-delete", CurrentUser: "admin"}
	cleanup := types.OrganizationReBACCleanup{
		OrganizationUUID: organizationUUID.String(),
		NamespaceUUID:    "organization-namespace-uuid",
		UserUUIDs:        []string{"member-uuid"},
	}
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().FindForDeletion(ctx, database.OrganizationDeletionLookup{Path: req.Name}).Return(database.Organization{
		ID: 17, Name: req.Name, UUID: organizationUUID,
		Namespace: &database.Namespace{UUID: cleanup.NamespaceUUID, NamespaceType: database.OrgNamespace},
	}, nil).Once()
	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(ctx, req.CurrentUser).Return(database.User{
		Username: req.CurrentUser,
		UUID:     "admin-uuid",
	}, nil).Once()
	mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
	mockNamespaceStore.EXPECT().FindByPath(ctx, req.Name).Return(database.Namespace{
		Path:          req.Name,
		UUID:          cleanup.NamespaceUUID,
		NamespaceType: database.OrgNamespace,
	}, nil).Once()
	mockMemberStore := mockdb.NewMockMemberStore(t)
	mockOrgStore.EXPECT().Delete(ctx, req.Name).Return(database.OrganizationDeleteResult{OrganizationJobID: 99, DeletedReBACRelationships: []types.OrganizationReBACCleanup{cleanup}}, nil).Once()
	mockAuthorizer := mockrebac.NewMockAuthorizer(t)
	mockAuthorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject:     rebac.UserSubject("admin-uuid"),
		Relation:    rebac.NamespaceCanAdmin,
		Object:      rebac.NamespaceObject(cleanup.NamespaceUUID),
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: true}, nil).Once()
	mockSSO := mockrpc.NewMockSSOInterface(t)
	expectOrganizationReBACCleanup(t, mockAuthorizer, cleanup)
	mockSSO.EXPECT().DeleteUser(mock.Anything, organizationUUID.String()).Return(errors.New("SSO unavailable")).Once()

	c := &organizationComponentImpl{
		userStore: mockUserStore, nsStore: mockNamespaceStore,
		orgStore: mockOrgStore, memberStore: mockMemberStore,
		rebac: mockAuthorizer, sso: mockSSO,
	}
	require.NoError(t, c.Delete(ctx, req))
}

func TestOrganizationComponent_Delete_LeavesRepositoryCleanupToWorker(t *testing.T) {
	ctx := context.Background()
	organizationUUID := uuid.New()
	req := &types.DeleteOrgReq{Name: "org-repositories", CurrentUser: "admin"}
	cleanup := types.OrganizationReBACCleanup{
		OrganizationUUID: organizationUUID.String(), NamespaceUUID: "organization-namespace-uuid",
	}
	repository := database.DeletedRepository{ID: 42, RepositoryType: types.ModelRepo, Path: req.Name + "/model"}

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().FindForDeletion(ctx, database.OrganizationDeletionLookup{Path: req.Name}).Return(database.Organization{
		ID: 17, Name: req.Name, UUID: organizationUUID,
		Namespace: &database.Namespace{UUID: cleanup.NamespaceUUID, NamespaceType: database.OrgNamespace},
	}, nil).Once()
	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(ctx, req.CurrentUser).Return(database.User{Username: req.CurrentUser, UUID: "admin-uuid"}, nil).Once()
	mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
	mockNamespaceStore.EXPECT().FindByPath(ctx, req.Name).Return(database.Namespace{
		Path: req.Name, UUID: cleanup.NamespaceUUID, NamespaceType: database.OrgNamespace,
	}, nil).Once()
	mockMemberStore := mockdb.NewMockMemberStore(t)
	mockOrgStore.EXPECT().Delete(ctx, req.Name).Return(database.OrganizationDeleteResult{
		DeletedRepositories:       []database.DeletedRepository{repository},
		DeletedReBACRelationships: []types.OrganizationReBACCleanup{cleanup},
	}, nil).Once()
	mockAuthorizer := mockrebac.NewMockAuthorizer(t)
	mockAuthorizer.EXPECT().Check(ctx, mock.Anything).Return(rebac.Decision{Allowed: true}, nil).Once()
	expectOrganizationReBACCleanup(t, mockAuthorizer, cleanup)
	mockSSO := mockrpc.NewMockSSOInterface(t)
	mockSSO.EXPECT().DeleteUser(mock.Anything, organizationUUID.String()).Return(nil).Once()

	c := &organizationComponentImpl{
		userStore: mockUserStore, nsStore: mockNamespaceStore, orgStore: mockOrgStore,
		memberStore: mockMemberStore, rebac: mockAuthorizer, sso: mockSSO,
	}
	require.NoError(t, c.Delete(ctx, req))
}

// TestOrganizationComponent_DeleteRejectsHierarchyOrganization verifies the legacy endpoint cannot delete a hierarchy tree.
func TestOrganizationComponent_DeleteRejectsHierarchyOrganization(t *testing.T) {
	ctx := context.Background()
	req := &types.DeleteOrgReq{Name: "hierarchy-root", CurrentUser: "admin"}
	organizationUUID := uuid.New()

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().FindForDeletion(ctx, database.OrganizationDeletionLookup{Path: req.Name}).Return(database.Organization{
		ID: 1, Name: req.Name, UUID: organizationUUID, IsRoot: true, IsHierarchical: true,
	}, nil).Once()

	c := &organizationComponentImpl{
		orgStore: mockOrgStore,
	}

	err := c.Delete(ctx, req)
	require.ErrorContains(t, err, "hierarchy organizations must be deleted through the hierarchy organization API")
}

func TestOrganizationComponent_Create_NamespaceExists(t *testing.T) {
	req := &types.CreateOrgReq{
		Name:     "org1",
		Username: "user1",
	}
	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, req.Username).Return(database.User{
		Username: "user1",
	}, nil).Once()

	mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
	mockNamespaceStore.EXPECT().Exists(mock.Anything, req.Name).Return(true, nil).Once()

	c := &organizationComponentImpl{
		userStore: mockUserStore,
		nsStore:   mockNamespaceStore,
	}
	org, err := c.Create(context.Background(), req)
	require.Nil(t, org)
	require.ErrorIs(t, err, errorx.ErrNamespaceAlreadyExists)
}

func TestOrganizationComponent_Create_SSOUserExists(t *testing.T) {
	req := &types.CreateOrgReq{
		Name:     "org1",
		Username: "user1",
	}
	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, req.Username).Return(database.User{
		Username: "user1",
	}, nil).Once()

	mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
	mockNamespaceStore.EXPECT().Exists(mock.Anything, req.Name).Return(false, nil).Once()

	mockSSO := mockrpc.NewMockSSOInterface(t)
	mockSSO.EXPECT().IsExistByName(mock.Anything, req.Name).Return(true, nil).Once()

	c := &organizationComponentImpl{
		userStore: mockUserStore,
		nsStore:   mockNamespaceStore,
		sso:       mockSSO,
	}
	org, err := c.Create(context.Background(), req)
	require.Nil(t, org)
	require.ErrorIs(t, err, errorx.ErrNamespaceAlreadyExists)
}

func TestOrganizationComponent_Create_InvalidTagIDs(t *testing.T) {
	req := &types.CreateOrgReq{
		Name:     "org1",
		Username: "user1",
		TagIDs:   []int64{999},
	}
	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, req.Username).Return(database.User{
		Username: "user1",
	}, nil).Once()

	mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
	mockNamespaceStore.EXPECT().Exists(mock.Anything, req.Name).Return(false, nil).Once()

	mockSSO := mockrpc.NewMockSSOInterface(t)
	mockSSO.EXPECT().IsExistByName(mock.Anything, req.Name).Return(false, nil).Once()

	mockTagStore := mockdb.NewMockTagStore(t)
	mockTagStore.EXPECT().CheckTagIDsExistInScope(mock.Anything, []int64{999}, types.OrganizationTagScope, string(types.IndustryCategory)).Return(database.ErrTagIDsNotFoundInScope).Once()

	// No mocks for Git, SSO CreateUser, or DB Create — tag validation fails first
	c := &organizationComponentImpl{
		userStore: mockUserStore,
		nsStore:   mockNamespaceStore,
		sso:       mockSSO,
		tagStore:  mockTagStore,
	}
	org, err := c.Create(context.Background(), req)
	require.Nil(t, org)
	require.ErrorIs(t, err, errorx.ErrTagIDsNotExist)
}

func TestOrganizationComponent_Index(t *testing.T) {
	var dbOrgs []database.Organization
	dbOrgs = append(dbOrgs, database.Organization{
		ID:       1,
		Name:     "org1",
		Nickname: "org_nickname",
		Homepage: "org-homepage.com",
		Logo:     "org-logo.png",
		OrgType:  "school",
		Verified: false,
		Namespace: &database.Namespace{
			Path:          "org1",
			NamespaceType: database.OrgNamespace,
		},
	})
	dbOrgs = append(dbOrgs, database.Organization{
		ID:       2,
		Name:     "org2",
		Nickname: "org_nickname",
		Homepage: "org-homepage.com",
		Logo:     "org-logo.png",
		OrgType:  "school",
		Verified: false,
		Namespace: &database.Namespace{
			Path:          "org2",
			NamespaceType: database.OrgNamespace,
		},
	})
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().Search(mock.Anything, "", 10, 0, "", "", "").Return(dbOrgs, len(dbOrgs), nil).Once()
	mockOrgStore.EXPECT().GetOrganizationTagsByOrgIDs(mock.Anything, []int64{1, 2}).Return(map[int64][]database.Tag{}, nil).Once()

	c := &organizationComponentImpl{
		orgStore: mockOrgStore,
	}
	expectedOrgs, total, err := c.Index(context.Background(), "", 10, 0, "", "", "")

	require.NoError(t, err)
	require.Len(t, expectedOrgs, 2)
	require.Equal(t, 2, total)
	require.Condition(t, func() bool {

		for i := 0; i < len(expectedOrgs); i++ {
			if expectedOrgs[i].Name != dbOrgs[i].Name {
				return false
			}
			if expectedOrgs[i].Nickname != dbOrgs[i].Nickname {
				return false
			}
			if expectedOrgs[i].Homepage != dbOrgs[i].Homepage {
				return false
			}
			if expectedOrgs[i].Logo != dbOrgs[i].Logo {
				return false
			}
			if expectedOrgs[i].OrgType != dbOrgs[i].OrgType {
				return false
			}
			if expectedOrgs[i].Verified != dbOrgs[i].Verified {
				return false
			}
			if expectedOrgs[i].Namespace == nil {
				return false
			}
			if expectedOrgs[i].Namespace.Path != dbOrgs[i].Namespace.Path {
				return false
			}
		}
		return true
	})
}

func TestOrganizationComponent_ListUserOrgs_Admin(t *testing.T) {
	var dbOrgs []database.Organization
	dbOrgs = append(dbOrgs, database.Organization{
		ID:      1,
		Name:    "org1",
		OrgType: "school",
		Namespace: &database.Namespace{
			Path:          "org1",
			NamespaceType: database.OrgNamespace,
		},
	})

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().SearchUserBelongOrgs(mock.Anything, int64(1), "", 10, 1, "", "", "", "").Return(dbOrgs, len(dbOrgs), nil).Once()
	mockOrgStore.EXPECT().GetOrganizationTagsByOrgIDs(mock.Anything, []int64{1}).Return(map[int64][]database.Tag{}, nil).Once()

	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "admin1").Return(database.User{
		ID:       1,
		Username: "admin1",
		RoleMask: "admin",
	}, nil)

	c := &organizationComponentImpl{
		orgStore:  mockOrgStore,
		userStore: mockUserStore,
	}
	orgs, total, err := c.ListUserOrgs(context.Background(), &types.ListUserOrgsReq{
		Username: "admin1", Per: 10, Page: 1,
	})

	require.NoError(t, err)
	require.Len(t, orgs, 1)
	require.Equal(t, 1, total)
	require.Equal(t, "org1", orgs[0].Name)
}

func TestOrganizationComponent_ListUserOrgs_RegularUser(t *testing.T) {
	var dbOrgs []database.Organization
	dbOrgs = append(dbOrgs, database.Organization{
		ID:      1,
		Name:    "org1",
		OrgType: "school",
		Namespace: &database.Namespace{
			Path:          "org1",
			NamespaceType: database.OrgNamespace,
		},
	})

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().SearchUserBelongOrgs(mock.Anything, int64(2), "", 10, 1, "", "", "", "").Return(dbOrgs, len(dbOrgs), nil).Once()
	mockOrgStore.EXPECT().GetOrganizationTagsByOrgIDs(mock.Anything, []int64{1}).Return(map[int64][]database.Tag{}, nil).Once()

	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "user1").Return(database.User{
		ID:       2,
		Username: "user1",
		RoleMask: "",
	}, nil)

	c := &organizationComponentImpl{
		orgStore:  mockOrgStore,
		userStore: mockUserStore,
	}
	orgs, total, err := c.ListUserOrgs(context.Background(), &types.ListUserOrgsReq{
		Username: "user1", Per: 10, Page: 1,
	})

	require.NoError(t, err)
	require.Len(t, orgs, 1)
	require.Equal(t, 1, total)
	require.Equal(t, "org1", orgs[0].Name)
}

func TestOrganizationComponent_ListUserOrgs_RegularUserWithFilters(t *testing.T) {
	var dbOrgs []database.Organization
	dbOrgs = append(dbOrgs, database.Organization{
		ID:      1,
		Name:    "org1",
		OrgType: "school",
		Namespace: &database.Namespace{
			Path:          "org1",
			NamespaceType: database.OrgNamespace,
		},
	})

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().SearchUserBelongOrgs(mock.Anything, int64(2), "search", 5, 2, "school", "approved", "", "").Return(dbOrgs, len(dbOrgs), nil).Once()
	mockOrgStore.EXPECT().GetOrganizationTagsByOrgIDs(mock.Anything, []int64{1}).Return(map[int64][]database.Tag{}, nil).Once()

	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "user1").Return(database.User{
		ID:       2,
		Username: "user1",
		RoleMask: "",
	}, nil)

	c := &organizationComponentImpl{
		orgStore:  mockOrgStore,
		userStore: mockUserStore,
	}
	orgs, total, err := c.ListUserOrgs(context.Background(), &types.ListUserOrgsReq{
		Username: "user1", Search: "search", Per: 5, Page: 2, OrgType: "school", VerifyStatus: "approved",
	})

	require.NoError(t, err)
	require.Len(t, orgs, 1)
	require.Equal(t, 1, total)
	require.Equal(t, "org1", orgs[0].Name)
}

func TestOrganizationComponent_ListUserOrgs_EmptyUsername(t *testing.T) {
	c := &organizationComponentImpl{}
	orgs, total, err := c.ListUserOrgs(context.Background(), &types.ListUserOrgsReq{})

	require.Error(t, err)
	require.Nil(t, orgs)
	require.Equal(t, 0, total)
	require.Contains(t, err.Error(), "username is required")
}

func TestOrganizationComponent_ListUserOrgs_WriteRole(t *testing.T) {
	var dbOrgs []database.Organization
	dbOrgs = append(dbOrgs, database.Organization{
		ID:      1,
		Name:    "org1",
		OrgType: "school",
		Namespace: &database.Namespace{
			Path:          "org1",
			NamespaceType: database.OrgNamespace,
		},
	})

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().SearchUserBelongOrgs(mock.Anything, int64(2), "", 10, 1, "", "", string(types.UserWrite), "").Return(dbOrgs, len(dbOrgs), nil).Once()
	mockOrgStore.EXPECT().GetOrganizationTagsByOrgIDs(mock.Anything, []int64{1}).Return(map[int64][]database.Tag{}, nil).Once()

	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "user1").Return(database.User{
		ID:       2,
		Username: "user1",
		RoleMask: "",
	}, nil)

	c := &organizationComponentImpl{
		orgStore:  mockOrgStore,
		userStore: mockUserStore,
	}
	orgs, total, err := c.ListUserOrgs(context.Background(), &types.ListUserOrgsReq{
		Username: "user1", Per: 10, Page: 1, Role: string(types.UserWrite),
	})

	require.NoError(t, err)
	require.Len(t, orgs, 1)
	require.Equal(t, 1, total)
	require.Equal(t, "org1", orgs[0].Name)
}

func TestOrganizationComponent_ListUserOrgs_AdminRole(t *testing.T) {
	var dbOrgs []database.Organization
	dbOrgs = append(dbOrgs, database.Organization{
		ID:      1,
		Name:    "org1",
		OrgType: "school",
		Namespace: &database.Namespace{
			Path:          "org1",
			NamespaceType: database.OrgNamespace,
		},
	})

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().SearchUserBelongOrgs(mock.Anything, int64(2), "", 10, 1, "", "", string(types.UserAdmin), "").Return(dbOrgs, len(dbOrgs), nil).Once()
	mockOrgStore.EXPECT().GetOrganizationTagsByOrgIDs(mock.Anything, []int64{1}).Return(map[int64][]database.Tag{}, nil).Once()

	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "user1").Return(database.User{
		ID:       2,
		Username: "user1",
		RoleMask: "",
	}, nil)

	c := &organizationComponentImpl{
		orgStore:  mockOrgStore,
		userStore: mockUserStore,
	}
	orgs, total, err := c.ListUserOrgs(context.Background(), &types.ListUserOrgsReq{
		Username: "user1", Per: 10, Page: 1, Role: string(types.UserAdmin),
	})

	require.NoError(t, err)
	require.Len(t, orgs, 1)
	require.Equal(t, 1, total)
	require.Equal(t, "org1", orgs[0].Name)
}

func TestOrganizationComponent_toOrgList(t *testing.T) {
	dbOrgs := []database.Organization{
		{
			ID:           1,
			Name:         "org1",
			Nickname:     "nick1",
			Description:  "desc1",
			Homepage:     "https://org1.com",
			Logo:         "logo1.png",
			OrgType:      "school",
			Verified:     true,
			VerifyStatus: "approved",
			UUID:         uuid.New(),
			Namespace: &database.Namespace{
				Path:          "org1",
				NamespaceType: database.OrgNamespace,
			},
		},
		{
			ID:      2,
			Name:    "org2",
			OrgType: "company",
		},
	}

	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().GetOrganizationTagsByOrgIDs(mock.Anything, []int64{1, 2}).Return(map[int64][]database.Tag{}, nil).Once()

	c := &organizationComponentImpl{
		orgStore: mockOrgStore,
	}
	orgs, err := c.toOrgList(context.Background(), dbOrgs)
	require.NoError(t, err)

	require.Len(t, orgs, 2)
	require.Equal(t, "org1", orgs[0].Name)
	require.Equal(t, "nick1", orgs[0].Nickname)
	require.Equal(t, "desc1", orgs[0].Description)
	require.Equal(t, "https://org1.com", orgs[0].Homepage)
	require.Equal(t, "logo1.png", orgs[0].Logo)
	require.Equal(t, "school", orgs[0].OrgType)
	require.True(t, orgs[0].Verified)
	require.Equal(t, "approved", orgs[0].VerifyStatus)
	require.NotNil(t, orgs[0].Namespace)
	require.Equal(t, "org1", orgs[0].Namespace.Path)

	require.Equal(t, "org2", orgs[1].Name)
	require.Nil(t, orgs[1].Namespace)
}

func TestOrganizationComponent_Update(t *testing.T) {
	org := database.Organization{
		ID:        1,
		UserID:    1,
		Name:      "org1",
		Nickname:  "org_nickname",
		Homepage:  "org-homepage.com",
		Logo:      "org-logo.png",
		OrgType:   "school",
		Verified:  false,
		Namespace: &database.Namespace{Path: "org1", UUID: "namespace-uuid"},
	}
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().FindByPath(mock.Anything, "org1").Return(org, nil)
	mockOrgStore.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)

	newDesc := "org1 description"
	mockUserStore := mockdb.NewMockUserStore(t)
	mockUserStore.EXPECT().FindByUsername(mock.Anything, "op").Return(database.User{
		Username: "op",
		UUID:     "op-uuid",
	}, nil).Once()
	mockNamespaceStore := mockdb.NewMockNamespaceStore(t)
	mockNamespaceStore.EXPECT().FindByPath(mock.Anything, "org1").Return(database.Namespace{
		Path:          "org1",
		UUID:          "namespace-uuid",
		NamespaceType: database.OrgNamespace,
	}, nil).Once()
	mockAuthorizer := mockrebac.NewMockAuthorizer(t)
	mockAuthorizer.EXPECT().Check(mock.Anything, rebac.CheckRequest{
		Subject:     rebac.UserSubject("op-uuid"),
		Relation:    rebac.NamespaceCanAdmin,
		Object:      rebac.NamespaceObject("namespace-uuid"),
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: true}, nil).Once()

	c := &organizationComponentImpl{
		orgStore: mockOrgStore, userStore: mockUserStore,
		nsStore: mockNamespaceStore, rebac: mockAuthorizer,
	}

	returnOrg, err := c.Update(context.Background(), &types.EditOrgReq{
		Name:        "org1",
		CurrentUser: "op",
		Description: &newDesc,
	})

	require.NoError(t, err)
	require.Equal(t, "org1", returnOrg.Name)
	require.Equal(t, newDesc, returnOrg.Description)
	require.Equal(t, int64(1), returnOrg.UserID)
}

func TestOrganizationComponent_Get(t *testing.T) {
	dbOrg := database.Organization{
		ID:       1,
		Nickname: "org1",
		Name:     "org_path",
		Homepage: "https://org1.com",
		Logo:     "https://org1.com/logo.png",
		OrgType:  "company",
		Verified: true,
		Namespace: &database.Namespace{
			ID:            1,
			Path:          "org_path",
			NamespaceType: database.OrgNamespace,
			UUID:          "ns-uuid-1",
		},
	}
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().FindByPath(mock.Anything, "org_path").Return(dbOrg, nil)
	mockOrgStore.EXPECT().GetOrganizationTags(mock.Anything, dbOrg.ID).Return([]database.Tag{}, nil)

	c := &organizationComponentImpl{
		orgStore: mockOrgStore,
	}
	org, err := c.Get(context.Background(), "org_path")
	require.NoError(t, err)
	require.Equal(t, "org_path", org.Name)
	require.Equal(t, "org1", org.Nickname)
	require.Equal(t, "https://org1.com", org.Homepage)
	require.NotNil(t, org.Namespace)
	require.Equal(t, "org_path", org.Namespace.Path)
}

func TestOrganizationComponent_GetByUUID(t *testing.T) {
	dbOrg := &database.Organization{
		ID:       1,
		Nickname: "org1",
		Name:     "org_path",
		Homepage: "https://org1.com",
		Logo:     "https://org1.com/logo.png",
		OrgType:  "company",
		Verified: true,
		Namespace: &database.Namespace{
			ID:            1,
			Path:          "org_path",
			NamespaceType: database.OrgNamespace,
			UUID:          "ns-uuid-1",
		},
	}
	mockOrgStore := mockdb.NewMockOrgStore(t)
	mockOrgStore.EXPECT().FindByUUID(mock.Anything, "org-uuid-123").Return(dbOrg, nil)
	mockOrgStore.EXPECT().GetOrganizationTags(mock.Anything, dbOrg.ID).Return([]database.Tag{}, nil)

	c := &organizationComponentImpl{
		orgStore: mockOrgStore,
	}
	org, err := c.GetByUUID(context.Background(), "org-uuid-123")
	require.NoError(t, err)
	require.Equal(t, "org_path", org.Name)
	require.Equal(t, "org1", org.Nickname)
	require.Equal(t, "https://org1.com", org.Homepage)
	require.NotNil(t, org.Namespace)
	require.Equal(t, "org_path", org.Namespace.Path)
}

// deletionTestAuthorizer models missing tuples and a transient cleanup failure.
func deletionTestAuthorizer(t *testing.T, failCleanup *bool) *mockrebac.MockAuthorizer {
	t.Helper()
	authorizer := mockrebac.NewMockAuthorizer(t)
	authorizer.EXPECT().Check(mock.Anything, mock.MatchedBy(func(req rebac.CheckRequest) bool {
		return req.Relation == rebac.NamespaceCanAdmin || req.Relation == rebac.OrganizationCanAdmin
	})).Return(rebac.Decision{Allowed: true}, nil).Once()
	authorizer.EXPECT().Check(mock.Anything, mock.MatchedBy(func(req rebac.CheckRequest) bool {
		return req.Relation != rebac.NamespaceCanAdmin && req.Relation != rebac.OrganizationCanAdmin
	})).Return(rebac.Decision{Allowed: false}, nil).Maybe()
	authorizer.EXPECT().BatchCheck(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, req rebac.BatchCheckRequest) (rebac.BatchCheckResult, error) {
		if *failCleanup {
			return rebac.BatchCheckResult{}, errors.New("ReBAC unavailable")
		}
		result := rebac.BatchCheckResult{Results: make(map[string]rebac.BatchCheckOutcome)}
		for _, check := range req.Checks {
			result.Results[check.CorrelationID] = rebac.BatchCheckOutcome{Decision: rebac.Decision{Allowed: false}}
		}
		return result, nil
	})
	return authorizer
}

// TestOrganizationComponent_DeleteDefersFailedCleanup uses real tombstones and
// verifies repeated requests do not become the cleanup retry mechanism.
func TestOrganizationComponent_DeleteDefersFailedCleanup(t *testing.T) {
	for _, failure := range []string{"SSO", "ReBAC"} {
		t.Run(failure, func(t *testing.T) {
			db := tests.InitTestDB()
			defer db.Close()
			previous := database.GetDB()
			database.SetDB(db)
			defer database.SetDB(previous)
			ctx := context.Background()
			actor := &database.User{Username: "retry-admin", UUID: uuid.NewString()}
			outsider := &database.User{Username: "outsider", UUID: uuid.NewString()}
			_, err := db.Core.NewInsert().Model(actor).Exec(ctx)
			require.NoError(t, err)
			_, err = db.Core.NewInsert().Model(outsider).Exec(ctx)
			require.NoError(t, err)
			store := database.NewOrgStore(false, nil)
			org := &database.Organization{Name: "retry-org", UUID: uuid.New(), UserID: actor.ID}
			ns := &database.Namespace{Path: org.Name, UUID: org.UUID.String()}
			require.NoError(t, store.CreateWithRelations(ctx, org, ns, nil))
			failReBAC := failure == "ReBAC"
			authorizer := deletionTestAuthorizer(t, &failReBAC)
			sso := mockrpc.NewMockSSOInterface(t)
			if failure == "SSO" {
				sso.EXPECT().DeleteUser(mock.Anything, org.UUID.String()).Return(errors.New("SSO unavailable")).Once()
			} else {
				sso.EXPECT().DeleteUser(mock.Anything, org.UUID.String()).Return(nil).Once()
			}
			component := &organizationComponentImpl{orgStore: store, nsStore: database.NewNamespaceStoreWithDB(db), userStore: database.NewUserStoreWithDB(db), rebac: authorizer, sso: sso}
			req := &types.DeleteOrgReq{Name: org.Name, CurrentUser: actor.Username}
			require.NoError(t, component.Delete(ctx, req))
			retained, err := store.FindForDeletion(ctx, database.OrganizationDeletionLookup{Path: org.Name})
			require.NoError(t, err)
			require.False(t, retained.DeletedAt.IsZero())
			require.False(t, retained.Namespace.DeletedAt.IsZero())
			require.NoError(t, component.Delete(ctx, &types.DeleteOrgReq{Name: org.Name, CurrentUser: outsider.Username}))
			failReBAC = false
			require.NoError(t, component.Delete(ctx, req))
		})
	}
}

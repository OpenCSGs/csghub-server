package component

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockrebac "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rebac"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// expectNamespaceReBACWrite verifies one missing namespace tuple is checked and written.
func expectNamespaceReBACWrite(authorizer *mockrebac.MockAuthorizer, relation rebac.Relation, count int) {
	authorizer.EXPECT().Check(mock.Anything, mock.MatchedBy(func(request rebac.CheckRequest) bool {
		return request.Relation == relation && request.Object.Type == rebac.ObjectTypeNamespace &&
			request.Consistency == rebac.ConsistencyHigher
	})).Return(rebac.Decision{Allowed: false}, nil).Times(count)
	authorizer.EXPECT().Write(mock.Anything, mock.MatchedBy(func(relationships []rebac.Relationship) bool {
		return len(relationships) == 1 && relationships[0].Relation == relation &&
			relationships[0].Object.Type == rebac.ObjectTypeNamespace
	})).Return(nil).Times(count)
}

// TestEnsureNamespaceRelationshipWritesUserOwner verifies personal namespaces receive an owner tuple.
func TestEnsureNamespaceRelationshipWritesUserOwner(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationship := rebac.Relationship{
		Subject:  rebac.UserSubject("user-uuid"),
		Relation: rebac.RelationOwner,
		Object:   rebac.NamespaceObject("namespace-uuid"),
	}
	authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject: relationship.Subject, Relation: relationship.Relation, Object: relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: false}, nil).Once()
	authorizer.EXPECT().Write(ctx, []rebac.Relationship{relationship}).Return(nil).Once()

	err := ensureNamespaceRelationship(ctx, authorizer, database.Namespace{
		Path: "alice", UUID: "namespace-uuid", NamespaceType: database.UserNamespace,
	}, "user-uuid")
	require.NoError(t, err)
}

// TestEnsureNamespaceRelationshipWritesOrganization verifies organization namespaces receive an organization tuple.
func TestEnsureNamespaceRelationshipWritesOrganization(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationship := rebac.Relationship{
		Subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, "organization-uuid"),
		Relation: rebac.RelationOrganization,
		Object:   rebac.NamespaceObject("namespace-uuid"),
	}
	authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject: relationship.Subject, Relation: relationship.Relation, Object: relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: false}, nil).Once()
	authorizer.EXPECT().Write(ctx, []rebac.Relationship{relationship}).Return(nil).Once()

	err := ensureNamespaceRelationship(ctx, authorizer, database.Namespace{
		Path: "acme", UUID: "namespace-uuid", NamespaceType: database.OrgNamespace,
	}, "organization-uuid")
	require.NoError(t, err)
}

// TestEnsureUserObjectOwnerRelationshipWritesOwner verifies a user object receives its owner tuple.
func TestEnsureUserObjectOwnerRelationshipWritesOwner(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationship := rebac.Relationship{
		Subject: rebac.UserSubject("user-uuid"), Relation: rebac.RelationOwner, Object: rebac.UserObject("user-uuid"),
	}
	authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject: relationship.Subject, Relation: relationship.Relation, Object: relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: false}, nil).Once()
	authorizer.EXPECT().Write(ctx, []rebac.Relationship{relationship}).Return(nil).Once()

	require.NoError(t, ensureUserObjectOwnerRelationship(ctx, authorizer, "user-uuid"))
}

// TestDeleteUserObjectOwnerRelationshipDeletesOwner verifies a user object owner tuple is removed.
func TestDeleteUserObjectOwnerRelationshipDeletesOwner(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationship := rebac.Relationship{
		Subject: rebac.UserSubject("user-uuid"), Relation: rebac.RelationOwner, Object: rebac.UserObject("user-uuid"),
	}
	authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject: relationship.Subject, Relation: relationship.Relation, Object: relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: true}, nil).Once()
	authorizer.EXPECT().Delete(ctx, []rebac.Relationship{relationship}).Return(nil).Once()

	require.NoError(t, deleteUserObjectOwnerRelationship(ctx, authorizer, relationship))
}

// TestLoadUserRepositoryRelationshipsResolvesPersonalAndOrganizationRepositories verifies bulk deletion preserves namespace ownership semantics.
func TestLoadUserRepositoryRelationshipsResolvesPersonalAndOrganizationRepositories(t *testing.T) {
	ctx := context.Background()
	repoStore := mockdatabase.NewMockRepoStore(t)
	namespaceStore := mockdatabase.NewMockNamespaceStore(t)
	organizationStore := mockdatabase.NewMockOrgStore(t)
	user := database.User{ID: 7, Username: "alice", UUID: "alice-uuid"}
	organizationUUID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	repositories := []database.Repository{
		{ID: 41, Path: "alice/personal"},
		{ID: 42, Path: "acme/shared"},
	}

	repoStore.EXPECT().ByUser(ctx, user.ID, 1000, 0).Return(repositories, nil).Once()
	repoStore.EXPECT().ByUser(ctx, user.ID, 1000, 1).Return([]database.Repository{}, nil).Once()
	namespaceStore.EXPECT().FindByPath(ctx, "alice").Return(database.Namespace{
		Path: "alice", NamespaceType: database.UserNamespace, User: database.User{UUID: user.UUID},
	}, nil).Once()
	namespaceStore.EXPECT().FindByPath(ctx, "acme").Return(database.Namespace{
		Path: "acme", NamespaceType: database.OrgNamespace,
	}, nil).Once()
	organizationStore.EXPECT().FindByPath(ctx, "acme").Return(database.Organization{UUID: organizationUUID}, nil).Once()

	gotRepositories, relationships, err := loadUserRepositoryRelationships(
		ctx, repoStore, namespaceStore, organizationStore, user,
	)
	require.NoError(t, err)
	require.Equal(t, repositories, gotRepositories)
	require.Equal(t, []rebac.Relationship{
		{Subject: rebac.UserSubject(user.UUID), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(41)},
		{
			Subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, organizationUUID.String()),
			Relation: rebac.RelationOrganization,
			Object:   rebac.RepositoryObject(42),
		},
	}, relationships)
}

// TestLoadUserRepositoryRelationshipsResolvesDeletedPersonalNamespace verifies retained repositories can be cleaned after account soft deletion.
func TestLoadUserRepositoryRelationshipsResolvesDeletedPersonalNamespace(t *testing.T) {
	ctx := context.Background()
	repoStore := mockdatabase.NewMockRepoStore(t)
	namespaceStore := mockdatabase.NewMockNamespaceStore(t)
	user := database.User{ID: 7, Username: "alice", UUID: "alice-uuid"}
	repository := database.Repository{ID: 41, Path: "alice/personal"}

	repoStore.EXPECT().ByUser(ctx, user.ID, 1000, 0).Return([]database.Repository{repository}, nil).Once()
	repoStore.EXPECT().ByUser(ctx, user.ID, 1000, 1).Return([]database.Repository{}, nil).Once()
	namespaceStore.EXPECT().FindByPath(ctx, "alice").Return(database.Namespace{}, errors.New("namespace deleted")).Once()

	_, relationships, err := loadUserRepositoryRelationships(ctx, repoStore, namespaceStore, nil, user)
	require.NoError(t, err)
	require.Equal(t, []rebac.Relationship{{
		Subject: rebac.UserSubject(user.UUID), Relation: rebac.RelationOwner, Object: rebac.RepositoryObject(repository.ID),
	}}, relationships)
}

// TestDeleteUserNamespaceRelationshipRemovesOwner verifies user deletion removes the personal namespace owner tuple.
func TestDeleteUserNamespaceRelationshipRemovesOwner(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationship := rebac.Relationship{
		Subject:  rebac.UserSubject("user-uuid"),
		Relation: rebac.RelationOwner,
		Object:   rebac.NamespaceObject("namespace-uuid"),
	}
	correlationID := rebac.BatchCheckCorrelationID(0)
	authorizer.EXPECT().BatchCheck(ctx, rebac.BatchCheckRequest{Checks: []rebac.BatchCheckItem{{
		CorrelationID: correlationID,
		Check: rebac.CheckRequest{
			Subject: relationship.Subject, Relation: relationship.Relation, Object: relationship.Object,
			Consistency: rebac.ConsistencyHigher,
		},
	}}}).Return(rebac.BatchCheckResult{Results: map[string]rebac.BatchCheckOutcome{
		correlationID: {Decision: rebac.Decision{Allowed: true}},
	}}, nil).Once()
	authorizer.EXPECT().Delete(ctx, []rebac.Relationship{relationship}).Return(nil).Once()

	err := deleteNamespaceRelationship(ctx, authorizer, relationship)
	require.NoError(t, err)
}

// TestDeleteUserNamespaceRelationshipSkipsMissingTuple verifies repeated user deletion is idempotent.
func TestDeleteUserNamespaceRelationshipSkipsMissingTuple(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationship := rebac.Relationship{
		Subject:  rebac.UserSubject("user-uuid"),
		Relation: rebac.RelationOwner,
		Object:   rebac.NamespaceObject("namespace-uuid"),
	}
	correlationID := rebac.BatchCheckCorrelationID(0)
	authorizer.EXPECT().BatchCheck(ctx, rebac.BatchCheckRequest{Checks: []rebac.BatchCheckItem{{
		CorrelationID: correlationID,
		Check: rebac.CheckRequest{
			Subject: relationship.Subject, Relation: relationship.Relation, Object: relationship.Object,
			Consistency: rebac.ConsistencyHigher,
		},
	}}}).Return(rebac.BatchCheckResult{Results: map[string]rebac.BatchCheckOutcome{
		correlationID: {Decision: rebac.Decision{}},
	}}, nil).Once()

	err := deleteNamespaceRelationship(ctx, authorizer, relationship)
	require.NoError(t, err)
}

// expectOrganizationReBACCleanup verifies direct member roles and the namespace tuple are deleted.
func expectOrganizationReBACCleanup(
	t *testing.T,
	authorizer *mockrebac.MockAuthorizer,
	cleanup types.OrganizationReBACCleanup,
) {
	expectOrganizationReBACCleanups(t, authorizer, []types.OrganizationReBACCleanup{cleanup})
}

// expectOrganizationReBACCleanups verifies member cleanup and batched namespace tuple deletion.
func expectOrganizationReBACCleanups(
	t *testing.T,
	authorizer *mockrebac.MockAuthorizer,
	cleanups []types.OrganizationReBACCleanup,
) {
	t.Helper()
	namespaceRelationships := make([]rebac.Relationship, 0, len(cleanups))
	for _, cleanup := range cleanups {
		checks := make([]rebac.BatchCheckItem, 0, len(cleanup.UserUUIDs)*len(organizationMemberRelations))
		results := make(map[string]rebac.BatchCheckOutcome, cap(checks))
		memberRelationships := make([]rebac.Relationship, 0, cap(checks))
		for userIndex, userUUID := range cleanup.UserUUIDs {
			for relationIndex, relation := range organizationMemberRelations {
				correlationID := organizationMemberCorrelationID(userIndex, relationIndex)
				checks = append(checks, rebac.BatchCheckItem{
					CorrelationID: correlationID,
					Check: rebac.CheckRequest{
						Subject: rebac.UserSubject(userUUID), Relation: relation,
						Object: rebac.OrganizationObject(cleanup.OrganizationUUID), Consistency: rebac.ConsistencyHigher,
					},
				})
				results[correlationID] = rebac.BatchCheckOutcome{Decision: rebac.Decision{Allowed: true}}
				memberRelationships = append(memberRelationships, rebac.Relationship{
					Subject: rebac.UserSubject(userUUID), Relation: relation,
					Object: rebac.OrganizationObject(cleanup.OrganizationUUID),
				})
			}
		}
		if len(checks) > 0 {
			authorizer.EXPECT().BatchCheck(mock.Anything, rebac.BatchCheckRequest{Checks: checks}).
				Return(rebac.BatchCheckResult{Results: results}, nil).Once()
			authorizer.EXPECT().Delete(mock.Anything, memberRelationships).Return(nil).Once()
		}
		if cleanup.NamespaceUUID != "" {
			namespaceRelationships = append(namespaceRelationships, rebac.Relationship{
				Subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, cleanup.OrganizationUUID),
				Relation: rebac.RelationOrganization, Object: rebac.NamespaceObject(cleanup.NamespaceUUID),
			})
		}
	}
	if len(namespaceRelationships) == 0 {
		return
	}
	checks := make([]rebac.BatchCheckItem, 0, len(namespaceRelationships))
	results := make(map[string]rebac.BatchCheckOutcome, len(namespaceRelationships))
	for index, relationship := range namespaceRelationships {
		correlationID := rebac.BatchCheckCorrelationID(index)
		checks = append(checks, rebac.BatchCheckItem{
			CorrelationID: correlationID,
			Check: rebac.CheckRequest{
				Subject: relationship.Subject, Relation: relationship.Relation, Object: relationship.Object,
				Consistency: rebac.ConsistencyHigher,
			},
		})
		results[correlationID] = rebac.BatchCheckOutcome{Decision: rebac.Decision{Allowed: true}}
	}
	authorizer.EXPECT().BatchCheck(mock.Anything, rebac.BatchCheckRequest{Checks: checks}).
		Return(rebac.BatchCheckResult{Results: results}, nil).Once()
	authorizer.EXPECT().Delete(mock.Anything, namespaceRelationships).Return(nil).Once()
}

// TestDeleteOrganizationReBACRelationshipsRemovesMembersAndNamespace verifies organization deletion removes all direct tuples.
func TestDeleteOrganizationReBACRelationshipsRemovesMembersAndNamespace(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	cleanup := types.OrganizationReBACCleanup{
		OrganizationUUID: "organization-uuid",
		NamespaceUUID:    "namespace-uuid",
		UserUUIDs:        []string{"user-uuid"},
	}
	expectOrganizationReBACCleanup(t, authorizer, cleanup)

	err := deleteOrganizationReBACRelationships(ctx, authorizer, []types.OrganizationReBACCleanup{cleanup})
	require.NoError(t, err)
}

// TestReconcileOrganizationMemberRelationshipsSplitsLargeBatches verifies OpenFGA's batch limit is respected.
func TestReconcileOrganizationMemberRelationshipsSplitsLargeBatches(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	userUUIDs := make([]string, 17)
	for index := range userUUIDs {
		userUUIDs[index] = fmt.Sprintf("user-%02d", index)
	}
	batchSizes := make([]int, 0, 2)
	authorizer.EXPECT().BatchCheck(ctx, mock.Anything).RunAndReturn(func(_ context.Context, request rebac.BatchCheckRequest) (rebac.BatchCheckResult, error) {
		batchSizes = append(batchSizes, len(request.Checks))
		results := make(map[string]rebac.BatchCheckOutcome, len(request.Checks))
		for _, check := range request.Checks {
			results[check.CorrelationID] = rebac.BatchCheckOutcome{}
		}
		return rebac.BatchCheckResult{Results: results}, nil
	}).Twice()

	err := reconcileOrganizationMemberRelationships(ctx, authorizer, "organization-uuid", userUUIDs, nil)
	require.NoError(t, err)
	require.Equal(t, []int{48, 3}, batchSizes)
}

// TestDeleteNamespaceRelationshipsSplitsLargeBatches verifies the OpenFGA batch limit for namespace cleanup.
func TestDeleteNamespaceRelationshipsSplitsLargeBatches(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationships := make([]rebac.Relationship, 51)
	for index := range relationships {
		relationships[index] = rebac.Relationship{
			Subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, fmt.Sprintf("org-%03d", index)),
			Relation: rebac.RelationOrganization,
			Object:   rebac.NamespaceObject(fmt.Sprintf("namespace-%03d", index)),
		}
	}
	batchSizes := make([]int, 0, 2)
	authorizer.EXPECT().BatchCheck(ctx, mock.Anything).RunAndReturn(func(_ context.Context, request rebac.BatchCheckRequest) (rebac.BatchCheckResult, error) {
		batchSizes = append(batchSizes, len(request.Checks))
		results := make(map[string]rebac.BatchCheckOutcome, len(request.Checks))
		for _, check := range request.Checks {
			results[check.CorrelationID] = rebac.BatchCheckOutcome{Decision: rebac.Decision{Allowed: true}}
		}
		return rebac.BatchCheckResult{Results: results}, nil
	}).Twice()
	authorizer.EXPECT().Delete(ctx, mock.MatchedBy(func(batch []rebac.Relationship) bool {
		return len(batch) == 50
	})).Return(nil).Once()
	authorizer.EXPECT().Delete(ctx, mock.MatchedBy(func(batch []rebac.Relationship) bool {
		return len(batch) == 1
	})).Return(nil).Once()

	require.NoError(t, deleteNamespaceRelationships(ctx, authorizer, relationships))
	require.Equal(t, []int{50, 1}, batchSizes)
}

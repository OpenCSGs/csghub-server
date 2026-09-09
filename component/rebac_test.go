package component

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	mockrebac "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rebac"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
)

// TestEnsureRepositoryNamespaceRelationshipWritesUserOwner verifies personal repositories receive an owner tuple.
func TestEnsureRepositoryNamespaceRelationshipWritesUserOwner(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationship := rebac.Relationship{
		Subject:  rebac.UserSubject("user-uuid"),
		Relation: rebac.RelationOwner,
		Object:   rebac.RepositoryObject(42),
	}
	authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject:     relationship.Subject,
		Relation:    relationship.Relation,
		Object:      relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: false}, nil).Once()
	authorizer.EXPECT().Write(ctx, []rebac.Relationship{relationship}).Return(nil).Once()

	err := ensureRepositoryNamespaceRelationship(ctx, authorizer, nil, database.Namespace{
		Path:          "alice",
		NamespaceType: database.UserNamespace,
		User:          database.User{UUID: "user-uuid"},
	}, 42)
	require.NoError(t, err)
}

// TestEnsureRepositoryNamespaceRelationshipWritesOrganization verifies organization repositories receive an organization tuple.
func TestEnsureRepositoryNamespaceRelationshipWritesOrganization(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	orgStore := mockdatabase.NewMockOrgStore(t)
	organizationUUID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgStore.EXPECT().FindByPath(ctx, "acme").Return(database.Organization{UUID: organizationUUID}, nil).Once()
	relationship := rebac.Relationship{
		Subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, organizationUUID.String()),
		Relation: rebac.RelationOrganization,
		Object:   rebac.RepositoryObject(84),
	}
	authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject:     relationship.Subject,
		Relation:    relationship.Relation,
		Object:      relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: false}, nil).Once()
	authorizer.EXPECT().Write(ctx, []rebac.Relationship{relationship}).Return(nil).Once()

	err := ensureRepositoryNamespaceRelationship(ctx, authorizer, orgStore, database.Namespace{
		Path:          "acme",
		NamespaceType: database.OrgNamespace,
	}, 84)
	require.NoError(t, err)
}

// TestEnsureRepositoryNamespaceRelationshipSkipsExistingTuple verifies retries do not write duplicate tuples.
func TestEnsureRepositoryNamespaceRelationshipSkipsExistingTuple(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationship := rebac.Relationship{
		Subject:  rebac.UserSubject("user-uuid"),
		Relation: rebac.RelationOwner,
		Object:   rebac.RepositoryObject(42),
	}
	authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject:     relationship.Subject,
		Relation:    relationship.Relation,
		Object:      relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: true}, nil).Once()

	err := ensureRepositoryNamespaceRelationship(ctx, authorizer, nil, database.Namespace{
		Path:          "alice",
		NamespaceType: database.UserNamespace,
		User:          database.User{UUID: "user-uuid"},
	}, 42)
	require.NoError(t, err)
}

// TestEnsureRepositoryNamespaceRelationshipReturnsReBACError verifies tuple write failures use the ReBAC business error.
func TestEnsureRepositoryNamespaceRelationshipReturnsReBACError(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	writeErr := errors.New("write failed")
	relationship := rebac.Relationship{
		Subject:  rebac.UserSubject("user-uuid"),
		Relation: rebac.RelationOwner,
		Object:   rebac.RepositoryObject(42),
	}
	authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject:     relationship.Subject,
		Relation:    relationship.Relation,
		Object:      relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: false}, nil).Once()
	authorizer.EXPECT().Write(ctx, []rebac.Relationship{relationship}).Return(writeErr).Once()

	err := ensureRepositoryNamespaceRelationship(ctx, authorizer, nil, database.Namespace{
		Path:          "alice",
		NamespaceType: database.UserNamespace,
		User:          database.User{UUID: "user-uuid"},
	}, 42)
	require.ErrorIs(t, err, errorx.ErrReBACNamespacePermissionCreateFailed)
	require.ErrorIs(t, err, writeErr)
}

// TestEnsureNamespaceRelationshipWritesUserOwner verifies synthetic user namespaces receive an owner tuple.
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

// TestDeleteRepositoryNamespaceRelationshipDeletesUserOwner verifies personal repository deletion removes its owner tuple.
func TestDeleteRepositoryNamespaceRelationshipDeletesUserOwner(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationship := rebac.Relationship{
		Subject:  rebac.UserSubject("user-uuid"),
		Relation: rebac.RelationOwner,
		Object:   rebac.RepositoryObject(42),
	}
	authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject:     relationship.Subject,
		Relation:    relationship.Relation,
		Object:      relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: true}, nil).Once()
	authorizer.EXPECT().Delete(ctx, []rebac.Relationship{relationship}).Return(nil).Once()

	err := deleteRepositoryNamespaceRelationship(ctx, authorizer, nil, database.Namespace{
		Path:          "alice",
		NamespaceType: database.UserNamespace,
		User:          database.User{UUID: "user-uuid"},
	}, 42)
	require.NoError(t, err)
}

// TestDeleteRepositoryNamespaceRelationshipDeletesOrganization verifies organization repository deletion removes its organization tuple.
func TestDeleteRepositoryNamespaceRelationshipDeletesOrganization(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	orgStore := mockdatabase.NewMockOrgStore(t)
	organizationUUID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	orgStore.EXPECT().FindByPath(ctx, "acme").Return(database.Organization{UUID: organizationUUID}, nil).Once()
	relationship := rebac.Relationship{
		Subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, organizationUUID.String()),
		Relation: rebac.RelationOrganization,
		Object:   rebac.RepositoryObject(84),
	}
	authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject:     relationship.Subject,
		Relation:    relationship.Relation,
		Object:      relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: true}, nil).Once()
	authorizer.EXPECT().Delete(ctx, []rebac.Relationship{relationship}).Return(nil).Once()

	err := deleteRepositoryNamespaceRelationship(ctx, authorizer, orgStore, database.Namespace{
		Path:          "acme",
		NamespaceType: database.OrgNamespace,
	}, 84)
	require.NoError(t, err)
}

// TestDeleteRepositoryNamespaceRelationshipSkipsMissingTuple verifies deletion retries converge when the tuple is already absent.
func TestDeleteRepositoryNamespaceRelationshipSkipsMissingTuple(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	relationship := rebac.Relationship{
		Subject:  rebac.UserSubject("user-uuid"),
		Relation: rebac.RelationOwner,
		Object:   rebac.RepositoryObject(42),
	}
	authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject:     relationship.Subject,
		Relation:    relationship.Relation,
		Object:      relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: false}, nil).Once()

	err := deleteRepositoryNamespaceRelationship(ctx, authorizer, nil, database.Namespace{
		Path:          "alice",
		NamespaceType: database.UserNamespace,
		User:          database.User{UUID: "user-uuid"},
	}, 42)
	require.NoError(t, err)
}

// TestDeleteRepositoryNamespaceRelationshipReturnsReBACError verifies tuple deletion failures use the ReBAC business error.
func TestDeleteRepositoryNamespaceRelationshipReturnsReBACError(t *testing.T) {
	ctx := context.Background()
	authorizer := mockrebac.NewMockAuthorizer(t)
	deleteErr := errors.New("delete failed")
	relationship := rebac.Relationship{
		Subject:  rebac.UserSubject("user-uuid"),
		Relation: rebac.RelationOwner,
		Object:   rebac.RepositoryObject(42),
	}
	authorizer.EXPECT().Check(ctx, rebac.CheckRequest{
		Subject:     relationship.Subject,
		Relation:    relationship.Relation,
		Object:      relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	}).Return(rebac.Decision{Allowed: true}, nil).Once()
	authorizer.EXPECT().Delete(ctx, []rebac.Relationship{relationship}).Return(deleteErr).Once()

	err := deleteRepositoryNamespaceRelationship(ctx, authorizer, nil, database.Namespace{
		Path:          "alice",
		NamespaceType: database.UserNamespace,
		User:          database.User{UUID: "user-uuid"},
	}, 42)
	require.ErrorIs(t, err, errorx.ErrReBACNamespacePermissionDeleteFailed)
	require.ErrorIs(t, err, deleteErr)
}

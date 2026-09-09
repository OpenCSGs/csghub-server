package component

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
)

// ensureNamespaceRelationship reconciles the direct owner of a user or organization namespace.
func ensureNamespaceRelationship(ctx context.Context, authorizer rebac.Authorizer, namespace database.Namespace, subjectUUID string) error {
	if authorizer == nil {
		return fmt.Errorf("namespace ReBAC authorizer is required")
	}
	relationship, err := namespaceRelationship(namespace, subjectUUID)
	if err != nil {
		return err
	}
	decision, err := authorizer.Check(ctx, rebac.CheckRequest{
		Subject:     relationship.Subject,
		Relation:    relationship.Relation,
		Object:      relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	})
	if err != nil {
		return fmt.Errorf("failed to check namespace relationship: %w", err)
	}
	if decision.Allowed {
		return nil
	}
	if err := authorizer.Write(ctx, []rebac.Relationship{relationship}); err != nil {
		return errorx.ReBACNamespacePermissionCreateFailed(err, errorx.Ctx().
			Set("namespace", namespace.Path).
			Set("namespace_uuid", namespace.UUID))
	}
	return nil
}

// namespaceRelationship builds the direct tuple between a namespace and its user or organization subject.
func namespaceRelationship(namespace database.Namespace, subjectUUID string) (rebac.Relationship, error) {
	if namespace.UUID == "" {
		return rebac.Relationship{}, fmt.Errorf("namespace %q has no UUID", namespace.Path)
	}
	if subjectUUID == "" {
		return rebac.Relationship{}, fmt.Errorf("namespace %q subject has no UUID", namespace.Path)
	}

	var subject rebac.Subject
	var relation rebac.Relation
	switch namespace.NamespaceType {
	case database.UserNamespace:
		subject = rebac.UserSubject(subjectUUID)
		relation = rebac.RelationOwner
	case database.OrgNamespace:
		subject = rebac.NewSubject(rebac.ObjectTypeOrganization, subjectUUID)
		relation = rebac.RelationOrganization
	default:
		return rebac.Relationship{}, fmt.Errorf("unsupported namespace type %q for namespace %q", namespace.NamespaceType, namespace.Path)
	}

	return rebac.Relationship{
		Subject:  subject,
		Relation: relation,
		Object:   rebac.NamespaceObject(namespace.UUID),
	}, nil
}

// ensureUserObjectOwnerRelationship reconciles the owner tuple for a user object.
// The tuple is separate from the owner tuple of the user's namespace.
func ensureUserObjectOwnerRelationship(ctx context.Context, authorizer rebac.Authorizer, userUUID string) error {
	if authorizer == nil {
		return fmt.Errorf("user object ReBAC authorizer is required")
	}
	if userUUID == "" {
		return fmt.Errorf("user UUID is required")
	}

	relationship := rebac.Relationship{
		Subject:  rebac.UserSubject(userUUID),
		Relation: rebac.RelationOwner,
		Object:   rebac.UserObject(userUUID),
	}
	decision, err := authorizer.Check(ctx, rebac.CheckRequest{
		Subject:     relationship.Subject,
		Relation:    relationship.Relation,
		Object:      relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	})
	if err != nil {
		return fmt.Errorf("failed to check user object owner relationship: %w", err)
	}
	if decision.Allowed {
		return nil
	}
	if err := authorizer.Write(ctx, []rebac.Relationship{relationship}); err != nil {
		return fmt.Errorf("failed to write user object owner relationship: %w", err)
	}
	return nil
}

// ensureRepositoryNamespaceRelationship reconciles the direct relationship between a repository and its namespace owner.
// User namespaces write repository#owner@user, while organization namespaces write repository#organization@organization.
func ensureRepositoryNamespaceRelationship(ctx context.Context, authorizer rebac.Authorizer, orgStore database.OrgStore, namespace database.Namespace, repositoryID int64) error {
	if authorizer == nil {
		return fmt.Errorf("repository ReBAC authorizer is required")
	}

	relationship, err := repositoryNamespaceRelationship(ctx, orgStore, namespace, repositoryID)
	if err != nil {
		return err
	}

	decision, err := authorizer.Check(ctx, rebac.CheckRequest{
		Subject:     relationship.Subject,
		Relation:    relationship.Relation,
		Object:      relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	})
	if err != nil {
		return fmt.Errorf("failed to check repository namespace relationship: %w", err)
	}
	if decision.Allowed {
		return nil
	}

	if err := authorizer.Write(ctx, []rebac.Relationship{relationship}); err != nil {
		return errorx.ReBACNamespacePermissionCreateFailed(err, errorx.Ctx().
			Set("namespace", namespace.Path).
			Set("repository_id", repositoryID))
	}
	return nil
}

// deleteRepositoryNamespaceRelationship removes the direct relationship between a repository and its namespace owner.
func deleteRepositoryNamespaceRelationship(ctx context.Context, authorizer rebac.Authorizer, orgStore database.OrgStore, namespace database.Namespace, repositoryID int64) error {
	if authorizer == nil {
		return fmt.Errorf("repository ReBAC authorizer is required")
	}

	relationship, err := repositoryNamespaceRelationship(ctx, orgStore, namespace, repositoryID)
	if err != nil {
		return err
	}

	decision, err := authorizer.Check(ctx, rebac.CheckRequest{
		Subject:     relationship.Subject,
		Relation:    relationship.Relation,
		Object:      relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	})
	if err != nil {
		return fmt.Errorf("failed to check repository namespace relationship before deletion: %w", err)
	}
	if !decision.Allowed {
		return nil
	}

	if err := authorizer.Delete(ctx, []rebac.Relationship{relationship}); err != nil {
		return errorx.ReBACNamespacePermissionDeleteFailed(err, errorx.Ctx().
			Set("namespace", namespace.Path).
			Set("repository_id", repositoryID))
	}
	return nil
}

// repositoryNamespaceRelationship builds the direct tuple required by one repository namespace.
func repositoryNamespaceRelationship(ctx context.Context, orgStore database.OrgStore, namespace database.Namespace, repositoryID int64) (rebac.Relationship, error) {
	object := rebac.RepositoryObject(repositoryID)
	if object.ID == "" {
		return rebac.Relationship{}, fmt.Errorf("repository ID must be positive")
	}

	switch namespace.NamespaceType {
	case database.UserNamespace:
		if namespace.User.UUID == "" {
			return rebac.Relationship{}, fmt.Errorf("user namespace %q has no user UUID", namespace.Path)
		}
		return rebac.Relationship{
			Subject:  rebac.UserSubject(namespace.User.UUID),
			Relation: rebac.RelationOwner,
			Object:   object,
		}, nil
	case database.OrgNamespace:
		if orgStore == nil {
			return rebac.Relationship{}, fmt.Errorf("organization store is required for namespace %q", namespace.Path)
		}
		organization, err := orgStore.FindByPath(ctx, namespace.Path)
		if err != nil {
			return rebac.Relationship{}, fmt.Errorf("failed to find organization for namespace %q: %w", namespace.Path, err)
		}
		if organization.UUID == uuid.Nil {
			return rebac.Relationship{}, fmt.Errorf("organization namespace %q has no organization UUID", namespace.Path)
		}
		return rebac.Relationship{
			Subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, organization.UUID.String()),
			Relation: rebac.RelationOrganization,
			Object:   object,
		}, nil
	default:
		return rebac.Relationship{}, fmt.Errorf("unsupported namespace type %q for namespace %q", namespace.NamespaceType, namespace.Path)
	}
}

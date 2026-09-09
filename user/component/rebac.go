package component

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

var organizationMemberRelations = []rebac.Relation{
	rebac.RelationReader,
	rebac.RelationWriter,
	rebac.RelationAdmin,
}

// ensureNamespaceRelationship reconciles the direct owner of a user or organization namespace.
func ensureNamespaceRelationship(ctx context.Context, authorizer rebac.Authorizer, namespace database.Namespace, subjectUUID string) error {
	if authorizer == nil {
		return fmt.Errorf("ReBAC authorizer is nil")
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
		return fmt.Errorf("check namespace ReBAC relationship: %w", err)
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

// userObjectOwnerRelationship builds the direct owner tuple for a user object.
func userObjectOwnerRelationship(userUUID string) (rebac.Relationship, error) {
	if userUUID == "" {
		return rebac.Relationship{}, fmt.Errorf("user UUID is required")
	}
	return rebac.Relationship{
		Subject:  rebac.UserSubject(userUUID),
		Relation: rebac.RelationOwner,
		Object:   rebac.UserObject(userUUID),
	}, nil
}

// ensureUserObjectOwnerRelationship creates the owner tuple for a user object when missing.
func ensureUserObjectOwnerRelationship(ctx context.Context, authorizer rebac.Authorizer, userUUID string) error {
	if authorizer == nil {
		return fmt.Errorf("ReBAC authorizer is nil")
	}
	relationship, err := userObjectOwnerRelationship(userUUID)
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
		return fmt.Errorf("check user owner relationship: %w", err)
	}
	if decision.Allowed {
		return nil
	}
	if err := authorizer.Write(ctx, []rebac.Relationship{relationship}); err != nil {
		return fmt.Errorf("write user owner relationship: %w", err)
	}
	return nil
}

// deleteUserObjectOwnerRelationship removes the owner tuple for a user object when present.
func deleteUserObjectOwnerRelationship(ctx context.Context, authorizer rebac.Authorizer, relationship rebac.Relationship) error {
	if authorizer == nil {
		return fmt.Errorf("ReBAC authorizer is nil")
	}
	decision, err := authorizer.Check(ctx, rebac.CheckRequest{
		Subject:     relationship.Subject,
		Relation:    relationship.Relation,
		Object:      relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	})
	if err != nil {
		return fmt.Errorf("check user owner relationship before deletion: %w", err)
	}
	if !decision.Allowed {
		return nil
	}
	if err := authorizer.Delete(ctx, []rebac.Relationship{relationship}); err != nil {
		return fmt.Errorf("delete user owner relationship: %w", err)
	}
	return nil
}

// loadUserNamespaceRelationship loads the owner tuple for a user's namespace, including a soft-deleted namespace.
func loadUserNamespaceRelationship(
	ctx context.Context,
	namespaceStore database.NamespaceStore,
	user database.User,
) (rebac.Relationship, error) {
	if namespaceStore == nil {
		return rebac.Relationship{}, fmt.Errorf("namespace store is required")
	}
	namespace, err := namespaceStore.FindByPathWithDeleted(ctx, user.Username)
	if err != nil {
		return rebac.Relationship{}, fmt.Errorf("find user namespace %q including deleted records: %w", user.Username, err)
	}
	if namespace.NamespaceType != database.UserNamespace {
		return rebac.Relationship{}, fmt.Errorf("namespace %q is not a user namespace", namespace.Path)
	}
	return namespaceRelationship(namespace, user.UUID)
}

// deleteNamespaceRelationship removes an existing direct namespace tuple.
func deleteNamespaceRelationship(ctx context.Context, authorizer rebac.Authorizer, relationship rebac.Relationship) error {
	return deleteNamespaceRelationships(ctx, authorizer, []rebac.Relationship{relationship})
}

// deleteNamespaceRelationships removes existing namespace tuples in bounded batches.
// Missing tuples are skipped so they do not prevent cleanup of remaining namespaces.
func deleteNamespaceRelationships(ctx context.Context, authorizer rebac.Authorizer, relationships []rebac.Relationship) error {
	if len(relationships) == 0 {
		return nil
	}
	if authorizer == nil {
		return fmt.Errorf("ReBAC authorizer is nil")
	}
	for start := 0; start < len(relationships); start += rebac.DefaultMaxBatchSize {
		end := min(start+rebac.DefaultMaxBatchSize, len(relationships))
		batch := relationships[start:end]
		checks := make([]rebac.BatchCheckItem, 0, len(batch))
		for index, relationship := range batch {
			checks = append(checks, rebac.BatchCheckItem{
				CorrelationID: rebac.BatchCheckCorrelationID(index),
				Check: rebac.CheckRequest{
					Subject:     relationship.Subject,
					Relation:    relationship.Relation,
					Object:      relationship.Object,
					Consistency: rebac.ConsistencyHigher,
				},
			})
		}

		result, err := authorizer.BatchCheck(ctx, rebac.BatchCheckRequest{Checks: checks})
		if err != nil {
			return fmt.Errorf("check namespace ReBAC relationships before deletion: %w", err)
		}

		deletes := make([]rebac.Relationship, 0, len(batch))
		for index, relationship := range batch {
			correlationID := rebac.BatchCheckCorrelationID(index)
			outcome, exists := result.Results[correlationID]
			if !exists {
				return fmt.Errorf("missing namespace ReBAC batch result %q for relationship %q", correlationID, relationship.String())
			}
			if outcome.Err != nil {
				return fmt.Errorf("check namespace ReBAC relationship %q with correlation ID %q: %w", relationship.String(), correlationID, outcome.Err)
			}
			if outcome.Decision.Allowed {
				deletes = append(deletes, relationship)
			}
		}
		if len(deletes) == 0 {
			continue
		}
		if err := authorizer.Delete(ctx, deletes); err != nil {
			return errorx.ReBACNamespacePermissionDeleteFailed(err, errorx.Ctx().
				Set("namespace_count", len(deletes)))
		}
	}
	return nil
}

// deleteOrganizationReBACRelationships removes direct member roles and the organization namespace tuple.
func deleteOrganizationReBACRelationships(
	ctx context.Context,
	authorizer rebac.Authorizer,
	cleanups []types.OrganizationReBACCleanup,
) error {
	namespaceRelationships := make([]rebac.Relationship, 0, len(cleanups))
	for _, cleanup := range cleanups {
		if cleanup.OrganizationUUID == "" {
			return fmt.Errorf("organization UUID is required for ReBAC cleanup")
		}
		if err := reconcileOrganizationMemberRelationships(
			ctx, authorizer, cleanup.OrganizationUUID, cleanup.UserUUIDs, nil,
		); err != nil {
			return fmt.Errorf("delete organization %s member relationships: %w", cleanup.OrganizationUUID, err)
		}
		if cleanup.NamespaceUUID == "" {
			continue
		}
		relationship, err := namespaceRelationship(database.Namespace{
			UUID:          cleanup.NamespaceUUID,
			NamespaceType: database.OrgNamespace,
		}, cleanup.OrganizationUUID)
		if err != nil {
			return err
		}
		namespaceRelationships = append(namespaceRelationships, relationship)
	}
	if err := deleteNamespaceRelationships(ctx, authorizer, namespaceRelationships); err != nil {
		return fmt.Errorf("delete organization namespace relationships: %w", err)
	}
	return nil
}

// loadUserOrganizationReBACCleanups loads the organizations whose direct member tuples belong to a user.
func loadUserOrganizationReBACCleanups(
	ctx context.Context,
	orgStore database.OrgStore,
	user database.User,
) ([]types.OrganizationReBACCleanup, error) {
	if orgStore == nil {
		return nil, fmt.Errorf("organization store is required")
	}
	if user.UUID == "" {
		return nil, fmt.Errorf("user UUID is required for ReBAC cleanup")
	}

	organizations, err := orgStore.GetUserBelongOrgs(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("load organizations for user %q: %w", user.UUID, err)
	}

	cleanups := make([]types.OrganizationReBACCleanup, 0, len(organizations))
	seen := make(map[string]struct{}, len(organizations))
	for _, organization := range organizations {
		if organization.UUID == uuid.Nil {
			return nil, fmt.Errorf("organization %d has no UUID for ReBAC cleanup", organization.ID)
		}
		organizationUUID := organization.UUID.String()
		if _, exists := seen[organizationUUID]; exists {
			continue
		}
		seen[organizationUUID] = struct{}{}
		cleanups = append(cleanups, types.OrganizationReBACCleanup{
			OrganizationUUID: organizationUUID,
			UserUUIDs:        []string{user.UUID},
		})
	}
	return cleanups, nil
}

// deleteUserOrganizationReBACRelationships removes a user's direct roles from every organization.
func deleteUserOrganizationReBACRelationships(
	ctx context.Context,
	authorizer rebac.Authorizer,
	cleanups []types.OrganizationReBACCleanup,
) error {
	for _, cleanup := range cleanups {
		if err := reconcileOrganizationMemberRelationships(
			ctx, authorizer, cleanup.OrganizationUUID, cleanup.UserUUIDs, nil,
		); err != nil {
			return fmt.Errorf("delete user relationships from organization %s: %w", cleanup.OrganizationUUID, err)
		}
	}
	return nil
}

// desiredOrganizationMemberRoles assigns one desired role to every user UUID.
func desiredOrganizationMemberRoles(userUUIDs []string, role types.UserRole) map[string]types.UserRole {
	roles := make(map[string]types.UserRole, len(userUUIDs))
	for _, userUUID := range userUUIDs {
		roles[userUUID] = role
	}
	return roles
}

// reconcileOrganizationMemberRelationships makes direct role tuples match the database state.
// Checking all direct role relations makes retries repair partial OpenFGA writes and stale tuples.
func reconcileOrganizationMemberRelationships(
	ctx context.Context,
	authorizer rebac.Authorizer,
	organizationUUID string,
	userUUIDs []string,
	desiredRoles map[string]types.UserRole,
) error {
	if len(userUUIDs) == 0 {
		return nil
	}
	if authorizer == nil {
		return fmt.Errorf("ReBAC authorizer is nil")
	}

	uniqueUserUUIDs := make([]string, 0, len(userUUIDs))
	seen := make(map[string]struct{}, len(userUUIDs))
	for _, userUUID := range userUUIDs {
		if _, exists := seen[userUUID]; exists {
			continue
		}
		seen[userUUID] = struct{}{}
		uniqueUserUUIDs = append(uniqueUserUUIDs, userUUID)
	}
	sort.Strings(uniqueUserUUIDs)

	desiredRelations := make(map[string]rebac.Relation, len(desiredRoles))
	for userUUID, role := range desiredRoles {
		if _, exists := seen[userUUID]; !exists {
			return fmt.Errorf("desired organization member %q is not included in reconciliation targets", userUUID)
		}
		relation, ok := role.ReBACRelation()
		if !ok {
			return fmt.Errorf("unsupported organization role %q", role)
		}
		desiredRelations[userUUID] = relation
	}

	usersPerBatch := rebac.DefaultMaxBatchSize / len(organizationMemberRelations)
	if usersPerBatch == 0 {
		return fmt.Errorf("organization member relations exceed the ReBAC batch limit")
	}
	for start := 0; start < len(uniqueUserUUIDs); start += usersPerBatch {
		end := min(start+usersPerBatch, len(uniqueUserUUIDs))
		if err := reconcileOrganizationMemberRelationshipBatch(
			ctx, authorizer, organizationUUID, uniqueUserUUIDs[start:end], desiredRelations,
		); err != nil {
			return err
		}
	}
	return nil
}

// reconcileOrganizationMemberRelationshipBatch reconciles one OpenFGA-compatible member batch.
func reconcileOrganizationMemberRelationshipBatch(
	ctx context.Context,
	authorizer rebac.Authorizer,
	organizationUUID string,
	userUUIDs []string,
	desiredRelations map[string]rebac.Relation,
) error {
	checks := make([]rebac.BatchCheckItem, 0, len(userUUIDs)*len(organizationMemberRelations))
	for userIndex, userUUID := range userUUIDs {
		for relationIndex, relation := range organizationMemberRelations {
			checks = append(checks, rebac.BatchCheckItem{
				CorrelationID: organizationMemberCorrelationID(userIndex, relationIndex),
				Check: rebac.CheckRequest{
					Subject:     rebac.UserSubject(userUUID),
					Relation:    relation,
					Object:      rebac.OrganizationObject(organizationUUID),
					Consistency: rebac.ConsistencyHigher,
				},
			})
		}
	}
	result, err := authorizer.BatchCheck(ctx, rebac.BatchCheckRequest{Checks: checks})
	if err != nil {
		return fmt.Errorf("check organization member ReBAC relationships: %w", err)
	}

	writes := make([]rebac.Relationship, 0, len(userUUIDs))
	deletes := make([]rebac.Relationship, 0, len(userUUIDs)*len(organizationMemberRelations))
	for userIndex, userUUID := range userUUIDs {
		desiredRelation, hasDesiredRole := desiredRelations[userUUID]
		for relationIndex, relation := range organizationMemberRelations {
			correlationID := organizationMemberCorrelationID(userIndex, relationIndex)
			relationship := rebac.Relationship{
				Subject:  rebac.UserSubject(userUUID),
				Relation: relation,
				Object:   rebac.OrganizationObject(organizationUUID),
			}
			outcome, exists := result.Results[correlationID]
			if !exists {
				return fmt.Errorf("missing ReBAC batch result %q for organization member relationship %q", correlationID, relationship.String())
			}
			if outcome.Err != nil {
				return fmt.Errorf("check organization member ReBAC relationship %q with correlation ID %q: %w", relationship.String(), correlationID, outcome.Err)
			}
			switch {
			case outcome.Decision.Allowed && (!hasDesiredRole || relation != desiredRelation):
				deletes = append(deletes, relationship)
			case !outcome.Decision.Allowed && hasDesiredRole && relation == desiredRelation:
				writes = append(writes, relationship)
			}
		}
	}

	if len(deletes) > 0 {
		if err := authorizer.Delete(ctx, deletes); err != nil {
			return fmt.Errorf("delete organization member ReBAC relationships: %w", err)
		}
	}
	if len(writes) > 0 {
		if err := authorizer.Write(ctx, writes); err != nil {
			return fmt.Errorf("write organization member ReBAC relationships: %w", err)
		}
	}
	return nil
}

// organizationMemberCorrelationID creates a valid batch-local identifier for one direct role check.
func organizationMemberCorrelationID(userIndex, relationIndex int) string {
	return rebac.BatchCheckCorrelationID(userIndex*len(organizationMemberRelations) + relationIndex)
}

// repositoryNamespaceGrant identifies the subject and direct relation assigned by one repository namespace.
type repositoryNamespaceGrant struct {
	subject  rebac.Subject
	relation rebac.Relation
}

// loadUserRepositoryRelationships loads active repositories created by a user and resolves their namespace tuples.
// Repository ownership is derived from each repository path because repository.user_id records the creator and may include organization repositories.
func loadUserRepositoryRelationships(
	ctx context.Context,
	repoStore database.RepoStore,
	namespaceStore database.NamespaceStore,
	organizationStore database.OrgStore,
	user database.User,
) ([]database.Repository, []rebac.Relationship, error) {
	const batchSize = 1000

	var repositories []database.Repository
	var relationships []rebac.Relationship
	grants := make(map[string]repositoryNamespaceGrant)
	for batch := 0; ; batch++ {
		batchRepositories, err := repoStore.ByUser(ctx, user.ID, batchSize, batch)
		if err != nil {
			return nil, nil, fmt.Errorf("load repositories created by user %d: %w", user.ID, err)
		}
		if len(batchRepositories) == 0 {
			break
		}

		for _, repository := range batchRepositories {
			namespacePath, _ := repository.NamespaceAndName()
			grant, exists := grants[namespacePath]
			if !exists {
				namespace, err := namespaceStore.FindByPath(ctx, namespacePath)
				if err == nil {
					grant, err = repositoryNamespaceGrantForUserDeletion(ctx, organizationStore, namespace)
				} else {
					// Account soft deletion also removes namespaces, so permanent deletion must resolve retained repositories from their path.
					grant, err = repositoryNamespaceGrantForDeletedUser(ctx, organizationStore, user, namespacePath)
				}
				if err != nil {
					return nil, nil, fmt.Errorf("resolve namespace %q for repository %d: %w", namespacePath, repository.ID, err)
				}
				grants[namespacePath] = grant
			}
			relationships = append(relationships, rebac.Relationship{
				Subject:  grant.subject,
				Relation: grant.relation,
				Object:   rebac.RepositoryObject(repository.ID),
			})
		}
		repositories = append(repositories, batchRepositories...)
	}
	return repositories, relationships, nil
}

// repositoryNamespaceGrantForUserDeletion resolves a personal or organization namespace to its repository grant.
func repositoryNamespaceGrantForUserDeletion(ctx context.Context, organizationStore database.OrgStore, namespace database.Namespace) (repositoryNamespaceGrant, error) {
	switch namespace.NamespaceType {
	case database.UserNamespace:
		if namespace.User.UUID == "" {
			return repositoryNamespaceGrant{}, fmt.Errorf("user namespace %q has no user UUID", namespace.Path)
		}
		return repositoryNamespaceGrant{
			subject:  rebac.UserSubject(namespace.User.UUID),
			relation: rebac.RelationOwner,
		}, nil
	case database.OrgNamespace:
		if organizationStore == nil {
			return repositoryNamespaceGrant{}, fmt.Errorf("organization store is required for namespace %q", namespace.Path)
		}
		organization, err := organizationStore.FindByPath(ctx, namespace.Path)
		if err != nil {
			return repositoryNamespaceGrant{}, fmt.Errorf("find organization for namespace %q: %w", namespace.Path, err)
		}
		if organization.UUID == uuid.Nil {
			return repositoryNamespaceGrant{}, fmt.Errorf("organization namespace %q has no organization UUID", namespace.Path)
		}
		return repositoryNamespaceGrant{
			subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, organization.UUID.String()),
			relation: rebac.RelationOrganization,
		}, nil
	default:
		return repositoryNamespaceGrant{}, fmt.Errorf("unsupported namespace type %q for namespace %q", namespace.NamespaceType, namespace.Path)
	}
}

// repositoryNamespaceGrantForDeletedUser resolves retained repositories after their namespace was soft-deleted.
func repositoryNamespaceGrantForDeletedUser(
	ctx context.Context,
	organizationStore database.OrgStore,
	user database.User,
	namespacePath string,
) (repositoryNamespaceGrant, error) {
	if namespacePath == user.Username && user.UUID != "" {
		return repositoryNamespaceGrant{
			subject:  rebac.UserSubject(user.UUID),
			relation: rebac.RelationOwner,
		}, nil
	}
	if organizationStore == nil {
		return repositoryNamespaceGrant{}, fmt.Errorf("organization store is required for namespace %q", namespacePath)
	}
	organization, err := organizationStore.FindByPath(ctx, namespacePath)
	if err != nil {
		return repositoryNamespaceGrant{}, fmt.Errorf("find organization for deleted namespace %q: %w", namespacePath, err)
	}
	if organization.UUID == uuid.Nil {
		return repositoryNamespaceGrant{}, fmt.Errorf("organization namespace %q has no organization UUID", namespacePath)
	}
	return repositoryNamespaceGrant{
		subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, organization.UUID.String()),
		relation: rebac.RelationOrganization,
	}, nil
}

// deleteRepositoryNamespaceRelationships removes existing repository ownership tuples in bounded batches.
func deleteRepositoryNamespaceRelationships(ctx context.Context, authorizer rebac.Authorizer, relationships []rebac.Relationship) error {
	if len(relationships) == 0 {
		return nil
	}
	if authorizer == nil {
		return fmt.Errorf("ReBAC authorizer is nil")
	}

	for start := 0; start < len(relationships); start += rebac.DefaultMaxBatchSize {
		end := min(start+rebac.DefaultMaxBatchSize, len(relationships))
		batch := relationships[start:end]
		checks := make([]rebac.BatchCheckItem, 0, len(batch))
		for index, relationship := range batch {
			checks = append(checks, rebac.BatchCheckItem{
				CorrelationID: rebac.BatchCheckCorrelationID(index),
				Check: rebac.CheckRequest{
					Subject:     relationship.Subject,
					Relation:    relationship.Relation,
					Object:      relationship.Object,
					Consistency: rebac.ConsistencyHigher,
				},
			})
		}

		result, err := authorizer.BatchCheck(ctx, rebac.BatchCheckRequest{Checks: checks})
		if err != nil {
			return fmt.Errorf("check repository namespace relationships before deletion: %w", err)
		}
		deletes := make([]rebac.Relationship, 0, len(batch))
		for index, relationship := range batch {
			correlationID := rebac.BatchCheckCorrelationID(index)
			outcome, exists := result.Results[correlationID]
			if !exists {
				return fmt.Errorf("missing ReBAC batch result %q for repository namespace relationship %q", correlationID, relationship.String())
			}
			if outcome.Err != nil {
				return fmt.Errorf("check repository namespace relationship %q with correlation ID %q: %w", relationship.String(), correlationID, outcome.Err)
			}
			if outcome.Decision.Allowed {
				deletes = append(deletes, relationship)
			}
		}
		if len(deletes) == 0 {
			continue
		}
		if err := authorizer.Delete(ctx, deletes); err != nil {
			return errorx.ReBACNamespacePermissionDeleteFailed(err, errorx.Ctx().Set("repository_count", len(deletes)))
		}
	}
	return nil
}

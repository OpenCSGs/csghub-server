package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// ensureMirrorOrgNamespace creates the target organization when it does not
// exist yet, so a mirrored repository can land under the resolved source
// namespace instead of a hardcoded fallback. It is a no-op when the namespace
// already exists.
//
// The organization is owned by the admin user that initiated the mirror
// request. Only the rows required for repository creation are written:
// organization, namespace, the creator admin membership, and the ReBAC
// tuples that make the namespace usable. No SSO user is created, because
// repository creation does not require one.
func (m *mirrorComponentImpl) ensureMirrorOrgNamespace(ctx context.Context, namespacePath, ownerUsername string) error {
	namespacePath = strings.TrimSpace(namespacePath)
	if namespacePath == "" || ownerUsername == "" {
		return nil
	}

	exists, err := m.namespaceStore.Exists(ctx, namespacePath)
	if err != nil {
		return fmt.Errorf("failed to check mirror target namespace existence: %w", err)
	}
	if exists {
		return m.reconcileMirrorOrgNamespace(ctx, namespacePath)
	}

	user, err := m.userStore.FindByUsername(ctx, ownerUsername)
	if err != nil {
		return fmt.Errorf("failed to find owner user for mirror target organization: %w", err)
	}

	org := &database.Organization{
		Name:   namespacePath,
		UserID: user.ID,
		User:   &user,
		UUID:   uuid.New(),
		IsRoot: true,
	}
	namespace := database.Namespace{
		Path:          namespacePath,
		UserID:        user.ID,
		UUID:          uuid.New().String(),
		NamespaceType: database.OrgNamespace,
	}

	if err := m.orgStore.CreateWithRelations(ctx, org, &namespace, nil); err != nil {
		// A concurrent mirror request may have created the namespace between
		// the existence check and the insert. Treat that as success so both
		// requests proceed instead of failing the slower one.
		if created, raceErr := m.namespaceStore.Exists(ctx, namespacePath); raceErr == nil && created {
			return m.reconcileMirrorOrgNamespace(ctx, namespacePath)
		}
		return fmt.Errorf("failed to create mirror target organization: %w", err)
	}

	if err := ensureNamespaceRelationship(ctx, m.rebac, namespace, org.UUID.String()); err != nil {
		return fmt.Errorf("synchronize mirror target organization namespace to ReBAC: %w", err)
	}
	if err := m.ensureMirrorOrgAdminRelationship(ctx, org.UUID.String(), user.UUID); err != nil {
		return fmt.Errorf("synchronize mirror target organization admin to ReBAC: %w", err)
	}

	slog.InfoContext(ctx, "auto created mirror target organization",
		slog.String("namespace", namespacePath),
		slog.String("owner", ownerUsername),
		slog.String("organization_uuid", org.UUID.String()),
	)
	return nil
}

// reconcileMirrorOrgNamespace repairs the ReBAC state for an existing
// organization namespace. The historical creator receives an admin tuple only
// while their current database membership still has the admin role.
func (m *mirrorComponentImpl) reconcileMirrorOrgNamespace(ctx context.Context, namespacePath string) error {
	namespace, err := m.namespaceStore.FindByPath(ctx, namespacePath)
	if err != nil {
		return fmt.Errorf("failed to load mirror target namespace: %w", err)
	}
	if namespace.NamespaceType != database.OrgNamespace {
		return nil
	}

	org, err := m.orgStore.FindByPath(ctx, namespace.Path)
	if err != nil {
		return fmt.Errorf("failed to load mirror target organization: %w", err)
	}
	if err := ensureNamespaceRelationship(ctx, m.rebac, namespace, org.UUID.String()); err != nil {
		return fmt.Errorf("synchronize mirror target organization namespace to ReBAC: %w", err)
	}
	member, err := m.memberStore.Find(ctx, org.ID, org.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to load mirror target organization creator membership: %w", err)
	}
	if member.Role != string(types.UserAdmin) {
		return nil
	}

	owner, err := m.userStore.FindByID(ctx, member.UserID)
	if err != nil {
		return fmt.Errorf("failed to load mirror target organization owner: %w", err)
	}
	if err := m.ensureMirrorOrgAdminRelationship(ctx, org.UUID.String(), owner.UUID); err != nil {
		return fmt.Errorf("synchronize mirror target organization admin to ReBAC: %w", err)
	}
	return nil
}

// ensureMirrorOrgAdminRelationship writes the direct admin membership tuple
// for the organization owner when it is not already present.
func (m *mirrorComponentImpl) ensureMirrorOrgAdminRelationship(ctx context.Context, organizationUUID, userUUID string) error {
	if m.rebac == nil {
		return errors.New("ReBAC authorizer is required")
	}
	if organizationUUID == "" || userUUID == "" {
		return errors.New("organization UUID and user UUID are required")
	}

	relationship := rebac.Relationship{
		Subject:  rebac.UserSubject(userUUID),
		Relation: rebac.RelationAdmin,
		Object:   rebac.OrganizationObject(organizationUUID),
	}
	decision, err := m.rebac.Check(ctx, rebac.CheckRequest{
		Subject:     relationship.Subject,
		Relation:    relationship.Relation,
		Object:      relationship.Object,
		Consistency: rebac.ConsistencyHigher,
	})
	if err != nil {
		return fmt.Errorf("check organization admin relationship: %w", err)
	}
	if decision.Allowed {
		return nil
	}
	if err := m.rebac.Write(ctx, []rebac.Relationship{relationship}); err != nil {
		return fmt.Errorf("write organization admin relationship: %w", err)
	}
	return nil
}

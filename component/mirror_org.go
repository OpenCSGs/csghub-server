package component

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/store/database"
)

// ensureMirrorOrgNamespace creates a single-level target organization when it does not
// exist yet, so a mirrored repository can land under the resolved source
// namespace instead of a hardcoded fallback, regardless of the runtime organization mode.
// Existing namespaces retain their organization type and have their ReBAC relationships reconciled.
//
// The organization is owned by the admin user that initiated the mirror
// request. Only the rows required for repository creation are written:
// organization, namespace, the creator admin membership, and the ReBAC
// tuples that make the namespace usable. No SSO user is created, because
// repository creation does not require one.
func (m *mirrorComponentImpl) ensureMirrorOrgNamespace(ctx context.Context, namespacePath, ownerUsername string) error {
	namespacePath = strings.TrimSpace(namespacePath)
	if namespacePath == "" || strings.TrimSpace(ownerUsername) == "" {
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
	return m.createMirrorOrgNamespace(ctx, namespacePath, user)
}

func (m *mirrorComponentImpl) createMirrorOrgNamespace(ctx context.Context, namespacePath string, user database.User) error {
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
		UUID:          org.UUID.String(),
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

	slog.InfoContext(ctx, "auto created mirror target organization",
		slog.String("namespace", namespacePath),
		slog.String("owner", user.Username),
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
	return m.orgStore.ReconcileNamespaceTuples(ctx, org.ID)
}

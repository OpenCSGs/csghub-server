package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// hierarchyOrganizationStore decorates organization deletion with hierarchy cleanup.
type hierarchyOrganizationStore struct {
	OrgStore
	db *DB
}

// NewHierarchyOrgStoreWithDB creates a hierarchy-aware organization Store with an explicit database.
func NewHierarchyOrgStoreWithDB(db *DB) OrgStore {
	return &hierarchyOrganizationStore{OrgStore: NewOrgStoreWithMode(db, true), db: db}
}

// GetUserBelongOrgs returns organizations whose active hierarchy memberships include the user.
func (s *hierarchyOrganizationStore) GetUserBelongOrgs(ctx context.Context, userID int64) ([]Organization, error) {
	var organizations []Organization
	err := s.db.Core.NewSelect().
		Model(&organizations).
		Relation("Namespace").
		ColumnExpr("organization.*").
		ColumnExpr("member.role AS role").
		Join("JOIN members AS member ON member.organization_id = organization.id AND member.deleted_at IS NULL").
		Where("member.user_id = ? AND organization.is_unit = TRUE", userID).
		Scan(ctx, &organizations)
	return organizations, errorx.HandleDBError(err, nil)
}

// SearchUserBelongOrgs searches the active hierarchy memberships of one user.
func (s *hierarchyOrganizationStore) SearchUserBelongOrgs(ctx context.Context, userID int64, search string, per int, page int, orgType string, verifyStatus string, role string, tag string) ([]Organization, int, error) {
	organizations := make([]Organization, 0)
	search = strings.ToLower(search)
	query := s.db.Core.NewSelect().Model(&organizations).Relation("Namespace")

	// Keep the legacy owner filter as an alias for the administrator role.
	// Organization ownership is no longer a separate membership permission.
	if role == "owner" {
		role = string(types.UserAdmin)
	}
	query = query.
		ColumnExpr("organization.*").
		ColumnExpr("member.role AS role").
		Join("JOIN members AS member ON member.organization_id = organization.id AND member.deleted_at IS NULL").
		Where("member.user_id = ? AND organization.is_unit = TRUE", userID)
	switch types.UserRole(role) {
	case types.UserWrite:
		query = query.Where("member.role = ?", types.UserWrite)
	case types.UserAdmin:
		query = query.Where("member.role = ?", types.UserAdmin)
	}

	if search != "" {
		query.Where("LOWER(organization.name) LIKE ? OR LOWER(organization.path) LIKE ?", fmt.Sprintf("%%%s%%", search), fmt.Sprintf("%%%s%%", search))
		query.OrderExpr(`
			CASE
				WHEN LOWER(organization.path) = ? THEN 0
				WHEN LOWER(organization.path) LIKE ? THEN 1
				WHEN LOWER(organization.name) = ? THEN 2
				WHEN LOWER(organization.name) LIKE ? THEN 3
				ELSE 4
			END
		`, search, fmt.Sprintf("%s%%", search), search, fmt.Sprintf("%s%%", search))
	}
	if orgType != "" {
		query.Where("org_type = ?", orgType)
	}
	if verifyStatus != "" {
		query.Where("verify_status = ?", verifyStatus)
	}
	if tag != "" {
		query.Where(`EXISTS (
			SELECT 1 FROM organization_tags AS ot
			JOIN tags AS t ON t.id = ot.tag_id
			WHERE ot.organization_id = organization.id AND LOWER(t.name) = ?
		)`, strings.ToLower(tag))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return organizations, total, errorx.HandleDBError(err, nil)
	}
	query.Order("organization.id ASC").Limit(per).Offset((page - 1) * per)
	if err := query.Scan(ctx, &organizations); err != nil {
		return organizations, total, errorx.HandleDBError(err, nil)
	}
	return organizations, total, nil
}

// GetSharedOrgIDs returns organizations that contain every user through active hierarchy memberships.
func (s *hierarchyOrganizationStore) GetSharedOrgIDs(ctx context.Context, userIDs []int64) ([]int64, error) {
	organizationIDs := make([]int64, 0)
	if len(userIDs) == 0 {
		return organizationIDs, nil
	}
	err := s.db.Core.NewSelect().
		Model((*Organization)(nil)).
		Column("organization.id").
		Join("JOIN members AS member ON member.organization_id = organization.id AND member.deleted_at IS NULL").
		Where("member.user_id IN (?) AND organization.is_unit = TRUE", bun.In(userIDs)).
		Group("organization.id").
		Having("COUNT(DISTINCT member.user_id) = ?", len(userIDs)).
		Scan(ctx, &organizationIDs)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return organizationIDs, nil
}

// Delete removes a top-level organization or a child organization subtree.
func (s *hierarchyOrganizationStore) Delete(ctx context.Context, path string) error {
	return s.db.BunDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var organization Organization
		if err := tx.NewSelect().Model(&organization).
			Where("path = ? AND is_unit = TRUE AND deleted_at IS NULL", path).For("UPDATE").Scan(ctx); err != nil {
			return err
		}
		if organization.IsRoot {
			return deleteRootOrganizationTx(ctx, tx, &organization)
		}
		var unit OrganizationUnit
		if err := tx.NewSelect().Model(&unit).
			Where("organization_id = ? AND deleted_at IS NULL", organization.ID).Scan(ctx); err != nil {
			return err
		}
		return deleteUnitSubtreeTx(ctx, tx, unit.RootOrganizationID, unit.ID)
	})
}

// deleteRootOrganizationTx removes hierarchy records before physically removing the root.
func deleteRootOrganizationTx(ctx context.Context, tx bun.Tx, root *Organization) error {
	var unitIDs []int64
	if err := tx.NewSelect().Model((*OrganizationUnit)(nil)).WhereAllWithDeleted().Column("id").
		Where("root_organization_id = ?", root.ID).Scan(ctx, &unitIDs); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("load root organization units: %w", err)
	}
	if _, err := tx.NewDelete().Model((*OrganizationUnitClosure)(nil)).
		Where("root_organization_id = ?", root.ID).Exec(ctx); err != nil {
		return fmt.Errorf("delete root organization closure: %w", err)
	}
	if _, err := tx.NewUpdate().Model((*OrganizationUnit)(nil)).
		Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
		Where("root_organization_id = ? AND deleted_at IS NULL", root.ID).Exec(ctx); err != nil {
		return fmt.Errorf("soft-delete root organization units: %w", err)
	}
	if _, err := tx.NewUpdate().Model((*Member)(nil)).
		Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
		Where("organization_id = ? AND deleted_at IS NULL", root.ID).Exec(ctx); err != nil {
		return fmt.Errorf("remove root organization members: %w", err)
	}
	var childIDs []int64
	if err := tx.NewSelect().Model((*OrganizationUnit)(nil)).WhereAllWithDeleted().Column("organization_id").
		Where("root_organization_id = ?", root.ID).Scan(ctx, &childIDs); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("load child organization IDs: %w", err)
	}
	if len(childIDs) > 0 {
		var namespaceIDs []int64
		if err := tx.NewSelect().Model((*Organization)(nil)).Column("namespace_id").
			Where("id IN (?) AND deleted_at IS NULL", bun.In(childIDs)).Scan(ctx, &namespaceIDs); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("load child organization namespaces: %w", err)
		}
		if _, err := tx.NewUpdate().Model((*Member)(nil)).Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
			Where("organization_id IN (?) AND deleted_at IS NULL", bun.In(childIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("remove child organization members: %w", err)
		}
		if _, err := tx.NewUpdate().Model((*Organization)(nil)).Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
			Where("id IN (?) AND is_root = FALSE AND deleted_at IS NULL", bun.In(childIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("soft-delete child organizations: %w", err)
		}
		if _, err := tx.NewDelete().Model((*OrganizationTag)(nil)).Where("organization_id IN (?)", bun.In(childIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("delete child organization tags: %w", err)
		}
		if len(namespaceIDs) > 0 {
			if _, err := tx.NewUpdate().Model((*Namespace)(nil)).Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
				Where("id IN (?) AND deleted_at IS NULL", bun.In(namespaceIDs)).Exec(ctx); err != nil {
				return fmt.Errorf("soft-delete child organization namespaces: %w", err)
			}
		}
	}
	if _, err := tx.NewDelete().Model((*OrganizationTag)(nil)).Where("organization_id = ?", root.ID).Exec(ctx); err != nil {
		return fmt.Errorf("delete root organization tags: %w", err)
	}
	if _, err := tx.NewDelete().Model((*Organization)(nil)).Where("id = ?", root.ID).ForceDelete().Exec(ctx); err != nil {
		return fmt.Errorf("delete root organization: %w", err)
	}
	if _, err := tx.NewDelete().Model((*Namespace)(nil)).Where("id = ?", root.NamespaceID).ForceDelete().Exec(ctx); err != nil {
		return fmt.Errorf("delete root organization namespace: %w", err)
	}
	return nil
}

// deleteUnitSubtreeTx soft-deletes a child organization subtree and its relationships.
func deleteUnitSubtreeTx(ctx context.Context, tx bun.Tx, rootOrganizationID, unitID int64) error {
	var unitIDs []int64
	if err := tx.NewSelect().Model((*OrganizationUnitClosure)(nil)).Column("descendant_unit_id").
		Where("root_organization_id = ? AND ancestor_unit_id = ?", rootOrganizationID, unitID).
		Scan(ctx, &unitIDs); err != nil {
		return fmt.Errorf("load organization unit subtree: %w", err)
	}
	if len(unitIDs) == 0 {
		return sql.ErrNoRows
	}
	var organizationIDs []int64
	if err := tx.NewSelect().Model((*OrganizationUnit)(nil)).Column("organization_id").
		Where("root_organization_id = ? AND id IN (?)", rootOrganizationID, bun.In(unitIDs)).Scan(ctx, &organizationIDs); err != nil {
		return fmt.Errorf("load subtree organizations: %w", err)
	}
	var namespaceIDs []int64
	if err := tx.NewSelect().Model((*Organization)(nil)).Column("namespace_id").
		Where("id IN (?) AND deleted_at IS NULL", bun.In(organizationIDs)).Scan(ctx, &namespaceIDs); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("load subtree organization namespaces: %w", err)
	}
	if _, err := tx.NewDelete().Model((*OrganizationUnitClosure)(nil)).
		Where("root_organization_id = ? AND (ancestor_unit_id IN (?) OR descendant_unit_id IN (?))", rootOrganizationID, bun.In(unitIDs), bun.In(unitIDs)).Exec(ctx); err != nil {
		return fmt.Errorf("delete subtree closure: %w", err)
	}
	if _, err := tx.NewUpdate().Model((*OrganizationUnit)(nil)).Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
		Where("root_organization_id = ? AND id IN (?) AND deleted_at IS NULL", rootOrganizationID, bun.In(unitIDs)).Exec(ctx); err != nil {
		return fmt.Errorf("soft-delete subtree units: %w", err)
	}
	if _, err := tx.NewUpdate().Model((*Member)(nil)).Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
		Where("organization_id IN (?) AND deleted_at IS NULL", bun.In(organizationIDs)).Exec(ctx); err != nil {
		return fmt.Errorf("remove subtree organization members: %w", err)
	}
	if _, err := tx.NewUpdate().Model((*Organization)(nil)).Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
		Where("id IN (?) AND is_root = FALSE AND deleted_at IS NULL", bun.In(organizationIDs)).Exec(ctx); err != nil {
		return fmt.Errorf("soft-delete subtree organizations: %w", err)
	}
	if _, err := tx.NewDelete().Model((*OrganizationTag)(nil)).
		Where("organization_id IN (?)", bun.In(organizationIDs)).Exec(ctx); err != nil {
		return fmt.Errorf("delete subtree organization tags: %w", err)
	}
	if len(namespaceIDs) > 0 {
		if _, err := tx.NewUpdate().Model((*Namespace)(nil)).Set("deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP").
			Where("id IN (?) AND deleted_at IS NULL", bun.In(namespaceIDs)).Exec(ctx); err != nil {
			return fmt.Errorf("soft-delete subtree organization namespaces: %w", err)
		}
	}
	return nil
}

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// AddOrganizationMembersInput contains the atomic membership write inputs.
type AddOrganizationMembersInput struct {
	RootOrganizationID int64
	OrganizationID     int64
	OrganizationUnitID *int64
	UserUUIDs          []string
	Role               types.UserRole
}

// RemoveOrganizationMembersInput contains direct relationships to remove.
type RemoveOrganizationMembersInput struct {
	RootOrganizationID int64
	OrganizationID     int64
	OrganizationUnitID *int64
	UserUUIDs          []string
}

// UpdateOrganizationMemberRoleInput contains one organization role update.
type UpdateOrganizationMemberRoleInput struct {
	RootOrganizationID int64
	OrganizationID     int64
	OrganizationUnitID *int64
	OrganizationUUID   string
	UserID             int64
	Role               types.UserRole
}

// ListOrganizationMembersInput contains organization member filters and pagination.
type ListOrganizationMembersInput struct {
	OrganizationID int64
	Search         string
	Role           types.UserRole
	Per            int
	Page           int
}

// OrganizationUnitMemberStore provides hierarchy-aware operations over common organization memberships.
type OrganizationUnitMemberStore interface {
	UpsertMembers(ctx context.Context, input AddOrganizationMembersInput) (*types.OrganizationMemberMutationResp, error)
	RemoveMembers(ctx context.Context, input RemoveOrganizationMembersInput) (*types.OrganizationMemberMutationResp, error)
	UpdateMemberRole(ctx context.Context, input UpdateOrganizationMemberRoleInput) error
	ListMembers(ctx context.Context, input ListOrganizationMembersInput) ([]types.OrganizationMember, int, error)
	FindRole(ctx context.Context, organizationID, userID int64) (types.UserRole, error)
}

// organizationUnitMemberStoreImpl uses one database connection for hierarchy-aware membership transactions.
type organizationUnitMemberStoreImpl struct {
	db *DB
}

// NewOrganizationUnitMemberStore creates a hierarchy-aware membership Store.
func NewOrganizationUnitMemberStore() OrganizationUnitMemberStore {
	return NewOrganizationUnitMemberStoreWithDB(GetDB())
}

// NewOrganizationUnitMemberStoreWithDB creates a membership Store with an explicit database.
func NewOrganizationUnitMemberStoreWithDB(db *DB) OrganizationUnitMemberStore {
	return &organizationUnitMemberStoreImpl{db: db}
}

// UpsertMembers atomically creates or updates direct memberships while preserving an administrator.
func (s *organizationUnitMemberStoreImpl) UpsertMembers(ctx context.Context, input AddOrganizationMembersInput) (*types.OrganizationMemberMutationResp, error) {
	response := &types.OrganizationMemberMutationResp{
		MembersAdded: []string{},
		Skipped:      []string{},
	}
	if err := validateOrganizationUnitRole(input.Role); err != nil {
		return nil, err
	}
	err := s.db.BunDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockOrganization(ctx, tx, input.RootOrganizationID); err != nil {
			return err
		}
		if err := validateOrganizationMembershipTarget(ctx, tx, input.RootOrganizationID, input.OrganizationID, input.OrganizationUnitID); err != nil {
			return err
		}
		users, err := lockUsersByUUID(ctx, tx, input.UserUUIDs)
		if err != nil {
			return err
		}
		if input.Role != types.UserAdmin {
			adminUserIDs, err := lockOrganizationAdminUserIDs(ctx, tx, input.OrganizationID)
			if err != nil {
				return err
			}
			if err := ensureAdminRemains(adminUserIDs, users); err != nil {
				return err
			}
		}
		for _, user := range users {
			if err := upsertOrganizationMember(ctx, tx, input.OrganizationID, user.ID, input.Role); err != nil {
				return fmt.Errorf("upsert organization membership for user %s: %w", user.UUID, err)
			}
			response.MembersAdded = append(response.MembersAdded, user.UUID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

// validateOrganizationUnitRole restricts hierarchy relationships to supported permissions.
func validateOrganizationUnitRole(role types.UserRole) error {
	switch role {
	case types.UserAdmin, types.UserWrite, types.UserRead:
		return nil
	default:
		return errorx.ReqParamInvalid(fmt.Errorf("unsupported organization unit role %q", role), nil)
	}
}

// validateOrganizationMembershipTarget verifies root memberships and child unit mappings.
func validateOrganizationMembershipTarget(ctx context.Context, tx bun.Tx, rootOrganizationID, organizationID int64, organizationUnitID *int64) error {
	if organizationUnitID == nil {
		if rootOrganizationID != organizationID {
			return errorx.ReqParamInvalid(errors.New("root organization membership cannot reference another organization"), nil)
		}
		var isRoot bool
		if err := tx.NewSelect().Model((*Organization)(nil)).Column("is_root").
			Where("organization.id = ? AND organization.is_unit = TRUE AND organization.deleted_at IS NULL", organizationID).Scan(ctx, &isRoot); err != nil {
			return err
		}
		if !isRoot {
			return errorx.ReqParamInvalid(errors.New("child organization membership requires an organization unit"), nil)
		}
		return nil
	}
	unit, err := selectUnitForUpdate(ctx, tx, rootOrganizationID, *organizationUnitID)
	if err != nil {
		return err
	}
	if unit.OrganizationID != organizationID {
		return errorx.ReqParamInvalid(errors.New("organization unit does not belong to organization"), nil)
	}
	return nil
}

// upsertOrganizationMember keeps an active relationship idempotent and updates changed roles.
func upsertOrganizationMember(ctx context.Context, tx bun.Tx, organizationID, userID int64, role types.UserRole) error {
	var relation Member
	err := tx.NewSelect().Model(&relation).
		Where("organization_id = ? AND user_id = ?", organizationID, userID).
		Scan(ctx)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		relation = Member{
			OrganizationID: organizationID,
			UserID:         userID,
			Role:           string(role),
		}
		_, err = tx.NewInsert().Model(&relation).Exec(ctx)
		return err
	case err != nil:
		return err
	case relation.Role == string(role):
		return nil
	default:
		_, err = tx.NewUpdate().Model((*Member)(nil)).
			Set("role = ?, updated_at = CURRENT_TIMESTAMP", string(role)).
			Where("id = ?", relation.ID).Exec(ctx)
		return err
	}
}

// lockUsersByUUID resolves and locks every requested active user in stable order.
func lockUsersByUUID(ctx context.Context, tx bun.Tx, userUUIDs []string) ([]User, error) {
	uniqueUUIDs := uniqueStrings(userUUIDs)
	var users []User
	err := tx.NewSelect().Model(&users).
		Where("uuid IN (?)", bun.In(uniqueUUIDs)).
		Order("uuid ASC").
		For("UPDATE").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock organization unit users: %w", err)
	}
	if len(users) != len(uniqueUUIDs) {
		found := make(map[string]struct{}, len(users))
		for _, user := range users {
			found[user.UUID] = struct{}{}
		}
		for _, userUUID := range uniqueUUIDs {
			if _, ok := found[userUUID]; !ok {
				return nil, errorx.ReqParamInvalid(fmt.Errorf("user %s does not exist", userUUID), nil)
			}
		}
	}
	return users, nil
}

// uniqueStrings removes duplicate batch values and returns stable ordering.
func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// lockOrganizationAdminUserIDs locks and returns every active administrator in one organization.
func lockOrganizationAdminUserIDs(ctx context.Context, tx bun.Tx, organizationID int64) ([]int64, error) {
	var userIDs []int64
	err := tx.NewSelect().Model((*Member)(nil)).
		Column("user_id").
		Where("organization_id = ? AND role = ? AND deleted_at IS NULL", organizationID, types.UserAdmin).
		Order("user_id ASC").
		For("UPDATE").
		Scan(ctx, &userIDs)
	if err != nil {
		return nil, fmt.Errorf("lock organization administrators: %w", err)
	}
	return userIDs, nil
}

// ensureAdminRemains rejects a batch that would revoke every active administrator role.
func ensureAdminRemains(adminUserIDs []int64, affectedUsers []User) error {
	if len(adminUserIDs) == 0 {
		return nil
	}
	affectedUserIDs := make(map[int64]struct{}, len(affectedUsers))
	for _, user := range affectedUsers {
		affectedUserIDs[user.ID] = struct{}{}
	}
	for _, adminUserID := range adminUserIDs {
		if _, affected := affectedUserIDs[adminUserID]; !affected {
			return nil
		}
	}
	return errorx.ErrLastOrgAdmin
}

// RemoveMembers removes only the requested users while preserving at least one organization administrator.
func (s *organizationUnitMemberStoreImpl) RemoveMembers(ctx context.Context, input RemoveOrganizationMembersInput) (*types.OrganizationMemberMutationResp, error) {
	response := &types.OrganizationMemberMutationResp{
		MembersRemoved: []string{},
		Skipped:        []string{},
	}
	err := s.db.BunDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockOrganization(ctx, tx, input.RootOrganizationID); err != nil {
			return err
		}
		if err := validateOrganizationMembershipTarget(ctx, tx, input.RootOrganizationID, input.OrganizationID, input.OrganizationUnitID); err != nil {
			return err
		}
		users, err := lockUsersByUUID(ctx, tx, input.UserUUIDs)
		if err != nil {
			return err
		}
		adminUserIDs, err := lockOrganizationAdminUserIDs(ctx, tx, input.OrganizationID)
		if err != nil {
			return err
		}
		if err := ensureAdminRemains(adminUserIDs, users); err != nil {
			return err
		}
		for _, user := range users {
			result, err := tx.NewDelete().Model((*Member)(nil)).
				Where("organization_id = ? AND user_id = ? AND deleted_at IS NULL", input.OrganizationID, user.ID).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("remove organization membership for user %s: %w", user.UUID, err)
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if affected == 0 {
				response.Skipped = append(response.Skipped, user.UUID)
			} else {
				response.MembersRemoved = append(response.MembersRemoved, user.UUID)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return response, nil
}

// UpdateMemberRole atomically preserves an administrator and updates one direct membership.
func (s *organizationUnitMemberStoreImpl) UpdateMemberRole(ctx context.Context, input UpdateOrganizationMemberRoleInput) error {
	if err := validateOrganizationUnitRole(input.Role); err != nil {
		return err
	}
	return s.db.BunDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockOrganization(ctx, tx, input.RootOrganizationID); err != nil {
			return err
		}
		if err := validateOrganizationMembershipTarget(ctx, tx, input.RootOrganizationID, input.OrganizationID, input.OrganizationUnitID); err != nil {
			return err
		}
		var currentRole types.UserRole
		err := tx.NewSelect().Model((*Member)(nil)).Column("role").
			Where("organization_id = ? AND user_id = ? AND deleted_at IS NULL", input.OrganizationID, input.UserID).
			For("UPDATE").
			Scan(ctx, &currentRole)
		if errors.Is(err, sql.ErrNoRows) {
			return errorx.OrganizationMemberNotFound(input.OrganizationUUID, input.UserID)
		}
		if err != nil {
			return fmt.Errorf("lock organization member role: %w", err)
		}
		if currentRole == types.UserAdmin && input.Role != types.UserAdmin {
			adminUserIDs, err := lockOrganizationAdminUserIDs(ctx, tx, input.OrganizationID)
			if err != nil {
				return err
			}
			if len(adminUserIDs) == 1 {
				return errorx.ErrLastOrgAdmin
			}
		}
		result, err := tx.NewUpdate().Model((*Member)(nil)).
			Set("role = ?, updated_at = CURRENT_TIMESTAMP", input.Role).
			Where("organization_id = ? AND user_id = ? AND deleted_at IS NULL", input.OrganizationID, input.UserID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("update organization member role: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("count updated organization member role: %w", err)
		}
		if affected == 0 {
			return errorx.OrganizationMemberNotFound(input.OrganizationUUID, input.UserID)
		}
		return nil
	})
}

// organizationMemberRow is the projection used by direct member queries.
type organizationMemberRow struct {
	UserID   int64          `bun:"user_id"`
	UserUUID string         `bun:"user_uuid"`
	Username string         `bun:"username"`
	Nickname string         `bun:"nickname"`
	Email    string         `bun:"email"`
	Avatar   string         `bun:"avatar"`
	UserRole types.UserRole `bun:"user_role"`
}

// ListMembers returns users assigned directly to one organization.
func (s *organizationUnitMemberStoreImpl) ListMembers(ctx context.Context, input ListOrganizationMembersInput) ([]types.OrganizationMember, int, error) {
	base := s.db.Core.NewSelect().
		Model((*Member)(nil)).
		Join("JOIN users AS u ON u.id = member.user_id AND u.deleted_at IS NULL").
		Join("JOIN organizations AS organization ON organization.id = member.organization_id AND organization.is_unit = TRUE AND organization.deleted_at IS NULL").
		Where("member.organization_id = ? AND member.deleted_at IS NULL", input.OrganizationID)
	if input.Role != "" {
		base = base.Where("member.role = ?", input.Role)
	}
	if search := strings.TrimSpace(input.Search); search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		base = base.Where("(LOWER(u.username) LIKE ? OR LOWER(u.name) LIKE ? OR LOWER(u.email) LIKE ?)", pattern, pattern, pattern)
	}
	total, err := base.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	var rows []organizationMemberRow
	err = base.ColumnExpr("u.uuid AS user_uuid, u.username, u.name AS nickname, COALESCE(u.email, '') AS email, COALESCE(u.avatar, '') AS avatar, member.role AS user_role").
		OrderExpr("LOWER(u.username) ASC, u.uuid ASC").
		Limit(input.Per).Offset((input.Page-1)*input.Per).
		Scan(ctx, &rows)
	return convertOrganizationMembers(rows), total, err
}

// convertOrganizationMembers maps query rows to common response types.
func convertOrganizationMembers(rows []organizationMemberRow) []types.OrganizationMember {
	result := make([]types.OrganizationMember, 0, len(rows))
	for _, row := range rows {
		result = append(result, types.OrganizationMember{
			UserUUID: row.UserUUID, Username: row.Username, Nickname: row.Nickname, Email: row.Email, Avatar: row.Avatar,
			UserRole: row.UserRole,
		})
	}
	return result
}

// FindRole returns one user's active role in exactly one organization.
func (s *organizationUnitMemberStoreImpl) FindRole(ctx context.Context, organizationID, userID int64) (types.UserRole, error) {
	var role string
	err := s.db.Core.NewSelect().Model((*Member)(nil)).Column("role").
		Where("organization_id = ? AND user_id = ?", organizationID, userID).
		Scan(ctx, &role)
	return types.UserRole(role), errorx.HandleDBError(err, errorx.Ctx().Set("user_id", userID).Set("organization_id", organizationID))
}

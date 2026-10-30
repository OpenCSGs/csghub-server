package database

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

// Role stores metadata for both administrator and ordinary roles.
type Role struct {
	bun.BaseModel `bun:"table:roles,alias:ar"`

	RoleCode    string `bun:",pk,notnull" json:"role_code"`
	DisplayName string `bun:",notnull" json:"display_name"`
	Description string `bun:",notnull,default:''" json:"description"`
	// IsAdmin distinguishes administrator roles from ordinary roles.
	IsAdmin bool `bun:",notnull" json:"is_admin"`
	// IsBuiltin identifies roles supplied by the system.
	IsBuiltin bool `bun:",notnull" json:"is_builtin"`
	// CanDelete controls deletion of the role definition, not removal of user assignments.
	CanDelete bool `bun:",notnull" json:"can_delete"`

	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
	UserID    int64     `bun:",notnull" json:"user_id"`
	// Permissions contains the grants stored for this role.
	Permissions []AdminRolePermission `bun:"rel:has-many,join:role_code=role_code" json:"-"`
}

// AdminRolePermission records one permission granted to an administrator role.
// A role with multiple permissions has multiple rows in admin_role_permissions.
type AdminRolePermission struct {
	bun.BaseModel `bun:"table:admin_role_permissions,alias:arp"`

	RoleCode       string                `bun:",pk,notnull" json:"role_code"`
	PermissionCode types.AdminPermission `bun:",pk,notnull" json:"permission_code"`
	CreatedAt      time.Time             `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
}

// AdminRoleStore reads role definitions and checks administrator membership and grants.
type AdminRoleStore interface {
	// List returns every administrator role and its grants, newest first with role code as a tiebreaker.
	List(ctx context.Context) ([]types.AdminRoleDetail, error)
	// ListByUsername returns only an active user's active administrator roles and their grants.
	ListByUsername(ctx context.Context, username string) ([]types.AdminRoleDetail, error)
	// HasPermission checks whether an active user's roles grant any required permission.
	// AdminPermissionAny only requires active membership in an existing administrator role.
	// AdminPermissionAll requires a global wildcard grant. Other permissions must be concrete and valid.
	// An empty list denies access, and every supplied permission is validated before querying.
	HasPermission(ctx context.Context, username string, required ...types.AdminPermission) (bool, error)
}

type adminRoleStoreImpl struct {
	db *DB
}

// NewAdminRoleStore creates a store using the process database connection.
func NewAdminRoleStore() AdminRoleStore {
	return NewAdminRoleStoreWithDB(defaultDB)
}

// NewAdminRoleStoreWithDB creates a store using the provided database connection.
func NewAdminRoleStoreWithDB(db *DB) AdminRoleStore {
	return &adminRoleStoreImpl{db: db}
}

// List loads only administrator roles newest first, including roles with no permissions.
func (s *adminRoleStoreImpl) List(ctx context.Context) ([]types.AdminRoleDetail, error) {
	return s.list(ctx, s.db.Core.NewSelect())
}

// ListByUsername scopes role discovery to active assignments owned by the supplied user.
// An empty username returns an empty list without querying the role catalog.
func (s *adminRoleStoreImpl) ListByUsername(ctx context.Context, username string) ([]types.AdminRoleDetail, error) {
	if username == "" {
		return []types.AdminRoleDetail{}, nil
	}
	assignments := s.db.Core.NewSelect().Model((*UserRole)(nil)).ColumnExpr("1").
		Join("JOIN users AS u ON u.id = ur.user_id").
		Where("ur.role_code = ar.role_code").
		Where("ur.deleted_at IS NULL").
		Where("u.username = ? AND u.deleted_at IS NULL", username)
	return s.list(ctx, s.db.Core.NewSelect().Where("EXISTS (?)", assignments))
}

// list projects administrator metadata and batch-loads grants for the selected roles only.
func (s *adminRoleStoreImpl) list(ctx context.Context, query *bun.SelectQuery) ([]types.AdminRoleDetail, error) {
	var roles []Role
	if err := query.Model(&roles).Where("ar.is_admin = ?", true).Order("created_at DESC", "role_code ASC").Scan(ctx); err != nil {
		return nil, fmt.Errorf("list administrator roles: %w", err)
	}
	result := make([]types.AdminRoleDetail, 0, len(roles))
	if len(roles) == 0 {
		return result, nil
	}
	indexes := make(map[string]int, len(roles))
	roleCodes := make([]string, 0, len(roles))
	for _, role := range roles {
		roleCodes = append(roleCodes, role.RoleCode)
		indexes[role.RoleCode] = len(result)
		result = append(result, types.AdminRoleDetail{
			AdminRoleSummary: types.AdminRoleSummary{
				RoleCode: role.RoleCode, DisplayName: role.DisplayName, Description: role.Description,
			},
			IsAdmin: role.IsAdmin, IsBuiltin: role.IsBuiltin, CanDelete: role.CanDelete,
			CreatedAt: role.CreatedAt, UserID: role.UserID, Permissions: []types.AdminPermission{},
		})
	}
	var grants []AdminRolePermission
	if err := s.db.Core.NewSelect().Model(&grants).Where("arp.role_code IN (?)", bun.In(roleCodes)).
		Order("role_code ASC", "permission_code ASC").Scan(ctx); err != nil {
		return nil, fmt.Errorf("list administrator role permissions: %w", err)
	}
	for _, grant := range grants {
		if index, exists := indexes[grant.RoleCode]; exists {
			result[index].Permissions = append(result[index].Permissions, grant.PermissionCode)
		}
	}
	return result, nil
}

// hasAdminRole checks membership without requiring a grant for a particular module.
// An active assignment must match a role definition with IsAdmin set.
func (s *adminRoleStoreImpl) hasAdminRole(ctx context.Context, username string) (bool, error) {
	allowed, err := s.db.Operator.Core.NewSelect().
		Model((*Role)(nil)).
		ColumnExpr("1").
		Join("JOIN user_roles AS ur ON ur.role_code = ar.role_code").
		Join("JOIN users AS u ON u.id = ur.user_id").
		Where("u.username = ? AND u.deleted_at IS NULL", username).
		Where("ur.deleted_at IS NULL").
		Where("ar.is_admin = ?", true).
		Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("check admin role for user %q: %w", username, err)
	}
	return allowed, nil
}

// HasPermission reports whether an active user has an active administrator role
// granting any required permission or a matching parent wildcard in one query.
// AdminPermissionAny accepts any existing administrator role without querying grants.
// AdminPermissionAll requires a stored global wildcard and does not infer privileges from role names.
func (s *adminRoleStoreImpl) HasPermission(ctx context.Context, username string, required ...types.AdminPermission) (bool, error) {
	candidates, err := adminPermissionCandidates(required...)
	if err != nil {
		return false, err
	}
	if username == "" || len(required) == 0 {
		return false, nil
	}
	if slices.Contains(required, types.AdminPermissionAny) {
		return s.hasAdminRole(ctx, username)
	}

	allowed, err := s.db.Operator.Core.NewSelect().
		Model((*AdminRolePermission)(nil)).
		ColumnExpr("1").
		Join("JOIN roles AS ar ON ar.role_code = arp.role_code").
		Join("JOIN user_roles AS ur ON ur.role_code = arp.role_code").
		Join("JOIN users AS u ON u.id = ur.user_id").
		Where("u.username = ? AND u.deleted_at IS NULL", username).
		Where("ur.deleted_at IS NULL").
		Where("ar.is_admin = ?", true).
		Where("arp.permission_code IN (?)", bun.In(candidates)).
		Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("check admin permission for user %q: %w", username, err)
	}
	return allowed, nil
}

// adminPermissionCandidates validates route requirements and deduplicates their grant candidates.
// AdminPermissionAny is a membership requirement and produces no stored grant candidates.
// AdminPermissionAll produces only the global wildcard candidate.
func adminPermissionCandidates(required ...types.AdminPermission) ([]string, error) {
	if err := types.ValidateAdminPermissions(required...); err != nil {
		return nil, err
	}
	var candidates []string
	seen := make(map[string]struct{})
	for _, permission := range required {
		if permission == types.AdminPermissionAny || permission == types.AdminPermissionAll {
			continue
		}
		parts := strings.Split(string(permission), "/")

		for i := len(parts); i > 0; i-- {
			candidate := strings.Join(parts[:i], "/")
			if i < len(parts) {
				candidate += "/*"
			}
			if _, exists := seen[candidate]; exists {
				continue
			}
			seen[candidate] = struct{}{}
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) > 0 || slices.Contains(required, types.AdminPermissionAll) {
		candidates = append(candidates, string(types.AdminPermissionAll))
	}
	return candidates, nil
}

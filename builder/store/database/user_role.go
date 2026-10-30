package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// UserRole records one role assigned to a user, including non-admin roles.
// A role is administrative only when its definition in roles has IsAdmin set.
type UserRole struct {
	bun.BaseModel `bun:"table:user_roles,alias:ur"`

	ID             int64     `bun:",pk,autoincrement" json:"id"`
	UserID         int64     `bun:",notnull" json:"user_id"`
	RoleCode       string    `bun:",notnull" json:"role_code"`
	OperatorUserID *int64    `json:"operator_user_id,omitempty"`
	CreatedAt      time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
	DeletedAt      time.Time `bun:",soft_delete,nullzero" json:"deleted_at,omitempty"`
	// Role is optional because legacy assignments may have no role definition.
	Role *Role `bun:"rel:belongs-to,join:role_code=role_code" json:"-"`
}

// UserRoleStore manages administrator assignments while preserving unrelated user roles.
type UserRoleStore interface {
	// ListAdminUsers returns distinct active administrators and their roles, optionally restricted to global grants.
	ListAdminUsers(ctx context.Context, req types.AdminUserListRequest) ([]types.AdminUser, int, error)
	// AssignAdminRole creates an active assignment once and records the authenticated operator.
	AssignAdminRole(ctx context.Context, req types.AdminUserRoleRequest) error
	// RemoveAdminRole revokes an active assignment without removing its audit history.
	RemoveAdminRole(ctx context.Context, req types.AdminUserRoleRequest) error
}

// userRoleStoreImpl persists role assignments using the shared database connection.
type userRoleStoreImpl struct {
	db *DB
}

// NewUserRoleStore creates a role-assignment store using the process database.
func NewUserRoleStore() UserRoleStore {
	return NewUserRoleStoreWithDB(defaultDB)
}

// NewUserRoleStoreWithDB creates a role-assignment store with an explicit connection.
func NewUserRoleStoreWithDB(db *DB) UserRoleStore {
	return &userRoleStoreImpl{db: db}
}

// ListAdminUsers applies role and global-grant filters before pagination and projects matching administrator roles.
func (s *userRoleStoreImpl) ListAdminUsers(ctx context.Context, req types.AdminUserListRequest) ([]types.AdminUser, int, error) {
	if req.GlobalOnly && req.RoleCode != "" {
		allowed, err := s.db.Core.NewSelect().Model((*Role)(nil)).ColumnExpr("1").
			Where("ar.role_code = ?", req.RoleCode).Where("ar.is_admin = ?", true).
			Where("EXISTS (?)", globalAdminRoleGrantQuery(s.db.Core)).Exists(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("validate global administrator role %q: %w", req.RoleCode, err)
		}
		if !allowed {
			return nil, 0, errorx.ErrForbidden
		}
	}
	var users []struct {
		UserID   int64
		UUID     string
		Username string
		Nickname string
		Avatar   string
	}
	roles := s.db.Core.NewSelect().Model((*UserRole)(nil)).ColumnExpr("1").
		Join("JOIN roles AS ar ON ar.role_code = ur.role_code").
		Where("ar.is_admin = ?", true).
		Where("ur.user_id = u.id").Where("ur.deleted_at IS NULL")
	if req.RoleCode != "" {
		roles.Where("ur.role_code = ?", req.RoleCode)
	}
	if req.GlobalOnly {
		roles.Where("EXISTS (?)", globalAdminRoleGrantQuery(s.db.Core))
	}
	query := s.db.Core.NewSelect().TableExpr("users AS u").
		ColumnExpr("u.id AS user_id, u.uuid, u.username, u.name AS nickname, u.avatar").
		Where("u.deleted_at IS NULL").Where("EXISTS (?)", roles).
		OrderExpr("u.id DESC").Limit(req.Per).Offset((req.Page - 1) * req.Per)
	total, err := query.ScanAndCount(ctx, &users)
	if err != nil {
		return nil, 0, fmt.Errorf("list administrator users: %w", err)
	}
	result := make([]types.AdminUser, 0, len(users))
	if len(users) == 0 {
		return result, total, nil
	}
	for _, user := range users {
		result = append(result, types.AdminUser{
			UserID: user.UserID, UUID: user.UUID, Username: user.Username,
			Nickname: user.Nickname, Avatar: user.Avatar, Roles: []types.AdminRoleSummary{},
		})
	}
	if err := s.loadAdminUserRoles(ctx, result, req.GlobalOnly); err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

// loadAdminUserRoles batch-loads active administrator summaries and optionally excludes roles without a global grant.
func (s *userRoleStoreImpl) loadAdminUserRoles(ctx context.Context, users []types.AdminUser, globalOnly bool) error {
	ids := make([]int64, 0, len(users))
	indexes := make(map[int64]int, len(users))
	for i, user := range users {
		ids = append(ids, user.UserID)
		indexes[user.UserID] = i
	}
	var roles []struct {
		UserID      int64
		RoleCode    string
		DisplayName string
		Description string
	}
	query := s.db.Core.NewSelect().Model((*UserRole)(nil)).
		ColumnExpr("ur.user_id, ar.role_code, ar.display_name, ar.description").
		Join("JOIN roles AS ar ON ar.role_code = ur.role_code").
		Where("ar.is_admin = ?", true).
		Where("ur.user_id IN (?)", bun.In(ids)).Where("ur.deleted_at IS NULL").
		OrderExpr("ur.user_id ASC, ar.role_code ASC")
	if globalOnly {
		query.Where("EXISTS (?)", globalAdminRoleGrantQuery(s.db.Core))
	}
	if err := query.Scan(ctx, &roles); err != nil {
		return fmt.Errorf("list administrator user roles: %w", err)
	}
	for _, role := range roles {
		index := indexes[role.UserID]
		users[index].Roles = append(users[index].Roles, types.AdminRoleSummary{
			RoleCode: role.RoleCode, DisplayName: role.DisplayName, Description: role.Description,
		})
	}
	return nil
}

// globalAdminRoleGrantQuery correlates a stored global grant with the enclosing roles alias ar.
// EXISTS keeps users and role summaries distinct when a role has multiple grants.
func globalAdminRoleGrantQuery(db bun.IDB) *bun.SelectQuery {
	return db.NewSelect().Model((*AdminRolePermission)(nil)).ColumnExpr("1").
		Where("arp.role_code = ar.role_code").Where("arp.permission_code = ?", types.AdminPermissionAll)
}

// AssignAdminRole uses the active-assignment unique index to make repeated grants idempotent.
// A revoked assignment remains historical; a later grant creates a new row.
func (s *userRoleStoreImpl) AssignAdminRole(ctx context.Context, req types.AdminUserRoleRequest) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		operatorID, err := adminRoleOperator(ctx, tx, req)
		if err != nil {
			return err
		}
		role := &UserRole{UserID: req.UserID, RoleCode: req.RoleCode, OperatorUserID: &operatorID}
		_, err = tx.NewInsert().Model(role).
			On("CONFLICT (user_id, role_code) WHERE deleted_at IS NULL DO NOTHING").Exec(ctx)
		if err != nil {
			return fmt.Errorf("assign administrator role %q to user %d: %w", req.RoleCode, req.UserID, err)
		}
		return nil
	})
}

// RemoveAdminRole retains an account administrator while revoking an active assignment and recording the operator.
// Read-committed snapshots observe revocations committed by the previous PostgreSQL lock holder.
func (s *userRoleStoreImpl) RemoveAdminRole(ctx context.Context, req types.AdminUserRoleRequest) error {
	return s.db.Core.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted}, func(ctx context.Context, tx bun.Tx) error {
		if err := lockAdminRoleRevocation(ctx, tx); err != nil {
			return err
		}
		operatorID, err := adminRoleOperator(ctx, tx, req)
		if err != nil {
			return err
		}
		if err := ensureAccountAdminRemains(ctx, tx, req); err != nil {
			return err
		}
		_, err = tx.NewUpdate().Model((*UserRole)(nil)).
			Set("deleted_at = CURRENT_TIMESTAMP").Set("operator_user_id = ?", operatorID).
			Where("user_id = ? AND role_code = ?", req.UserID, req.RoleCode).
			Where("deleted_at IS NULL").Exec(ctx)
		if err != nil {
			return fmt.Errorf("remove administrator role %q from user %d: %w", req.RoleCode, req.UserID, err)
		}
		return nil
	})
}

// adminRoleRevocationLockKey identifies the shared administrator revocation and user deletion lock ("CSGADMIN").
const adminRoleRevocationLockKey int64 = 0x43534741444D494E

// lockAdminRoleRevocation serializes role revocations and user deletions across service instances until the transaction ends.
func lockAdminRoleRevocation(ctx context.Context, tx bun.Tx) error {
	_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?)", adminRoleRevocationLockKey)
	if err != nil {
		return fmt.Errorf("lock administrator role revocation: %w", err)
	}
	return nil
}

// accountAdminAssignmentsQuery selects active assignments granting administrator management by actual permissions.
func accountAdminAssignmentsQuery(tx bun.Tx, candidates []string) *bun.SelectQuery {
	return tx.NewSelect().Model((*UserRole)(nil)).ColumnExpr("1").
		Join("JOIN users AS u ON u.id = ur.user_id").
		Join("JOIN roles AS ar ON ar.role_code = ur.role_code").
		Join("JOIN admin_role_permissions AS arp ON arp.role_code = ar.role_code").
		Where("u.deleted_at IS NULL").Where("ur.deleted_at IS NULL").
		Where("ar.is_admin = ?", true).
		Where("arp.permission_code IN (?)", bun.In(candidates))
}

// ensureAccountAdminRemains permits absent or unrelated assignments and checks all remaining users and roles.
// GlobalOnly counts only global grants, matching the caller's administrator management policy.
// The caller must hold the revocation lock so concurrent removals cannot both pass this check.
func ensureAccountAdminRemains(ctx context.Context, tx bun.Tx, req types.AdminUserRoleRequest) error {
	required := types.AdminAccountAdmin
	if req.GlobalOnly {
		required = types.AdminPermissionAll
	}
	candidates, err := adminPermissionCandidates(required)
	if err != nil {
		return err
	}
	affected, err := accountAdminAssignmentsQuery(tx, candidates).
		Where("ur.user_id = ? AND ur.role_code = ?", req.UserID, req.RoleCode).Exists(ctx)
	if err != nil {
		return fmt.Errorf("check revoked administrator management grant: %w", err)
	}
	if !affected {
		return nil
	}
	remaining, err := accountAdminAssignmentsQuery(tx, candidates).
		Where("NOT (ur.user_id = ? AND ur.role_code = ?)", req.UserID, req.RoleCode).Exists(ctx)
	if err != nil {
		return fmt.Errorf("check remaining administrator management grants: %w", err)
	}
	if !remaining {
		return errorx.ErrLastAccountAdmin
	}
	return nil
}

// protectAdministratorFromUserDeletion rechecks live grants before any deletion side effects.
// The shared revocation lock prevents concurrent role removals; the user row lock blocks new assignments.
// A deleted user is not an active administrator and may still be purged by existing cleanup workflows.
func protectAdministratorFromUserDeletion(ctx context.Context, tx bun.Tx, userID int64) error {
	if err := lockAdminRoleRevocation(ctx, tx); err != nil {
		return err
	}
	var lockedID int64
	if err := tx.NewSelect().Model((*User)(nil)).Column("id").WhereAllWithDeleted().
		Where("id = ?", userID).For("UPDATE").Scan(ctx, &lockedID); err != nil {
		return fmt.Errorf("lock user for administrator deletion protection: %w", err)
	}
	required := config.PlatformAdminPermissions([]types.AdminPermission{types.AdminAccountAdmin})
	candidates, err := adminPermissionCandidates(required...)
	if err != nil {
		return err
	}
	affected, err := accountAdminAssignmentsQuery(tx, candidates).Where("ur.user_id = ?", userID).Exists(ctx)
	if err != nil {
		return fmt.Errorf("check deleted user's administrator management grants: %w", err)
	}
	if affected {
		remaining, err := accountAdminAssignmentsQuery(tx, candidates).Where("ur.user_id <> ?", userID).Exists(ctx)
		if err != nil {
			return fmt.Errorf("check administrator management grants after user deletion: %w", err)
		}
		if !remaining {
			return errorx.ErrLastAccountAdmin
		}
	}
	administrator, err := tx.NewSelect().Model((*UserRole)(nil)).ColumnExpr("1").
		Join("JOIN users AS u ON u.id = ur.user_id").
		Join("JOIN roles AS ar ON ar.role_code = ur.role_code").
		Where("ur.user_id = ? AND ur.deleted_at IS NULL", userID).
		Where("u.deleted_at IS NULL AND ar.is_admin = ?", true).Exists(ctx)
	if err != nil {
		return fmt.Errorf("check administrator membership before user deletion: %w", err)
	}
	if administrator {
		return errorx.ErrAdminUserCannotBeDeleted
	}
	return nil
}

// adminRoleOperator validates mutation references and any global-grant requirement in the transaction before writes.
func adminRoleOperator(ctx context.Context, tx bun.Tx, req types.AdminUserRoleRequest) (int64, error) {
	exists, err := tx.NewSelect().Model((*User)(nil)).ColumnExpr("1").Where("id = ?", req.UserID).Exists(ctx)
	if err != nil {
		return 0, fmt.Errorf("find administrator target user %d: %w", req.UserID, err)
	}
	if !exists {
		return 0, fmt.Errorf("administrator target user %d: %w", req.UserID, errorx.ErrNotFound)
	}
	var operatorID int64
	err = tx.NewSelect().Model((*User)(nil)).Column("id").Where("username = ?", req.CurrentUser).Scan(ctx, &operatorID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errorx.ErrUnauthorized
	}
	if err != nil {
		return 0, fmt.Errorf("resolve administrator role operator: %w", err)
	}
	role := tx.NewSelect().Model((*Role)(nil)).ColumnExpr("1").
		Where("ar.role_code = ?", req.RoleCode).Where("ar.is_admin = ?", true)
	if req.GlobalOnly {
		role.Where("EXISTS (?)", globalAdminRoleGrantQuery(tx))
	}
	exists, err = role.Exists(ctx)
	if err != nil {
		return 0, fmt.Errorf("find administrator role %q: %w", req.RoleCode, err)
	}
	if !exists {
		if req.GlobalOnly {
			return 0, errorx.ErrForbidden
		}
		return 0, fmt.Errorf("administrator role %q: %w", req.RoleCode, errorx.ErrNotFound)
	}
	return operatorID, nil
}

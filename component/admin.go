package component

import (
	"context"
	"fmt"
	"slices"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// AdminComponent manages the administrator role catalog and user assignments.
// Route middleware authorizes callers; stores validate role and user existence within write transactions.
type AdminComponent interface {
	// CheckPermissions applies the same edition and grant policy as PlatformAdmin.
	CheckPermissions(ctx context.Context, username string, req types.AdminPermissionCheckRequest) (bool, error)
	// Roles returns administrator definitions supported by the current edition.
	Roles(ctx context.Context) ([]types.AdminRoleDetail, error)
	// CurrentRoles returns the authenticated user's active administrator roles and grants.
	CurrentRoles(ctx context.Context, username string) ([]types.AdminRoleDetail, error)
	// Users returns a page of administrators with the edition-supported active roles.
	Users(ctx context.Context, req types.AdminUserListRequest) ([]types.AdminUser, int, error)
	// AssignRole adds an administrator role idempotently and records the authenticated operator.
	AssignRole(ctx context.Context, req types.AdminUserRoleRequest) error
	// RemoveRole revokes an administrator role idempotently without changing other user roles.
	RemoveRole(ctx context.Context, req types.AdminUserRoleRequest) error
}

// adminComponentImpl coordinates role definitions and transactional user role assignments.
type adminComponentImpl struct {
	adminRoles database.AdminRoleStore
	userRoles  database.UserRoleStore
}

// NewAdminComponent creates the administrator role management service.
func NewAdminComponent() AdminComponent {
	return &adminComponentImpl{
		adminRoles: database.NewAdminRoleStore(),
		userRoles:  database.NewUserRoleStore(),
	}
}

// CheckPermissions queries current assignments on every call; no cross-request grant cache is used.
func (c *adminComponentImpl) CheckPermissions(ctx context.Context, username string, req types.AdminPermissionCheckRequest) (bool, error) {
	if username == "" {
		return false, errorx.ErrUnauthorized
	}
	if len(req.Permissions) == 0 || len(req.Permissions) > 100 {
		return false, errorx.ReqParamInvalid(fmt.Errorf("permissions must contain between 1 and 100 requirements"), errorx.Ctx().Set("param", "permissions"))
	}
	if err := types.ValidateAdminPermissions(req.Permissions...); err != nil {
		return false, errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "permissions"))
	}
	return c.adminRoles.HasPermission(ctx, username, config.PlatformAdminPermissions(req.Permissions)...)
}

// Roles retrieves administrator definitions supported by the edition, using their stored grants.
func (c *adminComponentImpl) Roles(ctx context.Context) ([]types.AdminRoleDetail, error) {
	roles, err := c.adminRoles.List(ctx)
	if err != nil || config.SupportsMultipleAdminRoles() {
		return roles, err
	}
	supported := make([]types.AdminRoleDetail, 0, len(roles))
	for _, role := range roles {
		if isGlobalAdminRole(role) {
			supported = append(supported, role)
		}
	}
	return supported, nil
}

// isGlobalAdminRole checks the stored wildcard capability rather than relying on a role name.
func isGlobalAdminRole(role types.AdminRoleDetail) bool {
	return role.IsAdmin && slices.Contains(role.Permissions, types.AdminPermissionAll)
}

// CurrentRoles rejects missing identity and users without active administrator roles.
func (c *adminComponentImpl) CurrentRoles(ctx context.Context, username string) ([]types.AdminRoleDetail, error) {
	if username == "" {
		return nil, errorx.ErrUnauthorized
	}
	roles, err := c.adminRoles.ListByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	// Editions requiring global administration only expose roles with a stored wildcard grant.
	if !config.SupportsMultipleAdminRoles() {
		global := make([]types.AdminRoleDetail, 0, len(roles))
		for _, role := range roles {
			if isGlobalAdminRole(role) {
				global = append(global, role)
			}
		}
		roles = global
	}
	if len(roles) == 0 {
		return nil, errorx.ErrForbidden
	}
	return roles, nil
}

// Users applies the edition's grant scope before database pagination and role projection.
func (c *adminComponentImpl) Users(ctx context.Context, req types.AdminUserListRequest) ([]types.AdminUser, int, error) {
	req.GlobalOnly = !config.SupportsMultipleAdminRoles()
	return c.userRoles.ListAdminUsers(ctx, req)
}

// AssignRole preserves assignment history and checks edition-supported grants inside the write transaction.
func (c *adminComponentImpl) AssignRole(ctx context.Context, req types.AdminUserRoleRequest) error {
	req.GlobalOnly = !config.SupportsMultipleAdminRoles()
	return c.userRoles.AssignAdminRole(ctx, req)
}

// RemoveRole revokes supported assignments idempotently after transactional grant validation.
func (c *adminComponentImpl) RemoveRole(ctx context.Context, req types.AdminUserRoleRequest) error {
	req.GlobalOnly = !config.SupportsMultipleAdminRoles()
	return c.userRoles.RemoveAdminRole(ctx, req)
}

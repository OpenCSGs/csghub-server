package middleware

import (
	"log/slog"
	"slices"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// NewPlatformAdmin creates a gate factory using the edition's administrator policy.
// CE requires the global wildcard grant; EE and SaaS accept any requested permission.
// AdminPermissionAny preserves business authorization in every edition.
func NewPlatformAdmin() func(...types.AdminPermission) gin.HandlerFunc {
	return newPlatformAdmin(database.NewAdminRoleStore())
}

// newPlatformAdmin constructs the gate factory with an injectable permission store.
func newPlatformAdmin(store database.AdminRoleStore) func(...types.AdminPermission) gin.HandlerFunc {
	return func(permissions ...types.AdminPermission) gin.HandlerFunc {
		permissions = slices.Clone(permissions)
		permissionErr := types.ValidateAdminPermissions(permissions...)
		lookupPermissions := config.PlatformAdminPermissions(permissions)
		return func(ctx *gin.Context) {
			// Authenticator sets the current user for every supported credential type.
			username := httpbase.GetCurrentUser(ctx)
			if username == "" {
				httpbase.UnauthorizedError(ctx, errorx.ErrUnauthorized)
				ctx.Abort()
				return
			}

			// An empty permission list must never authorize an administrator request.
			if len(permissions) == 0 {
				httpbase.ForbiddenError(ctx, errorx.ErrForbidden)
				ctx.Abort()
				return
			}

			var allowed bool
			err := permissionErr
			if err == nil {
				allowed, err = store.HasPermission(ctx.Request.Context(), username, lookupPermissions...)
			}
			if err != nil {
				slog.ErrorContext(ctx.Request.Context(), "failed to check admin permission", "permissions", permissions, "error", err)
				httpbase.ServerError(ctx, errorx.ErrInternalServerError)
				ctx.Abort()
				return
			}
			if !allowed {
				httpbase.ForbiddenError(ctx, errorx.ErrForbidden)
				ctx.Abort()
				return
			}

			// Any preserves business authorization even when CE requires a global grant.
			// Only explicit module or global requirements permit a business-permission bypass.
			if !slices.Contains(permissions, types.AdminPermissionAny) {
				ctx.Request = ctx.Request.WithContext(types.WithAdminAuthorization(ctx.Request.Context()))
			}
			ctx.Next()
		}
	}
}

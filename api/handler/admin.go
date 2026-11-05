package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
)

// AdminHandler exposes administrator roles and user role assignments.
type AdminHandler struct {
	admin component.AdminComponent
}

// NewAdminHandler initializes administrator management with the default component.
func NewAdminHandler() *AdminHandler {
	return &AdminHandler{admin: component.NewAdminComponent()}
}

// CheckPermissions checks the authenticated operator against caller-supplied route requirements.
// @Security ApiKey
// @Summary Check current user's administrator permissions
// @Description Requires login only at the route. JSON body is limited to 64 KiB. Checks live administrator grants with PlatformAdmin edition policy: CE requires global *; EE/SaaS use OR for multiple requirements. Does not authorize subsequent requests or set business bypass markers.
// @Tags Admin Account Administrator
// @Accept json
// @Produce json
// @Param body body types.AdminPermissionCheckRequest true "Non-empty route requirements; any, *, or concrete module/submodule codes"
// @Success 200 {object} types.Response{data=types.AdminPermissionCheckResult}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/admin/me/permissions/check [post]
func (h *AdminHandler) CheckPermissions(ctx *gin.Context) {
	username := httpbase.GetCurrentUser(ctx)
	if username == "" {
		respondAdminError(ctx, errorx.ErrUnauthorized)
		return
	}
	// Authorization requests contain only a small list of codes; bound decoding before allocation.
	body, err := io.ReadAll(http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 64<<10))
	if err != nil {
		respondAdminError(ctx, errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "permissions")))
		return
	}
	var req types.AdminPermissionCheckRequest
	if err := json.Unmarshal(body, &req); err != nil {
		respondAdminError(ctx, errorx.ReqParamInvalid(err, errorx.Ctx().Set("param", "permissions")))
		return
	}
	allowed, err := h.admin.CheckPermissions(ctx.Request.Context(), username, req)
	if err != nil {
		respondAdminError(ctx, err)
		return
	}
	if !allowed {
		respondAdminError(ctx, errorx.ErrForbidden)
		return
	}
	httpbase.OK(ctx, types.AdminPermissionCheckResult{Allowed: true})
}

// Roles lists administrator role definitions and their permissions.
// @Security ApiKey
// @Summary List administrator roles
// @Description Requires AdminPermissionAny with edition policy. CE returns administrator roles carrying a global * grant regardless of their names; EE/SaaS return the complete administrator catalog.
// @Tags Admin Account Administrator
// @Produce json
// @Success 200 {object} types.Response{data=[]types.AdminRoleDetail}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/admin/roles [get]
func (h *AdminHandler) Roles(ctx *gin.Context) {
	roles, err := h.admin.Roles(ctx.Request.Context())
	if err != nil {
		respondAdminError(ctx, err)
		return
	}
	httpbase.OK(ctx, roles)
}

// CurrentRoles lists the authenticated user's administrator roles and their permissions.
// @Security ApiKey
// @Summary List current user's administrator roles
// @Description Requires AdminPermissionAny. Identity comes only from the authenticated request context; revoked assignments and ordinary roles are excluded. CE returns only roles carrying an active global * grant.
// @Tags Admin Account Administrator
// @Produce json
// @Success 200 {object} types.Response{data=[]types.AdminRoleDetail}
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/admin/me/roles [get]
func (h *AdminHandler) CurrentRoles(ctx *gin.Context) {
	roles, err := h.admin.CurrentRoles(ctx.Request.Context(), httpbase.GetCurrentUser(ctx))
	if err != nil {
		respondAdminError(ctx, err)
		return
	}
	httpbase.OK(ctx, roles)
}

// Users lists administrator users with their assigned administrator roles.
// @Security ApiKey
// @Summary List administrator users
// @Description Requires account/admin administrator permission. EE/SaaS retain all roles when filtering. CE accepts any global administrator role filter and returns only roles carrying *; filtering module-only roles is forbidden.
// @Tags Admin Account Administrator
// @Produce json
// @Param role_code query string false "Administrator role code, at most 64 characters without surrounding whitespace"
// @Param page query int false "Page number" default(1) minimum(1)
// @Param per query int false "Page size" default(50) minimum(1) maximum(100)
// @Success 200 {object} types.ResponseWithTotal{data=[]types.AdminUser}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/admin/users [get]
func (h *AdminHandler) Users(ctx *gin.Context) {
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		respondAdminError(ctx, err)
		return
	}
	if page-1 > math.MaxInt/per {
		respondAdminError(ctx, errorx.ReqParamInvalid(errors.New("pagination offset is too large"), errorx.Ctx().Set("param", "page")))
		return
	}
	roleCode := ctx.Query("role_code")
	if roleCode != "" {
		if err := validateAdminRoleCode(roleCode); err != nil {
			respondAdminError(ctx, err)
			return
		}
	}
	users, total, err := h.admin.Users(ctx.Request.Context(), types.AdminUserListRequest{
		RoleCode: roleCode,
		Page:     page,
		Per:      per,
	})
	if err != nil {
		respondAdminError(ctx, err)
		return
	}
	httpbase.OKWithTotal(ctx, users, total)
}

// AssignRole assigns an administrator role to a user without duplicating an existing assignment.
// @Security ApiKey
// @Summary Assign an administrator role
// @Description Requires account/admin administrator permission. The operator is taken from the authenticated request context. Repeated assignment succeeds. CE can assign only administrator roles with a global * grant, regardless of their names.
// @Tags Admin Account Administrator
// @Produce json
// @Param user_id path int true "Positive user ID" minimum(1)
// @Param role_code path string true "Administrator role code, at most 64 characters without surrounding whitespace"
// @Success 200 {object} types.Response{data=nil}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} types.APINotFound
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/admin/users/{user_id}/roles/{role_code} [put]
func (h *AdminHandler) AssignRole(ctx *gin.Context) {
	req, err := parseAdminUserRoleRequest(ctx)
	if err != nil {
		respondAdminError(ctx, err)
		return
	}
	if err := h.admin.AssignRole(ctx.Request.Context(), req); err != nil {
		respondAdminError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// RemoveRole revokes an administrator role and succeeds if that assignment is already absent.
// @Security ApiKey
// @Summary Remove an administrator role
// @Description Requires account/admin administrator permission. At least one active user must retain administrator management permission through an administrator role. EE and SaaS accept account/admin, account/* or *; CE requires a remaining global * grant. The operator is taken from the authenticated request context. Repeated removal succeeds. CE can revoke any administrator role with a global * grant, regardless of its name.
// @Tags Admin Account Administrator
// @Produce json
// @Param user_id path int true "Positive user ID" minimum(1)
// @Param role_code path string true "Administrator role code, at most 64 characters without surrounding whitespace"
// @Success 200 {object} types.Response{data=nil}
// @Failure 400 {object} types.APIBadRequest
// @Failure 401 {object} types.APIUnauthorized
// @Failure 403 {object} types.APIForbidden
// @Failure 404 {object} types.APINotFound
// @Failure 409 {object} types.APIBadRequest "The last administrator management grant cannot be revoked"
// @Failure 500 {object} types.APIInternalServerError
// @Router /admin/account/admin/users/{user_id}/roles/{role_code} [delete]
func (h *AdminHandler) RemoveRole(ctx *gin.Context) {
	req, err := parseAdminUserRoleRequest(ctx)
	if err != nil {
		respondAdminError(ctx, err)
		return
	}
	if err := h.admin.RemoveRole(ctx.Request.Context(), req); err != nil {
		respondAdminError(ctx, err)
		return
	}
	httpbase.OK(ctx, nil)
}

// validateAdminRoleCode preserves role identifiers while enforcing their storage length and whitespace rules.
func validateAdminRoleCode(roleCode string) error {
	if roleCode == "" || strings.TrimSpace(roleCode) != roleCode || utf8.RuneCountInString(roleCode) > 64 {
		return errorx.ReqParamInvalid(errors.New("role_code must contain 1 to 64 characters without surrounding whitespace"), errorx.Ctx().Set("param", "role_code"))
	}
	return nil
}

// parseAdminUserRoleRequest resolves the target from path parameters and the operator from authenticated context.
func parseAdminUserRoleRequest(ctx *gin.Context) (types.AdminUserRoleRequest, error) {
	req := types.AdminUserRoleRequest{CurrentUser: httpbase.GetCurrentUser(ctx)}
	if req.CurrentUser == "" {
		return req, errorx.ErrUnauthorized
	}
	userID, err := strconv.ParseInt(ctx.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		return req, errorx.ReqParamInvalid(errors.New("user_id must be a positive integer"), errorx.Ctx().Set("param", "user_id"))
	}
	req.UserID = userID
	req.RoleCode = ctx.Param("role_code")
	if err := validateAdminRoleCode(req.RoleCode); err != nil {
		return req, err
	}
	return req, nil
}

// respondAdminError maps administrator component failures to the public HTTP error contract.
func respondAdminError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, errorx.ErrLastAccountAdmin):
		httpbase.ConflictError(ctx, err)
	case errors.Is(err, errorx.ErrNotFound):
		httpbase.NotFoundError(ctx, err)
	case errors.Is(err, errorx.ErrUnauthorized):
		httpbase.UnauthorizedError(ctx, err)
	case errors.Is(err, errorx.ErrForbidden):
		httpbase.ForbiddenError(ctx, err)
	case errors.Is(err, errorx.ErrReqParamInvalid), errors.Is(err, errorx.ErrBadRequest):
		httpbase.BadRequestWithExt(ctx, err)
	default:
		slog.ErrorContext(ctx.Request.Context(), "administrator operation failed", slog.Any("error", err))
		httpbase.ServerError(ctx, err)
	}
}

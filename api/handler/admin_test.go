package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcom "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/api/middleware"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/i18n"
	"opencsg.com/csghub-server/common/types"
)

// newAdminHandlerTestRouter registers the actual handler methods with a controlled authenticated operator.
func newAdminHandlerTestRouter(t *testing.T, currentUser string) (*gin.Engine, *mockcom.MockAdminComponent) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	admin := mockcom.NewMockAdminComponent(t)
	h := &AdminHandler{admin: admin}
	engine := gin.New()
	engine.Use(func(ctx *gin.Context) {
		httpbase.SetCurrentUser(ctx, currentUser)
		ctx.Next()
	})
	group := engine.Group("/api/v1/admin/account/admin")
	group.GET("/roles", h.Roles)
	group.GET("/me/roles", h.CurrentRoles)
	group.POST("/me/permissions/check", h.CheckPermissions)
	group.GET("/users", h.Users)
	group.PUT("/users/:user_id/roles/:role_code", h.AssignRole)
	group.DELETE("/users/:user_id/roles/:role_code", h.RemoveRole)
	return engine, admin
}

// TestAdminHandlerRoles verifies role flags, metadata, and permission codes in the HTTP response.
func TestAdminHandlerRoles(t *testing.T) {
	engine, admin := newAdminHandlerTestRouter(t, "operator")
	admin.EXPECT().Roles(mock.Anything).Return([]types.AdminRoleDetail{{
		AdminRoleSummary: types.AdminRoleSummary{RoleCode: "asset_admin", DisplayName: "资产管理员", Description: "Asset management"},
		IsAdmin:          true,
		IsBuiltin:        true,
		CanDelete:        true,
		CreatedAt:        time.Date(2026, time.September, 29, 8, 0, 0, 0, time.UTC),
		UserID:           7,
		Permissions:      []types.AdminPermission{"asset/*", types.AdminSystemDashboard},
	}}, nil).Once()
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/account/admin/roles", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"msg":"OK","data":[{"role_code":"asset_admin","display_name":"资产管理员","description":"Asset management","is_admin":true,"is_builtin":true,"can_delete":true,"created_at":"2026-09-29T08:00:00Z","user_id":7,"permissions":["asset/*","system/dashboard"]}]}`, response.Body.String())
}

// TestAdminHandlerCurrentRoles verifies multiple roles and ignores client-supplied identity filters.
func TestAdminHandlerCurrentRoles(t *testing.T) {
	engine, admin := newAdminHandlerTestRouter(t, "operator")
	roles := []types.AdminRoleDetail{
		{AdminRoleSummary: types.AdminRoleSummary{RoleCode: "asset_admin", DisplayName: "Asset Admin", Description: "Asset management"},
			IsAdmin: true, IsBuiltin: true, CanDelete: true, Permissions: []types.AdminPermission{"asset/*"}},
		{AdminRoleSummary: types.AdminRoleSummary{RoleCode: "finance_admin", DisplayName: "Finance Admin", Description: "Finance management"},
			IsAdmin: true, IsBuiltin: true, CanDelete: true, Permissions: []types.AdminPermission{"accounting/*"}},
	}
	admin.EXPECT().CurrentRoles(mock.Anything, "operator").Return(roles, nil).Once()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/account/admin/me/roles?username=other&current_user=other&role_code=super_admin&user_id=999", nil)
	engine.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	var result struct {
		Data []types.AdminRoleDetail `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.Equal(t, roles, result.Data)
}

// TestAdminHandlerUsers verifies pagination, optional role filters, and complete role summaries in each user.
func TestAdminHandlerUsers(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query string
		want  types.AdminUserListRequest
	}{
		{"defaults", "", types.AdminUserListRequest{Page: 1, Per: 50}},
		{"role filter", "?role_code=asset_admin&page=2&per=20", types.AdminUserListRequest{RoleCode: "asset_admin", Page: 2, Per: 20}},
		{"unicode role boundary", "?role_code=" + url.QueryEscape(strings.Repeat("角", 64)), types.AdminUserListRequest{RoleCode: strings.Repeat("角", 64), Page: 1, Per: 50}},
		{"largest safe offset", "?per=100&page=" + strconv.Itoa(math.MaxInt/100+1), types.AdminUserListRequest{Page: math.MaxInt/100 + 1, Per: 100}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine, admin := newAdminHandlerTestRouter(t, "operator")
			users := []types.AdminUser{{
				UserID: 42, UUID: "user-uuid", Username: "alice", Nickname: "Alice", Avatar: "https://example.com/alice.png",
				Roles: []types.AdminRoleSummary{
					{RoleCode: "asset_admin", DisplayName: "资产管理员", Description: "Asset management"},
					{RoleCode: "sec_admin", DisplayName: "安全管理员", Description: "Security management"},
				},
			}}
			admin.EXPECT().Users(mock.Anything, tc.want).Return(users, 3, nil).Once()
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/account/admin/users"+tc.query, nil))
			require.Equal(t, http.StatusOK, response.Code)
			require.JSONEq(t, `{"msg":"OK","total":3,"data":[{"user_id":42,"uuid":"user-uuid","username":"alice","nickname":"Alice","avatar":"https://example.com/alice.png","roles":[{"role_code":"asset_admin","display_name":"资产管理员","description":"Asset management"},{"role_code":"sec_admin","display_name":"安全管理员","description":"Security management"}]}]}`, response.Body.String())
		})
	}
	t.Run("empty page", func(t *testing.T) {
		engine, admin := newAdminHandlerTestRouter(t, "operator")
		admin.EXPECT().Users(mock.Anything, types.AdminUserListRequest{Page: 1, Per: 50}).Return(nil, 0, nil).Once()
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/account/admin/users", nil))
		require.Equal(t, http.StatusOK, response.Code)
		require.JSONEq(t, `{"msg":"OK","data":[]}`, response.Body.String())
	})
}

// TestAdminHandlerInvalidUserQueries rejects malformed filters and pagination before calling the component.
func TestAdminHandlerInvalidUserQueries(t *testing.T) {
	for _, query := range []string{
		"per=0", "per=-1", "per=101", "per=abc", "page=0", "page=-1", "page=abc",
		"page=999999999999999999999999999999", "per=100&page=" + strconv.Itoa(math.MaxInt/100+2),
		"role_code=%20asset_admin", "role_code=asset_admin%20", "role_code=%20",
		"role_code=" + strings.Repeat("a", 65), "role_code=" + url.QueryEscape(strings.Repeat("角", 65)),
	} {
		t.Run(query, func(t *testing.T) {
			engine, _ := newAdminHandlerTestRouter(t, "operator")
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/admin/account/admin/users?"+query, nil))
			require.Equal(t, http.StatusBadRequest, response.Code)
		})
	}
}

// TestAdminHandlerRoleMutations keeps the target in the path and the operator in context across repeated requests.
func TestAdminHandlerRoleMutations(t *testing.T) {
	for _, operation := range []struct{ method, componentMethod string }{
		{http.MethodPut, "AssignRole"}, {http.MethodDelete, "RemoveRole"},
	} {
		for _, target := range []struct {
			userID   int64
			roleCode string
		}{{42, "asset_admin"}, {math.MaxInt64, strings.Repeat("角", 64)}} {
			t.Run(operation.method+"/"+target.roleCode, func(t *testing.T) {
				engine, admin := newAdminHandlerTestRouter(t, "real-operator")
				want := types.AdminUserRoleRequest{UserID: target.userID, RoleCode: target.roleCode, CurrentUser: "real-operator"}
				admin.On(operation.componentMethod, mock.Anything, want).Return(nil).Twice()
				path := "/api/v1/admin/account/admin/users/" + strconv.FormatInt(target.userID, 10) + "/roles/" + url.PathEscape(target.roleCode) + "?current_user=forged&user_id=999&role_code=super_admin"
				for range 2 {
					request := httptest.NewRequest(operation.method, path, strings.NewReader(`{"current_user":"forged","user_id":999,"role_code":"super_admin"}`))
					request.Header.Set("Content-Type", "application/json")
					response := httptest.NewRecorder()
					engine.ServeHTTP(response, request)
					require.Equal(t, http.StatusOK, response.Code)
					require.JSONEq(t, `{"msg":"OK"}`, response.Body.String())
				}
			})
		}
	}
}

// TestAdminHandlerInvalidRoleMutations rejects invalid targets and does not trust client-supplied operator names.
func TestAdminHandlerInvalidRoleMutations(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		for _, tc := range []struct {
			name, userID, roleCode, actor string
			status                        int
		}{
			{"missing actor", "42", "asset_admin", "", http.StatusUnauthorized},
			{"zero user", "0", "asset_admin", "operator", http.StatusBadRequest},
			{"negative user", "-1", "asset_admin", "operator", http.StatusBadRequest},
			{"invalid user", "abc", "asset_admin", "operator", http.StatusBadRequest},
			{"overflow user", "9223372036854775808", "asset_admin", "operator", http.StatusBadRequest},
			{"leading space", "42", " asset_admin", "operator", http.StatusBadRequest},
			{"trailing space", "42", "asset_admin ", "operator", http.StatusBadRequest},
			{"blank role", "42", " ", "operator", http.StatusBadRequest},
			{"long role", "42", strings.Repeat("a", 65), "operator", http.StatusBadRequest},
			{"long unicode role", "42", strings.Repeat("角", 65), "operator", http.StatusBadRequest},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				engine, _ := newAdminHandlerTestRouter(t, tc.actor)
				path := "/api/v1/admin/account/admin/users/" + tc.userID + "/roles/" + url.PathEscape(tc.roleCode) + "?current_user=forged"
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(`{"current_user":"forged"}`)))
				require.Equal(t, tc.status, response.Code)
			})
		}
	}
}

// TestAdminHandlerErrors verifies wrapped component errors retain their HTTP classifications for every operation.
func TestAdminHandlerErrors(t *testing.T) {
	for _, operation := range []struct{ method, path, componentMethod string }{
		{http.MethodGet, "/roles", "Roles"},
		{http.MethodGet, "/me/roles", "CurrentRoles"},
		{http.MethodGet, "/users", "Users"},
		{http.MethodPut, "/users/42/roles/asset_admin", "AssignRole"},
		{http.MethodDelete, "/users/42/roles/asset_admin", "RemoveRole"},
	} {
		for _, failure := range []struct {
			name   string
			err    error
			status int
		}{
			{"not found", errorx.ErrNotFound, http.StatusNotFound},
			{"unauthorized", errorx.ErrUnauthorized, http.StatusUnauthorized},
			{"forbidden", errorx.ErrForbidden, http.StatusForbidden},
			{"last account administrator", errorx.ErrLastAccountAdmin, http.StatusConflict},
			{"invalid parameter", errorx.ErrReqParamInvalid, http.StatusBadRequest},
			{"bad request", errorx.ErrBadRequest, http.StatusBadRequest},
			{"internal error", errors.New("database unavailable"), http.StatusInternalServerError},
		} {
			t.Run(operation.componentMethod+"/"+failure.name, func(t *testing.T) {
				engine, admin := newAdminHandlerTestRouter(t, "operator")
				err := fmt.Errorf("administrator operation: %w", failure.err)
				switch operation.componentMethod {
				case "Roles":
					admin.EXPECT().Roles(mock.Anything).Return(nil, err).Once()
				case "CurrentRoles":
					admin.EXPECT().CurrentRoles(mock.Anything, "operator").Return(nil, err).Once()
				case "Users":
					admin.EXPECT().Users(mock.Anything, types.AdminUserListRequest{Page: 1, Per: 50}).Return(nil, 0, err).Once()
				default:
					admin.On(operation.componentMethod, mock.Anything, types.AdminUserRoleRequest{UserID: 42, RoleCode: "asset_admin", CurrentUser: "operator"}).Return(err).Once()
				}
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, httptest.NewRequest(operation.method, "/api/v1/admin/account/admin"+operation.path, nil))
				require.Equal(t, failure.status, response.Code)
			})
		}
	}
}

// TestAdminHandlerRemoveLastAccountAdmin translates the conflict without losing its stable error code.
func TestAdminHandlerRemoveLastAccountAdmin(t *testing.T) {
	i18n.InitLocalizersFromEmbedFile()
	for _, tc := range []struct{ language, message string }{
		{"zh-CN", "无法执行此操作：至少需要保留一名有权管理管理员的用户。"},
		{"zh-HK", "無法執行此操作：至少需要保留一名有權管理管理員的使用者。"},
		{"en-US", "Cannot perform this operation: at least one user must retain permission to manage administrators."},
	} {
		t.Run(tc.language, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			admin := mockcom.NewMockAdminComponent(t)
			admin.EXPECT().RemoveRole(mock.Anything, types.AdminUserRoleRequest{
				UserID: 42, RoleCode: "custom", CurrentUser: "operator",
			}).Return(fmt.Errorf("remove administrator role: %w", errorx.ErrLastAccountAdmin)).Once()
			engine := gin.New()
			engine.Use(middleware.ModifyAcceptLanguageMiddleware(), middleware.LocalizedErrorMiddleware(), func(ctx *gin.Context) {
				httpbase.SetCurrentUser(ctx, "operator")
				ctx.Next()
			})
			engine.DELETE("/api/v1/admin/account/admin/users/:user_id/roles/:role_code", (&AdminHandler{admin: admin}).RemoveRole)
			request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/account/admin/users/42/roles/custom", nil)
			request.Header.Set("Accept-Language", tc.language)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, http.StatusConflict, response.Code)
			var result httpbase.R
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
			require.Equal(t, "SYS-ERR-9", result.Code)
			require.Equal(t, "SYS-ERR-9: "+tc.message, result.Msg)
		})
	}
}

// TestAdminHandlerCheckPermissions verifies that only the authenticated identity controls the query.
func TestAdminHandlerCheckPermissions(t *testing.T) {
	for _, tc := range []struct {
		name, user, body string
		allowed          bool
		err              error
		status           int
		call             bool
	}{
		{name: "anonymous", body: `{"permissions":["system/error_log"]}`, status: 401},
		{name: "malformed", user: "operator", body: `{`, status: 400},
		{name: "wrong type", user: "operator", body: `{"permissions":"*"}`, status: 400},
		{name: "oversized body", user: "operator", body: `{"permissions":["system/` + strings.Repeat("x", 70<<10) + `"]}`, status: 400},
		{name: "oversized padding", user: "operator", body: `{"permissions":["system/error_log"]}` + strings.Repeat(" ", 70<<10), status: 400},
		{name: "multiple documents", user: "operator", body: `{"permissions":["system/error_log"]}{}`, status: 400},
		{name: "allowed", user: "operator", body: `{"permissions":["system/error_log"]}`, allowed: true, status: 200, call: true},
		{name: "ignore supplied identity", user: "operator", body: `{"permissions":["system/error_log"],"username":"root","user_id":1}`, allowed: true, status: 200, call: true},
		{name: "denied", user: "member", body: `{"permissions":["system/error_log"]}`, status: 403, call: true},
		{name: "invalid requirements", user: "operator", body: `{"permissions":["system/error_log"]}`, err: errorx.ReqParamInvalid(errors.New("invalid"), errorx.Ctx().Set("param", "permissions")), status: 400, call: true},
		{name: "database error", user: "operator", body: `{"permissions":["system/error_log"]}`, err: errors.New("database unavailable"), status: 500, call: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router, admin := newAdminHandlerTestRouter(t, tc.user)
			if tc.call {
				admin.EXPECT().CheckPermissions(mock.Anything, tc.user, types.AdminPermissionCheckRequest{Permissions: []types.AdminPermission{types.AdminSystemErrorLog}}).Return(tc.allowed, tc.err).Once()
			}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/account/admin/me/permissions/check?username=root", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, tc.status, response.Code, response.Body.String())
			if tc.status == 200 {
				require.JSONEq(t, `{"msg":"OK","data":{"allowed":true}}`, response.Body.String())
			}
		})
	}
}

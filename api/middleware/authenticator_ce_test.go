//go:build !saas && !ee

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// expectedPlatformAdminGlobalOnly fixes the CE contract independently of production configuration.
const expectedPlatformAdminGlobalOnly = true

func TestIsValidOAuthToken_CE(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/oauth", nil)

	assert.False(t, isValidOAuthToken(ctx, nil, "casdoor-oauth-token"))
}

// TestPlatformAdminCERequiresGlobalPermission verifies the CE gate against persisted grants and assignments.
func TestPlatformAdminCERequiresGlobalPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name              string
		roleCode          string
		grant             string
		roleMask          string
		permissions       []types.AdminPermission
		deletedUser       bool
		deletedAssignment bool
		ordinaryRole      bool
		allowed           bool
	}{
		{name: "custom role with global grant", roleCode: "owner_custom", grant: "*", allowed: true},
		{name: "ordinary role with global grant", roleCode: "member", grant: "*", ordinaryRole: true},
		{name: "super admin name with concrete grant", roleCode: "super_admin", grant: "account/user"},
		{name: "super admin name with module wildcard", roleCode: "super_admin", grant: "account/*"},
		{name: "role without grants", roleCode: "reader"},
		{name: "any requires global grant", roleCode: "owner_custom", grant: "*", permissions: []types.AdminPermission{types.AdminPermissionAny}, allowed: true},
		{name: "any rejects concrete grant", roleCode: "super_admin", grant: "account/user", permissions: []types.AdminPermission{types.AdminPermissionAny}},
		{name: "any rejects role without grants", roleCode: "reader", permissions: []types.AdminPermission{types.AdminPermissionAny}},
		{name: "multiple requirements accept global grant", roleCode: "owner_custom", grant: "*", permissions: []types.AdminPermission{types.AdminAccountUser, types.AdminSystemDashboard}, allowed: true},
		{name: "multiple requirements reject matching concrete grant", roleCode: "super_admin", grant: "account/user", permissions: []types.AdminPermission{types.AdminAccountUser, types.AdminSystemDashboard}},
		{name: "mixed any preserves business authorization", roleCode: "owner_custom", grant: "*", permissions: []types.AdminPermission{types.AdminAccountUser, types.AdminPermissionAny}, allowed: true},
		{name: "mixed any still requires global grant", roleCode: "super_admin", grant: "account/user", permissions: []types.AdminPermission{types.AdminAccountUser, types.AdminPermissionAny}},
		{name: "deleted user", roleCode: "owner_custom", grant: "*", deletedUser: true},
		{name: "revoked assignment", roleCode: "owner_custom", grant: "*", deletedAssignment: true},
		{name: "legacy admin without role assignment", roleMask: "admin"},
		{name: "legacy admin with concrete grant", roleCode: "super_admin", grant: "account/user", roleMask: "admin"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db := newCEAdminPermissionTestDB(t)
			var userDeletedAt, assignmentDeletedAt any
			if tc.deletedUser {
				userDeletedAt = time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC)
			}
			if tc.deletedAssignment {
				assignmentDeletedAt = time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC)
			}
			_, err := db.BunDB.ExecContext(ctx, `INSERT INTO users VALUES (1, 'alice', ?, ?)`, tc.roleMask, userDeletedAt)
			require.NoError(t, err)
			if tc.roleCode != "" {
				_, err = db.BunDB.ExecContext(ctx, `INSERT INTO roles (role_code, is_admin) VALUES (?, ?)`, tc.roleCode, !tc.ordinaryRole)
				require.NoError(t, err)
				_, err = db.BunDB.ExecContext(ctx, `INSERT INTO user_roles VALUES (1, ?, ?)`, tc.roleCode, assignmentDeletedAt)
				require.NoError(t, err)
			}
			if tc.grant != "" {
				_, err = db.BunDB.ExecContext(ctx, `INSERT INTO admin_role_permissions VALUES (?, ?)`, tc.roleCode, tc.grant)
				require.NoError(t, err)
			}
			if len(tc.permissions) == 0 {
				tc.permissions = []types.AdminPermission{types.AdminAccountUser}
			}
			store := database.NewAdminRoleStoreWithDB(db)
			var called, authorized bool
			router := gin.New()
			router.GET("/admin/users", func(ctx *gin.Context) {
				httpbase.SetCurrentUser(ctx, "alice")
				defer func() { authorized = types.IsAdminAuthorized(ctx.Request.Context()) }()
				ctx.Next()
			}, newPlatformAdmin(store)(tc.permissions...), func(ctx *gin.Context) {
				called = true
				ctx.Status(http.StatusNoContent)
			})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/users", nil))
			wantStatus := http.StatusForbidden
			if tc.allowed {
				wantStatus = http.StatusNoContent
			}
			require.Equal(t, wantStatus, response.Code, response.Body.String())
			require.Equal(t, tc.allowed, called)
			require.Equal(t, tc.allowed && !slices.Contains(tc.permissions, types.AdminPermissionAny), authorized)
		})
	}
}

// newCEAdminPermissionTestDB creates the minimal schema used by the real permission store.
func newCEAdminPermissionTestDB(t *testing.T) *database.DB {
	t.Helper()
	ctx := context.Background()
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := database.NewDB(ctx, database.DBConfig{Dialect: database.DialectSQLite, DSN: dsn, DisableAuditHook: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	for _, statement := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE, role_mask TEXT, deleted_at TIMESTAMP NULL)`,
		`CREATE TABLE user_roles (user_id INTEGER NOT NULL, role_code TEXT NOT NULL, deleted_at TIMESTAMP NULL)`,
		`CREATE TABLE roles (role_code TEXT PRIMARY KEY, is_admin BOOLEAN NOT NULL DEFAULT FALSE)`,
		`CREATE TABLE admin_role_permissions (role_code TEXT NOT NULL, permission_code TEXT NOT NULL)`,
	} {
		_, err = db.BunDB.ExecContext(ctx, statement)
		require.NoError(t, err)
	}
	return db
}

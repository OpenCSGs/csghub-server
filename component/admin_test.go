package component

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/testutil"
	"opencsg.com/csghub-server/common/types"
)

// TestAdminComponentCurrentRoles checks identity, administrator membership, and store errors.
func TestAdminComponentCurrentRoles(t *testing.T) {
	roles := []types.AdminRoleDetail{
		{AdminRoleSummary: types.AdminRoleSummary{RoleCode: "asset_admin"}, IsAdmin: true, Permissions: []types.AdminPermission{"asset/*"}},
		{AdminRoleSummary: types.AdminRoleSummary{RoleCode: "finance_admin"}, IsAdmin: true, Permissions: []types.AdminPermission{"accounting/*"}},
	}
	storeErr := errors.New("database unavailable")
	for _, tc := range []struct {
		name     string
		username string
		roles    []types.AdminRoleDetail
		storeErr error
		wantErr  error
	}{
		{name: "missing identity", wantErr: errorx.ErrUnauthorized},
		{name: "multiple administrator roles", username: "operator", roles: roles},
		{name: "no administrator roles", username: "member", roles: []types.AdminRoleDetail{}, wantErr: errorx.ErrForbidden},
		{name: "revoked roles", username: "revoked", wantErr: errorx.ErrForbidden},
		{name: "database error", username: "operator", storeErr: storeErr, wantErr: storeErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			store := mockdb.NewMockAdminRoleStore(t)
			if tc.username != "" {
				store.EXPECT().ListByUsername(ctx, tc.username).Return(tc.roles, tc.storeErr).Once()
			}
			admin := &adminComponentImpl{adminRoles: store}
			if !config.SupportsMultipleAdminRoles() && tc.name == "multiple administrator roles" {
				tc.wantErr = errorx.ErrForbidden
			}
			result, err := admin.CurrentRoles(ctx, tc.username)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.roles, result)
			}
		})
	}
}

// TestAdminComponentCheckPermissions verifies identity, validation, live queries, and denied grants.
func TestAdminComponentCheckPermissions(t *testing.T) {
	databaseError := errors.New("database unavailable")
	for _, tc := range []struct {
		name, user        string
		required          []types.AdminPermission
		allowed           bool
		storeErr, wantErr error
		query             bool
	}{
		{name: "anonymous", required: []types.AdminPermission{types.AdminSystemErrorLog}, wantErr: errorx.ErrUnauthorized},
		{name: "empty", user: "operator", wantErr: errorx.ErrReqParamInvalid},
		{name: "invalid", user: "operator", required: []types.AdminPermission{types.AdminPermissionAny, "system/*"}, wantErr: errorx.ErrReqParamInvalid},
		{name: "too many", user: "operator", required: make([]types.AdminPermission, 101), wantErr: errorx.ErrReqParamInvalid},
		{name: "allowed", user: "operator", required: []types.AdminPermission{types.AdminSystemErrorLog}, allowed: true, query: true},
		{name: "OR", user: "operator", required: []types.AdminPermission{types.AdminSystemErrorLog, types.AdminMarketingArticle}, allowed: true, query: true},
		{name: "membership", user: "operator", required: []types.AdminPermission{types.AdminPermissionAny}, allowed: true, query: true},
		{name: "global", user: "operator", required: []types.AdminPermission{types.AdminPermissionAll}, allowed: true, query: true},
		{name: "revoked or non-admin", user: "operator", required: []types.AdminPermission{types.AdminSystemErrorLog}, query: true},
		{name: "store failure", user: "operator", required: []types.AdminPermission{types.AdminSystemErrorLog}, query: true, storeErr: databaseError, wantErr: databaseError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := mockdb.NewMockAdminRoleStore(t)
			ctx := context.Background()
			if tc.query {
				store.EXPECT().HasPermission(ctx, tc.user, adminPermissionMockArguments(tc.required)...).Return(tc.allowed, tc.storeErr).Once()
			}
			allowed, err := (&adminComponentImpl{adminRoles: store}).CheckPermissions(ctx, tc.user, types.AdminPermissionCheckRequest{Permissions: tc.required})
			require.Equal(t, tc.allowed, allowed)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestAdminComponentCheckPermissionsRevocation confirms subsequent requests do not reuse grants.
func TestAdminComponentCheckPermissionsRevocation(t *testing.T) {
	ctx := context.Background()
	store := mockdb.NewMockAdminRoleStore(t)
	req := types.AdminPermissionCheckRequest{Permissions: []types.AdminPermission{types.AdminSystemErrorLog}}
	store.EXPECT().HasPermission(ctx, "operator", adminPermissionMockArguments(req.Permissions)...).Return(true, nil).Once()
	store.EXPECT().HasPermission(ctx, "operator", adminPermissionMockArguments(req.Permissions)...).Return(false, nil).Once()
	admin := &adminComponentImpl{adminRoles: store}
	allowed, err := admin.CheckPermissions(ctx, "operator", req)
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, err = admin.CheckPermissions(ctx, "operator", req)
	require.NoError(t, err)
	require.False(t, allowed)
}

// adminPermissionMockArguments converts variadic requirements for the generated expecter.
func adminPermissionMockArguments(required []types.AdminPermission) []interface{} {
	mapped := config.PlatformAdminPermissions(required)
	result := make([]interface{}, len(mapped))
	for i, permission := range mapped {
		result[i] = permission
	}
	return result
}

// TestAdminComponentEditionCatalog verifies CE does not expose assignable module roles or invalid global definitions.
func TestAdminComponentEditionCatalog(t *testing.T) {
	ctx := context.Background()
	global := types.AdminRoleDetail{AdminRoleSummary: types.AdminRoleSummary{RoleCode: types.SuperAdminRoleCode}, IsAdmin: true, Permissions: []types.AdminPermission{types.AdminPermissionAll}}
	module := types.AdminRoleDetail{AdminRoleSummary: types.AdminRoleSummary{RoleCode: "asset_admin"}, IsAdmin: true, Permissions: []types.AdminPermission{"asset/*"}}
	custom := global
	custom.RoleCode = "test"
	revoked := global
	revoked.Permissions = []types.AdminPermission{types.AdminSystemDashboard}
	nonAdmin := global
	nonAdmin.IsAdmin = false
	for _, tc := range []struct {
		name   string
		roles  []types.AdminRoleDetail
		global []types.AdminRoleDetail
	}{
		{name: "named and custom global grants", roles: []types.AdminRoleDetail{global, module, custom}, global: []types.AdminRoleDetail{global, custom}},
		{name: "module only", roles: []types.AdminRoleDetail{module}},
		{name: "super_admin without wildcard", roles: []types.AdminRoleDetail{revoked}},
		{name: "ordinary role with wildcard", roles: []types.AdminRoleDetail{nonAdmin}},
		{name: "empty catalog", roles: []types.AdminRoleDetail{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := mockdb.NewMockAdminRoleStore(t)
			store.EXPECT().List(ctx).Return(tc.roles, nil).Once()
			actual, err := (&adminComponentImpl{adminRoles: store}).Roles(ctx)
			require.NoError(t, err)
			if config.SupportsMultipleAdminRoles() {
				require.Equal(t, tc.roles, actual)
			} else if len(tc.global) > 0 {
				require.Equal(t, tc.global, actual)
			} else {
				require.Empty(t, actual)
			}
		})
	}
	store := mockdb.NewMockAdminRoleStore(t)
	failure := errors.New("catalog unavailable")
	store.EXPECT().List(ctx).Return(nil, failure).Once()
	_, err := (&adminComponentImpl{adminRoles: store}).Roles(ctx)
	require.ErrorIs(t, err, failure)
}

// TestAdminComponentEditionCurrentRoles preserves global capability while denying module-only users in CE.
func TestAdminComponentEditionCurrentRoles(t *testing.T) {
	ctx := context.Background()
	global := types.AdminRoleDetail{AdminRoleSummary: types.AdminRoleSummary{RoleCode: "test"}, IsAdmin: true, Permissions: []types.AdminPermission{types.AdminPermissionAll}}
	module := types.AdminRoleDetail{AdminRoleSummary: types.AdminRoleSummary{RoleCode: "asset_admin"}, IsAdmin: true, Permissions: []types.AdminPermission{"asset/*"}}
	nameOnly := module
	nameOnly.RoleCode = types.SuperAdminRoleCode
	for _, roles := range [][]types.AdminRoleDetail{{global, module}, {module}, {nameOnly}} {
		store := mockdb.NewMockAdminRoleStore(t)
		store.EXPECT().ListByUsername(ctx, "operator").Return(roles, nil).Once()
		actual, err := (&adminComponentImpl{adminRoles: store}).CurrentRoles(ctx, "operator")
		if config.SupportsMultipleAdminRoles() {
			require.NoError(t, err)
			require.Equal(t, roles, actual)
		} else if len(roles) == 2 {
			require.NoError(t, err)
			require.Equal(t, []types.AdminRoleDetail{global}, actual)
		} else {
			require.ErrorIs(t, err, errorx.ErrForbidden)
			require.Nil(t, actual)
		}
	}
}

// TestAdminComponentEditionUsers delegates pagination and projection with a server-controlled grant scope.
func TestAdminComponentEditionUsers(t *testing.T) {
	ctx := context.Background()
	source := []types.AdminUser{{UserID: 42, Roles: []types.AdminRoleSummary{{RoleCode: "test"}, {RoleCode: "custom_owner"}}}}
	for _, code := range []string{"", "test", types.SuperAdminRoleCode, "asset_admin"} {
		t.Run("role="+code, func(t *testing.T) {
			roles := mockdb.NewMockAdminRoleStore(t)
			users := mockdb.NewMockUserRoleStore(t)
			req := types.AdminUserListRequest{RoleCode: code, Page: 2, Per: 10, GlobalOnly: config.SupportsMultipleAdminRoles()}
			expected := req
			expected.GlobalOnly = !config.SupportsMultipleAdminRoles()
			users.EXPECT().ListAdminUsers(ctx, expected).Return(source, 21, nil).Once()
			actual, total, err := (&adminComponentImpl{adminRoles: roles, userRoles: users}).Users(ctx, req)
			require.NoError(t, err)
			require.Equal(t, 21, total)
			require.Equal(t, source, actual)
			roles.AssertNotCalled(t, "List", mock.Anything)
		})
	}
}

// TestAdminComponentEditionAssignments delegates grant validation and preserves transactional store errors.
func TestAdminComponentEditionAssignments(t *testing.T) {
	ctx := context.Background()
	databaseErr := errors.New("database unavailable")
	for _, tc := range []struct {
		name, code string
		failure    error
		global     bool
	}{
		{name: "custom global", code: "test", global: true},
		{name: "module role", code: "finance_admin"},
		{name: "name does not imply wildcard", code: types.SuperAdminRoleCode},
		{name: "store failure", code: "test", global: true, failure: databaseErr},
		{name: "missing operator", code: "test", global: true, failure: errorx.ErrUnauthorized},
		{name: "missing role", failure: errorx.ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			roles := mockdb.NewMockAdminRoleStore(t)
			users := mockdb.NewMockUserRoleStore(t)
			req := types.AdminUserRoleRequest{UserID: 42, RoleCode: tc.code, CurrentUser: "operator", GlobalOnly: config.SupportsMultipleAdminRoles()}
			expected := req
			expected.GlobalOnly = !config.SupportsMultipleAdminRoles()
			failure := tc.failure
			if failure == nil && !tc.global && expected.GlobalOnly {
				failure = errorx.ErrForbidden
			}
			users.EXPECT().AssignAdminRole(ctx, expected).Return(failure).Once()
			users.EXPECT().RemoveAdminRole(ctx, expected).Return(failure).Once()
			admin := &adminComponentImpl{adminRoles: roles, userRoles: users}
			assignErr, removeErr := admin.AssignRole(ctx, req), admin.RemoveRole(ctx, req)
			if failure == nil {
				require.NoError(t, assignErr)
				require.NoError(t, removeErr)
			} else {
				require.ErrorIs(t, assignErr, failure)
				require.ErrorIs(t, removeErr, failure)
			}
			roles.AssertNotCalled(t, "List", mock.Anything)
		})
	}
}

// TestAdminComponentCEFailures preserves store failures without querying an unrelated role catalog.
func TestAdminComponentCEFailures(t *testing.T) {
	if config.SupportsMultipleAdminRoles() {
		t.Skip("CE-only policy")
	}
	ctx := context.Background()
	for _, failure := range []error{errorx.ErrForbidden, errors.New("database unavailable")} {
		roles := mockdb.NewMockAdminRoleStore(t)
		users := mockdb.NewMockUserRoleStore(t)
		req := types.AdminUserListRequest{RoleCode: "test", Page: 1, Per: 10}
		expected := req
		expected.GlobalOnly = true
		users.EXPECT().ListAdminUsers(ctx, expected).Return(nil, 0, failure).Once()
		admin := &adminComponentImpl{adminRoles: roles, userRoles: users}
		result, total, err := admin.Users(ctx, req)
		require.ErrorIs(t, err, failure)
		require.Nil(t, result)
		require.Zero(t, total)
		roles.AssertNotCalled(t, "List", mock.Anything)
	}
}

// newAdminComponentGrantTestDB creates active, revoked, and deleted assignments with name-independent grants.
func newAdminComponentGrantTestDB(t *testing.T) (*database.DB, *adminComponentImpl) {
	t.Helper()
	ctx := context.Background()
	db, err := database.NewDB(ctx, database.DBConfig{
		Dialect: database.DialectSQLite, DSN: "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared", DisableAuditHook: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	connection, err := testutil.OpenSQLiteWithTransactionLocks("file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared")
	require.NoError(t, err)
	require.NoError(t, db.BunDB.DB.Close())
	db.BunDB.DB = connection
	for _, statement := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, uuid TEXT NOT NULL, username TEXT NOT NULL UNIQUE, name TEXT NOT NULL, avatar TEXT NOT NULL DEFAULT '', deleted_at TIMESTAMP NULL)`,
		`CREATE TABLE roles (role_code TEXT PRIMARY KEY, display_name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', is_admin BOOLEAN NOT NULL DEFAULT FALSE, is_builtin BOOLEAN NOT NULL DEFAULT FALSE, can_delete BOOLEAN NOT NULL DEFAULT TRUE, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, user_id INTEGER NOT NULL)`,
		`CREATE TABLE admin_role_permissions (role_code TEXT NOT NULL REFERENCES roles(role_code), permission_code TEXT NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (role_code, permission_code))`,
		`CREATE TABLE user_roles (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL REFERENCES users(id), role_code TEXT NOT NULL, operator_user_id INTEGER NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at TIMESTAMP NULL)`,
		`CREATE UNIQUE INDEX user_roles_active_user_role_idx ON user_roles(user_id, role_code) WHERE deleted_at IS NULL`,
		`INSERT INTO users (id, uuid, username, name, avatar, deleted_at) VALUES
		 (1, 'uuid-1', 'operator', 'Operator', '', NULL), (2, 'uuid-2', 'target', 'Target', '', NULL),
		 (3, 'uuid-3', 'mixed', 'Mixed', 'avatar-3', NULL), (4, 'uuid-4', 'custom', 'Custom', '', NULL),
		 (5, 'uuid-5', 'name-only', 'Name Only', '', NULL), (6, 'uuid-6', 'module', 'Module', '', NULL),
		 (7, 'uuid-7', 'revoked', 'Revoked', '', NULL), (8, 'uuid-8', 'deleted', 'Deleted', '', CURRENT_TIMESTAMP),
		 (9, 'uuid-9', 'ordinary', 'Ordinary', '', NULL)`,
		`INSERT INTO roles (role_code, display_name, description, user_id, is_admin) VALUES
		 ('test', 'Test Owner', 'Custom global role', 0, TRUE), ('custom_owner', 'Custom Owner', 'Another global role', 0, TRUE),
		 ('super_admin', 'Name Only', 'No global grant', 0, TRUE), ('module_admin', 'Module Admin', 'Account grants only', 0, TRUE),
		 ('member', 'Member', 'Ordinary role with a wildcard', 0, FALSE)`,
		`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES
		 ('test', '*'), ('test', 'account/user'), ('custom_owner', '*'), ('custom_owner', 'system/dashboard'),
		 ('super_admin', 'account/admin'), ('module_admin', 'account/*'), ('member', '*')`,
		`INSERT INTO user_roles (user_id, role_code, deleted_at) VALUES
		 (1, 'test', NULL), (3, 'test', NULL), (3, 'custom_owner', NULL), (3, 'custom_owner', CURRENT_TIMESTAMP),
		 (3, 'super_admin', NULL), (3, 'module_admin', NULL), (3, 'member', NULL), (4, 'test', NULL),
		 (5, 'super_admin', NULL), (6, 'module_admin', NULL), (7, 'test', CURRENT_TIMESTAMP),
		 (8, 'test', NULL), (9, 'member', NULL)`,
	} {
		_, err := db.BunDB.ExecContext(ctx, statement)
		require.NoError(t, err)
	}
	return db, &adminComponentImpl{
		adminRoles: database.NewAdminRoleStoreWithDB(db),
		userRoles:  database.NewUserRoleStoreWithDB(db),
	}
}

// adminRoleDetailCodes extracts role identifiers while retaining their store order.
func adminRoleDetailCodes(roles []types.AdminRoleDetail) []string {
	codes := make([]string, 0, len(roles))
	for _, role := range roles {
		codes = append(codes, role.RoleCode)
	}
	return codes
}

// adminUserRoleCodes extracts projected role identifiers without depending on permission details.
func adminUserRoleCodes(user types.AdminUser) []string {
	codes := make([]string, 0, len(user.Roles))
	for _, role := range user.Roles {
		codes = append(codes, role.RoleCode)
	}
	return codes
}

// TestAdminComponentEditionGlobalCatalog uses real grants for catalog, current-role, and permission checks.
func TestAdminComponentEditionGlobalCatalog(t *testing.T) {
	ctx := context.Background()
	_, admin := newAdminComponentGrantTestDB(t)
	globalCodes := []string{"custom_owner", "test"}
	allCodes := []string{"custom_owner", "module_admin", "super_admin", "test"}
	wantCodes := globalCodes
	if config.SupportsMultipleAdminRoles() {
		wantCodes = allCodes
	}
	roles, err := admin.Roles(ctx)
	require.NoError(t, err)
	require.Equal(t, wantCodes, adminRoleDetailCodes(roles))
	roles, err = admin.CurrentRoles(ctx, "mixed")
	require.NoError(t, err)
	require.Equal(t, wantCodes, adminRoleDetailCodes(roles))
	for _, role := range roles {
		require.NotEmpty(t, role.DisplayName)
		require.NotEmpty(t, role.Description)
	}
	roles, err = admin.CurrentRoles(ctx, "custom")
	require.NoError(t, err)
	require.Equal(t, []string{"test"}, adminRoleDetailCodes(roles))
	allowed, err := admin.CheckPermissions(ctx, "custom", types.AdminPermissionCheckRequest{Permissions: []types.AdminPermission{types.AdminAccountUser}})
	require.NoError(t, err)
	require.True(t, allowed)
	for _, username := range []string{"name-only", "module", "revoked", "deleted", "ordinary"} {
		t.Run(username, func(t *testing.T) {
			roles, err := admin.CurrentRoles(ctx, username)
			if config.SupportsMultipleAdminRoles() && (username == "name-only" || username == "module") {
				require.NoError(t, err)
				require.Len(t, roles, 1)
			} else {
				require.ErrorIs(t, err, errorx.ErrForbidden)
				require.Nil(t, roles)
			}
			allowed, err := admin.CheckPermissions(ctx, username, types.AdminPermissionCheckRequest{Permissions: []types.AdminPermission{types.AdminPermissionAll}})
			require.NoError(t, err)
			require.False(t, allowed)
			allowed, err = admin.CheckPermissions(ctx, username, types.AdminPermissionCheckRequest{Permissions: []types.AdminPermission{types.AdminPermissionAny}})
			require.NoError(t, err)
			require.Equal(t, config.SupportsMultipleAdminRoles() && (username == "name-only" || username == "module"), allowed)
		})
	}
}

// TestAdminComponentEditionGlobalUsers verifies grant filtering precedes pagination and role summaries remain complete.
func TestAdminComponentEditionGlobalUsers(t *testing.T) {
	ctx := context.Background()
	_, admin := newAdminComponentGrantTestDB(t)
	for _, tc := range []struct {
		name, code string
		page, per  int
		ceIDs      []int64
		multiIDs   []int64
		ceTotal    int
		multiTotal int
		ceDenied   bool
	}{
		{name: "first page", page: 1, per: 1, ceIDs: []int64{4}, multiIDs: []int64{6}, ceTotal: 3, multiTotal: 5},
		{name: "second page", page: 2, per: 1, ceIDs: []int64{3}, multiIDs: []int64{5}, ceTotal: 3, multiTotal: 5},
		{name: "complete page", page: 1, per: 10, ceIDs: []int64{4, 3, 1}, multiIDs: []int64{6, 5, 4, 3, 1}, ceTotal: 3, multiTotal: 5},
		{name: "past last page", page: 9, per: 1, ceIDs: []int64{}, multiIDs: []int64{}, ceTotal: 3, multiTotal: 5},
		{name: "custom global role", code: "test", page: 1, per: 10, ceIDs: []int64{4, 3, 1}, multiIDs: []int64{4, 3, 1}, ceTotal: 3, multiTotal: 3},
		{name: "another global role", code: "custom_owner", page: 1, per: 10, ceIDs: []int64{3}, multiIDs: []int64{3}, ceTotal: 1, multiTotal: 1},
		{name: "name without global grant", code: types.SuperAdminRoleCode, page: 1, per: 10, multiIDs: []int64{5, 3}, multiTotal: 2, ceDenied: true},
		{name: "module role", code: "module_admin", page: 1, per: 10, multiIDs: []int64{6, 3}, multiTotal: 2, ceDenied: true},
		{name: "ordinary role", code: "member", page: 1, per: 10, multiIDs: []int64{}, ceDenied: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := types.AdminUserListRequest{RoleCode: tc.code, Page: tc.page, Per: tc.per, GlobalOnly: config.SupportsMultipleAdminRoles()}
			users, total, err := admin.Users(ctx, req)
			if !config.SupportsMultipleAdminRoles() && tc.ceDenied {
				require.ErrorIs(t, err, errorx.ErrForbidden)
				require.Nil(t, users)
				require.Zero(t, total)
				return
			}
			require.NoError(t, err)
			wantIDs, wantTotal := tc.ceIDs, tc.ceTotal
			if config.SupportsMultipleAdminRoles() {
				wantIDs, wantTotal = tc.multiIDs, tc.multiTotal
			}
			require.Equal(t, wantTotal, total)
			ids := make([]int64, 0, len(users))
			for _, user := range users {
				ids = append(ids, user.UserID)
				if user.UserID == 3 {
					wantCodes := []string{"custom_owner", "test"}
					if config.SupportsMultipleAdminRoles() {
						wantCodes = []string{"custom_owner", "module_admin", "super_admin", "test"}
					}
					require.Equal(t, wantCodes, adminUserRoleCodes(user))
					require.Equal(t, "uuid-3", user.UUID)
					require.Equal(t, "mixed", user.Username)
					require.Equal(t, "Mixed", user.Nickname)
					require.Equal(t, "avatar-3", user.Avatar)
				}
				for _, role := range user.Roles {
					require.NotEmpty(t, role.DisplayName)
					require.NotEmpty(t, role.Description)
				}
			}
			require.Equal(t, wantIDs, ids)
		})
	}
}

// TestAdminComponentEditionGlobalRoleMutations keeps custom global grants idempotent and revocations immediately effective.
func TestAdminComponentEditionGlobalRoleMutations(t *testing.T) {
	for _, code := range []string{"test", "custom_owner"} {
		t.Run(code, func(t *testing.T) {
			ctx := context.Background()
			db, admin := newAdminComponentGrantTestDB(t)
			req := types.AdminUserRoleRequest{UserID: 2, RoleCode: code, CurrentUser: "operator", GlobalOnly: config.SupportsMultipleAdminRoles()}
			for range 2 {
				require.NoError(t, admin.AssignRole(ctx, req))
			}
			var active int
			require.NoError(t, db.BunDB.NewRaw(`SELECT COUNT(*) FROM user_roles WHERE user_id = ? AND role_code = ? AND deleted_at IS NULL`, req.UserID, code).Scan(ctx, &active))
			require.Equal(t, 1, active)
			var operatorID int64
			require.NoError(t, db.BunDB.NewRaw(`SELECT operator_user_id FROM user_roles WHERE user_id = ? AND role_code = ? AND deleted_at IS NULL`, req.UserID, code).Scan(ctx, &operatorID))
			require.Equal(t, int64(1), operatorID)
			roles, err := admin.CurrentRoles(ctx, "target")
			require.NoError(t, err)
			require.Equal(t, []string{code}, adminRoleDetailCodes(roles))
			allowed, err := admin.adminRoles.HasPermission(ctx, "target", types.AdminPermissionAll)
			require.NoError(t, err)
			require.True(t, allowed)
			for range 2 {
				require.NoError(t, admin.RemoveRole(ctx, req))
			}
			allowed, err = admin.adminRoles.HasPermission(ctx, "target", types.AdminPermissionAll)
			require.NoError(t, err)
			require.False(t, allowed)
			roles, err = admin.CurrentRoles(ctx, "target")
			require.ErrorIs(t, err, errorx.ErrForbidden)
			require.Nil(t, roles)
			require.NoError(t, db.BunDB.NewRaw(`SELECT COUNT(*) FROM user_roles WHERE user_id = ? AND role_code = ? AND deleted_at IS NULL`, req.UserID, code).Scan(ctx, &active))
			require.Zero(t, active)
			var historical int
			require.NoError(t, db.BunDB.NewRaw(`SELECT COUNT(*) FROM user_roles WHERE user_id = ? AND role_code = ? AND deleted_at IS NOT NULL AND operator_user_id = ?`, req.UserID, code, operatorID).Scan(ctx, &historical))
			require.Equal(t, 1, historical)
		})
	}
}

// TestAdminComponentEditionLastAdminProtection counts only grants accepted by the edition's administrator gate.
func TestAdminComponentEditionLastAdminProtection(t *testing.T) {
	for _, tc := range []struct {
		name, remainingRole string
		holderID            int64
		global              bool
	}{
		{name: "another user with exact grant", remainingRole: "super_admin", holderID: 2},
		{name: "another user with account wildcard", remainingRole: "module_admin", holderID: 2},
		{name: "own other module role", remainingRole: "module_admin", holderID: 1},
		{name: "another user with global grant", remainingRole: "custom_owner", holderID: 2, global: true},
		{name: "own other global role", remainingRole: "custom_owner", holderID: 1, global: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db, admin := newAdminComponentGrantTestDB(t)
			_, err := db.Core.ExecContext(ctx, `DELETE FROM user_roles`)
			require.NoError(t, err)
			_, err = db.Core.ExecContext(ctx, `INSERT INTO user_roles (user_id, role_code) VALUES (1, 'test'), (?, ?)`, tc.holderID, tc.remainingRole)
			require.NoError(t, err)
			// Deliberately supply the opposite policy to prove the component derives it from the edition.
			req := types.AdminUserRoleRequest{UserID: 1, RoleCode: "test", CurrentUser: "operator", GlobalOnly: config.SupportsMultipleAdminRoles()}
			err = admin.RemoveRole(ctx, req)
			allowed := config.SupportsMultipleAdminRoles() || tc.global
			if allowed {
				require.NoError(t, err)
				require.NoError(t, admin.RemoveRole(ctx, req), "repeated revocation remains idempotent")
			} else {
				require.ErrorIs(t, err, errorx.ErrLastAccountAdmin)
			}
			var active int
			require.NoError(t, db.Core.NewRaw(`SELECT COUNT(*) FROM user_roles WHERE user_id = 1 AND role_code = 'test' AND deleted_at IS NULL`).Scan(ctx, &active))
			require.Equal(t, !allowed, active == 1)
			holder := "target"
			if tc.holderID == 1 {
				holder = "operator"
			}
			permissions := config.PlatformAdminPermissions([]types.AdminPermission{types.AdminAccountAdmin})
			hasPermission, err := admin.adminRoles.HasPermission(ctx, holder, permissions...)
			require.NoError(t, err)
			require.True(t, hasPermission || !allowed, "an allowed removal must retain a user accepted by the administrator gate")
		})
	}
}

// TestAdminComponentEditionNonGlobalRoleMutations denies non-global CE writes while preserving module roles in other editions.
func TestAdminComponentEditionNonGlobalRoleMutations(t *testing.T) {
	for _, code := range []string{"module_admin", types.SuperAdminRoleCode, "member"} {
		t.Run(code, func(t *testing.T) {
			ctx := context.Background()
			db, admin := newAdminComponentGrantTestDB(t)
			req := types.AdminUserRoleRequest{UserID: 2, RoleCode: code, CurrentUser: "operator", GlobalOnly: config.SupportsMultipleAdminRoles()}
			assignErr, removeErr := admin.AssignRole(ctx, req), admin.RemoveRole(ctx, req)
			switch {
			case !config.SupportsMultipleAdminRoles():
				require.ErrorIs(t, assignErr, errorx.ErrForbidden)
				require.ErrorIs(t, removeErr, errorx.ErrForbidden)
				if code != "member" {
					req.UserID = 5
					if code == "module_admin" {
						req.UserID = 6
					}
					require.ErrorIs(t, admin.RemoveRole(ctx, req), errorx.ErrForbidden)
					var active int
					require.NoError(t, db.BunDB.NewRaw(`SELECT COUNT(*) FROM user_roles WHERE user_id = ? AND role_code = ? AND deleted_at IS NULL`, req.UserID, code).Scan(ctx, &active))
					require.Equal(t, 1, active)
				}
			case code == "member":
				require.ErrorIs(t, assignErr, errorx.ErrNotFound)
				require.ErrorIs(t, removeErr, errorx.ErrNotFound)
			default:
				require.NoError(t, assignErr)
				require.NoError(t, removeErr)
			}
			allowed, err := admin.adminRoles.HasPermission(ctx, "target", types.AdminPermissionAll)
			require.NoError(t, err)
			require.False(t, allowed)
			var active int
			require.NoError(t, db.BunDB.NewRaw(`SELECT COUNT(*) FROM user_roles WHERE user_id = 2 AND deleted_at IS NULL`).Scan(ctx, &active))
			require.Zero(t, active)
			require.NoError(t, db.BunDB.NewRaw(`SELECT COUNT(*) FROM user_roles WHERE user_id = 3 AND role_code = 'member' AND deleted_at IS NULL`).Scan(ctx, &active))
			require.Equal(t, 1, active)
		})
	}
}

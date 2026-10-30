package database

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/testutil"
	"opencsg.com/csghub-server/common/types"
)

// newUserRoleRelationTestDB builds full user models for relationship-loading tests.
func newUserRoleRelationTestDB(t *testing.T) *DB {
	t.Helper()
	db := newAdminRoleManagementTestDB(t)
	ctx := context.Background()
	_, err := db.Core.NewDropTable().Model((*User)(nil)).Exec(ctx)
	require.NoError(t, err)
	for _, model := range []any{(*User)(nil), (*Namespace)(nil)} {
		_, err = db.Core.NewCreateTable().Model(model).Exec(ctx)
		require.NoError(t, err)
	}
	users := []User{
		{ID: 1, Username: "role-one", UUID: "role-uuid-one", Email: "one@example.com"},
		{ID: 2, Username: "role-two", UUID: "role-uuid-two", Email: "two@example.com"},
		{ID: 3, Username: "role-revoked", UUID: "role-uuid-revoked"},
		{ID: 4, Username: "role-legacy", UUID: "role-uuid-legacy", RoleMask: "admin,super_user"},
		{ID: 5, Username: "role-deleted", UUID: "role-uuid-deleted", DeletedAt: time.Now()},
	}
	_, err = db.Core.NewInsert().Model(&users).Exec(ctx)
	require.NoError(t, err)
	for _, statement := range []string{
		`INSERT INTO roles (role_code, display_name, user_id, is_admin) VALUES
		 ('custom_owner', 'Custom owner', 0, TRUE), ('operator', 'Operator', 0, TRUE), ('super_admin', 'Name alone', 0, TRUE),
		 ('personal_user', 'Personal user', 0, FALSE)`,
		`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES
		 ('custom_owner', '*'), ('operator', 'account/*'), ('personal_user', '*')`,
		`INSERT INTO user_roles (user_id, role_code, deleted_at) VALUES
		 (1, 'personal_user', NULL), (1, 'custom_owner', NULL), (1, 'operator', CURRENT_TIMESTAMP),
		 (2, 'operator', NULL), (2, 'super_admin', NULL), (3, 'custom_owner', CURRENT_TIMESTAMP), (3, 'personal_user', NULL),
		 (5, 'custom_owner', NULL)`,
	} {
		_, err = db.Core.ExecContext(ctx, statement)
		require.NoError(t, err)
	}
	return db
}

// requireUserRoleRelations checks projected roles and both administrator flags.
func requireUserRoleRelations(t *testing.T, user User) {
	t.Helper()
	switch user.ID {
	case 1:
		require.ElementsMatch(t, []string{"personal_user", "custom_owner"}, user.Roles())
		require.True(t, user.HasAdminRole())
		require.True(t, user.HasSuperAdminPermission())
		require.True(t, user.CanAdmin())
	case 2:
		require.ElementsMatch(t, []string{"operator", "super_admin"}, user.Roles())
		require.True(t, user.HasAdminRole())
		require.False(t, user.HasSuperAdminPermission())
		require.False(t, user.CanAdmin())
	case 3:
		require.Equal(t, []string{"personal_user"}, user.Roles())
		require.False(t, user.HasAdminRole())
		require.False(t, user.HasSuperAdminPermission())
		require.False(t, user.CanAdmin())
	case 4:
		require.Empty(t, user.Roles())
		require.False(t, user.HasAdminRole())
		require.False(t, user.HasSuperAdminPermission())
		require.False(t, user.CanAdmin())
	default:
		t.Fatalf("unexpected active user %d", user.ID)
	}
}

// TestUserStoreRoleRelationsDetail covers real Bun loading through user detail lookups.
func TestUserStoreRoleRelationsDetail(t *testing.T) {
	ctx := context.Background()
	db := newUserRoleRelationTestDB(t)
	store := NewUserStoreWithDB(db)
	for _, tc := range []struct {
		name string
		find func() (User, error)
	}{
		{name: "username", find: func() (User, error) { return store.FindByUsername(ctx, "role-one", types.UserWithAdminPermissions) }},
		{name: "email", find: func() (User, error) { return store.FindByEmail(ctx, "one@example.com", types.UserWithAdminPermissions) }},
		{name: "ID", find: func() (User, error) { return store.FindByID(ctx, 1, types.UserWithAdminPermissions) }},
		{name: "revoked", find: func() (User, error) { return store.FindByUsername(ctx, "role-revoked", types.UserWithAdminPermissions) }},
		{name: "legacy", find: func() (User, error) { return store.FindByUsername(ctx, "role-legacy", types.UserWithAdminPermissions) }},
		{name: "including deleted", find: func() (User, error) {
			return store.FindByUsernameWithDeleted(ctx, "role-one", types.UserWithAdminPermissions)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user, err := tc.find()
			require.NoError(t, err)
			requireUserRoleRelations(t, user)
		})
	}
	user, err := store.FindByUUID(ctx, "role-uuid-one", types.UserWithAdminPermissions)
	require.NoError(t, err)
	requireUserRoleRelations(t, *user)
	_, err = store.FindByUsername(ctx, "role-deleted", types.UserWithAdminPermissions)
	require.ErrorIs(t, err, errorx.ErrDatabaseNoRows)
	deleted, err := store.FindByUsernameWithDeleted(ctx, "role-deleted", types.UserWithAdminPermissions)
	require.NoError(t, err)
	require.Equal(t, []string{"custom_owner"}, deleted.Roles())
	require.False(t, deleted.HasAdminRole())
	require.False(t, deleted.HasSuperAdminPermission())
	require.False(t, deleted.CanAdmin())
}

// userRoleRelationQueryHook records relationship queries to detect per-user loading.
type userRoleRelationQueryHook struct {
	queries []string
}

// BeforeQuery preserves the query context.
func (h *userRoleRelationQueryHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}

// AfterQuery records successfully executed SQL for relationship query assertions.
func (h *userRoleRelationQueryHook) AfterQuery(_ context.Context, event *bun.QueryEvent) {
	if event.Err == nil {
		h.queries = append(h.queries, event.Query)
	}
}

// TestUserStoreRoleRelationsList verifies pagination and batch loading of administrator grants.
func TestUserStoreRoleRelationsList(t *testing.T) {
	ctx := context.Background()
	db := newUserRoleRelationTestDB(t)
	store := NewUserStoreWithDB(db)
	hook := &userRoleRelationQueryHook{}
	db.BunDB.AddQueryHook(hook)
	users, total, err := store.IndexWithSearch(ctx, types.UserListReq{Page: 1, Per: 2, Search: "role-"}, types.UserWithAdminPermissions)
	require.NoError(t, err)
	require.Equal(t, 4, total)
	require.Len(t, users, 2)
	for _, user := range users {
		requireUserRoleRelations(t, user)
	}
	for _, table := range []string{`FROM "user_roles"`, `FROM "admin_role_permissions"`} {
		count := 0
		for _, query := range hook.queries {
			if strings.Contains(query, table) {
				count++
			}
		}
		require.Equal(t, 1, count, "batch query for %s", table)
	}
	users, total, err = store.IndexWithSearch(ctx, types.UserListReq{Page: 2, Per: 2}, types.UserWithAdminPermissions)
	require.NoError(t, err)
	require.Equal(t, 4, total)
	require.Len(t, users, 2)
	for _, user := range users {
		requireUserRoleRelations(t, user)
	}
	for _, tc := range []struct {
		name string
		list func() ([]User, error)
	}{
		{name: "index", list: func() ([]User, error) { return store.Index(ctx, types.UserWithAdminPermissions) }},
		{name: "IDs", list: func() ([]User, error) {
			return store.FindByIDs(ctx, []int64{1, 2, 3, 4}, types.UserWithAdminPermissions)
		}},
		{name: "keyword", list: func() ([]User, error) {
			return store.SearchByKeywordExcludingIDs(ctx, "role-", nil, 10, types.UserWithAdminPermissions)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			users, err := tc.list()
			require.NoError(t, err)
			require.Len(t, users, 4)
			for _, user := range users {
				requireUserRoleRelations(t, user)
			}
		})
	}
	uuidUsers, err := store.FindByUUIDs(ctx, []string{"role-uuid-one", "role-uuid-two"}, types.UserWithAdminPermissions)
	require.NoError(t, err)
	require.Len(t, uuidUsers, 2)
	for _, user := range uuidUsers {
		requireUserRoleRelations(t, *user)
	}
}

// TestUserStoreRolesWithoutPermissions verifies ordinary queries preserve roles without reading grant tables.
func TestUserStoreRolesWithoutPermissions(t *testing.T) {
	ctx := context.Background()
	db := newUserRoleRelationTestDB(t)
	store := NewUserStoreWithDB(db)
	hook := &userRoleRelationQueryHook{}
	db.BunDB.AddQueryHook(hook)

	user, err := store.FindByUsername(ctx, "role-one")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"personal_user", "custom_owner"}, user.Roles())
	require.False(t, user.HasAdminRole())
	require.False(t, user.CanAdmin())
	for _, assignment := range user.RoleAssignments {
		require.Nil(t, assignment.Role)
	}
	users, count, err := store.IndexWithSearch(ctx, types.UserListReq{Page: 1, Per: 2})
	require.NoError(t, err)
	require.Equal(t, 4, count)
	require.Len(t, users, 2)
	require.ElementsMatch(t, []string{"personal_user", "custom_owner"}, users[0].Roles())
	require.ElementsMatch(t, []string{"operator", "super_admin"}, users[1].Roles())
	for _, query := range hook.queries {
		require.NotContains(t, query, `JOIN "roles"`)
		require.NotContains(t, query, `FROM "roles"`)
		require.NotContains(t, query, "admin_role_permissions")
	}
}

// TestLoadUserRoleAssignments verifies the relation loader used after cursor batches.
func TestLoadUserRoleAssignments(t *testing.T) {
	ctx := context.Background()
	db := newUserRoleRelationTestDB(t)
	users := []User{{ID: 1}, {ID: 2}, {ID: 3}, {ID: 4, RoleMask: "admin"}}
	require.NoError(t, loadUserRoleAssignments(ctx, db.Core, users, types.UserWithAdminPermissions))
	for _, user := range users {
		requireUserRoleRelations(t, user)
	}
	// Reloading only role codes must clear administrator grants from a previous load.
	hook := &userRoleRelationQueryHook{}
	db.BunDB.AddQueryHook(hook)
	require.NoError(t, loadUserRoleAssignments(ctx, db.Core, users))
	require.ElementsMatch(t, []string{"personal_user", "custom_owner"}, users[0].Roles())
	for _, user := range users {
		require.False(t, user.HasAdminRole())
		require.False(t, user.CanAdmin())
	}
	for _, query := range hook.queries {
		require.NotContains(t, query, `JOIN "roles"`)
		require.NotContains(t, query, `FROM "roles"`)
		require.NotContains(t, query, "admin_role_permissions")
	}
	require.NoError(t, loadUserRoleAssignments(ctx, db.Core, nil, types.UserWithAdminPermissions))
}

// newAdminRoleManagementTestDB creates only the tables used by administrator role management.
func newAdminRoleManagementTestDB(t *testing.T) *DB {
	t.Helper()
	return newAdminRoleManagementTestDBWithDSN(t, "file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared")
}

// newAdminRoleManagementTestDBWithDSN also supports file databases shared by independent store connections.
func newAdminRoleManagementTestDBWithDSN(t *testing.T, dsn string) *DB {
	t.Helper()
	db, err := NewDB(context.Background(), DBConfig{
		Dialect: DialectSQLite, DSN: dsn, DisableAuditHook: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	connection, err := testutil.OpenSQLiteWithTransactionLocks(dsn)
	require.NoError(t, err)
	require.NoError(t, db.BunDB.DB.Close())
	db.BunDB.DB = connection
	for _, statement := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, uuid TEXT NOT NULL, username TEXT NOT NULL UNIQUE, name TEXT NOT NULL, avatar TEXT NOT NULL DEFAULT '', role_mask TEXT NOT NULL DEFAULT '', deleted_at TIMESTAMP NULL)`,
		`CREATE TABLE roles (role_code TEXT PRIMARY KEY, display_name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', is_admin BOOLEAN NOT NULL DEFAULT FALSE, is_builtin BOOLEAN NOT NULL DEFAULT FALSE, can_delete BOOLEAN NOT NULL DEFAULT TRUE, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, user_id INTEGER NOT NULL)`,
		`CREATE TABLE admin_role_permissions (role_code TEXT NOT NULL REFERENCES roles(role_code), permission_code TEXT NOT NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (role_code, permission_code))`,
		`CREATE TABLE user_roles (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL REFERENCES users(id), role_code TEXT NOT NULL, operator_user_id INTEGER NULL, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, deleted_at TIMESTAMP NULL)`,
		`CREATE UNIQUE INDEX user_roles_active_user_role_idx ON user_roles(user_id, role_code) WHERE deleted_at IS NULL`,
	} {
		_, err = db.BunDB.ExecContext(context.Background(), statement)
		require.NoError(t, err)
	}
	return db
}

// TestUserRoleStoreListAdminUsers checks distinct pagination, role filtering, and complete role projection.
func TestUserRoleStoreListAdminUsers(t *testing.T) {
	ctx := context.Background()
	db := newAdminRoleManagementTestDB(t)
	for _, statement := range []string{
		`INSERT INTO users (id, uuid, username, name, avatar, deleted_at) VALUES
		 (1, 'uuid-1', 'one', 'One', 'avatar-1', NULL), (2, 'uuid-2', 'two', 'Two', '', NULL),
		 (3, 'uuid-3', 'three', 'Three', '', NULL), (4, 'uuid-4', 'deleted', 'Deleted', '', CURRENT_TIMESTAMP),
		 (5, 'uuid-5', 'ordinary', 'Ordinary', '', NULL), (6, 'uuid-6', 'revoked', 'Revoked', '', NULL),
		 (7, 'uuid-7', 'undefined', 'Undefined', '', NULL), (8, 'uuid-8', 'no-role', 'No Role', '', NULL)`,
		`INSERT INTO roles (role_code, display_name, description, user_id, is_admin) VALUES
		 ('alpha', 'Alpha', 'First role', 0, TRUE), ('beta', 'Beta', 'Second role', 0, TRUE), ('zeta', 'Zeta', 'Last role', 0, TRUE),
		 ('member', 'Member', 'Ordinary role', 0, FALSE)`,
		`INSERT INTO user_roles (user_id, role_code, deleted_at) VALUES
		 (1, 'alpha', NULL), (1, 'zeta', NULL), (1, 'member', NULL), (1, 'beta', CURRENT_TIMESTAMP),
		 (2, 'alpha', NULL), (3, 'beta', NULL), (4, 'alpha', NULL), (5, 'member', NULL),
		 (6, 'alpha', CURRENT_TIMESTAMP), (7, 'deleted_admin', NULL)`,
	} {
		_, err := db.BunDB.ExecContext(ctx, statement)
		require.NoError(t, err)
	}
	store := NewUserRoleStoreWithDB(db)
	for _, tc := range []struct {
		name  string
		req   types.AdminUserListRequest
		ids   []int64
		total int
	}{
		{name: "first distinct page", req: types.AdminUserListRequest{Page: 1, Per: 2}, ids: []int64{3, 2}, total: 3},
		{name: "second page", req: types.AdminUserListRequest{Page: 2, Per: 2}, ids: []int64{1}, total: 3},
		{name: "role filters users only", req: types.AdminUserListRequest{RoleCode: "alpha", Page: 1, Per: 10}, ids: []int64{2, 1}, total: 2},
		{name: "revoked role excluded", req: types.AdminUserListRequest{RoleCode: "beta", Page: 1, Per: 10}, ids: []int64{3}, total: 1},
		{name: "ordinary role excluded", req: types.AdminUserListRequest{RoleCode: "member", Page: 1, Per: 10}, ids: []int64{}, total: 0},
		{name: "unknown role", req: types.AdminUserListRequest{RoleCode: "missing", Page: 1, Per: 10}, ids: []int64{}, total: 0},
		{name: "role filter is bound", req: types.AdminUserListRequest{RoleCode: "alpha' OR 1=1 --", Page: 1, Per: 10}, ids: []int64{}, total: 0},
		{name: "page beyond end retains count", req: types.AdminUserListRequest{Page: 3, Per: 2}, ids: []int64{}, total: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			users, total, err := store.ListAdminUsers(ctx, tc.req)
			require.NoError(t, err)
			require.NotNil(t, users)
			require.Equal(t, tc.total, total)
			ids := make([]int64, 0, len(users))
			for _, user := range users {
				ids = append(ids, user.UserID)
				codes := make([]string, 0, len(user.Roles))
				for _, role := range user.Roles {
					codes = append(codes, role.RoleCode)
					require.NotEmpty(t, role.DisplayName)
					require.NotEmpty(t, role.Description)
				}
				switch user.UserID {
				case 1:
					require.Equal(t, []string{"alpha", "zeta"}, codes)
					require.Equal(t, "uuid-1", user.UUID)
					require.Equal(t, "one", user.Username)
					require.Equal(t, "One", user.Nickname)
					require.Equal(t, "avatar-1", user.Avatar)
				case 2:
					require.Equal(t, []string{"alpha"}, codes)
				case 3:
					require.Equal(t, []string{"beta"}, codes)
				}
			}
			require.Equal(t, tc.ids, ids)
		})
	}
}

// TestUserRoleStoreListGlobalAdminUsers filters by grants before pagination and retains every active global role summary.
func TestUserRoleStoreListGlobalAdminUsers(t *testing.T) {
	ctx := context.Background()
	db := newAdminRoleManagementTestDB(t)
	for _, statement := range []string{
		`INSERT INTO users (id, uuid, username, name, avatar, deleted_at) VALUES
		 (1, 'uuid-1', 'one', 'One', 'avatar-1', NULL), (2, 'uuid-2', 'two', 'Two', '', NULL),
		 (3, 'uuid-3', 'name-only', 'Name Only', '', NULL), (4, 'uuid-4', 'deleted', 'Deleted', '', CURRENT_TIMESTAMP),
		 (5, 'uuid-5', 'ordinary', 'Ordinary', '', NULL), (6, 'uuid-6', 'revoked', 'Revoked', '', NULL),
		 (7, 'uuid-7', 'undefined', 'Undefined', '', NULL), (8, 'uuid-8', 'no-role', 'No Role', '', NULL),
		 (9, 'uuid-9', 'custom-only', 'Custom Only', '', NULL)`,
		`INSERT INTO roles (role_code, display_name, description, user_id, is_admin) VALUES
		 ('test', 'Test', 'Global test role', 0, TRUE), ('custom_owner', 'Custom Owner', 'Another global role', 0, TRUE),
		 ('super_admin', 'Name alone', 'No global grant', 0, TRUE), ('module_admin', 'Module Admin', 'Module grant only', 0, TRUE),
		 ('member', 'Member', 'Ordinary role with a wildcard', 0, FALSE)`,
		`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES
		 ('test', '*'), ('test', 'account/user'), ('custom_owner', '*'), ('custom_owner', 'system/dashboard'),
		 ('super_admin', 'account/admin'), ('module_admin', 'account/*'), ('member', '*')`,
		`INSERT INTO user_roles (user_id, role_code, deleted_at) VALUES
		 (1, 'test', NULL), (1, 'custom_owner', NULL), (1, 'custom_owner', CURRENT_TIMESTAMP),
		 (1, 'super_admin', NULL), (1, 'module_admin', NULL), (1, 'member', NULL),
		 (2, 'test', NULL), (3, 'super_admin', NULL), (3, 'module_admin', NULL), (4, 'test', NULL),
		 (5, 'member', NULL), (6, 'test', CURRENT_TIMESTAMP), (7, 'undefined_role', NULL), (9, 'custom_owner', NULL)`,
	} {
		_, err := db.Core.ExecContext(ctx, statement)
		require.NoError(t, err)
	}
	store := NewUserRoleStoreWithDB(db)
	for _, tc := range []struct {
		name  string
		req   types.AdminUserListRequest
		ids   []int64
		total int
	}{
		{name: "first page excludes non-global users before counting", req: types.AdminUserListRequest{Page: 1, Per: 2, GlobalOnly: true}, ids: []int64{9, 2}, total: 3},
		{name: "second page has one distinct multi-role user", req: types.AdminUserListRequest{Page: 2, Per: 2, GlobalOnly: true}, ids: []int64{1}, total: 3},
		{name: "page beyond end retains filtered count", req: types.AdminUserListRequest{Page: 3, Per: 2, GlobalOnly: true}, ids: []int64{}, total: 3},
		{name: "custom role filter preserves other global summaries", req: types.AdminUserListRequest{RoleCode: "test", Page: 1, Per: 10, GlobalOnly: true}, ids: []int64{2, 1}, total: 2},
		{name: "second global role filter", req: types.AdminUserListRequest{RoleCode: "custom_owner", Page: 1, Per: 10, GlobalOnly: true}, ids: []int64{9, 1}, total: 2},
		{name: "unrestricted mode retains all administrator roles", req: types.AdminUserListRequest{Page: 1, Per: 10}, ids: []int64{9, 3, 2, 1}, total: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			users, total, err := store.ListAdminUsers(ctx, tc.req)
			require.NoError(t, err)
			require.NotNil(t, users)
			require.Equal(t, tc.total, total)
			ids := make([]int64, 0, len(users))
			for _, user := range users {
				ids = append(ids, user.UserID)
				codes := make([]string, 0, len(user.Roles))
				for _, role := range user.Roles {
					codes = append(codes, role.RoleCode)
					require.NotEmpty(t, role.DisplayName)
					require.NotEmpty(t, role.Description)
				}
				switch user.UserID {
				case 1:
					if tc.req.GlobalOnly {
						require.Equal(t, []string{"custom_owner", "test"}, codes)
					} else {
						require.Equal(t, []string{"custom_owner", "module_admin", "super_admin", "test"}, codes)
					}
					require.Equal(t, "uuid-1", user.UUID)
					require.Equal(t, "one", user.Username)
					require.Equal(t, "One", user.Nickname)
					require.Equal(t, "avatar-1", user.Avatar)
				case 2:
					require.Equal(t, []string{"test"}, codes)
				case 3:
					require.False(t, tc.req.GlobalOnly)
					require.Equal(t, []string{"module_admin", "super_admin"}, codes)
				case 9:
					require.Equal(t, []string{"custom_owner"}, codes)
				default:
					t.Fatalf("unexpected administrator user %d", user.UserID)
				}
			}
			require.Equal(t, tc.ids, ids)
		})
	}
	for _, roleCode := range []string{"super_admin", "module_admin", "member", "missing", "test' OR 1=1 --"} {
		t.Run("invalid global filter "+roleCode, func(t *testing.T) {
			_, total, err := store.ListAdminUsers(ctx, types.AdminUserListRequest{RoleCode: roleCode, Page: 1, Per: 10, GlobalOnly: true})
			require.ErrorIs(t, err, errorx.ErrForbidden)
			require.Zero(t, total)
		})
	}
}

// seedAdminRoleMutationUsers supplies active targets, authenticated operators, and deleted users.
func seedAdminRoleMutationUsers(t *testing.T, db *DB) {
	t.Helper()
	for _, statement := range []string{
		`INSERT INTO users (id, uuid, username, name, role_mask, deleted_at) VALUES
		 (1, 'target-uuid', 'target', 'Target', 'legacy-role', NULL),
		 (2, 'actor-uuid', 'actor', 'Actor', '', NULL),
		 (3, 'second-uuid', 'second-actor', 'Second Actor', '', NULL),
		 (4, 'deleted-target-uuid', 'deleted-target', 'Deleted Target', '', CURRENT_TIMESTAMP),
		 (5, 'deleted-actor-uuid', 'deleted-actor', 'Deleted Actor', '', CURRENT_TIMESTAMP)`,
		`INSERT INTO roles (role_code, display_name, user_id, is_admin) VALUES ('reader', 'Reader', 0, TRUE), ('writer', 'Writer', 0, TRUE), ('member', 'Member', 0, FALSE)`,
		`INSERT INTO user_roles (user_id, role_code, operator_user_id) VALUES (1, 'member', 2)`,
	} {
		_, err := db.BunDB.ExecContext(context.Background(), statement)
		require.NoError(t, err)
	}
}

// TestUserRoleStoreAdminRoleLifecycle verifies idempotency, audit operators, and regrant history.
func TestUserRoleStoreAdminRoleLifecycle(t *testing.T) {
	ctx := context.Background()
	db := newAdminRoleManagementTestDB(t)
	seedAdminRoleMutationUsers(t, db)
	store := NewUserRoleStoreWithDB(db)
	req := types.AdminUserRoleRequest{UserID: 1, RoleCode: "reader", CurrentUser: "actor"}
	require.NoError(t, store.AssignAdminRole(ctx, req))
	req.CurrentUser = "second-actor"
	require.NoError(t, store.AssignAdminRole(ctx, req))
	var rows []UserRole
	require.NoError(t, db.Core.NewSelect().Model(&rows).WhereAllWithDeleted().Where("role_code = ?", "reader").Order("id").Scan(ctx))
	require.Len(t, rows, 1)
	require.Equal(t, int64(2), *rows[0].OperatorUserID)
	require.False(t, rows[0].CreatedAt.IsZero())
	require.True(t, rows[0].DeletedAt.IsZero())
	firstID := rows[0].ID
	require.NoError(t, store.RemoveAdminRole(ctx, req))
	req.CurrentUser = "actor"
	require.NoError(t, store.RemoveAdminRole(ctx, req))
	rows = nil
	require.NoError(t, db.Core.NewSelect().Model(&rows).WhereAllWithDeleted().Where("role_code = ?", "reader").Order("id").Scan(ctx))
	require.Len(t, rows, 1)
	require.False(t, rows[0].DeletedAt.IsZero())
	require.Equal(t, int64(3), *rows[0].OperatorUserID, "a repeated removal must not rewrite the revocation operator")
	require.NoError(t, store.AssignAdminRole(ctx, req))
	rows = nil
	require.NoError(t, db.Core.NewSelect().Model(&rows).WhereAllWithDeleted().Where("role_code = ?", "reader").Order("id").Scan(ctx))
	require.Len(t, rows, 2)
	require.Equal(t, firstID, rows[0].ID)
	require.False(t, rows[0].DeletedAt.IsZero())
	require.Greater(t, rows[1].ID, firstID)
	require.True(t, rows[1].DeletedAt.IsZero())
	require.Equal(t, int64(2), *rows[1].OperatorUserID)
	req.RoleCode = "writer"
	require.NoError(t, store.RemoveAdminRole(ctx, req))
	var roleMask string
	require.NoError(t, db.Core.NewSelect().TableExpr("users").Column("role_mask").Where("id = 1").Scan(ctx, &roleMask))
	require.Equal(t, "legacy-role", roleMask)
	ordinaryExists, err := db.Core.NewSelect().Model((*UserRole)(nil)).Where("user_id = 1 AND role_code = 'member'").Exists(ctx)
	require.NoError(t, err)
	require.True(t, ordinaryExists)
}

// seedGlobalAdminRoleMutationUsers supplies global roles with arbitrary names and incompatible existing assignments.
func seedGlobalAdminRoleMutationUsers(t *testing.T, db *DB) {
	t.Helper()
	seedAdminRoleMutationUsers(t, db)
	for _, statement := range []string{
		`INSERT INTO roles (role_code, display_name, user_id, is_admin) VALUES
		 ('test', 'Global test role', 0, TRUE), ('custom_owner', 'Custom Owner', 0, TRUE), ('super_admin', 'Name Alone', 0, TRUE)`,
		`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES
		 ('test', '*'), ('test', 'system/dashboard'), ('custom_owner', '*'),
		 ('reader', 'account/*'), ('super_admin', 'account/admin'), ('member', '*')`,
		`INSERT INTO user_roles (user_id, role_code, operator_user_id) VALUES (1, 'reader', 2), (1, 'super_admin', 2)`,
	} {
		_, err := db.Core.ExecContext(context.Background(), statement)
		require.NoError(t, err)
	}
}

// TestUserRoleStoreGlobalAdminRoleLifecycle grants and revokes a global role independently of its name and preserves audit history.
func TestUserRoleStoreGlobalAdminRoleLifecycle(t *testing.T) {
	ctx := context.Background()
	db := newAdminRoleManagementTestDB(t)
	seedGlobalAdminRoleMutationUsers(t, db)
	// Keep another global administrator so this lifecycle test can exercise successful revocation.
	_, err := db.Core.ExecContext(ctx, `INSERT INTO user_roles (user_id, role_code, operator_user_id) VALUES (2, 'custom_owner', 2)`)
	require.NoError(t, err)
	store := NewUserRoleStoreWithDB(db)
	var unrelated []UserRole
	require.NoError(t, db.Core.NewSelect().Model(&unrelated).WhereAllWithDeleted().Order("id").Scan(ctx))
	req := types.AdminUserRoleRequest{UserID: 1, RoleCode: "test", CurrentUser: "actor", GlobalOnly: true}
	require.NoError(t, store.AssignAdminRole(ctx, req))
	req.CurrentUser = "second-actor"
	require.NoError(t, store.AssignAdminRole(ctx, req))
	var rows []UserRole
	require.NoError(t, db.Core.NewSelect().Model(&rows).WhereAllWithDeleted().Where("role_code = ?", "test").Order("id").Scan(ctx))
	require.Len(t, rows, 1)
	require.Equal(t, int64(2), *rows[0].OperatorUserID)
	require.False(t, rows[0].CreatedAt.IsZero())
	require.True(t, rows[0].DeletedAt.IsZero())
	firstID := rows[0].ID
	require.NoError(t, store.RemoveAdminRole(ctx, req))
	req.CurrentUser = "actor"
	require.NoError(t, store.RemoveAdminRole(ctx, req))
	rows = nil
	require.NoError(t, db.Core.NewSelect().Model(&rows).WhereAllWithDeleted().Where("role_code = ?", "test").Order("id").Scan(ctx))
	require.Len(t, rows, 1)
	require.False(t, rows[0].DeletedAt.IsZero())
	require.Equal(t, int64(3), *rows[0].OperatorUserID)
	require.NoError(t, store.AssignAdminRole(ctx, req))
	rows = nil
	require.NoError(t, db.Core.NewSelect().Model(&rows).WhereAllWithDeleted().Where("role_code = ?", "test").Order("id").Scan(ctx))
	require.Len(t, rows, 2)
	require.Equal(t, firstID, rows[0].ID)
	require.False(t, rows[0].DeletedAt.IsZero())
	require.Greater(t, rows[1].ID, firstID)
	require.True(t, rows[1].DeletedAt.IsZero())
	require.Equal(t, int64(2), *rows[1].OperatorUserID)
	req.RoleCode = "custom_owner"
	require.NoError(t, store.RemoveAdminRole(ctx, req), "removing an absent valid global assignment is idempotent")
	var preserved []UserRole
	require.NoError(t, db.Core.NewSelect().Model(&preserved).WhereAllWithDeleted().Where("role_code != ?", "test").Order("id").Scan(ctx))
	require.Equal(t, unrelated, preserved)
	var roleMask string
	require.NoError(t, db.Core.NewSelect().TableExpr("users").Column("role_mask").Where("id = ?", 1).Scan(ctx, &roleMask))
	require.Equal(t, "legacy-role", roleMask)
}

// TestUserRoleStoreGlobalAdminRoleValidation rejects incompatible grants inside transactions without changing assignments.
func TestUserRoleStoreGlobalAdminRoleValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  types.AdminUserRoleRequest
		want error
	}{
		{name: "super administrator name without global grant", req: types.AdminUserRoleRequest{UserID: 1, RoleCode: "super_admin", CurrentUser: "actor", GlobalOnly: true}, want: errorx.ErrForbidden},
		{name: "module wildcard without global grant", req: types.AdminUserRoleRequest{UserID: 1, RoleCode: "reader", CurrentUser: "actor", GlobalOnly: true}, want: errorx.ErrForbidden},
		{name: "administrator without any grants", req: types.AdminUserRoleRequest{UserID: 1, RoleCode: "writer", CurrentUser: "actor", GlobalOnly: true}, want: errorx.ErrForbidden},
		{name: "ordinary role with global grant", req: types.AdminUserRoleRequest{UserID: 1, RoleCode: "member", CurrentUser: "actor", GlobalOnly: true}, want: errorx.ErrForbidden},
		{name: "missing global role", req: types.AdminUserRoleRequest{UserID: 1, RoleCode: "missing", CurrentUser: "actor", GlobalOnly: true}, want: errorx.ErrForbidden},
		{name: "global role code is bound", req: types.AdminUserRoleRequest{UserID: 1, RoleCode: "test' OR 1=1 --", CurrentUser: "actor", GlobalOnly: true}, want: errorx.ErrForbidden},
		{name: "deleted target", req: types.AdminUserRoleRequest{UserID: 4, RoleCode: "test", CurrentUser: "actor", GlobalOnly: true}, want: errorx.ErrNotFound},
		{name: "deleted operator", req: types.AdminUserRoleRequest{UserID: 1, RoleCode: "test", CurrentUser: "deleted-actor", GlobalOnly: true}, want: errorx.ErrUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db := newAdminRoleManagementTestDB(t)
			seedGlobalAdminRoleMutationUsers(t, db)
			store := NewUserRoleStoreWithDB(db)
			var before, after []UserRole
			require.NoError(t, db.Core.NewSelect().Model(&before).WhereAllWithDeleted().Order("id").Scan(ctx))
			require.ErrorIs(t, store.AssignAdminRole(ctx, tc.req), tc.want)
			require.ErrorIs(t, store.RemoveAdminRole(ctx, tc.req), tc.want)
			require.NoError(t, db.Core.NewSelect().Model(&after).WhereAllWithDeleted().Order("id").Scan(ctx))
			require.Equal(t, before, after)
		})
	}
}

// TestUserRoleStoreGlobalAdminGrantRevocation revalidates the grant for every idempotent mutation.
func TestUserRoleStoreGlobalAdminGrantRevocation(t *testing.T) {
	ctx := context.Background()
	db := newAdminRoleManagementTestDB(t)
	seedGlobalAdminRoleMutationUsers(t, db)
	store := NewUserRoleStoreWithDB(db)
	req := types.AdminUserRoleRequest{UserID: 1, RoleCode: "test", CurrentUser: "actor", GlobalOnly: true}
	require.NoError(t, store.AssignAdminRole(ctx, req))
	_, err := db.Core.NewDelete().Model((*AdminRolePermission)(nil)).
		Where("role_code = ?", "test").Where("permission_code = ?", types.AdminPermissionAll).Exec(ctx)
	require.NoError(t, err)
	var before, after []UserRole
	require.NoError(t, db.Core.NewSelect().Model(&before).WhereAllWithDeleted().Order("id").Scan(ctx))
	require.ErrorIs(t, store.AssignAdminRole(ctx, req), errorx.ErrForbidden)
	require.ErrorIs(t, store.RemoveAdminRole(ctx, req), errorx.ErrForbidden)
	require.NoError(t, db.Core.NewSelect().Model(&after).WhereAllWithDeleted().Order("id").Scan(ctx))
	require.Equal(t, before, after)
	users, total, err := store.ListAdminUsers(ctx, types.AdminUserListRequest{Page: 1, Per: 10, GlobalOnly: true})
	require.NoError(t, err)
	require.Empty(t, users)
	require.Zero(t, total)
}

// TestUserRoleStoreGlobalAdminDatabaseFailures fails closed when the global-grant table cannot be queried.
func TestUserRoleStoreGlobalAdminDatabaseFailures(t *testing.T) {
	ctx := context.Background()
	db := newAdminRoleManagementTestDB(t)
	seedGlobalAdminRoleMutationUsers(t, db)
	store := NewUserRoleStoreWithDB(db)
	var before, after []UserRole
	require.NoError(t, db.Core.NewSelect().Model(&before).WhereAllWithDeleted().Order("id").Scan(ctx))
	_, err := db.Core.ExecContext(ctx, `DROP TABLE admin_role_permissions`)
	require.NoError(t, err)
	for _, code := range []string{"", "test"} {
		_, _, err := store.ListAdminUsers(ctx, types.AdminUserListRequest{RoleCode: code, Page: 1, Per: 10, GlobalOnly: true})
		require.Error(t, err)
	}
	req := types.AdminUserRoleRequest{UserID: 1, RoleCode: "test", CurrentUser: "actor", GlobalOnly: true}
	require.Error(t, store.AssignAdminRole(ctx, req))
	require.Error(t, store.RemoveAdminRole(ctx, req))
	require.NoError(t, db.Core.NewSelect().Model(&after).WhereAllWithDeleted().Order("id").Scan(ctx))
	require.Equal(t, before, after)
}

// TestUserRoleStoreAdminRoleValidation checks invalid references fail before mutating role history.
func TestUserRoleStoreAdminRoleValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  types.AdminUserRoleRequest
		want error
	}{
		{name: "missing target", req: types.AdminUserRoleRequest{UserID: 99, RoleCode: "reader", CurrentUser: "actor"}, want: errorx.ErrNotFound},
		{name: "deleted target", req: types.AdminUserRoleRequest{UserID: 4, RoleCode: "reader", CurrentUser: "actor"}, want: errorx.ErrNotFound},
		{name: "ordinary role", req: types.AdminUserRoleRequest{UserID: 1, RoleCode: "member", CurrentUser: "actor"}, want: errorx.ErrNotFound},
		{name: "missing role", req: types.AdminUserRoleRequest{UserID: 1, RoleCode: "missing", CurrentUser: "actor"}, want: errorx.ErrNotFound},
		{name: "missing operator", req: types.AdminUserRoleRequest{UserID: 1, RoleCode: "reader", CurrentUser: "missing"}, want: errorx.ErrUnauthorized},
		{name: "deleted operator", req: types.AdminUserRoleRequest{UserID: 1, RoleCode: "reader", CurrentUser: "deleted-actor"}, want: errorx.ErrUnauthorized},
		{name: "empty operator", req: types.AdminUserRoleRequest{UserID: 1, RoleCode: "reader"}, want: errorx.ErrUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db := newAdminRoleManagementTestDB(t)
			seedAdminRoleMutationUsers(t, db)
			_, err := db.BunDB.ExecContext(ctx, `INSERT INTO user_roles (user_id, role_code, operator_user_id) VALUES (1, 'reader', 2)`)
			require.NoError(t, err)
			store := NewUserRoleStoreWithDB(db)
			var before, after []UserRole
			require.NoError(t, db.Core.NewSelect().Model(&before).WhereAllWithDeleted().Order("id").Scan(ctx))
			require.ErrorIs(t, store.AssignAdminRole(ctx, tc.req), tc.want)
			require.ErrorIs(t, store.RemoveAdminRole(ctx, tc.req), tc.want)
			require.NoError(t, db.Core.NewSelect().Model(&after).WhereAllWithDeleted().Order("id").Scan(ctx))
			require.Equal(t, before, after)
		})
	}
}

// TestUserRoleStoreDatabaseFailures verifies list and transaction errors propagate without successful mutations.
func TestUserRoleStoreDatabaseFailures(t *testing.T) {
	ctx := context.Background()
	db := newAdminRoleManagementTestDB(t)
	seedAdminRoleMutationUsers(t, db)
	_, err := db.BunDB.ExecContext(ctx, `DROP TABLE user_roles`)
	require.NoError(t, err)
	store := NewUserRoleStoreWithDB(db)
	_, _, err = store.ListAdminUsers(ctx, types.AdminUserListRequest{Page: 1, Per: 10})
	require.Error(t, err)
	req := types.AdminUserRoleRequest{UserID: 1, RoleCode: "reader", CurrentUser: "actor"}
	require.Error(t, store.AssignAdminRole(ctx, req))
	require.Error(t, store.RemoveAdminRole(ctx, req))
}

// TestUserRoleStoreRemoveRoleRetainsAccountAdmin checks matching grants, inactive holders, multiple roles, and idempotency.
func TestUserRoleStoreRemoveRoleRetainsAccountAdmin(t *testing.T) {
	for _, tc := range []struct {
		name, grant, actor string
		statements         []string
		allowed            bool
		globalOnly         bool
	}{
		{name: "last exact grant self revocation", grant: "account/admin", actor: "target"},
		{name: "last account wildcard self revocation", grant: "account/*", actor: "target"},
		{name: "last global grant arbitrary role name", grant: "*", actor: "target"},
		{name: "last grant revoked by another operator", grant: "account/admin", actor: "actor"},
		{name: "another user with exact grant", grant: "*", actor: "target", allowed: true, statements: []string{
			`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES ('writer', 'account/admin')`,
			`INSERT INTO user_roles (user_id, role_code) VALUES (2, 'writer')`,
		}},
		{name: "another user with account wildcard", grant: "account/admin", actor: "target", allowed: true, statements: []string{
			`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES ('writer', 'account/*')`,
			`INSERT INTO user_roles (user_id, role_code) VALUES (2, 'writer')`,
		}},
		{name: "same user retains another global role", grant: "account/admin", actor: "target", allowed: true, statements: []string{
			`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES ('writer', '*')`,
			`INSERT INTO user_roles (user_id, role_code) VALUES (1, 'writer')`,
		}},
		{name: "global policy excludes another exact grant", grant: "*", actor: "target", globalOnly: true, statements: []string{
			`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES ('writer', 'account/admin')`,
			`INSERT INTO user_roles (user_id, role_code) VALUES (2, 'writer')`,
		}},
		{name: "global policy excludes another account wildcard", grant: "*", actor: "target", globalOnly: true, statements: []string{
			`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES ('writer', 'account/*')`,
			`INSERT INTO user_roles (user_id, role_code) VALUES (2, 'writer')`,
		}},
		{name: "global policy excludes own other module role", grant: "*", actor: "target", globalOnly: true, statements: []string{
			`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES ('writer', 'account/admin')`,
			`INSERT INTO user_roles (user_id, role_code) VALUES (1, 'writer')`,
		}},
		{name: "global policy retains another global user", grant: "*", actor: "target", globalOnly: true, allowed: true, statements: []string{
			`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES ('writer', '*')`,
			`INSERT INTO user_roles (user_id, role_code) VALUES (2, 'writer')`,
		}},
		{name: "global policy retains own other global role", grant: "*", actor: "target", globalOnly: true, allowed: true, statements: []string{
			`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES ('writer', '*')`,
			`INSERT INTO user_roles (user_id, role_code) VALUES (1, 'writer')`,
		}},
		{name: "duplicate grants are not another holder", grant: "account/admin", actor: "target", statements: []string{
			`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES ('reader', 'account/*'), ('reader', '*')`,
		}},
		{name: "deleted user does not count", grant: "account/admin", actor: "target", statements: []string{
			`INSERT INTO user_roles (user_id, role_code) VALUES (4, 'reader')`,
		}},
		{name: "revoked relation does not count", grant: "account/admin", actor: "target", statements: []string{
			`INSERT INTO user_roles (user_id, role_code, deleted_at) VALUES (2, 'reader', CURRENT_TIMESTAMP)`,
		}},
		{name: "ordinary role wildcard does not count", grant: "account/admin", actor: "target", statements: []string{
			`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES ('member', '*')`,
		}},
		{name: "administrator role without grant does not count", grant: "account/admin", actor: "target", statements: []string{
			`INSERT INTO user_roles (user_id, role_code) VALUES (2, 'writer')`,
		}},
		{name: "other account submodule does not count", grant: "account/admin", actor: "target", statements: []string{
			`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES ('writer', 'account/user')`,
			`INSERT INTO user_roles (user_id, role_code) VALUES (2, 'writer')`,
		}},
		{name: "non administrator definition does not count", grant: "account/admin", actor: "target", statements: []string{
			`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES ('writer', '*')`,
			`INSERT INTO user_roles (user_id, role_code) VALUES (2, 'writer')`,
			`UPDATE roles SET is_admin = FALSE WHERE role_code = 'writer'`,
		}},
		{name: "already revoked management assignment is idempotent", grant: "account/admin", actor: "target", allowed: true, statements: []string{
			`UPDATE user_roles SET deleted_at = CURRENT_TIMESTAMP WHERE user_id = 1 AND role_code = 'reader'`,
		}},
		{name: "unrelated grant can be removed", grant: "asset/model", actor: "target", allowed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db := newAdminRoleManagementTestDB(t)
			seedAdminRoleMutationUsers(t, db)
			_, err := db.Core.ExecContext(ctx, `INSERT INTO admin_role_permissions (role_code, permission_code) VALUES ('reader', ?)`, tc.grant)
			require.NoError(t, err)
			_, err = db.Core.ExecContext(ctx, `INSERT INTO user_roles (user_id, role_code, operator_user_id) VALUES (1, 'reader', 2)`)
			require.NoError(t, err)
			for _, statement := range tc.statements {
				_, err = db.Core.ExecContext(ctx, statement)
				require.NoError(t, err)
			}
			store := NewUserRoleStoreWithDB(db)
			req := types.AdminUserRoleRequest{UserID: 1, RoleCode: "reader", CurrentUser: tc.actor, GlobalOnly: tc.globalOnly}
			err = store.RemoveAdminRole(ctx, req)
			if tc.allowed {
				require.NoError(t, err)
				require.NoError(t, store.RemoveAdminRole(ctx, req), "repeated removal stays idempotent")
			} else {
				require.ErrorIs(t, err, errorx.ErrLastAccountAdmin)
			}
			var assignment UserRole
			require.NoError(t, db.Core.NewSelect().Model(&assignment).WhereAllWithDeleted().Where("user_id = 1 AND role_code = 'reader'").Scan(ctx))
			require.Equal(t, tc.allowed, !assignment.DeletedAt.IsZero())
			if !tc.allowed {
				require.Equal(t, int64(2), *assignment.OperatorUserID, "a rejected removal preserves audit data")
			}
		})
	}
}

// TestUserRoleStoreRemoveRoleLocksBeforeChecking proves PostgreSQL acquires the shared transaction lock before reads.
func TestUserRoleStoreRemoveRoleLocksBeforeChecking(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		require.NoError(t, bunDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})
	store := NewUserRoleStoreWithDB(&DB{BunDB: bunDB, Operator: Operator{Core: bunDB}})
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf("SELECT pg_advisory_xact_lock(%d)", adminRoleRevocationLockKey))).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT EXISTS`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`SELECT .*id.* FROM "users"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(`SELECT EXISTS`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`SELECT EXISTS`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`SELECT EXISTS`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectRollback()
	mock.ExpectClose()
	err = store.RemoveAdminRole(context.Background(), types.AdminUserRoleRequest{UserID: 1, RoleCode: "custom", CurrentUser: "actor"})
	require.ErrorIs(t, err, errorx.ErrLastAccountAdmin)
}

// adminRoleRevocationLockTestHook coordinates overlapping lock requests on independent database connections.
type adminRoleRevocationLockTestHook struct {
	locked, attempted chan struct{}
	release           <-chan struct{}
}

// BeforeQuery signals when the second connection attempts to acquire the revocation writer lock.
func (h *adminRoleRevocationLockTestHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if h.attempted != nil && strings.HasPrefix(event.Query, "SELECT pg_advisory_xact_lock(") {
		close(h.attempted)
	}
	return ctx
}

// AfterQuery holds the first writer until the competing request has attempted its own lock.
func (h *adminRoleRevocationLockTestHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	if h.locked != nil && event.Err == nil && strings.HasPrefix(event.Query, "SELECT pg_advisory_xact_lock(") {
		close(h.locked)
		select {
		case <-h.release:
		case <-ctx.Done():
		}
	}
}

// TestUserRoleStoreConcurrentAccountAdminRevocations forces overlapping transactions to retain exactly one eligible user.
func TestUserRoleStoreConcurrentAccountAdminRevocations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dsn := "file:" + filepath.Join(t.TempDir(), "admin_roles.db")
	first := newAdminRoleManagementTestDBWithDSN(t, dsn)
	second, err := NewDB(ctx, DBConfig{Dialect: DialectSQLite, DSN: dsn, DisableAuditHook: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, second.Close()) })
	connection, err := testutil.OpenSQLiteWithTransactionLocks(dsn)
	require.NoError(t, err)
	require.NoError(t, second.BunDB.DB.Close())
	second.BunDB.DB = connection
	for _, db := range []*DB{first, second} {
		db.BunDB.SetMaxOpenConns(1)
		_, err = db.Core.ExecContext(ctx, `PRAGMA busy_timeout = 5000`)
		require.NoError(t, err)
	}
	seedAdminRoleMutationUsers(t, first)
	for _, statement := range []string{
		`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES ('reader', 'account/admin'), ('writer', 'account/*')`,
		`INSERT INTO user_roles (user_id, role_code) VALUES (1, 'reader'), (2, 'writer')`,
	} {
		_, err = first.Core.ExecContext(ctx, statement)
		require.NoError(t, err)
	}
	locked, attempted, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	releaseWriter := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseWriter)
	first.BunDB.AddQueryHook(&adminRoleRevocationLockTestHook{locked: locked, release: release})
	second.BunDB.AddQueryHook(&adminRoleRevocationLockTestHook{attempted: attempted})
	results := make(chan error, 2)
	for _, operation := range []struct {
		db  *DB
		req types.AdminUserRoleRequest
	}{
		{first, types.AdminUserRoleRequest{UserID: 1, RoleCode: "reader", CurrentUser: "target"}},
		{second, types.AdminUserRoleRequest{UserID: 2, RoleCode: "writer", CurrentUser: "actor"}},
	} {
		go func() {
			if operation.db == second {
				select {
				case <-locked:
				case <-ctx.Done():
					results <- ctx.Err()
					return
				}
			}
			results <- NewUserRoleStoreWithDB(operation.db).RemoveAdminRole(ctx, operation.req)
		}()
	}
	select {
	case <-attempted:
		releaseWriter()
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var succeeded, rejected int
	for range 2 {
		select {
		case err = <-results:
			if err == nil {
				succeeded++
			} else {
				require.True(t, errors.Is(err, errorx.ErrLastAccountAdmin), "unexpected removal failure: %v", err)
				rejected++
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	require.Equal(t, 1, succeeded)
	require.Equal(t, 1, rejected)
	candidates, err := adminPermissionCandidates(types.AdminAccountAdmin)
	require.NoError(t, err)
	require.NoError(t, first.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		count, err := accountAdminAssignmentsQuery(tx, candidates).Count(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)
		return nil
	}))
}

package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
)

// TestAdminRoleStoreList checks newest-first ordering with stable ties, metadata, and permission sets.
func TestAdminRoleStoreList(t *testing.T) {
	ctx := context.Background()
	db := newAdminRoleManagementTestDB(t)
	store := NewAdminRoleStoreWithDB(db)
	roles, err := store.List(ctx)
	require.NoError(t, err)
	require.Equal(t, []types.AdminRoleDetail{}, roles)
	_, err = db.BunDB.ExecContext(ctx, `INSERT INTO roles (role_code, display_name, description, user_id, created_at, is_admin, is_builtin, can_delete) VALUES
	 ('middle', 'Middle', 'Selected modules', 0, '2026-09-28 08:00:00', TRUE, TRUE, TRUE),
	 ('alpha', 'Alpha', 'No grants yet', 7, '2026-09-28 08:00:00', TRUE, FALSE, TRUE),
	 ('zeta', 'Zeta', 'All modules', 42, '2026-09-29 08:00:00', TRUE, TRUE, FALSE),
	 ('member', 'Member', 'Ordinary role', 0, '2026-09-30 08:00:00', FALSE, TRUE, TRUE)`)
	require.NoError(t, err)
	_, err = db.BunDB.ExecContext(ctx, `INSERT INTO admin_role_permissions (role_code, permission_code) VALUES
	 ('zeta', '*'), ('middle', 'asset/*'), ('middle', 'account/user'), ('member', '*')`)
	require.NoError(t, err)
	roles, err = store.List(ctx)
	require.NoError(t, err)
	require.Len(t, roles, 3)
	require.Equal(t, "zeta", roles[0].RoleCode)
	require.True(t, roles[0].IsAdmin)
	require.True(t, roles[0].IsBuiltin)
	require.False(t, roles[0].CanDelete)
	require.Equal(t, []types.AdminPermission{"*"}, roles[0].Permissions)
	require.True(t, roles[0].CreatedAt.After(roles[1].CreatedAt))
	require.Equal(t, "alpha", roles[1].RoleCode)
	require.True(t, roles[1].IsAdmin)
	require.False(t, roles[1].IsBuiltin)
	require.True(t, roles[1].CanDelete)
	require.Equal(t, "Alpha", roles[1].DisplayName)
	require.Equal(t, "No grants yet", roles[1].Description)
	require.Equal(t, int64(7), roles[1].UserID)
	require.False(t, roles[1].CreatedAt.IsZero())
	require.Equal(t, []types.AdminPermission{}, roles[1].Permissions)
	require.Equal(t, roles[1].CreatedAt, roles[2].CreatedAt)
	require.Equal(t, "middle", roles[2].RoleCode)
	require.Equal(t, []types.AdminPermission{"account/user", "asset/*"}, roles[2].Permissions)
	_, err = db.BunDB.ExecContext(ctx, `DROP TABLE admin_role_permissions`)
	require.NoError(t, err)
	_, err = store.List(ctx)
	require.Error(t, err)
}

// TestAdminRoleStoreListByUsername verifies user scope, multiple roles, and inactive assignments.
func TestAdminRoleStoreListByUsername(t *testing.T) {
	ctx := context.Background()
	db := newAdminRoleManagementTestDB(t)
	for _, statement := range []string{
		`INSERT INTO users (id, uuid, username, name, deleted_at) VALUES
		 (1, 'alice-uuid', 'alice', 'Alice', NULL), (2, 'bob-uuid', 'bob', 'Bob', NULL),
		 (3, 'deleted-uuid', 'deleted', 'Deleted', CURRENT_TIMESTAMP),
		 (4, 'member-uuid', 'member', 'Member', NULL), (5, 'revoked-uuid', 'revoked', 'Revoked', NULL)`,
		`INSERT INTO roles (role_code, display_name, description, is_admin, is_builtin, can_delete, created_at, user_id) VALUES
		 ('asset_admin', 'Asset Admin', 'Asset management', TRUE, TRUE, TRUE, '2026-09-29 08:00:00', 0),
		 ('custom_admin', 'Custom Admin', 'No grants yet', TRUE, FALSE, TRUE, '2026-09-28 08:00:00', 42),
		 ('revoked_admin', 'Revoked Admin', 'Revoked role', TRUE, TRUE, TRUE, '2026-09-30 08:00:00', 0),
		 ('other_admin', 'Other Admin', 'Another user only', TRUE, TRUE, FALSE, '2026-09-30 08:00:00', 0),
		 ('member', 'Member', 'Ordinary role', FALSE, TRUE, TRUE, '2026-09-30 08:00:00', 0)`,
		`INSERT INTO admin_role_permissions (role_code, permission_code) VALUES
		 ('asset_admin', 'asset/*'), ('asset_admin', 'system/dashboard'), ('revoked_admin', '*'), ('other_admin', '*'), ('member', '*')`,
		`INSERT INTO user_roles (user_id, role_code, deleted_at) VALUES
		 (1, 'asset_admin', NULL), (1, 'asset_admin', CURRENT_TIMESTAMP), (1, 'custom_admin', NULL),
		 (1, 'revoked_admin', CURRENT_TIMESTAMP), (1, 'member', NULL), (1, 'undefined_role', NULL),
		 (2, 'other_admin', NULL), (3, 'asset_admin', NULL), (4, 'member', NULL), (5, 'asset_admin', CURRENT_TIMESTAMP)`,
	} {
		_, err := db.Core.ExecContext(ctx, statement)
		require.NoError(t, err)
	}
	store := NewAdminRoleStoreWithDB(db)
	roles, err := store.ListByUsername(ctx, "alice")
	require.NoError(t, err)
	require.Len(t, roles, 2)
	require.Equal(t, "asset_admin", roles[0].RoleCode)
	require.Equal(t, "Asset Admin", roles[0].DisplayName)
	require.Equal(t, "Asset management", roles[0].Description)
	require.True(t, roles[0].IsAdmin)
	require.True(t, roles[0].IsBuiltin)
	require.True(t, roles[0].CanDelete)
	require.Equal(t, int64(0), roles[0].UserID)
	require.Equal(t, []types.AdminPermission{"asset/*", "system/dashboard"}, roles[0].Permissions)
	require.Equal(t, "custom_admin", roles[1].RoleCode)
	require.Equal(t, "No grants yet", roles[1].Description)
	require.False(t, roles[1].IsBuiltin)
	require.Equal(t, int64(42), roles[1].UserID)
	require.Equal(t, []types.AdminPermission{}, roles[1].Permissions)
	require.True(t, roles[0].CreatedAt.After(roles[1].CreatedAt))
	for _, username := range []string{"", "unknown", "deleted", "member", "revoked", "alice' OR 1=1 --"} {
		t.Run(username, func(t *testing.T) {
			roles, err := store.ListByUsername(ctx, username)
			require.NoError(t, err)
			require.Equal(t, []types.AdminRoleDetail{}, roles)
		})
	}
	roles, err = store.ListByUsername(ctx, "bob")
	require.NoError(t, err)
	require.Len(t, roles, 1)
	require.Equal(t, "other_admin", roles[0].RoleCode)
	require.False(t, roles[0].CanDelete)
	require.Equal(t, []types.AdminPermission{"*"}, roles[0].Permissions)
	_, err = db.Core.ExecContext(ctx, `DROP TABLE admin_role_permissions`)
	require.NoError(t, err)
	_, err = store.ListByUsername(ctx, "alice")
	require.Error(t, err)
	_, err = db.Core.ExecContext(ctx, `DROP TABLE users`)
	require.NoError(t, err)
	_, err = store.ListByUsername(ctx, "alice")
	require.Error(t, err)
	roles, err = store.ListByUsername(ctx, "")
	require.NoError(t, err)
	require.Equal(t, []types.AdminRoleDetail{}, roles)
}

// TestAdminRoleStoreAnyPermission verifies active membership independently of module grants.
func TestAdminRoleStoreAnyPermission(t *testing.T) {
	for _, tc := range []struct {
		name          string
		username      string
		roleCode      string
		deletedUser   bool
		deletedRole   bool
		otherUser     bool
		secondRole    bool
		missingRole   bool
		ordinaryRole  bool
		dropRoleTable bool
		wantAllowed   bool
		wantError     bool
	}{
		{name: "custom role without module grants", username: "alice", roleCode: "custom_admin", wantAllowed: true},
		{name: "administrator code without admin suffix", username: "alice", roleCode: "maintainer", wantAllowed: true},
		{name: "ordinary role without administrator definition", username: "alice", roleCode: "member", missingRole: true},
		{name: "defined ordinary role", username: "alice", roleCode: "member", ordinaryRole: true},
		{name: "defined ordinary role alongside administrator role", username: "alice", roleCode: "member", ordinaryRole: true, secondRole: true, wantAllowed: true},
		{name: "missing role definition", username: "alice", missingRole: true},
		{name: "revoked assignment", username: "alice", deletedRole: true},
		{name: "deleted user", username: "alice", deletedUser: true},
		{name: "different user", username: "alice", otherUser: true},
		{name: "second active administrator role", username: "alice", deletedRole: true, secondRole: true, wantAllowed: true},
		{name: "ordinary role alongside administrator role", username: "alice", roleCode: "member", missingRole: true, secondRole: true, wantAllowed: true},
		{name: "unknown user", username: "unknown"},
		{name: "username is a bound value", username: "alice' OR 1=1 --"},
		{name: "empty username skips database", dropRoleTable: true},
		{name: "database failure", username: "alice", dropRoleTable: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
			db, err := NewDB(ctx, DBConfig{Dialect: DialectSQLite, DSN: dsn, DisableAuditHook: true})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			for _, statement := range []string{
				`CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE, deleted_at TIMESTAMP NULL)`,
				`CREATE TABLE user_roles (user_id INTEGER NOT NULL, role_code TEXT NOT NULL, deleted_at TIMESTAMP NULL)`,
				`CREATE TABLE roles (role_code TEXT PRIMARY KEY, is_admin BOOLEAN NOT NULL DEFAULT FALSE)`,
			} {
				_, err = db.BunDB.ExecContext(ctx, statement)
				require.NoError(t, err)
			}
			var userDeletedAt, roleDeletedAt any
			if tc.deletedUser {
				userDeletedAt = time.Now()
			}
			if tc.deletedRole {
				roleDeletedAt = time.Now()
			}
			_, err = db.BunDB.ExecContext(ctx, `INSERT INTO users VALUES (1, 'alice', ?), (2, 'bob', NULL)`, userDeletedAt)
			require.NoError(t, err)
			roleCode := tc.roleCode
			if roleCode == "" {
				roleCode = "asset_admin"
			}
			if !tc.missingRole {
				_, err = db.BunDB.ExecContext(ctx, `INSERT INTO roles (role_code, is_admin) VALUES (?, ?)`, roleCode, !tc.ordinaryRole)
				require.NoError(t, err)
			}
			userID := 1
			if tc.otherUser {
				userID = 2
			}
			_, err = db.BunDB.ExecContext(ctx, `INSERT INTO user_roles VALUES (?, ?, ?)`, userID, roleCode, roleDeletedAt)
			require.NoError(t, err)
			if tc.secondRole {
				_, err = db.BunDB.ExecContext(ctx, `INSERT INTO roles (role_code, is_admin) VALUES ('finance_admin', TRUE)`)
				require.NoError(t, err)
				_, err = db.BunDB.ExecContext(ctx, `INSERT INTO user_roles VALUES (1, 'finance_admin', NULL)`)
				require.NoError(t, err)
			}
			if tc.dropRoleTable {
				_, err = db.BunDB.ExecContext(ctx, `DROP TABLE roles`)
				require.NoError(t, err)
			}
			allowed, err := NewAdminRoleStoreWithDB(db).HasPermission(ctx, tc.username, types.AdminPermissionAny)
			if tc.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantAllowed, allowed)
		})
	}
}

// TestAdminRoleMetadataPersistence verifies metadata and preserves explicit false deletion flags.
func TestAdminRoleMetadataPersistence(t *testing.T) {
	ctx := context.Background()
	db, err := NewDB(ctx, DBConfig{Dialect: DialectSQLite, DSN: "file:admin_role_metadata?mode=memory&cache=shared", DisableAuditHook: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	_, err = db.BunDB.ExecContext(ctx, `CREATE TABLE roles (
		role_code TEXT PRIMARY KEY,
		display_name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		is_admin BOOLEAN NOT NULL DEFAULT FALSE,
		is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
		can_delete BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		user_id INTEGER NOT NULL
	)`)
	require.NoError(t, err)

	role := &Role{RoleCode: "super_admin", DisplayName: "Super Admin", Description: "Manages everything", IsAdmin: true, IsBuiltin: true, CanDelete: false, UserID: 42}
	_, err = db.BunDB.NewInsert().Model(role).Exec(ctx)
	require.NoError(t, err)

	var stored Role
	err = db.BunDB.NewSelect().Model(&stored).Where("role_code = ?", role.RoleCode).Scan(ctx)
	require.NoError(t, err)
	require.Equal(t, role.RoleCode, stored.RoleCode)
	require.Equal(t, role.DisplayName, stored.DisplayName)
	require.Equal(t, role.Description, stored.Description)
	require.True(t, stored.IsAdmin)
	require.True(t, stored.IsBuiltin)
	require.False(t, stored.CanDelete)
	require.Equal(t, role.UserID, stored.UserID)
	require.False(t, stored.CreatedAt.IsZero())
	ordinary := &Role{RoleCode: "member", DisplayName: "Member", CanDelete: true}
	_, err = db.BunDB.NewInsert().Model(ordinary).Exec(ctx)
	require.NoError(t, err)
	var storedOrdinary Role
	require.NoError(t, db.BunDB.NewSelect().Model(&storedOrdinary).Where("role_code = ?", ordinary.RoleCode).Scan(ctx))
	require.False(t, storedOrdinary.IsAdmin)
	require.False(t, storedOrdinary.IsBuiltin)
	require.True(t, storedOrdinary.CanDelete)
}

// TestAdminRoleStoreHasPermission covers OR matching, wildcards, and inactive grants.
func TestAdminRoleStoreHasPermission(t *testing.T) {
	tests := []struct {
		name             string
		grant            string
		required         []types.AdminPermission
		grantRoleCode    string
		assignedRoleCode string
		roleUserID       int64
		ordinaryRole     bool
		nonAdminRole     bool
		deletedRole      bool
		deletedUser      bool
		extraAdminRole   bool
		dropGrantsTable  bool
		wantAllowed      bool
		wantError        bool
	}{
		{name: "exact grant", grant: "account/user", required: []types.AdminPermission{types.AdminAccountUser}, wantAllowed: true},
		{name: "all requires global wildcard", grant: "*", required: []types.AdminPermission{types.AdminPermissionAll}, wantAllowed: true},
		{name: "all rejects concrete grant", grant: "account/user", required: []types.AdminPermission{types.AdminPermissionAll}},
		{name: "all rejects module wildcard", grant: "account/*", required: []types.AdminPermission{types.AdminPermissionAll}},
		{name: "super admin name does not imply all", grant: "account/user", grantRoleCode: "super_admin", assignedRoleCode: "super_admin", required: []types.AdminPermission{types.AdminPermissionAll}},
		{name: "all excludes deleted assignment", grant: "*", deletedRole: true, required: []types.AdminPermission{types.AdminPermissionAll}},
		{name: "all excludes deleted user", grant: "*", deletedUser: true, required: []types.AdminPermission{types.AdminPermissionAll}},
		{name: "all excludes ordinary user", grant: "*", ordinaryRole: true, required: []types.AdminPermission{types.AdminPermissionAll}},
		{name: "ordinary definition rejects global grant", grant: "*", nonAdminRole: true, required: []types.AdminPermission{types.AdminPermissionAll}},
		{name: "ordinary definition rejects concrete grant", grant: "account/user", nonAdminRole: true, required: []types.AdminPermission{types.AdminAccountUser}},
		{name: "ordinary definition rejects any", grant: "*", nonAdminRole: true, required: []types.AdminPermission{types.AdminPermissionAny}},
		{name: "ordinary definition with administrator role", grant: "*", nonAdminRole: true, extraAdminRole: true, required: []types.AdminPermission{types.AdminAccountUser}, wantAllowed: true},
		{name: "all excludes another user's grant", grant: "*", roleUserID: 2, required: []types.AdminPermission{types.AdminPermissionAll}},
		{name: "all or concrete permits concrete", grant: "account/user", required: []types.AdminPermission{types.AdminPermissionAll, types.AdminAccountUser}, wantAllowed: true},
		{name: "all or any retains membership semantics", grant: "account/user", required: []types.AdminPermission{types.AdminPermissionAll, types.AdminPermissionAny}, dropGrantsTable: true, wantAllowed: true},
		{name: "all with invalid alternative fails closed", grant: "*", required: []types.AdminPermission{types.AdminPermissionAll, ""}, wantError: true},
		{name: "any and all cannot hide invalid alternative", grant: "*", required: []types.AdminPermission{types.AdminPermissionAny, types.AdminPermissionAll, "account/*"}, wantError: true},
		{name: "any role ignores unmatched concrete permission", grant: "account/user", required: []types.AdminPermission{types.AdminAssetModel, types.AdminPermissionAny}, dropGrantsTable: true, wantAllowed: true},
		{name: "any role before concrete permission", grant: "account/user", required: []types.AdminPermission{types.AdminPermissionAny, types.AdminAssetModel}, wantAllowed: true},
		{name: "any role with invalid permission fails closed", grant: "*", required: []types.AdminPermission{types.AdminPermissionAny, ""}, wantError: true},
		{name: "any role mixed with concrete denies ordinary user", grant: "account/user", required: []types.AdminPermission{types.AdminPermissionAny, types.AdminAccountUser}, ordinaryRole: true},
		{name: "stored any does not grant module access", grant: string(types.AdminPermissionAny), required: []types.AdminPermission{types.AdminAccountUser}},
		{name: "module wildcard", grant: "account/*", required: []types.AdminPermission{types.AdminAccountUser}, wantAllowed: true},
		{name: "global wildcard", grant: "*", required: []types.AdminPermission{types.AdminAccountUser}, wantAllowed: true},
		{name: "action wildcard", grant: "account/user/*", required: []types.AdminPermission{"account/user/read"}, wantAllowed: true},
		{name: "wrong submodule", grant: "account/organization", required: []types.AdminPermission{types.AdminAccountUser}},
		{name: "permission from second role", grant: "account/organization", required: []types.AdminPermission{types.AdminAccountUser}, extraAdminRole: true, wantAllowed: true},
		{name: "wrong role", grant: "account/user", grantRoleCode: "another_role", required: []types.AdminPermission{types.AdminAccountUser}},
		{name: "other user", grant: "account/user", roleUserID: 2, required: []types.AdminPermission{types.AdminAccountUser}},
		{name: "ordinary role without administrator definition", grant: "account/user", ordinaryRole: true, required: []types.AdminPermission{types.AdminAccountUser}},
		{name: "ordinary role alongside administrator role", grant: "account/organization", ordinaryRole: true, extraAdminRole: true, required: []types.AdminPermission{types.AdminAccountUser}, wantAllowed: true},
		{name: "deleted role", grant: "account/user", deletedRole: true, required: []types.AdminPermission{types.AdminAccountUser}},
		{name: "deleted user", grant: "account/user", deletedUser: true, required: []types.AdminPermission{types.AdminAccountUser}},
		{name: "invalid required permission", grant: "*", required: []types.AdminPermission{"account/*"}, wantError: true},
		{name: "first permission matches", grant: "account/user", required: []types.AdminPermission{types.AdminAccountUser, types.AdminAssetModel}, wantAllowed: true},
		{name: "second permission matches", grant: "asset/model", required: []types.AdminPermission{types.AdminAccountUser, types.AdminAssetModel}, wantAllowed: true},
		{name: "none match", grant: "account/organization", required: []types.AdminPermission{types.AdminAccountUser, types.AdminAssetModel}},
		{name: "second permission module wildcard", grant: "asset/*", required: []types.AdminPermission{types.AdminAccountUser, types.AdminAssetModel}, wantAllowed: true},
		{name: "sync exact permission", grant: "asset/sync", required: []types.AdminPermission{types.AdminAssetSync}, wantAllowed: true},
		{name: "sync module wildcard", grant: "asset/*", required: []types.AdminPermission{types.AdminAssetSync}, wantAllowed: true},
		{name: "sync global wildcard", grant: "*", required: []types.AdminPermission{types.AdminAssetSync}, wantAllowed: true},
		{name: "mirror does not grant sync", grant: "asset/mirror", required: []types.AdminPermission{types.AdminAssetSync}},
		{name: "sync does not grant mirror", grant: "asset/sync", required: []types.AdminPermission{types.AdminAssetMirror}},
		{name: "multiple permissions global wildcard", grant: "*", required: []types.AdminPermission{types.AdminAccountUser, types.AdminAssetModel}, wantAllowed: true},
		{name: "duplicate permissions", grant: "account/*", required: []types.AdminPermission{types.AdminAccountUser, types.AdminAccountUser}, wantAllowed: true},
		{name: "invalid after matching permission", grant: "account/user", required: []types.AdminPermission{types.AdminAccountUser, ""}, wantError: true},
		{name: "no permissions denies superadmin", grant: "*"},
		{name: "no permissions skips database", grant: "*", dropGrantsTable: true},
		{name: "database failure", grant: "account/user", required: []types.AdminPermission{types.AdminAccountUser}, dropGrantsTable: true, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
			db, err := NewDB(ctx, DBConfig{Dialect: DialectSQLite, DSN: dsn, DisableAuditHook: true})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, db.Close()) })

			for _, statement := range []string{
				`CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE, deleted_at TIMESTAMP NULL)`,
				`CREATE TABLE user_roles (user_id INTEGER NOT NULL, role_code TEXT NOT NULL, deleted_at TIMESTAMP NULL)`,
				`CREATE TABLE roles (role_code TEXT PRIMARY KEY, display_name TEXT NOT NULL, description TEXT NOT NULL, is_admin BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, user_id INTEGER NOT NULL)`,
				`CREATE TABLE admin_role_permissions (role_code TEXT NOT NULL REFERENCES roles(role_code), permission_code TEXT NOT NULL)`,
			} {
				_, err = db.BunDB.ExecContext(ctx, statement)
				require.NoError(t, err)
			}

			var userDeletedAt any
			if tt.deletedUser {
				userDeletedAt = time.Now()
			}
			_, err = db.BunDB.ExecContext(ctx, `INSERT INTO users (id, username, deleted_at) VALUES (1, 'test-user', ?), (2, 'other-user', NULL)`, userDeletedAt)
			require.NoError(t, err)
			_, err = db.BunDB.ExecContext(ctx, `INSERT INTO roles (role_code, display_name, description, user_id, is_admin) VALUES ('admin_role', 'Admin Role', '', 1, TRUE), ('another_role', 'Other Role', '', 1, TRUE), ('second_role', 'Second Role', '', 1, TRUE), ('super_admin', 'Super Admin', '', 0, TRUE)`)
			require.NoError(t, err)
			if tt.nonAdminRole {
				_, err = db.BunDB.ExecContext(ctx, `UPDATE roles SET is_admin = FALSE WHERE role_code = 'admin_role'`)
				require.NoError(t, err)
			}

			roleUserID := tt.roleUserID
			if roleUserID == 0 {
				roleUserID = 1
			}
			var roleDeletedAt any
			if tt.deletedRole {
				roleDeletedAt = time.Now()
			}
			roleCode := tt.assignedRoleCode
			if roleCode == "" {
				roleCode = "admin_role"
			}
			if tt.ordinaryRole {
				roleCode = "member"
			}
			_, err = db.BunDB.ExecContext(ctx, `INSERT INTO user_roles (user_id, role_code, deleted_at) VALUES (?, ?, ?)`, roleUserID, roleCode, roleDeletedAt)
			require.NoError(t, err)

			grantRoleCode := tt.grantRoleCode
			if grantRoleCode == "" {
				grantRoleCode = "admin_role"
			}
			_, err = db.BunDB.ExecContext(ctx, `INSERT INTO admin_role_permissions (role_code, permission_code) VALUES (?, ?)`, grantRoleCode, tt.grant)
			require.NoError(t, err)
			if tt.extraAdminRole {
				_, err = db.BunDB.ExecContext(ctx, `INSERT INTO user_roles (user_id, role_code) VALUES (1, 'second_role')`)
				require.NoError(t, err)
				_, err = db.BunDB.ExecContext(ctx, `INSERT INTO admin_role_permissions (role_code, permission_code) VALUES ('second_role', 'account/user')`)
				require.NoError(t, err)
			}
			if tt.dropGrantsTable {
				_, err = db.BunDB.ExecContext(ctx, `DROP TABLE admin_role_permissions`)
				require.NoError(t, err)
			}

			allowed, err := NewAdminRoleStoreWithDB(db).HasPermission(ctx, "test-user", tt.required...)
			if tt.wantError {
				require.Error(t, err)
				require.False(t, allowed)
				return
			}
			require.NoError(t, err, "grant %q should be queryable", tt.grant)
			require.Equal(t, tt.wantAllowed, allowed)
		})
	}
}

// TestAdminPermissionCandidates verifies wildcard expansion, deduplication, and invalid route permissions.
func TestAdminPermissionCandidates(t *testing.T) {
	tests := []struct {
		name     string
		required []types.AdminPermission
		want     []string
		wantErr  bool
	}{
		{name: "empty list"},
		{name: "all only matches global wildcard", required: []types.AdminPermission{types.AdminPermissionAll}, want: []string{"*"}},
		{name: "duplicate all requirements", required: []types.AdminPermission{types.AdminPermissionAll, types.AdminPermissionAll}, want: []string{"*"}},
		{name: "all with concrete permission", required: []types.AdminPermission{types.AdminPermissionAll, types.AdminAccountUser}, want: []string{"account/user", "account/*", "*"}},
		{name: "concrete with all permission", required: []types.AdminPermission{types.AdminAccountUser, types.AdminPermissionAll}, want: []string{"account/user", "account/*", "*"}},
		{name: "all with any permission", required: []types.AdminPermission{types.AdminPermissionAny, types.AdminPermissionAll}, want: []string{"*"}},
		{name: "invalid alternative after all", required: []types.AdminPermission{types.AdminPermissionAll, ""}, wantErr: true},
		{name: "invalid alternative before all", required: []types.AdminPermission{"account/*", types.AdminPermissionAll}, wantErr: true},
		{name: "multi-source synchronization", required: []types.AdminPermission{types.AdminAssetSync}, want: []string{"asset/sync", "asset/*", "*"}},
		{name: "any role has no grant candidates", required: []types.AdminPermission{types.AdminPermissionAny}},
		{name: "any role with concrete permission", required: []types.AdminPermission{types.AdminPermissionAny, types.AdminAccountUser}, want: []string{"account/user", "account/*", "*"}},
		{name: "invalid alternative after any role", required: []types.AdminPermission{types.AdminPermissionAny, ""}, wantErr: true},
		{name: "shared module and duplicates", required: []types.AdminPermission{types.AdminAccountUser, types.AdminAccountOrganization, types.AdminAccountUser}, want: []string{"account/user", "account/*", "account/organization", "*"}},
		{name: "nested action", required: []types.AdminPermission{"account/user/read"}, want: []string{"account/user/read", "account/user/*", "account/*", "*"}},
		{name: "empty permission", required: []types.AdminPermission{""}, wantErr: true},
		{name: "module only", required: []types.AdminPermission{"account"}, wantErr: true},
		{name: "whitespace around global wildcard", required: []types.AdminPermission{" * "}, wantErr: true},
		{name: "module wildcard", required: []types.AdminPermission{"account/*"}, wantErr: true},
		{name: "embedded wildcard", required: []types.AdminPermission{"account/us*er"}, wantErr: true},
		{name: "wildcard module", required: []types.AdminPermission{"*/user"}, wantErr: true},
		{name: "empty segment", required: []types.AdminPermission{"account//user"}, wantErr: true},
		{name: "whitespace", required: []types.AdminPermission{"account/ user"}, wantErr: true},
		{name: "leading whitespace", required: []types.AdminPermission{" account/user"}, wantErr: true},
		{name: "trailing whitespace", required: []types.AdminPermission{"account/user "}, wantErr: true},
		{name: "invalid alternative", required: []types.AdminPermission{types.AdminAccountUser, "asset/*"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates, err := adminPermissionCandidates(tt.required...)
			if tt.wantErr {
				require.Error(t, err)
				require.Empty(t, candidates)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, candidates)
		})
	}
}

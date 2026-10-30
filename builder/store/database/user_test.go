package database_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

type userDeletionJobClient struct {
	inputs []database.RepositoryDeletionJobInput
}

func (c *userDeletionJobClient) InsertRepositoryDeletionJobTx(_ context.Context, _ *sql.Tx, input database.RepositoryDeletionJobInput) (int64, error) {
	c.inputs = append(c.inputs, input)
	return int64(len(c.inputs)), nil
}

type userRepositoryWriteHookContextKey struct{}
type userDeletionHookContextKey struct{}

type userDeletionNamespaceLockHook struct {
	path                   string
	writeNamespaceLocked   chan struct{}
	resumeWrite            chan struct{}
	deleteNamespaceAttempt chan struct{}
	writeOnce              sync.Once
	deleteOnce             sync.Once
}

func (h *userDeletionNamespaceLockHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if ctx.Value(userDeletionHookContextKey{}) != true {
		return ctx
	}
	query := strings.ToUpper(event.Query)
	if strings.Contains(query, `DELETE FROM "NAMESPACES"`) ||
		(strings.Contains(query, `FROM "NAMESPACES"`) && strings.Contains(query, "FOR UPDATE")) {
		h.deleteOnce.Do(func() { close(h.deleteNamespaceAttempt) })
	}
	return ctx
}

func (h *userDeletionNamespaceLockHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	if ctx.Value(userRepositoryWriteHookContextKey{}) != true || event.Err != nil {
		return
	}
	query := strings.ToUpper(event.Query)
	if strings.Contains(query, `FROM "NAMESPACES"`) && strings.Contains(query, "FOR KEY SHARE") && strings.Contains(event.Query, h.path) {
		h.writeOnce.Do(func() { close(h.writeNamespaceLocked) })
		<-h.resumeWrite
	}
}

// TestUserStore_Roles verifies persisted roles are the only source of role output.
func TestUserStore_Roles(t *testing.T) {
	for _, tc := range []struct {
		name string
		user database.User
		want []string
	}{
		{name: "no roles", want: []string{}},
		{name: "legacy roles are not a fallback", user: database.User{RoleMask: "admin,super_user"}, want: []string{}},
		{
			name: "ordinary and administrator roles are retained",
			user: database.User{RoleMask: "legacy", RoleAssignments: []database.UserRole{
				{RoleCode: "personal_user"}, {RoleCode: "asset_admin"},
				{RoleCode: "revoked", DeletedAt: time.Now()},
			}},
			want: []string{"personal_user", "asset_admin"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.user.Roles())
		})
	}
}

// TestUserAdminRolePredicates checks administrator identity separately from global grants.
func TestUserAdminRolePredicates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		user   database.User
		admin  bool
		global bool
	}{
		{name: "legacy names do not grant membership", user: database.User{RoleMask: "admin,super_user,super_admin"}},
		{name: "undefined role is ordinary", user: database.User{RoleAssignments: []database.UserRole{{RoleCode: "super_admin"}}}},
		{
			name: "defined ordinary role with global grant is not an administrator",
			user: database.User{RoleAssignments: []database.UserRole{{RoleCode: "member", Role: &database.Role{
				RoleCode: "member", Permissions: []database.AdminRolePermission{{PermissionCode: types.AdminPermissionAll}},
			}}}},
		},
		{
			name:  "defined role without grants is an administrator",
			user:  database.User{RoleAssignments: []database.UserRole{{RoleCode: "super_admin", Role: &database.Role{IsAdmin: true, RoleCode: "super_admin"}}}},
			admin: true,
		},
		{
			name: "module wildcard is not global",
			user: database.User{RoleAssignments: []database.UserRole{{RoleCode: "operator", Role: &database.Role{IsAdmin: true,
				RoleCode: "operator", Permissions: []database.AdminRolePermission{{PermissionCode: "account/*"}},
			}}}},
			admin: true,
		},
		{
			name: "legacy mask does not elevate a scoped administrator",
			user: database.User{RoleMask: "admin,super_user", RoleAssignments: []database.UserRole{{RoleCode: "asset_admin", Role: &database.Role{IsAdmin: true,
				RoleCode: "asset_admin", Permissions: []database.AdminRolePermission{{PermissionCode: "asset/*"}},
			}}}},
			admin: true,
		},
		{
			name: "custom name with global grant",
			user: database.User{RoleAssignments: []database.UserRole{{RoleCode: "owner", Role: &database.Role{IsAdmin: true,
				RoleCode: "owner", Permissions: []database.AdminRolePermission{{PermissionCode: types.AdminPermissionAll}},
			}}}},
			admin: true, global: true,
		},
		{
			name: "revoked global role grants nothing",
			user: database.User{RoleAssignments: []database.UserRole{{RoleCode: "owner", DeletedAt: time.Now(), Role: &database.Role{IsAdmin: true,
				RoleCode: "owner", Permissions: []database.AdminRolePermission{{PermissionCode: types.AdminPermissionAll}},
			}}}},
		},
		{
			name: "deleted user grants nothing",
			user: database.User{DeletedAt: time.Now(), RoleAssignments: []database.UserRole{{RoleCode: "owner", Role: &database.Role{IsAdmin: true,
				RoleCode: "owner", Permissions: []database.AdminRolePermission{{PermissionCode: types.AdminPermissionAll}},
			}}}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.admin, tc.user.HasAdminRole())
			require.Equal(t, tc.global, tc.user.HasSuperAdminPermission())
			require.Equal(t, tc.global, tc.user.CanAdmin())
		})
	}
}

// TestUserStore_GetAdminRecipients verifies notifications use active global grants without duplicate recipients.
func TestUserStore_GetAdminRecipients(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	userStore := database.NewUserStoreWithDB(db)
	roles := []database.Role{
		{RoleCode: "recipient_owner", DisplayName: "Recipient owner", IsAdmin: true},
		{RoleCode: "recipient_operator", DisplayName: "Recipient operator", IsAdmin: true},
		{RoleCode: "recipient_member", DisplayName: "Recipient member", IsAdmin: false},
	}
	_, err := db.Core.NewInsert().Model(&roles).Exec(ctx)
	require.NoError(t, err)
	permissions := []database.AdminRolePermission{
		{RoleCode: "recipient_owner", PermissionCode: types.AdminPermissionAll},
		{RoleCode: "recipient_operator", PermissionCode: "compute/*"},
		{RoleCode: "recipient_member", PermissionCode: types.AdminPermissionAll},
	}
	_, err = db.Core.NewInsert().Model(&permissions).Exec(ctx)
	require.NoError(t, err)

	testUsers := []database.User{
		{GitID: 99001, Username: "admin-recipient", UUID: "admin-recipient-uuid", Email: "admin@example.com"},
		{GitID: 99002, Username: "super-user-recipient", UUID: "super-user-recipient-uuid", Email: "super-user@example.com"},
		{GitID: 99003, Username: "ordinary-recipient", UUID: "ordinary-recipient-uuid", Email: "ordinary@example.com", RoleMask: "user"},
		{GitID: 99004, Username: "admin-without-email", UUID: "admin-without-email-uuid"},
		{GitID: 99005, Username: "legacy-admin", UUID: "legacy-admin-uuid", Email: "legacy@example.com", RoleMask: "admin,super_user"},
		{GitID: 99006, Username: "scoped-admin", UUID: "scoped-admin-uuid", Email: "scoped@example.com", RoleMask: "admin"},
		{GitID: 99007, Username: "revoked-admin", UUID: "revoked-admin-uuid", Email: "revoked@example.com", RoleMask: "admin"},
		{GitID: 99008, Username: "deleted-admin", UUID: "deleted-admin-uuid", Email: "deleted@example.com", DeletedAt: time.Now()},
		{GitID: 99009, Username: "admin-without-uuid", Email: "no-uuid@example.com"},
	}
	for i := range testUsers {
		err := userStore.Create(ctx, &testUsers[i], &database.Namespace{
			Path: testUsers[i].Username,
		})
		require.NoError(t, err)
	}
	assignments := []database.UserRole{
		{UserID: testUsers[0].ID, RoleCode: "recipient_owner"},
		{UserID: testUsers[1].ID, RoleCode: "recipient_owner"},
		{UserID: testUsers[1].ID, RoleCode: "super_admin"},
		{UserID: testUsers[2].ID, RoleCode: "recipient_member"},
		{UserID: testUsers[3].ID, RoleCode: "recipient_owner"},
		{UserID: testUsers[5].ID, RoleCode: "recipient_operator"},
		{UserID: testUsers[6].ID, RoleCode: "recipient_owner", DeletedAt: time.Now()},
		{UserID: testUsers[7].ID, RoleCode: "recipient_owner"},
		{UserID: testUsers[8].ID, RoleCode: "recipient_owner"},
	}
	_, err = db.Core.NewInsert().Model(&assignments).Exec(ctx)
	require.NoError(t, err)

	adminUUIDs, err := userStore.GetAdminUserUUIDs(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		"admin-recipient-uuid",
		"super-user-recipient-uuid",
		"admin-without-email-uuid",
	}, adminUUIDs)

	adminEmails, err := userStore.GetAdminEmails(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		"admin@example.com",
		"super-user@example.com",
		"no-uuid@example.com",
	}, adminEmails)
}

func TestUserStore_SetRoles(t *testing.T) {
	type fields struct {
		RoleMask string
	}
	type args struct {
		roles []string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
		{
			name: "test no role",
			fields: fields{
				RoleMask: "",
			},
			args: args{
				roles: []string{""},
			},
		},
		{
			name: "test one role",
			fields: fields{
				RoleMask: "admin",
			},
			args: args{
				roles: []string{"admin"},
			},
		},
		{
			name: "test two roles",
			fields: fields{
				RoleMask: "admin,super_user",
			},
			args: args{
				roles: []string{"admin", "super_user"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &database.User{}
			u.SetRoles(tt.args.roles)
			if u.RoleMask != tt.fields.RoleMask {
				t.Errorf("User.SetRoles() = %v, want %v", u.RoleMask, tt.fields.RoleMask)
			}
		})
	}
}

func TestUserStore_IndexWithSearch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userStore := database.NewUserStoreWithDB(db)
	err := userStore.Create(ctx, &database.User{
		GitID:    3321,
		Username: "u-foo",
		UUID:     "1",
		Labels:   []string{"vip", "basic"},
	}, &database.Namespace{Path: "1"})
	require.Nil(t, err)

	err = userStore.Create(ctx, &database.User{
		GitID:    3322,
		Username: "u-bar",
		Email:    "efoo@z.com",
		UUID:     "2",
	}, &database.Namespace{Path: "2"})
	require.Nil(t, err)

	err = userStore.Create(ctx, &database.User{
		GitID:    3323,
		Username: "u-barz",
		Email:    "ebar@z.com",
		UUID:     "3",
	}, &database.Namespace{Path: "3"})
	require.Nil(t, err)

	cases := []struct {
		per      int
		page     int
		labels   []string
		total    int
		expected []int64
	}{
		{10, 1, []string{}, 2, []int64{3321, 3322}},
		{1, 1, []string{}, 2, []int64{3321}},
		{1, 2, []string{}, 2, []int64{3322}},
		{10, 1, []string{"vip"}, 1, []int64{3321}},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("page %d, per %d", c.page, c.per), func(t *testing.T) {

			req := types.UserListReq{
				Search: "foo",
				Labels: c.labels,
				Per:    c.per,
				Page:   c.page,
			}
			users, count, err := userStore.IndexWithSearch(ctx, req)
			require.Nil(t, err)
			require.Equal(t, c.total, count)

			gids := []int64{}
			for _, u := range users {
				gids = append(gids, u.GitID)
			}
			require.Equal(t, c.expected, gids)
		})
	}

}

func TestUserStore_IndexWithSearchExactMatch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userStore := database.NewUserStoreWithDB(db)
	err := userStore.Create(ctx, &database.User{
		GitID:    3321,
		Username: "u-foo",
		UUID:     "1",
	}, &database.Namespace{Path: "1"})
	require.Nil(t, err)

	err = userStore.Create(ctx, &database.User{
		GitID:    3322,
		Username: "u-bar",
		Email:    "efoo@z.com",
		UUID:     "2",
	}, &database.Namespace{Path: "2"})
	require.Nil(t, err)

	err = userStore.Create(ctx, &database.User{
		GitID:    3323,
		Username: "u-barz",
		Email:    "ebar@z.com",
		UUID:     "3",
	}, &database.Namespace{Path: "3"})
	require.Nil(t, err)

	_, count, err := userStore.IndexWithSearch(ctx, types.UserListReq{
		VisitorName: "u-foo",
		Per:         10,
		Page:        1,
		ExactMatch:  true,
	})
	require.Nil(t, err)
	require.Equal(t, 3, count)

	_, count, err = userStore.IndexWithSearch(ctx, types.UserListReq{
		Search:     "u-foo",
		Per:        10,
		Page:       1,
		ExactMatch: true,
	})
	require.Nil(t, err)
	require.Equal(t, 1, count)
}

func TestUserStore_CreateUser(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	us := database.NewUserStoreWithDB(db)
	uuid := uuid.New().String()
	err := us.Create(ctx, &database.User{
		GitID:    3321,
		Username: "u-foo",
		UUID:     uuid,
	}, &database.Namespace{Path: "u-foo"})
	require.Nil(t, err)

	user, err := us.FindByUsername(ctx, "u-foo")
	require.Nil(t, err)
	require.Equal(t, 3321, int(user.GitID))
	require.Equal(t, "u-foo", user.Username)

	userByUuid, err := us.FindByUUID(ctx, uuid)
	require.Empty(t, err)
	require.Equal(t, uuid, userByUuid.UUID)

	yes, err := us.IsExist(ctx, "u-foo")
	require.Nil(t, err)
	require.True(t, yes)

	yes, err = us.IsExistByUUID(ctx, uuid)
	require.Nil(t, err)
	require.True(t, yes)
}

func TestUserStore_ChangeUserName(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	us := database.NewUserStoreWithDB(db)
	err := us.Create(ctx, &database.User{
		GitID:    3321,
		Username: "u-foo",
	}, &database.Namespace{Path: "u-foo"})
	require.Nil(t, err)

	err = us.ChangeUserName(ctx, "u-foo", "u-bar")
	require.Nil(t, err)

	user, err := us.FindByUsername(ctx, "u-bar")
	require.Nil(t, err)
	require.Equal(t, "u-bar", user.Username)
}

func TestUserStore_FindByAccessToken(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	us := database.NewUserStoreWithDB(db)
	user1 := &database.User{
		GitID:    3321,
		Username: "u-foo",
	}
	err := us.Create(ctx, user1, &database.Namespace{Path: "u-foo"})
	require.Nil(t, err)

	//create access token for user u-foo
	at := &database.AccessToken{
		GitID:       1,
		Name:        "test_token",
		Token:       "token_" + uuid.NewString(),
		UserID:      user1.ID,
		Application: "git",
		Permission:  "",
		IsActive:    true,
		ExpiredAt:   time.Now().Add(time.Hour * 24),
	}
	_, err = db.Core.NewInsert().Model(at).Exec(ctx)
	require.Nil(t, err)

	user, err := us.FindByGitAccessToken(ctx, at.Token)
	require.NoError(t, err)
	require.Equal(t, "u-foo", user.Username)
}

func TestUserStore_CountUsers(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := db.Core.NewDelete().Model(&database.User{}).Where("1=1").Exec(ctx)
	require.Nil(t, err)

	us := database.NewUserStoreWithDB(db)
	err = us.Create(ctx, &database.User{
		GitID:    3321,
		Username: "u-foo",
		UUID:     "1",
	}, &database.Namespace{Path: "u-foo"})
	require.Nil(t, err)
	err = us.Create(ctx, &database.User{
		GitID:    3321,
		Username: "u-foo-2",
		UUID:     "2",
	}, &database.Namespace{Path: "u-foo-2"})
	require.Nil(t, err)
	// A mirrored user (imported via multi-sync) must be excluded from the count.
	err = us.Create(ctx, &database.User{
		GitID:    3321,
		Username: "u-mirrored",
		UUID:     "3",
	}, &database.Namespace{Path: "u-mirrored", Mirrored: true})
	require.Nil(t, err)
	// An organization namespace (namespace_type=organization) must not be
	// counted as a user, even though mirrored=false.
	_, err = db.Core.NewInsert().Model(&database.Namespace{
		Path:          "u-foo-org",
		UserID:        1,
		NamespaceType: database.OrgNamespace,
		Mirrored:      false,
	}).Exec(ctx)
	require.Nil(t, err)

	count, err := us.CountUsers(ctx)
	require.Nil(t, err)
	require.Equal(t, 2, count)
}

func TestUserStore_Update(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	us := database.NewUserStoreWithDB(db)
	err := us.Create(ctx, &database.User{
		GitID:    3321,
		UUID:     "123456",
		Username: "u-foo",
	}, &database.Namespace{Path: "u-foo"})
	require.Nil(t, err)

	user, err := us.FindByUsername(ctx, "u-foo")
	require.Nil(t, err)
	require.Equal(t, "u-foo", user.Username)

	ns := database.NewNamespaceStoreWithDB(db)
	namepsace, err := ns.FindByPath(ctx, "u-foo")
	require.NoError(t, err)
	require.Equal(t, namepsace.UserID, user.ID)

	changedUser := user
	changedUser.Username = "u-foo-changed"
	changedUser.Email = "email changed"
	changedUser.Phone = "phone changed"
	err = us.Update(ctx, &changedUser, "")
	require.NoError(t, err)

	user2, err := us.FindByUUID(ctx, user.UUID)
	require.NoError(t, err)
	require.Equal(t, "u-foo-changed", user2.Username)
	require.Equal(t, "email changed", user2.Email)
	require.Equal(t, "phone changed", user2.Phone)
	//namespace path not changed
	namepsace, err = ns.FindByPath(ctx, "u-foo")
	require.NoError(t, err)
	require.Equal(t, namepsace.UserID, user2.ID)

	err = us.Update(ctx, &changedUser, "u-foo")
	require.NoError(t, err)

	user3, err := us.FindByUUID(ctx, user.UUID)
	require.NoError(t, err)
	require.Equal(t, "u-foo-changed", user3.Username)
	require.Equal(t, "email changed", user3.Email)
	require.Equal(t, "phone changed", user3.Phone)
	//namespace path changed
	namepsace, err = ns.FindByPath(ctx, "u-foo-changed")
	require.NoError(t, err)
	require.Equal(t, namepsace.UserID, user3.ID)
}

func TestUserStore_UpdateLabels(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	us := database.NewUserStoreWithDB(db)
	uuid := uuid.New().String()

	err := us.Create(ctx, &database.User{
		GitID:    10001,
		UUID:     uuid,
		Username: "label-user",
	}, &database.Namespace{Path: "label-user"})
	require.NoError(t, err)

	newLabels := []string{"vip", "advanced"}
	err = us.UpdateLabels(ctx, uuid, newLabels)
	require.NoError(t, err)

	labelsUser, err := us.FindByUUID(ctx, uuid)
	require.NoError(t, err)
	require.ElementsMatch(t, newLabels, labelsUser.Labels)

	err = us.UpdateLabels(ctx, uuid, []string{})
	require.NoError(t, err)

	labelsUserEmpty, err := us.FindByUUID(ctx, uuid)
	require.NoError(t, err)
	require.Empty(t, labelsUserEmpty.Labels)

	uuids := []string{uuid, "not_uuid"}
	users, err := us.FindByUUIDs(ctx, uuids)
	require.Nil(t, err)
	require.Equal(t, 1, len(users))
}

func TestGetUserTags(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	uts := database.NewUserTagStoreWithDB(db)
	us := database.NewUserStoreWithDB(db)
	ts := database.NewTagStoreWithDB(db)

	tags := []*database.Tag{
		{
			Name:     "tag-1",
			Category: "category-1",
			Group:    "group-1",
		},
		{
			Name:     "tag-2",
			Category: "category-2",
			Group:    "group-2",
		},
		{
			Name:     "tag-3",
			Category: "category-3",
			Group:    "group-3",
		},
	}

	err := ts.SaveTags(ctx, tags)
	require.Nil(t, err)

	tags, err = ts.AllTags(ctx, nil)
	require.Nil(t, err)
	user := &database.User{
		GitID:    10001,
		UUID:     "1",
		Username: "u-foo",
	}

	err = us.Create(
		ctx,
		user,
		&database.Namespace{Path: "u-foo"},
	)
	require.Nil(t, err)

	dbUser, err := us.FindByUUID(ctx, user.UUID)
	require.Nil(t, err)

	tagIDs := make([]int64, 0, len(tags))
	for _, tag := range tags {
		tagIDs = append(tagIDs, tag.ID)
	}

	err = uts.ResetUserTags(ctx, dbUser.ID, tagIDs)
	require.Nil(t, err)

	tags, err = uts.GetUserTags(ctx, dbUser.ID)
	require.Nil(t, err)
	require.Equal(t, len(tags), len(tagIDs))
	for _, tag := range tags {
		require.Contains(t, tagIDs, tag.ID)
	}
}

// test update phone
func TestUserStore_UpdatePhone(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	us := database.NewUserStoreWithDB(db)
	err := us.Create(ctx, &database.User{
		GitID:     10001,
		UUID:      "1",
		Username:  "u-foo",
		Phone:     "12345678901",
		PhoneArea: "",
	}, &database.Namespace{Path: "u-foo"})
	require.NoError(t, err)

	user, err := us.FindByUUID(ctx, "1")
	require.NoError(t, err)
	require.Equal(t, "12345678901", user.Phone)
	require.Equal(t, "", user.PhoneArea)

	err = us.UpdatePhone(ctx, user.ID, "12345678902", "+86")
	require.NoError(t, err)

	user, err = us.FindByUUID(ctx, "1")
	require.NoError(t, err)
	require.Equal(t, "12345678902", user.Phone)
	require.Equal(t, "+86", user.PhoneArea)
}

// TestUserStore_IndexWithCursor1 verifies role relationships survive paginated cursor exports.
func TestUserStore_IndexWithCursor1(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	us := database.NewUserStoreWithDB(db)

	err := us.Create(ctx, &database.User{
		GitID:    10001,
		ID:       1,
		UUID:     "1",
		Username: "u-foo",
	}, &database.Namespace{Path: "u-foo"})
	require.NoError(t, err)

	err = us.Create(ctx, &database.User{
		GitID:    10002,
		ID:       2,
		UUID:     "2",
		Username: "u-foo-2",
	}, &database.Namespace{Path: "u-foo-2"})
	require.NoError(t, err)

	err = us.Create(ctx, &database.User{
		GitID:    10003,
		ID:       3,
		UUID:     "3",
		Username: "u-foo-3",
	}, &database.Namespace{Path: "u-foo-3"})
	require.NoError(t, err)

	req := types.UserIndexReq{
		Search: "u-foo",
		Per:    1,
	}
	role := database.Role{IsAdmin: true, RoleCode: "cursor_owner", DisplayName: "Cursor owner"}
	_, err = db.Core.NewInsert().Model(&role).Exec(ctx)
	require.NoError(t, err)
	grant := database.AdminRolePermission{RoleCode: role.RoleCode, PermissionCode: types.AdminPermissionAll}
	_, err = db.Core.NewInsert().Model(&grant).Exec(ctx)
	require.NoError(t, err)
	assignments := []database.UserRole{
		{UserID: 1, RoleCode: "personal_user"}, {UserID: 1, RoleCode: role.RoleCode},
		{UserID: 2, RoleCode: role.RoleCode, DeletedAt: time.Now()},
		{UserID: 3, RoleCode: "personal_user"},
	}
	_, err = db.Core.NewInsert().Model(&assignments).Exec(ctx)
	require.NoError(t, err)

	ch, err := us.IndexWithCursor(ctx, req, types.UserWithAdminPermissions)
	require.NoError(t, err)

	total := 0
	for wrapper := range ch {
		require.NoError(t, wrapper.Err)
		total += len(wrapper.Users)
		for _, user := range wrapper.Users {
			switch user.ID {
			case 1:
				require.ElementsMatch(t, []string{"personal_user", "cursor_owner"}, user.Roles())
				require.True(t, user.HasAdminRole())
				require.True(t, user.HasSuperAdminPermission())
			case 2:
				require.Empty(t, user.Roles())
				require.False(t, user.HasAdminRole())
				require.False(t, user.HasSuperAdminPermission())
			case 3:
				require.Equal(t, []string{"personal_user"}, user.Roles())
				require.False(t, user.HasAdminRole())
				require.False(t, user.HasSuperAdminPermission())
			}
		}
	}

	require.Equal(t, 3, total)
}

func TestUserStore_DeleteUserAndRelationsLastOrgAdmin(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	setOrgStoreTestDB(t, db)
	ctx := context.Background()

	us := database.NewUserStoreWithDB(db)
	os := database.NewOrgStore(false, nil)
	user := createDeleteUserTestFixtures(t, ctx, db, us, os, "delete-last-admin")

	err := us.DeleteUserAndRelations(ctx, *user, types.CloseAccountReq{})
	require.ErrorIs(t, err, errorx.ErrLastOrgAdmin)

	_, err = us.FindByUsername(ctx, user.Username)
	require.NoError(t, err)

	memberCount, err := db.Core.NewSelect().
		Model((*database.Member)(nil)).
		Where("member.user_id = ?", user.ID).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, memberCount)
}

func TestUserStore_DeleteUserAndRelationsDeletesRepositoriesOnce(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()

	jobClient := &userDeletionJobClient{}
	us := database.NewUserStoreWithDBAndDeletionJobClient(db, jobClient)
	user := &database.User{GitID: time.Now().UnixNano(), UUID: uuid.NewString(), Username: "delete-user-repositories"}
	require.NoError(t, us.Create(ctx, user, &database.Namespace{Path: user.Username}))

	repository := &database.Repository{
		UserID: user.ID, Name: "model", Path: user.Username + "/model",
		GitPath: "models_" + user.Username + "/model", RepositoryType: types.ModelRepo,
	}
	_, err := db.Core.NewInsert().Model(repository).Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&database.Model{RepositoryID: repository.ID}).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, us.DeleteUserAndRelations(ctx, *user, types.CloseAccountReq{}))

	repositoryCount, err := db.Core.NewSelect().Model((*database.Repository)(nil)).WhereAllWithDeleted().
		Where("id = ?", repository.ID).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, repositoryCount)
	var deletedRepository database.Repository
	require.NoError(t, db.Core.NewSelect().Model(&deletedRepository).WhereAllWithDeleted().
		Where("id = ?", repository.ID).Scan(ctx))
	require.False(t, deletedRepository.DeletedAt.IsZero())
	pendingCount, err := db.Core.NewSelect().Model((*database.PendingDeletion)(nil)).
		Where("table_name = ?", database.PendingDeletionTableNameRepository).
		Where("value = ?", repository.GitalyPath()).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, pendingCount)
	require.Len(t, jobClient.inputs, 1)
	require.Equal(t, user.UUID, jobClient.inputs[0].OwnerUUID)
	require.Equal(t, database.UserNamespace, jobClient.inputs[0].OwnerType)
}

func TestUserStore_DeleteUserAndRelationsResolvesSoftDeletedUserOwner(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()

	jobClient := &userDeletionJobClient{}
	us := database.NewUserStoreWithDBAndDeletionJobClient(db, jobClient)
	user := &database.User{GitID: time.Now().UnixNano(), UUID: uuid.NewString(), Username: "delete-soft-user-repositories"}
	require.NoError(t, us.Create(ctx, user, &database.Namespace{Path: user.Username}))
	repository := &database.Repository{
		UserID: user.ID, Name: "model", Path: user.Username + "/model",
		GitPath: "models_" + user.Username + "/model", RepositoryType: types.ModelRepo,
	}
	_, err := db.Core.NewInsert().Model(repository).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, us.SoftDeleteUserAndRelations(ctx, *user, types.CloseAccountReq{}))
	softDeletedUser, err := us.FindByUsernameWithDeleted(ctx, user.Username)
	require.NoError(t, err)
	require.NoError(t, us.DeleteUserAndRelations(ctx, softDeletedUser, types.CloseAccountReq{}))

	require.Len(t, jobClient.inputs, 1)
	require.Equal(t, user.UUID, jobClient.inputs[0].OwnerUUID)
	require.Equal(t, database.UserNamespace, jobClient.inputs[0].OwnerType)
}

func TestUserStore_DeleteUserAndRelationsSerializesWithRepositoryCreation(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()

	jobClient := &userDeletionJobClient{}
	userStore := database.NewUserStoreWithDBAndDeletionJobClient(db, jobClient)
	repoStore := database.NewRepoStoreWithDB(db)
	user := &database.User{GitID: time.Now().UnixNano(), UUID: uuid.NewString(), Username: "delete-user-concurrent-create"}
	require.NoError(t, userStore.Create(ctx, user, &database.Namespace{Path: user.Username}))

	hook := &userDeletionNamespaceLockHook{
		path: user.Username, writeNamespaceLocked: make(chan struct{}), resumeWrite: make(chan struct{}),
		deleteNamespaceAttempt: make(chan struct{}),
	}
	db.BunDB.AddQueryHook(hook)

	createResult := make(chan error, 1)
	go func() {
		_, err := repoStore.CreateRepo(context.WithValue(ctx, userRepositoryWriteHookContextKey{}, true), database.Repository{
			UserID: user.ID, Name: "new-repository", Path: user.Username + "/new-repository",
			GitPath: "models_" + user.Username + "/new-repository", RepositoryType: types.ModelRepo,
		})
		createResult <- err
	}()
	select {
	case <-hook.writeNamespaceLocked:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "repository creation did not acquire the user namespace lock")
	}

	deleteResult := make(chan error, 1)
	go func() {
		deleteResult <- userStore.DeleteUserAndRelations(
			context.WithValue(ctx, userDeletionHookContextKey{}, true), *user, types.CloseAccountReq{},
		)
	}()
	select {
	case <-hook.deleteNamespaceAttempt:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "user deletion did not attempt to lock or delete the user namespace")
	}
	close(hook.resumeWrite)

	require.NoError(t, <-createResult)
	require.NoError(t, <-deleteResult)
	active, err := db.Core.NewSelect().Model((*database.Repository)(nil)).
		Where("path = ?", user.Username+"/new-repository").Exists(ctx)
	require.NoError(t, err)
	require.False(t, active, "repository committed before user deletion must be included in deletion")
	require.Len(t, jobClient.inputs, 1)

	_, err = repoStore.CreateRepo(ctx, database.Repository{
		UserID: user.ID, Name: "late-repository", Path: user.Username + "/late-repository",
		GitPath: "models_" + user.Username + "/late-repository", RepositoryType: types.ModelRepo,
	})
	require.ErrorContains(t, err, fmt.Sprintf("repository namespace %q is deleted", user.Username))
}

func TestUserStore_DeleteUserAndRelationsSerializesWithRepositoryTransfer(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()

	jobClient := &userDeletionJobClient{}
	userStore := database.NewUserStoreWithDBAndDeletionJobClient(db, jobClient)
	repoStore := database.NewRepoStoreWithDB(db)
	source := &database.User{GitID: time.Now().UnixNano(), UUID: uuid.NewString(), Username: "a-delete-user-transfer"}
	target := &database.User{GitID: time.Now().UnixNano() + 1, UUID: uuid.NewString(), Username: "b-target-user-transfer"}
	require.NoError(t, userStore.Create(ctx, source, &database.Namespace{Path: source.Username}))
	require.NoError(t, userStore.Create(ctx, target, &database.Namespace{Path: target.Username}))
	repository, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID: source.ID, Name: "transferred-repository", Path: source.Username + "/transferred-repository",
		GitPath: "models_" + source.Username + "/transferred-repository", RepositoryType: types.ModelRepo,
	})
	require.NoError(t, err)

	hook := &userDeletionNamespaceLockHook{
		path: source.Username, writeNamespaceLocked: make(chan struct{}), resumeWrite: make(chan struct{}),
		deleteNamespaceAttempt: make(chan struct{}),
	}
	db.BunDB.AddQueryHook(hook)

	transferResult := make(chan error, 1)
	go func() {
		moved := *repository
		moved.UserID = target.ID
		moved.Path = target.Username + "/transferred-repository"
		moved.GitPath = "models_" + target.Username + "/transferred-repository"
		_, updateErr := repoStore.UpdateRepo(context.WithValue(ctx, userRepositoryWriteHookContextKey{}, true), moved)
		transferResult <- updateErr
	}()
	select {
	case <-hook.writeNamespaceLocked:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "repository transfer did not acquire the source namespace lock")
	}

	deleteResult := make(chan error, 1)
	go func() {
		deleteResult <- userStore.DeleteUserAndRelations(
			context.WithValue(ctx, userDeletionHookContextKey{}, true), *source, types.CloseAccountReq{},
		)
	}()
	select {
	case <-hook.deleteNamespaceAttempt:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "user deletion did not attempt to lock or delete the source namespace")
	}
	close(hook.resumeWrite)

	require.NoError(t, <-transferResult)
	require.NoError(t, <-deleteResult)
	var stored database.Repository
	require.NoError(t, db.Core.NewSelect().Model(&stored).Where("id = ?", repository.ID).Scan(ctx))
	require.Equal(t, target.ID, stored.UserID)
	require.Equal(t, target.Username+"/transferred-repository", stored.Path)
	require.Empty(t, jobClient.inputs)
}

func TestUserStore_DeleteUserAndRelationsRetainsRepositories(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()

	us := database.NewUserStoreWithDB(db)
	user := &database.User{GitID: time.Now().UnixNano(), UUID: uuid.NewString(), Username: "retain-user-repositories"}
	require.NoError(t, us.Create(ctx, user, &database.Namespace{Path: user.Username}))
	repository := &database.Repository{
		UserID: user.ID, Name: "model", Path: user.Username + "/model",
		GitPath: "models_" + user.Username + "/model", RepositoryType: types.ModelRepo,
	}
	_, err := db.Core.NewInsert().Model(repository).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, us.DeleteUserAndRelations(ctx, *user, types.CloseAccountReq{Repository: true}))

	repositoryCount, err := db.Core.NewSelect().Model((*database.Repository)(nil)).WhereAllWithDeleted().
		Where("id = ?", repository.ID).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, repositoryCount)
	pendingCount, err := db.Core.NewSelect().Model((*database.PendingDeletion)(nil)).
		Where("table_name = ?", database.PendingDeletionTableNameRepository).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, pendingCount)
}

func TestUserStore_SoftDeleteUserAndRelationsLastOrgAdmin(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	setOrgStoreTestDB(t, db)
	ctx := context.Background()

	us := database.NewUserStoreWithDB(db)
	os := database.NewOrgStore(false, nil)
	user := createDeleteUserTestFixtures(t, ctx, db, us, os, "soft-delete-last-admin")

	err := us.SoftDeleteUserAndRelations(ctx, *user, types.CloseAccountReq{})
	require.ErrorIs(t, err, errorx.ErrLastOrgAdmin)

	_, err = us.FindByUsername(ctx, user.Username)
	require.NoError(t, err)

	memberCount, err := db.Core.NewSelect().
		Model((*database.Member)(nil)).
		Where("member.user_id = ?", user.ID).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, memberCount)
}

// TestUserStore_UserDeletionAdminProtection rechecks current assignments before hard or soft deletion touches related data.
func TestUserStore_UserDeletionAdminProtection(t *testing.T) {
	for _, soft := range []bool{false, true} {
		for _, tc := range []struct {
			name, grant string
			isAdmin     bool
			revoked     bool
			another     bool
			multiple    bool
		}{
			{name: "last global administrator", grant: "*", isAdmin: true},
			{name: "last user with multiple management roles", grant: "*", isAdmin: true, multiple: true},
			{name: "last account administrator", grant: "account/admin", isAdmin: true},
			{name: "last account wildcard", grant: "account/*", isAdmin: true},
			{name: "administrator with another manager", grant: "account/admin", isAdmin: true, another: true},
			{name: "other module administrator", grant: "asset/model", isAdmin: true},
			{name: "administrator without permissions", isAdmin: true},
			{name: "revoked administrator", grant: "*", isAdmin: true, revoked: true},
			{name: "ordinary role with wildcard", grant: "*"},
			{name: "ordinary user"},
		} {
			t.Run(fmt.Sprintf("soft=%v/%s", soft, tc.name), func(t *testing.T) {
				db := tests.InitTestDB()
				defer db.Close()
				ctx := context.Background()
				store := database.NewUserStoreWithDB(db)
				target := &database.User{GitID: time.Now().UnixNano(), UUID: uuid.NewString(), Username: "admin-delete-target"}
				require.NoError(t, store.Create(ctx, target, &database.Namespace{Path: target.Username}))
				role := &database.Role{RoleCode: "delete_target", DisplayName: "Target", IsAdmin: tc.isAdmin}
				_, err := db.Core.NewInsert().Model(role).Exec(ctx)
				require.NoError(t, err)
				if tc.grant != "" {
					_, err = db.Core.NewInsert().Model(&database.AdminRolePermission{RoleCode: role.RoleCode, PermissionCode: types.AdminPermission(tc.grant)}).Exec(ctx)
					require.NoError(t, err)
				}
				assignment := &database.UserRole{UserID: target.ID, RoleCode: role.RoleCode}
				if tc.revoked {
					assignment.DeletedAt = time.Now()
				}
				_, err = db.Core.NewInsert().Model(assignment).Exec(ctx)
				require.NoError(t, err)
				if tc.multiple {
					_, err = db.Core.NewInsert().Model(&database.Role{RoleCode: "delete_second", DisplayName: "Second", IsAdmin: true}).Exec(ctx)
					require.NoError(t, err)
					_, err = db.Core.NewInsert().Model(&database.AdminRolePermission{RoleCode: "delete_second", PermissionCode: types.AdminPermissionAll}).Exec(ctx)
					require.NoError(t, err)
					_, err = db.Core.NewInsert().Model(&database.UserRole{UserID: target.ID, RoleCode: "delete_second"}).Exec(ctx)
					require.NoError(t, err)
				}
				if tc.another {
					other := &database.User{GitID: time.Now().UnixNano(), UUID: uuid.NewString(), Username: "admin-delete-other"}
					require.NoError(t, store.Create(ctx, other, &database.Namespace{Path: other.Username}))
					_, err = db.Core.NewInsert().Model(&database.UserRole{UserID: other.ID, RoleCode: role.RoleCode}).Exec(ctx)
					require.NoError(t, err)
				}
				if soft {
					err = store.SoftDeleteUserAndRelations(ctx, *target, types.CloseAccountReq{})
				} else {
					err = store.DeleteUserAndRelations(ctx, *target, types.CloseAccountReq{})
				}
				protected := tc.isAdmin && !tc.revoked
				if protected {
					management := tc.grant == "*" || config.SupportsMultipleAdminRoles() && (tc.grant == "account/admin" || tc.grant == "account/*")
					if management && !tc.another {
						require.ErrorIs(t, err, errorx.ErrLastAccountAdmin)
					} else {
						require.ErrorIs(t, err, errorx.ErrAdminUserCannotBeDeleted)
					}
					stored, err := store.FindByUsername(ctx, target.Username)
					require.NoError(t, err)
					require.Empty(t, stored.RetainData, "a rejection must not update the account")
					var namespace database.Namespace
					require.NoError(t, db.Core.NewSelect().Model(&namespace).Where("user_id = ?", target.ID).Scan(ctx))
				} else {
					require.NoError(t, err)
					_, err = store.FindByUsername(ctx, target.Username)
					require.Error(t, err)
				}
			})
		}
	}
}

// adminDeletionOperationKey distinguishes the competing transactions in the shared-lock test.
type adminDeletionOperationKey struct{}

// adminDeletionLockHook coordinates overlapping PostgreSQL deletion and revocation transactions.
type adminDeletionLockHook struct {
	first              string
	locked, attempted  chan struct{}
	release            <-chan struct{}
	lockedOnce, second sync.Once
}

// BeforeQuery records when the competing transaction tries to acquire the administrator lock.
func (h *adminDeletionLockHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if strings.HasPrefix(event.Query, "SELECT pg_advisory_xact_lock(") && ctx.Value(adminDeletionOperationKey{}) != h.first {
		h.second.Do(func() { close(h.attempted) })
	}
	return ctx
}

// AfterQuery keeps the first lock held until the competing transaction attempts the same lock.
func (h *adminDeletionLockHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	if event.Err == nil && strings.HasPrefix(event.Query, "SELECT pg_advisory_xact_lock(") && ctx.Value(adminDeletionOperationKey{}) == h.first {
		h.lockedOnce.Do(func() { close(h.locked) })
		select {
		case <-h.release:
		case <-ctx.Done():
		}
	}
}

// TestUserStore_UserDeletionSerializesWithRoleRevocation uses real PostgreSQL transactions in both execution orders.
func TestUserStore_UserDeletionSerializesWithRoleRevocation(t *testing.T) {
	for _, first := range []string{"delete", "revoke"} {
		t.Run(first, func(t *testing.T) {
			db := tests.InitTransactionTestDB()
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			store := database.NewUserStoreWithDB(db)
			users := []*database.User{
				{GitID: time.Now().UnixNano(), UUID: uuid.NewString(), Username: "delete-first"},
				{GitID: time.Now().UnixNano() + 1, UUID: uuid.NewString(), Username: "revoke-second"},
			}
			role := &database.Role{RoleCode: "concurrent_owner", DisplayName: "Owner", IsAdmin: true}
			_, err := db.Core.NewInsert().Model(role).Exec(ctx)
			require.NoError(t, err)
			_, err = db.Core.NewInsert().Model(&database.AdminRolePermission{RoleCode: role.RoleCode, PermissionCode: types.AdminPermissionAll}).Exec(ctx)
			require.NoError(t, err)
			for _, user := range users {
				require.NoError(t, store.Create(ctx, user, &database.Namespace{Path: user.Username}))
				_, err = db.Core.NewInsert().Model(&database.UserRole{UserID: user.ID, RoleCode: role.RoleCode}).Exec(ctx)
				require.NoError(t, err)
			}
			release := make(chan struct{})
			var releaseOnce sync.Once
			releaseLock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(releaseLock)
			hook := &adminDeletionLockHook{first: first, locked: make(chan struct{}), attempted: make(chan struct{}), release: release}
			db.BunDB.AddQueryHook(hook)
			deleteResult, revokeResult := make(chan error, 1), make(chan error, 1)
			for _, operation := range []string{"delete", "revoke"} {
				go func() {
					if operation != first {
						select {
						case <-hook.locked:
						case <-ctx.Done():
							if operation == "delete" {
								deleteResult <- ctx.Err()
							} else {
								revokeResult <- ctx.Err()
							}
							return
						}
					}
					operationCtx := context.WithValue(ctx, adminDeletionOperationKey{}, operation)
					if operation == "delete" {
						deleteResult <- store.DeleteUserAndRelations(operationCtx, *users[0], types.CloseAccountReq{})
					} else {
						revokeResult <- database.NewUserRoleStoreWithDB(db).RemoveAdminRole(operationCtx, types.AdminUserRoleRequest{
							UserID: users[1].ID, RoleCode: role.RoleCode, CurrentUser: users[0].Username, GlobalOnly: !config.SupportsMultipleAdminRoles(),
						})
					}
				}()
			}
			select {
			case <-hook.attempted:
				releaseLock()
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			select {
			case err = <-deleteResult:
				if first == "delete" {
					require.ErrorIs(t, err, errorx.ErrAdminUserCannotBeDeleted)
				} else {
					require.ErrorIs(t, err, errorx.ErrLastAccountAdmin)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			select {
			case err = <-revokeResult:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			var active int
			active, err = db.Core.NewSelect().Model((*database.UserRole)(nil)).Where("role_code = ?", role.RoleCode).Count(ctx)
			require.NoError(t, err)
			require.Equal(t, 1, active)
			_, err = store.FindByUsername(ctx, users[0].Username)
			require.NoError(t, err)
		})
	}
}

func createDeleteUserTestFixtures(t *testing.T, ctx context.Context, db *database.DB, us database.UserStore, os database.OrgStore, suffix string) *database.User {
	t.Helper()

	user := &database.User{
		GitID:    time.Now().UnixNano(),
		UUID:     uuid.NewString(),
		Username: "user-" + suffix,
	}
	require.NoError(t, us.Create(ctx, user, &database.Namespace{Path: user.Username}))

	org := &database.Organization{
		Name:     "org-" + suffix,
		Nickname: "Org " + suffix,
		UUID:     uuid.New(),
	}
	require.NoError(t, os.Create(ctx, org, &database.Namespace{Path: org.Name}))

	storedOrg := &database.Organization{}
	require.NoError(t, db.Core.NewSelect().Model(storedOrg).Where("path = ?", org.Name).Scan(ctx))
	_, err := db.Core.NewInsert().Model(&database.Member{
		OrganizationID: storedOrg.ID,
		UserID:         user.ID,
		Role:           string(types.UserAdmin),
	}).Exec(ctx)
	require.NoError(t, err)

	return user
}

package database_test

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
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

func TestUserStore_Roles(t *testing.T) {
	type fields struct {
		RoleMask string
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{
		// TODO: Add test cases.
		{
			name: "test no role",
			fields: fields{
				RoleMask: "",
			},
			want: []string{},
		},
		{
			name: "test one role",
			fields: fields{
				RoleMask: "admin",
			},
			want: []string{"admin"},
		},
		{
			name: "test two roles",
			fields: fields{
				RoleMask: "admin,super_user",
			},
			want: []string{"admin", "super_user"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &database.User{
				RoleMask: tt.fields.RoleMask,
			}
			if got := u.Roles(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("User.Roles() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUserStore_GetAdminRecipients(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	userStore := database.NewUserStoreWithDB(db)

	testUsers := []database.User{
		{GitID: 99001, Username: "admin-recipient", UUID: "admin-recipient-uuid", Email: "admin@example.com", RoleMask: "admin"},
		{GitID: 99002, Username: "super-user-recipient", UUID: "super-user-recipient-uuid", Email: "super-user@example.com", RoleMask: "user,super_user"},
		{GitID: 99003, Username: "ordinary-recipient", UUID: "ordinary-recipient-uuid", Email: "ordinary@example.com", RoleMask: "user"},
		{GitID: 99004, Username: "admin-without-email", UUID: "admin-without-email-uuid", RoleMask: "admin"},
	}
	for i := range testUsers {
		err := userStore.Create(ctx, &testUsers[i], &database.Namespace{
			Path: testUsers[i].Username,
		})
		require.NoError(t, err)
	}

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

	ch, err := us.IndexWithCursor(ctx, req)
	require.NoError(t, err)

	total := 0
	for wrapper := range ch {
		require.NoError(t, wrapper.Err)
		total += len(wrapper.Users)
	}

	require.Equal(t, 3, total)
}

func TestUserStore_DeleteUserAndRelationsLastOrgAdmin(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()

	us := database.NewUserStoreWithDB(db)
	os := database.NewOrgStoreWithDB(db)
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
	ctx := context.Background()

	us := database.NewUserStoreWithDB(db)
	os := database.NewOrgStoreWithDB(db)
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

package database_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

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

			users, count, err := userStore.IndexWithSearch(ctx, "foo", "", c.labels, c.per, c.page)
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
	require.Empty(t, err)
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

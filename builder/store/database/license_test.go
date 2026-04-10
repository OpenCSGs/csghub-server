package database_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestLicenseStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewLicenseStoreWithDB(db)

	err := store.Create(ctx, database.License{
		Key:        "key",
		Company:    "foo",
		Email:      "test@example.com",
		Product:    "test",
		Edition:    "standard",
		MaxUser:    10,
		StartTime:  time.Now().Add(-1 * time.Minute),
		ExpireTime: time.Now().Add(-1 * time.Hour),
		UserUUID:   "test-user-uuid",
		Issuer:     "tester",
	})
	require.Nil(t, err)

	l := &database.License{}
	err = db.Core.NewSelect().Model(l).Where("key=?", "key").Scan(ctx, l)
	require.Nil(t, err)
	require.Equal(t, "foo", l.Company)

	l, err = store.GetByID(ctx, l.ID)
	require.Nil(t, err)
	require.Equal(t, "foo", l.Company)

	l.Company = "bar"
	err = store.Update(ctx, *l)
	require.Nil(t, err)

	lVerify := &database.License{}
	err = db.Core.NewSelect().Model(lVerify).Where("key=?", "key").Scan(ctx, lVerify)
	require.Nil(t, err)
	require.Equal(t, "bar", lVerify.Company)

	l, err = store.GetByID(ctx, l.ID)
	require.Nil(t, err)
	require.Equal(t, "bar", l.Company)

	_, err = store.GetLatestActive(ctx)
	require.NotNil(t, err)

	l.ExpireTime = time.Now().Add(time.Hour)
	err = store.Update(ctx, *l)
	require.Nil(t, err)

	lVerify2 := &database.License{}
	err = db.Core.NewSelect().Model(lVerify2).Where("key=?", "key").Scan(ctx, lVerify2)
	require.Nil(t, err)

	err = store.Delete(ctx, *l)
	require.Nil(t, err)
	_, err = store.GetByID(ctx, l.ID)
	require.NotNil(t, err)

}

func TestLicenseStore_List(t *testing.T) {

	cases := []struct {
		req      types.QueryLicenseReq
		expected []string
	}{
		{
			req:      types.QueryLicenseReq{Product: "p1"},
			expected: []string{"k1"},
		},
		{
			req:      types.QueryLicenseReq{Edition: "e1"},
			expected: []string{"k3", "k1"},
		},
		{
			req:      types.QueryLicenseReq{Product: "p2", Edition: "e1"},
			expected: []string{},
		},
		{
			req:      types.QueryLicenseReq{Search: "foo"},
			expected: []string{"k2", "k1"},
		},
		{
			req:      types.QueryLicenseReq{Search: "bar", Edition: "e2"},
			expected: []string{"k4"},
		},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c.req), func(t *testing.T) {
			c.req.Page = 1
			c.req.Per = 10
			db := tests.InitTestDB()
			defer db.Close()
			ctx := context.TODO()

			store := database.NewLicenseStoreWithDB(db)
			ls := []database.License{
				{
					Key: "k1", Product: "p1",
					Edition: "e1", Company: "foo",
					Email: "u1@foo.com", Remark: "foo",
					MaxUser: 10, StartTime: time.Now(),
					ExpireTime: time.Now().Add(time.Hour),
					UserUUID:   "user1",
				},
				{
					Key: "k2", Product: "p2",
					Edition: "e2", Company: "foo",
					Email: "u2@foo.com", Remark: "foo-v2",
					MaxUser: 20, StartTime: time.Now(),
					ExpireTime: time.Now().Add(time.Hour),
					UserUUID:   "user2",
				},
				{
					Key: "k3", Product: "p3",
					Edition: "e1", Company: "bar",
					Email: "u1@bar.com", Remark: "bar",
					MaxUser: 30, StartTime: time.Now(),
					ExpireTime: time.Now().Add(time.Hour),
					UserUUID:   "user3",
				},
				{
					Key: "k4", Product: "p3",
					Edition: "e2", Company: "bar",
					Email: "u1@bar.com", Remark: "bar",
					MaxUser: 40, StartTime: time.Now(),
					ExpireTime: time.Now().Add(time.Hour),
					UserUUID:   "user4",
				},
			}
			for _, l := range ls {
				_, err := db.Core.NewInsert().Model(&l).Exec(ctx)
				require.Nil(t, err)
			}

			data, total, err := store.List(ctx, c.req)
			require.Nil(t, err)
			keys := []string{}
			for _, d := range data {
				keys = append(keys, d.Key)
			}
			require.Equal(t, c.expected, keys)
			require.Equal(t, len(c.expected), total)
		})
	}
}

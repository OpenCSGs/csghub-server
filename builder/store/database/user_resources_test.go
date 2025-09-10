package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestUserResourcesStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewUserResourcesStoreWithDB(db)

	err := store.AddUserResources(ctx, &database.UserResources{
		UserUID:       "u1",
		OrderId:       "o1",
		OrderDetailId: 123,
		EndTime:       time.Now(),
	})
	require.Nil(t, err)

	ur := &database.UserResources{}
	err = db.Core.NewSelect().Model(ur).Where("user_uid=?", "u1").Scan(ctx, ur)
	require.Nil(t, err)
	require.Equal(t, "o1", ur.OrderId)

	urs, total, err := store.GetUserResourcesByUserUID(ctx, 10, 1, "u1")
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "o1", urs[0].OrderId)

	ur, err = store.FindUserResourcesByOrderDetailId(ctx, "u1", 123)
	require.Nil(t, err)
	require.Equal(t, "o1", ur.OrderId)

	ur.PayMode = "foo"
	err = store.UpdateDeployId(ctx, ur)
	require.Nil(t, err)
	ur, err = store.FindUserResourcesByOrderDetailId(ctx, "u1", 123)
	require.Nil(t, err)
	require.Equal(t, "foo", ur.PayMode)

	err = store.DeleteUserResourcesByOrderDetailId(ctx, "u1", 123)
	require.Nil(t, err)
	_, err = store.FindUserResourcesByOrderDetailId(ctx, "u1", 123)
	require.NotNil(t, err)

	sp1 := &database.SpaceResource{
		ClusterID: "c1",
		Name:      "s1",
	}
	err = db.Core.NewInsert().Model(sp1).Scan(ctx, sp1)
	require.Nil(t, err)
	sp2 := &database.SpaceResource{
		ClusterID: "c2",
		Name:      "s2",
	}
	err = db.Core.NewInsert().Model(sp2).Scan(ctx, sp2)
	require.Nil(t, err)

	err = store.AddUserResources(ctx, &database.UserResources{
		UserUID:       "u1",
		OrderId:       "o1",
		OrderDetailId: 123,
		ResourceId:    sp1.ID,
		EndTime:       time.Now().Add(5 * time.Hour),
	})
	require.Nil(t, err)
	err = store.AddUserResources(ctx, &database.UserResources{
		UserUID:       "u1",
		OrderId:       "o2",
		OrderDetailId: 124,
		ResourceId:    sp2.ID,
		EndTime:       time.Now().Add(5 * time.Hour),
	})
	require.Nil(t, err)

	urs, err = store.GetReservedUserResources(ctx, "u1", "c1")
	require.Nil(t, err)
	require.Equal(t, 1, len(urs))
	require.Equal(t, "o1", urs[0].OrderId)

}

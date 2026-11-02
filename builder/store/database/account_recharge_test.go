package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAccountRechargeStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountRechargeStoreWithDB(db)
	err := store.CreateRecharge(ctx, &database.AccountRecharge{
		OrderNo:      "o1",
		UserUUID:     "u1",
		Amount:       100,
		RechargeUUID: "uuid",
	})
	require.Nil(t, err)

	rc := &database.AccountRecharge{}
	err = db.Core.NewSelect().Model(rc).Where("order_no=?", "o1").Scan(ctx, rc)
	require.Nil(t, err)
	require.Equal(t, 100, int(rc.Amount))

	rc, err = store.GetRecharge(ctx, "uuid")
	require.Nil(t, err)
	require.Equal(t, 100, int(rc.Amount))
	rc, err = store.GetRecharge(ctx, "uuid")
	require.Nil(t, err)
	require.Equal(t, 100, int(rc.Amount))

	rc, err = store.GetRechargeByOrderNo(ctx, "o1")
	require.Nil(t, err)
	require.Equal(t, 100, int(rc.Amount))

	rc.Description = "ddd"
	err = store.UpdateRecharge(ctx, rc)
	require.Nil(t, err)
	rc, err = store.GetRechargeByOrderNo(ctx, "o1")
	require.Nil(t, err)
	require.Equal(t, "ddd", rc.Description)

	rcs, err := store.ListRechargeByUserUUID(ctx, "u1", 10, 0)
	require.Nil(t, err)
	require.Equal(t, 1, len(rcs))
	require.Equal(t, 100, int(rcs[0].Amount))

	rcs, err = store.ListRecharges(ctx, "u1", database.RechargeFilter{})
	require.Nil(t, err)
	require.Equal(t, 1, len(rcs))
	require.Equal(t, 100, int(rcs[0].Amount))

	stats, err := store.CountRecharges(ctx, "u1", database.RechargeFilter{})
	require.Nil(t, err)
	require.Equal(t, &types.RechargeStats{
		Count: 1,
		Sum:   100,
	}, stats)

	rcs, err = store.ListRecharges(ctx, "", database.RechargeFilter{})
	require.Nil(t, err)
	require.Equal(t, 1, len(rcs))
	require.Equal(t, 100, int(rcs[0].Amount))

	stats, err = store.CountRecharges(ctx, "", database.RechargeFilter{})
	require.Nil(t, err)
	require.Equal(t, &types.RechargeStats{
		Count: 1,
		Sum:   100,
	}, stats)
}

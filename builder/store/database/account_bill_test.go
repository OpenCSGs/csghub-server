package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAccountBillStore_List(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	dt := time.Date(2022, 11, 22, 3, 0, 0, 0, time.UTC)

	bills := []database.AccountBill{
		{
			// included
			UserUUID: "foo", Value: 3, Consumption: 4,
			BillDate: dt.Add(-3 * 24 * time.Hour), CustomerID: "c1",
			Scene: types.ScenePayOrder,
		},
		{
			// not included, date
			UserUUID: "foo", Value: 4, Consumption: 5,
			BillDate: dt.Add(6 * 24 * time.Hour), CustomerID: "c1",
			Scene: types.ScenePayOrder,
		},
		{
			// included
			UserUUID: "foo", Value: 5, Consumption: 6,
			BillDate: dt.Add(-1 * 24 * time.Hour), CustomerID: "c1",
			Scene: types.ScenePayOrder,
		},
		{
			// not included, date
			UserUUID: "foo", Value: 10, Consumption: 11,
			BillDate: dt.Add(-6 * 24 * time.Hour), CustomerID: "c1",
			Scene: types.ScenePayOrder,
		},
		{
			// included
			UserUUID: "foo", Value: 20, Consumption: 21,
			BillDate: dt.Add(2 * 24 * time.Hour), CustomerID: "c1",
			Scene: types.ScenePayOrder,
		},
		{
			// included
			UserUUID: "foo", Value: 20, Consumption: 21,
			BillDate: dt.Add(3 * 24 * time.Hour), CustomerID: "c2",
			Scene: types.ScenePayOrder,
		},
		{
			// not included, scene
			UserUUID: "foo", Value: 30, Consumption: 31,
			BillDate: dt.Add(3 * 24 * time.Hour), CustomerID: "c2",
			Scene: types.SceneMultiSync,
		},
		{
			// not included, uuid
			UserUUID: "bar", Value: 6, Consumption: 7,
			BillDate: dt, CustomerID: "c1", Scene: types.ScenePayOrder,
		},
		{
			// not included, uuid
			UserUUID: "bar", Value: 7, Consumption: 8,
			BillDate: dt.Add(1 * 24 * time.Hour), CustomerID: "c1",
			Scene: types.ScenePayOrder,
		},
	}

	_, err := db.Operator.Core.NewInsert().Model(&bills).Exec(ctx)
	require.Nil(t, err)

	store := database.NewAccountBillStoreWithDB(db)
	res, err := store.ListByUserIDAndDate(ctx, types.AcctBillsReq{
		TargetUUID: "foo",
		Scene:      2, // ScenePayOrder
		StartDate:  dt.Add(-5 * 24 * time.Hour).Format(time.RFC3339),
		EndDate:    dt.Add(5 * 24 * time.Hour).Format(time.RFC3339),
		Per:        20,
		Page:       1,
	})
	require.Nil(t, err)
	require.Equal(t, 2, len(res.Data))
	expectedData := []types.ITEM{
		{Consumption: 31, InstanceName: "c1", Value: 28, PromptToken: 0, CompletionToken: 0},
		{Consumption: 21, InstanceName: "c2", Value: 20, PromptToken: 0, CompletionToken: 0},
	}
	require.Equal(t, expectedData, res.Data)

}

func TestAccountBillStore_ListWithDeployJoin(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	dt := time.Date(2022, 11, 22, 3, 0, 0, 0, time.UTC)

	deploys := []database.Deploy{
		{
			SvcName:   "c2",
			SpaceID:   1,
			Status:    1,
			GitPath:   "path",
			GitBranch: "main",
			Template:  "tpl",
			Hardware:  "hw",
			UserID:    1,
		},
		{
			SvcName:   "c1",
			SpaceID:   2,
			Status:    1,
			GitPath:   "path",
			GitBranch: "main",
			Template:  "tpl",
			Hardware:  "hw",
			UserID:    2,
		},
	}
	_, err := db.Operator.Core.NewInsert().Model(&deploys).Exec(ctx)
	require.Nil(t, err)

	// Give deploys distinct created_at values so d.created_at DESC is deterministic
	_, err = db.Operator.Core.NewUpdate().
		Model((*database.Deploy)(nil)).
		Set("created_at = ?", dt.Add(2*24*time.Hour)).
		Where("svc_name = 'c2'").
		Exec(ctx)
	require.Nil(t, err)
	_, err = db.Operator.Core.NewUpdate().
		Model((*database.Deploy)(nil)).
		Set("created_at = ?", dt.Add(1*24*time.Hour)).
		Where("svc_name = 'c1'").
		Exec(ctx)
	require.Nil(t, err)

	bills := []database.AccountBill{
		{
			UserUUID: "foo", Value: 3, Consumption: 4,
			BillDate: dt.Add(-3 * 24 * time.Hour), CustomerID: "c1",
			Scene: types.SceneSpace,
		},
		{
			UserUUID: "foo", Value: 5, Consumption: 6,
			BillDate: dt.Add(-1 * 24 * time.Hour), CustomerID: "c1",
			Scene: types.SceneSpace,
		},
		{
			UserUUID: "foo", Value: 20, Consumption: 21,
			BillDate: dt.Add(2 * 24 * time.Hour), CustomerID: "c1",
			Scene: types.SceneSpace,
		},
		{
			UserUUID: "foo", Value: 20, Consumption: 21,
			BillDate: dt.Add(3 * 24 * time.Hour), CustomerID: "c2",
			Scene: types.SceneSpace,
		},
	}
	_, err = db.Operator.Core.NewInsert().Model(&bills).Exec(ctx)
	require.Nil(t, err)

	store := database.NewAccountBillStoreWithDB(db)
	res, err := store.ListByUserIDAndDate(ctx, types.AcctBillsReq{
		TargetUUID: "foo",
		Scene:      types.SceneSpace,
		StartDate:  dt.Add(-5 * 24 * time.Hour).Format(time.RFC3339),
		EndDate:    dt.Add(5 * 24 * time.Hour).Format(time.RFC3339),
		Per:        20,
		Page:       1,
	})
	require.Nil(t, err)
	require.Equal(t, 2, len(res.Data))
	// c2 deploy has later created_at, so d.created_at DESC puts c2 first
	require.Equal(t, "c2", res.Data[0].InstanceName)
	require.Equal(t, float64(21), res.Data[0].Consumption)
	require.Equal(t, float64(20), res.Data[0].Value)
	require.Equal(t, "c1", res.Data[1].InstanceName)
	require.Equal(t, float64(31), res.Data[1].Consumption)
	require.Equal(t, float64(28), res.Data[1].Value)

}

func TestAccountBillStore_ListBillsDetailByUserID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	dt := time.Date(2022, 11, 22, 3, 0, 0, 0, time.UTC)
	bills := []database.AccountBill{
		{
			UserUUID: "foo", Value: 3, Consumption: 4,
			BillDate: dt.Add(-3 * 24 * time.Hour), CustomerID: "c1",
			Scene: types.ScenePayOrder, PromptToken: 10, CompletionToken: 20,
		},
		{
			UserUUID: "foo", Value: 4, Consumption: 5,
			BillDate: dt.Add(6 * 24 * time.Hour), CustomerID: "c1",
			Scene: types.ScenePayOrder, PromptToken: 15, CompletionToken: 25,
		},
		{
			UserUUID: "foo", Value: 5, Consumption: 6,
			BillDate: dt.Add(-1 * 24 * time.Hour), CustomerID: "c1",
			Scene: types.ScenePayOrder, PromptToken: 30, CompletionToken: 40,
		},
		{
			UserUUID: "foo", Value: 10, Consumption: 11,
			BillDate: dt.Add(-6 * 24 * time.Hour), CustomerID: "c1",
			Scene: types.ScenePayOrder, PromptToken: 50, CompletionToken: 60,
		},
		{
			UserUUID: "foo", Value: 20, Consumption: 21,
			BillDate: dt.Add(2 * 24 * time.Hour), CustomerID: "c1",
			Scene: types.ScenePayOrder, PromptToken: 70, CompletionToken: 80,
		},
		{
			UserUUID: "foo", Value: 20, Consumption: 21,
			BillDate: dt.Add(3 * 24 * time.Hour), CustomerID: "c2",
			Scene: types.ScenePayOrder, PromptToken: 90, CompletionToken: 100,
		},
		{
			UserUUID: "foo", Value: 30, Consumption: 31,
			BillDate: dt.Add(3 * 24 * time.Hour), CustomerID: "c2",
			Scene: types.SceneMultiSync, PromptToken: 110, CompletionToken: 120,
		},
		{
			UserUUID: "bar", Value: 6, Consumption: 7,
			BillDate: dt, CustomerID: "c1", Scene: types.ScenePayOrder,
			PromptToken: 130, CompletionToken: 140,
		},
		{
			UserUUID: "bar", Value: 7, Consumption: 8,
			BillDate: dt.Add(1 * 24 * time.Hour), CustomerID: "c1",
			Scene: types.ScenePayOrder, PromptToken: 150, CompletionToken: 160,
		},
	}

	_, err := db.Operator.Core.NewInsert().Model(&bills).Exec(ctx)
	require.Nil(t, err)

	store := database.NewAccountBillStoreWithDB(db)

	t.Run("list all bills detail without instance name filter", func(t *testing.T) {
		res, err := store.ListBillsDetailByUserID(ctx, types.AcctBillsDetailReq{
			TargetUUID: "foo",
			Scene:      int(types.ScenePayOrder),
			StartDate:  dt.Add(-5 * 24 * time.Hour).Format(time.RFC3339),
			EndDate:    dt.Add(5 * 24 * time.Hour).Format(time.RFC3339),
			Per:        20,
			Page:       1,
		})
		require.Nil(t, err)
		require.Equal(t, 4, len(res.Data))
		require.Equal(t, 4, res.Total)
	})

	t.Run("list bills detail with instance name filter", func(t *testing.T) {
		res, err := store.ListBillsDetailByUserID(ctx, types.AcctBillsDetailReq{
			TargetUUID:   "foo",
			Scene:        int(types.ScenePayOrder),
			StartDate:    dt.Add(-5 * 24 * time.Hour).Format(time.RFC3339),
			EndDate:      dt.Add(5 * 24 * time.Hour).Format(time.RFC3339),
			InstanceName: "c1",
			Per:          20,
			Page:         1,
		})
		require.Nil(t, err)
		require.Equal(t, 3, len(res.Data))
		require.Equal(t, 3, res.Total)
		for _, bill := range res.Data {
			require.Equal(t, "c1", bill.CustomerID)
		}
	})

	t.Run("list bills detail with pagination", func(t *testing.T) {
		res, err := store.ListBillsDetailByUserID(ctx, types.AcctBillsDetailReq{
			TargetUUID: "foo",
			Scene:      int(types.ScenePayOrder),
			StartDate:  dt.Add(-5 * 24 * time.Hour).Format(time.RFC3339),
			EndDate:    dt.Add(5 * 24 * time.Hour).Format(time.RFC3339),
			Per:        2,
			Page:       1,
		})
		require.Nil(t, err)
		require.Equal(t, 2, len(res.Data))
		require.Equal(t, 4, res.Total)
	})

	t.Run("list bills detail with page 2", func(t *testing.T) {
		res, err := store.ListBillsDetailByUserID(ctx, types.AcctBillsDetailReq{
			TargetUUID: "foo",
			Scene:      int(types.ScenePayOrder),
			StartDate:  dt.Add(-5 * 24 * time.Hour).Format(time.RFC3339),
			EndDate:    dt.Add(5 * 24 * time.Hour).Format(time.RFC3339),
			Per:        2,
			Page:       2,
		})
		require.Nil(t, err)
		require.Equal(t, 2, len(res.Data))
		require.Equal(t, 4, res.Total)
	})

	t.Run("list bills detail with different scene", func(t *testing.T) {
		res, err := store.ListBillsDetailByUserID(ctx, types.AcctBillsDetailReq{
			TargetUUID: "foo",
			Scene:      int(types.SceneMultiSync),
			StartDate:  dt.Add(-5 * 24 * time.Hour).Format(time.RFC3339),
			EndDate:    dt.Add(5 * 24 * time.Hour).Format(time.RFC3339),
			Per:        20,
			Page:       1,
		})
		require.Nil(t, err)
		require.Equal(t, 1, len(res.Data))
		require.Equal(t, 1, res.Total)
		require.Equal(t, types.SceneMultiSync, res.Data[0].Scene)
	})

	t.Run("list bills detail with different user", func(t *testing.T) {
		res, err := store.ListBillsDetailByUserID(ctx, types.AcctBillsDetailReq{
			TargetUUID: "bar",
			Scene:      int(types.ScenePayOrder),
			StartDate:  dt.Add(-5 * 24 * time.Hour).Format(time.RFC3339),
			EndDate:    dt.Add(5 * 24 * time.Hour).Format(time.RFC3339),
			Per:        20,
			Page:       1,
		})
		require.Nil(t, err)
		require.Equal(t, 2, len(res.Data))
		require.Equal(t, 2, res.Total)
	})
}

func TestAccountBillStore_SumValueByAPIKey(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	dt := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	bills := []database.AccountBill{
		{
			UserUUID: "user1", Value: 10, Consumption: 20,
			BillDate: dt, CustomerID: "c1",
			Scene:   types.SceneModelServerless,
			TokenID: 1,
		},
		{
			UserUUID: "user1", Value: 20, Consumption: 30,
			BillDate: dt.Add(1 * 24 * time.Hour), CustomerID: "c1",
			Scene:   types.SceneModelServerless,
			TokenID: 1,
		},
		{
			UserUUID: "user1", Value: 30, Consumption: 40,
			BillDate: dt.Add(2 * 24 * time.Hour), CustomerID: "c2",
			Scene:   types.SceneModelServerless,
			TokenID: 1,
		},
		{
			UserUUID: "user1", Value: 5, Consumption: 10,
			BillDate: dt, CustomerID: "c1",
			Scene:   types.SceneModelServerless,
			TokenID: 2,
		},
		{
			UserUUID: "user1", Value: 100, Consumption: 200,
			BillDate: dt, CustomerID: "c1",
			Scene:   types.ScenePayOrder,
			TokenID: 1,
		},
	}

	_, err := db.Operator.Core.NewInsert().Model(&bills).Exec(ctx)
	require.Nil(t, err)

	store := database.NewAccountBillStoreWithDB(db)

	t.Run("sum all values for api key 1", func(t *testing.T) {
		result, err := store.SumValueByAPIKey(ctx, 1)
		require.Nil(t, err)
		require.Equal(t, float64(60), result)
	})

	t.Run("sum all values for api key 2", func(t *testing.T) {
		result, err := store.SumValueByAPIKey(ctx, 2)
		require.Nil(t, err)
		require.Equal(t, float64(5), result)
	})

	t.Run("sum values for non-existent api key", func(t *testing.T) {
		result, err := store.SumValueByAPIKey(ctx, 3)
		require.Nil(t, err)
		require.Equal(t, float64(0), result)
	})
}

func TestAccountBillStore_SumValueByAPIKeyBetween(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	dt := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	bills := []database.AccountBill{
		{
			UserUUID: "user1", Value: 10, Consumption: 20,
			BillDate: dt.Add(-5 * 24 * time.Hour), CustomerID: "c1",
			Scene:   types.SceneModelServerless,
			TokenID: 1,
		},
		{
			UserUUID: "user1", Value: 20, Consumption: 30,
			BillDate: dt, CustomerID: "c1",
			Scene:   types.SceneModelServerless,
			TokenID: 1,
		},
		{
			UserUUID: "user1", Value: 30, Consumption: 40,
			BillDate: dt.Add(3 * 24 * time.Hour), CustomerID: "c2",
			Scene:   types.SceneModelServerless,
			TokenID: 1,
		},
		{
			UserUUID: "user1", Value: 40, Consumption: 50,
			BillDate: dt.Add(5 * 24 * time.Hour), CustomerID: "c2",
			Scene:   types.SceneModelServerless,
			TokenID: 1,
		},
		{
			UserUUID: "user1", Value: 50, Consumption: 60,
			BillDate: dt.Add(10 * 24 * time.Hour), CustomerID: "c2",
			Scene:   types.SceneModelServerless,
			TokenID: 1,
		},
		{
			UserUUID: "user1", Value: 100, Consumption: 200,
			BillDate: dt, CustomerID: "c1",
			Scene:   types.ScenePayOrder,
			TokenID: 1,
		},
	}

	_, err := db.Operator.Core.NewInsert().Model(&bills).Exec(ctx)
	require.Nil(t, err)

	store := database.NewAccountBillStoreWithDB(db)

	startDate := dt.Add(-1 * 24 * time.Hour)
	endDate := dt.Add(5 * 24 * time.Hour)

	t.Run("sum values within date range", func(t *testing.T) {
		result, err := store.SumValueByAPIKeyBetween(ctx, 1, startDate, endDate)
		require.Nil(t, err)
		require.Equal(t, float64(90), result)
	})

	t.Run("sum values for different api key", func(t *testing.T) {
		result, err := store.SumValueByAPIKeyBetween(ctx, 2, startDate, endDate)
		require.Nil(t, err)
		require.Equal(t, float64(0), result)
	})

	t.Run("sum values in range", func(t *testing.T) {
		inRangeStart := dt.Add(-3 * 24 * time.Hour)
		inRangeEnd := dt.Add(3 * 24 * time.Hour)
		result, err := store.SumValueByAPIKeyBetween(ctx, 1, inRangeStart, inRangeEnd)
		require.Nil(t, err)
		require.Equal(t, float64(50), result)
	})

	t.Run("sum values for different api key", func(t *testing.T) {
		result, err := store.SumValueByAPIKeyBetween(ctx, 2, startDate, endDate)
		require.Nil(t, err)
		require.Equal(t, float64(0), result)
	})
}

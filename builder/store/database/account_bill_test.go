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
	expectedData := []map[string]interface{}(
		[]map[string]interface{}{
			{"consumption": float64(31), "instance_name": "c1", "value": float64(28), "prompt_token": float64(0), "completion_token": float64(0)},
			{"consumption": float64(21), "instance_name": "c2", "value": float64(20), "prompt_token": float64(0), "completion_token": float64(0)},
		})
	require.Equal(t, expectedData, res.Data)

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

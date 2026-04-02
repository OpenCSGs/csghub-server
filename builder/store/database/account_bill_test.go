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
		UserUUID:  "foo",
		Scene:     2, // ScenePayOrder
		StartDate: dt.Add(-5 * 24 * time.Hour).Format(time.RFC3339),
		EndDate:   dt.Add(5 * 24 * time.Hour).Format(time.RFC3339),
		Per:       20,
		Page:      1,
	})
	require.Nil(t, err)
	require.Equal(t, 2, len(res.Data))
	expectedData := []map[string]interface{}(
		[]map[string]interface{}{
			// value: 3+5+20, consumption: 4+6+21
			{"consumption": float64(31), "instance_name": "c1", "value": float64(28)},
			// value: 20, consumption: 21
			{"consumption": float64(21), "instance_name": "c2", "value": float64(20)},
		})
	require.Equal(t, expectedData, res.Data)

}

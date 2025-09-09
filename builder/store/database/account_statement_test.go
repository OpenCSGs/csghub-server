package database_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAccountStatementStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	as := database.NewAccountStatementStoreWithDB(db)
	au := database.NewAccountUserStoreWithDB(db)

	err := au.Create(ctx, database.AccountUser{
		UserUUID:    "bd05a582-a185-42d7-bf19-ad8108c4523b",
		Balance:     100,
		CashBalance: 200,
	})
	require.Nil(t, err)

	uid := uuid.New()
	err = as.Create(ctx, database.AccountStatement{
		EventUUID:        uid,
		UserUUID:         "bd05a582-a185-42d7-bf19-ad8108c4523b",
		Value:            100,
		Scene:            types.SceneCashCharge,
		OpUID:            "1003",
		CustomerID:       "u-opencsg-test-streamlit-6",
		EventDate:        time.Now(),
		Price:            60,
		PriceUnit:        string(types.UnitMinute),
		Consumption:      10,
		ValueType:        0,
		ResourceID:       "629395367163770880",
		ResourceName:     "Streamlit",
		SkuID:            11,
		RecordedAt:       time.Now(),
		SkuUnit:          60,
		SkuUnitType:      types.UnitMinute,
		SkuPriceCurrency: "",
		EventValue:       100,
		IsCancel:         false,
	})

	require.Nil(t, err)

	user, err := au.FindUserByID(ctx, "bd05a582-a185-42d7-bf19-ad8108c4523b")
	require.Nil(t, err)
	require.Equal(t, float64(300), user.CashBalance)
}

func TestAccountStatementStore_CreateSimple(t *testing.T) {
	cases := []struct {
		scene           types.SceneType
		charge          bool
		balanceType     string
		balanceValue    float64
		userBalance     float64
		userCashBalance float64
		bill            bool
	}{
		{types.ScenePortalCharge, true, types.ChargeBalance, 70, 70, 30, false},
		{types.SceneCashCharge, true, types.ChargeCashBalance, 50, 50, 50, false},
		{types.SceneMultiSync, false, types.ChargeCashBalance, 10, 50, 10, false},
		{types.SceneSpace, false, types.ChargeCashBalance, 10, 50, 10, true},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%v %v", c.scene, c.charge), func(t *testing.T) {
			db := tests.InitTestDB()
			defer db.Close()
			ctx := context.TODO()

			au := &database.AccountUser{
				UserUUID:    "foo",
				Balance:     50,
				CashBalance: 30,
			}
			_, err := db.Core.NewInsert().Model(au).Exec(ctx)
			require.Nil(t, err)

			store := database.NewAccountStatementStoreWithDB(db)

			var v float64 = 20
			if !c.charge {
				v = -20
			}
			err = store.Create(ctx, database.AccountStatement{
				EventUUID: uuid.New(),
				UserUUID:  "foo",
				Scene:     c.scene,
				Value:     v,
			})
			require.Nil(t, err)

			as := &database.AccountStatement{}
			err = db.Core.NewSelect().Model(as).Where("user_uuid=?", "foo").Scan(ctx)
			require.Nil(t, err)
			require.Equal(t, c.balanceType, as.BalanceType)
			require.Equal(t, c.balanceValue, as.BalanceValue)

			acctUser := &database.AccountUser{}
			err = db.Core.NewSelect().Model(acctUser).Where("user_uuid = ?", "foo").Scan(ctx)
			require.Nil(t, err)
			require.Equal(t, c.userBalance, acctUser.Balance)
			require.Equal(t, c.userCashBalance, acctUser.CashBalance)

			if c.bill {
				bill := &database.AccountBill{}
				err = db.Core.NewSelect().Model(bill).Where("user_uuid=?", "foo").Scan(ctx)
				require.Nil(t, err)
				require.Equal(t, c.scene, bill.Scene)
				require.Equal(t, v, bill.Value)
			}

		})
	}
}

func TestAccountStatementStore_DeductAccountFee(t *testing.T) {

	cases := []struct {
		name                string
		remain              float64
		userBalance         float64
		userCashBalance     float64
		expectedUserBalance float64
		expectedCashBalance float64
	}{
		{"remain zero", 0, -20, -10, -20, -10},
		{"user balance enough", -10, -10, 10, -10, 0},
		{"user balance and cash balance enough", -10, 5, 5, 0, 0},
		{"not enough", -10, 4, 4, 0, -2},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := tests.InitTestDB()
			defer db.Close()
			ctx := context.TODO()

			err := db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
				au := database.AccountUser{
					UserUUID:    "foo",
					Balance:     c.userBalance,
					CashBalance: c.userCashBalance,
				}
				_, err := tx.NewInsert().Model(&au).Exec(ctx)
				require.Nil(t, err)

				err = database.DeductAccountFee(ctx, tx, database.AccountStatement{
					Value:    c.remain,
					UserUUID: "foo",
				})
				require.Nil(t, err)

				acctUser := &database.AccountUser{}
				err = tx.NewSelect().Model(acctUser).Where("user_uuid = ?", "foo").Scan(ctx)
				require.Nil(t, err)
				require.Equal(t, c.expectedUserBalance, acctUser.Balance)
				require.Equal(t, c.expectedCashBalance, acctUser.CashBalance)
				return nil
			})
			require.Nil(t, err)

		})
	}

}

func TestAccountStatementStore_DeductAccountFeeParallel(t *testing.T) {

	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.TODO()

	au := database.AccountUser{
		UserUUID:    "foo",
		Balance:     50,
		CashBalance: 50,
	}
	_, err := db.Core.NewInsert().Model(&au).Exec(ctx)
	require.Nil(t, err)

	var wg sync.WaitGroup

	// total 110
	for _, cost := range []float64{2, 4, 6, 8, 10, 12, 14, 16, 18, 20} {
		wg.Add(1)
		go func(cost float64) {
			defer wg.Done()
			tx, err := db.Core.BeginTx(ctx, nil)
			require.Nil(t, err)

			err = database.DeductAccountFee(ctx, tx, database.AccountStatement{
				Value:     -cost,
				UserUUID:  "foo",
				EventUUID: uuid.New(),
			})
			require.Nil(t, err)
			err = tx.Commit()
			require.Nil(t, err)
		}(cost)
	}
	wg.Wait()

	acctUser := &database.AccountUser{}
	err = db.Core.NewSelect().Model(acctUser).Where("user_uuid = ?", "foo").Scan(ctx)
	require.Nil(t, err)

	require.Equal(t, float64(0), acctUser.Balance)
	require.Equal(t, float64(-10), acctUser.CashBalance)

}

func TestAccountStatementStore_ListByUserIDAndTime(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	dt := time.Date(2022, 1, 12, 12, 12, 0, 0, time.UTC)
	statements := []database.AccountStatement{
		{
			UserUUID: "foo", Scene: types.SceneCashCharge, Value: 1, OpUID: "a",
			CustomerID: "bar", CreatedAt: dt.Add(-5 * time.Hour), Consumption: -1,
		},
		{
			UserUUID: "foo", Scene: types.SceneCashCharge, Value: 1, OpUID: "b",
			CustomerID: "bar", CreatedAt: dt.Add(-2 * time.Hour), Consumption: -1,
		},
		{
			UserUUID: "foo", Scene: types.SceneCashCharge, Value: 1, OpUID: "c",
			CustomerID: "bar", CreatedAt: dt.Add(2 * time.Hour), Consumption: -1,
		},
		// not match
		{
			UserUUID: "foo", Scene: types.SceneCashCharge, Value: 1, OpUID: "d",
			CustomerID: "bar", CreatedAt: dt.Add(7 * time.Hour), Consumption: -1,
		},
		{
			UserUUID: "bar", Scene: types.SceneCashCharge, Value: 1, OpUID: "e",
			CustomerID: "bar", CreatedAt: dt.Add(7 * time.Hour), Consumption: -1,
		},
		{
			UserUUID: "foo", Scene: types.SceneEvaluation, Value: 1, OpUID: "f",
			CustomerID: "bar", CreatedAt: dt.Add(-1 * time.Hour), Consumption: -1,
		},
		{
			UserUUID: "foo", Scene: types.SceneEvaluation, Value: 1, OpUID: "g",
			CustomerID: "bar", CreatedAt: dt.Add(1 * time.Hour), Consumption: -1,
		},
	}

	for _, s := range statements {
		_, err := db.Core.NewInsert().Model(&s).Exec(ctx)
		require.Nil(t, err)
	}

	store := database.NewAccountStatementStoreWithDB(db)
	result, err := store.ListByUserIDAndTime(ctx, types.ActStatementsReq{
		UserUUID:     "foo",
		Scene:        3,
		InstanceName: "bar",
		StartTime:    dt.Add(-6 * time.Hour).Format(time.RFC3339),
		EndTime:      dt.Add(6 * time.Hour).Format(time.RFC3339),
	})
	require.Nil(t, err)
	require.Equal(t, 3, result.Total)
	require.Equal(t, float64(3), result.TotalValue)
	require.Equal(t, float64(-3), result.TotalConsumption)

	uids := []string{}
	for _, d := range result.Data {
		uids = append(uids, d.OpUID)
	}
	require.ElementsMatch(t, []string{"a", "b", "c"}, uids)

}

func TestAccountStatementStore_GetByEventID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	uid := uuid.New()
	_, err := db.Core.NewInsert().Model(&database.AccountStatement{
		EventUUID: uid,
		OpUID:     "foo",
	}).Exec(ctx)
	require.Nil(t, err)

	ev, err := database.NewAccountStatementStoreWithDB(db).GetByEventID(ctx, uid)
	require.Nil(t, err)
	require.Equal(t, "foo", ev.OpUID)
}

func TestAccountStatementStore_ListRechargeByUserIDAndTime(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	uuid1 := uuid.New()
	uuid2 := uuid.New()

	dt := time.Date(2022, 1, 12, 12, 12, 0, 0, time.UTC)
	statements := []database.AccountStatement{
		{
			EventUUID: uuid1,
			UserUUID:  "foo", Scene: types.ScenePortalCharge, Value: 1, OpUID: "a",
			CustomerID: "", CreatedAt: dt.Add(-5 * time.Hour), Consumption: -1,
		},
		{
			EventUUID: uuid2,
			UserUUID:  "foo", Scene: types.ScenePortalCharge, Value: 1, OpUID: "b",
			CustomerID: "", CreatedAt: dt.Add(-2 * time.Hour), Consumption: -1,
		},
	}

	presents := []database.AccountPresent{
		{
			EventUUID:  uuid1,
			UserUUID:   "foo",
			ActivityID: 1001,
			Value:      100,
		}, {
			EventUUID:  uuid2,
			UserUUID:   "foo",
			ActivityID: 1002,
			Value:      200,
		},
	}

	for _, s := range statements {
		_, err := db.Core.NewInsert().Model(&s).Exec(ctx)
		require.Nil(t, err)
	}

	for _, p := range presents {
		_, err := db.Core.NewInsert().Model(&p).Exec(ctx)
		require.Nil(t, err)
	}

	store := database.NewAccountStatementStoreWithDB(db)
	result, err := store.ListRechargeByUserIDAndTime(ctx,
		types.AcctRechargeListReq{
			UserUUID:   "foo",
			Scene:      int(types.ScenePortalCharge),
			ActivityID: 0,
			StartTime:  dt.Add(-6 * time.Hour).Format(time.RFC3339),
			EndTime:    dt.Add(6 * time.Hour).Format(time.RFC3339),
		},
	)
	require.Nil(t, err)
	require.Equal(t, 2, result.Total)

	opids := []string{}
	aids := []int64{}
	for _, d := range result.Data {
		opids = append(opids, d.OpUID)
		aids = append(aids, d.Present.ActivityID)
	}
	require.ElementsMatch(t, []string{"a", "b"}, opids)
	require.ElementsMatch(t, []int64{int64(1001), int64(1002)}, aids)
}

func TestAccountStatementStore_ListGroupedByUserAndSku(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	dt := time.Date(2022, 1, 12, 12, 0, 0, 0, time.UTC)

	statements := []database.AccountStatement{
		{
			UserUUID: "user1", SkuID: 1001, Scene: types.SceneCashCharge,
			CustomerID: "cust1", CreatedAt: dt, Value: 10, Consumption: -2,
		},
		{
			UserUUID: "user1", SkuID: 1001, Scene: types.SceneCashCharge,
			CustomerID: "cust1", CreatedAt: dt, Value: 20, Consumption: -3,
		},
		{
			UserUUID: "user1", SkuID: 1002, Scene: types.SceneCashCharge,
			CustomerID: "cust1", CreatedAt: dt, Value: 30, Consumption: -4,
		},
		{
			UserUUID: "user2", SkuID: 1001, Scene: types.SceneCashCharge,
			CustomerID: "cust1", CreatedAt: dt, Value: 999, Consumption: -999,
		},
	}

	for _, s := range statements {
		_, err := db.Core.NewInsert().Model(&s).Exec(ctx)
		require.NoError(t, err)
	}

	store := database.NewAccountStatementStoreWithDB(db)

	res, total, err := store.ListStatementByUserAndSku(ctx, types.ActStatementsReq{
		UserUUID:     "user1",
		Scene:        int(types.SceneCashCharge),
		InstanceName: "cust1",
		StartTime:    dt.Add(-time.Hour).Format(time.RFC3339),
		EndTime:      dt.Add(time.Hour).Format(time.RFC3339),
		Per:          0,
	})
	require.NoError(t, err)

	require.Equal(t, 2, total)

	groupMap := map[int64]struct {
		TotalValue       float64
		TotalConsumption float64
	}{}
	for _, r := range res {
		groupMap[r.SkuID] = struct {
			TotalValue       float64
			TotalConsumption float64
		}{
			TotalValue:       r.TotalValue,
			TotalConsumption: r.TotalConsumption,
		}
	}

	require.Equal(t, float64(30), groupMap[1001].TotalValue)
	require.Equal(t, float64(-5), groupMap[1001].TotalConsumption)

	require.Equal(t, float64(30), groupMap[1002].TotalValue)
	require.Equal(t, float64(-4), groupMap[1002].TotalConsumption)
}

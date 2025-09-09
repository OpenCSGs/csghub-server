package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAccountOrderStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	aos := database.NewAccountOrderStoreWithDB(db)

	au := database.NewAccountUserStoreWithDB(db)

	err := au.Create(ctx, database.AccountUser{
		UserUUID:    "bd05a582-a185-42d7-bf19-ad8108c4523b",
		Balance:     200,
		CashBalance: 100,
	})
	require.Nil(t, err)

	uid := uuid.New()
	err = aos.Create(ctx, database.AccountOrder{
		OrderUUID:   uid.String(),
		UserUUID:    "bd05a582-a185-42d7-bf19-ad8108c4523b",
		OrderStatus: types.OrderCreated,
		Amount:      float64(100),
		CreatedAt:   time.Now(),
		EventUUID:   uid.String(),
		RecordedAt:  time.Now(),
		Details: []database.AccountOrderDetail{
			{
				OrderUUID:   uid.String(),
				ResourceID:  "abc",
				SkuType:     types.SKUCSGHub,
				SkuKind:     types.SKUTimeSpan,
				SkuUnitType: string(types.UnitMinute),
				OrderCount:  1,
				SkuPriceID:  1,
				Amount:      -float64(100),
				BeginTime:   time.Now(),
				EndTime:     time.Now().Add(time.Hour * 1),
				CreatedAt:   time.Now(),
				PresentUUID: "",
			},
		},
	}, database.AccountStatement{
		EventUUID:        uid,
		UserUUID:         "bd05a582-a185-42d7-bf19-ad8108c4523b",
		Value:            -float64(100),
		Scene:            types.ScenePayOrder,
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
	require.Equal(t, float64(0), user.CashBalance)
}

func TestAccountOrderStore_CreateAndGetSimple(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountOrderStoreWithDB(db)
	_, err := db.Core.NewInsert().Model(&database.AccountUser{
		UserUUID: "u1",
	}).Exec(ctx)
	require.Nil(t, err)

	err = store.Create(ctx, database.AccountOrder{
		OrderUUID: "o1",
		UserUUID:  "u1",
		Details: []database.AccountOrderDetail{
			{OrderUUID: "o1", Amount: 1},
			{OrderUUID: "o1", Amount: 2},
			{OrderUUID: "o2", Amount: 3},
		},
	}, database.AccountStatement{
		UserUUID: "u1",
		Value:    0,
		Scene:    types.ScenePayOrder,
	})
	require.Nil(t, err)

	order := &database.AccountOrder{}
	err = db.Core.NewSelect().Model(order).Relation("Details").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "o1", order.OrderUUID)
	require.Equal(t, 2, len(order.Details))
	amounts := []float64{}
	for _, d := range order.Details {
		amounts = append(amounts, d.Amount)
	}
	require.ElementsMatch(t, []float64{1, 2}, amounts)

	st := &database.AccountStatement{}
	err = db.Core.NewSelect().Model(st).Where("user_uuid = ?", "u1").Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, st.UserUUID, "u1")
	require.Equal(t, st.Value, float64(0))

	order, err = store.GetByID(ctx, "o1")
	require.Nil(t, err)
	require.Equal(t, "o1", order.OrderUUID)
	require.Equal(t, 2, len(order.Details))
	amounts = []float64{}
	for _, d := range order.Details {
		amounts = append(amounts, d.Amount)
	}
	require.ElementsMatch(t, []float64{1, 2}, amounts)

}

func TestAccountOrderStore_GetDetail(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountOrderStoreWithDB(db)
	detail := &database.AccountOrderDetail{OrderUUID: "uuid"}
	_, err := db.Core.NewInsert().Model(detail).Exec(ctx)
	require.Nil(t, err)

	dt, err := store.GetDetailByID(ctx, detail.ID)
	require.Nil(t, err)
	require.Equal(t, "uuid", dt.OrderUUID)
}

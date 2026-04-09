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

func TestAccountSubscriptionBillStore_CURD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	reqSkuType := types.SKUStarship
	reqSkuUnitType := types.UnitMonth
	userName := "user1"
	resourceID1 := "resource1"
	userUUID := "user-uuid"

	price1 := database.AccountPrice{
		SkuType:     reqSkuType,
		SkuPrice:    int64(100),
		SkuUnit:     int64(1),
		ResourceID:  resourceID1,
		SkuUnitType: reqSkuUnitType,
		SkuKind:     types.SKUTimeSpan,
	}

	priceStore := database.NewAccountPriceStoreWithDB(db)
	priceRes1, err := priceStore.Create(ctx, price1)
	require.Nil(t, err)

	acctUser := database.AccountUser{
		UserUUID:    userUUID,
		Balance:     float64(200),
		CashBalance: float64(200),
	}
	userStore := database.NewAccountUserStoreWithDB(db)
	err = userStore.Create(ctx, acctUser)
	require.Nil(t, err)

	subStore := database.NewAccountSubscriptionWithDB(db)

	eventUUID := uuid.New()

	createQeq := &types.SubscriptionUpdateReq{
		CurrentUser: userName,
		UserUUID:    userUUID,
		SkuType:     reqSkuType,
		ResourceID:  resourceID1,
		SkuUnitType: reqSkuUnitType,
		EventUUID:   eventUUID,
	}

	sub, err := subStore.CreateOrUpdate(ctx, createQeq)
	require.Nil(t, err)
	require.Equal(t, sub.PriceID, priceRes1.ID)
	require.Equal(t, sub.ResourceID, createQeq.ResourceID)
	require.Equal(t, sub.NextPriceID, priceRes1.ID)
	require.Equal(t, sub.NextResourceID, createQeq.ResourceID)
	require.Equal(t, sub.UserUUID, userUUID)
	require.Equal(t, sub.AmountPaidTotal, float64(price1.SkuPrice))
	require.Equal(t, sub.Status, types.SubscriptionStatusActive)

	billStore := database.NewAccountSubscriptionBillWithDB(db)
	bill, err := billStore.GetByID(ctx, sub.LastBillID)
	require.Nil(t, err)
	require.Equal(t, bill.SubID, sub.ID)

	req := &types.SubscriptionBillListReq{
		CurrentUser:   userName,
		UserUUID:      userUUID,
		QueryUserUUID: userUUID,
		Status:        types.BillingStatusPaid,
		Per:           10,
		Page:          1,
		StartTime:     time.Now().AddDate(0, 0, -1).Format("2006-01-02T15:04:05Z"),
		EndTime:       time.Now().AddDate(0, 0, 1).Format("2006-01-02T15:04:05Z"),
	}

	res, err := billStore.List(ctx, req)
	require.Nil(t, err)
	require.Equal(t, len(res.Data), 1)
	require.Equal(t, res.Total, 1)
}

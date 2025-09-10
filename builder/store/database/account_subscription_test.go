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

func TestAccountSubscriptionStore_CURD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	reqSkuType := types.SKUStarship
	reqSkuUnitType := types.UnitMonth
	userName := "user1"
	resourceID1 := "resource1"
	resourceID2 := "resource2"
	userUUID := "user-uuid"

	price1 := database.AccountPrice{
		SkuType:     reqSkuType,
		SkuPrice:    int64(100),
		SkuUnit:     int64(1),
		ResourceID:  resourceID1,
		SkuUnitType: reqSkuUnitType,
		SkuKind:     types.SKUTimeSpan,
	}

	price2 := database.AccountPrice{
		SkuType:     reqSkuType,
		SkuPrice:    int64(200),
		SkuUnit:     int64(1),
		ResourceID:  resourceID2,
		SkuUnitType: reqSkuUnitType,
		SkuKind:     types.SKUTimeSpan,
	}

	priceStore := database.NewAccountPriceStoreWithDB(db)
	priceRes1, err := priceStore.Create(ctx, price1)
	require.Nil(t, err)
	priceRes2, err := priceStore.Create(ctx, price2)
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

	createQeq := &types.SubscriptionCreateReq{
		CurrentUser: userName,
		UserUUID:    userUUID,
		SkuType:     reqSkuType,
		ResourceID:  resourceID1,
		SkuUnitType: string(reqSkuUnitType),
		EventUUID:   eventUUID,
	}

	sub, err := subStore.Create(ctx, createQeq)
	require.Nil(t, err)
	require.Equal(t, sub.PriceID, priceRes1.ID)
	require.Equal(t, sub.ResourceID, createQeq.ResourceID)
	require.Equal(t, sub.NextPriceID, priceRes1.ID)
	require.Equal(t, sub.NextResourceID, createQeq.ResourceID)
	require.Equal(t, sub.UserUUID, userUUID)
	require.Equal(t, sub.AmountPaidTotal, float64(price1.SkuPrice))
	require.Equal(t, sub.Status, types.SubscriptionStatusActive)

	resUser, err := userStore.FindUserByID(ctx, userUUID)
	require.Nil(t, err)
	require.Equal(t, float64(200), resUser.Balance)
	require.Equal(t, float64(100), resUser.CashBalance)

	billStore := database.NewAccountSubscriptionBillWithDB(db)
	item, err := billStore.GetByID(ctx, sub.LastBillID)
	require.Nil(t, err)
	require.Equal(t, float64(price1.SkuPrice), item.AmountPaid)
	require.Equal(t, item.UserUUID, userUUID)

	acctST := database.NewAccountStatementStoreWithDB(db)
	stmt, err := acctST.GetByEventID(ctx, eventUUID)
	require.Nil(t, err)
	require.Equal(t, float64(0-price1.SkuPrice), stmt.Value)

	listRes, err := subStore.List(ctx, &types.SubscriptionListReq{
		CurrentUser:   userName,
		UserUUID:      userUUID,
		Status:        string(types.SubscriptionStatusActive),
		SkuType:       int(reqSkuType),
		QueryUserUUID: userUUID,
		Per:           10,
		Page:          1,
		StartTime:     time.Now().AddDate(0, 0, -1).Format("2006-01-02T15:04:05Z"),
		EndTime:       time.Now().AddDate(0, 0, 1).Format("2006-01-02T15:04:05Z"),
	})
	require.Nil(t, err)
	require.Equal(t, 1, len(listRes.Data))
	require.Equal(t, 1, listRes.Total)

	resSub, err := subStore.GetByID(ctx, sub.ID)
	require.Nil(t, err)
	require.Equal(t, sub.ID, resSub.ID)

	resSub, err = subStore.StatusByUserUUID(ctx, userUUID, reqSkuType)
	require.Nil(t, err)
	require.Equal(t, sub.Status, types.SubscriptionStatusActive)

	newEvtUUID := uuid.New()
	updateReq := &types.SubscriptionUpdateReq{
		CurrentUser: userName,
		UserUUID:    userUUID,
		SubID:       resSub.ID,
		SkuType:     int(reqSkuType),
		ResourceID:  resourceID2,
		SkuUnitType: string(reqSkuUnitType),
		EventUUID:   newEvtUUID,
	}
	resSub, reasion, err := subStore.UpdateResource(ctx, updateReq, resSub)
	require.Nil(t, err)
	require.Equal(t, resSub.NextResourceID, resourceID2)
	require.Equal(t, resSub.NextPriceID, priceRes2.ID)
	require.Equal(t, reasion, string(types.BillingReasionSubscriptionUpgrade))

	newEndtime := time.Now().AddDate(0, 0, -1)
	resSub.LastPeriodEnd = newEndtime
	resSub, err = subStore.Update(ctx, *resSub)
	require.Nil(t, err)
	require.Equal(t, resSub.LastPeriodEnd, newEndtime)

	list, err := subStore.ListRenews(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(list))

	reNewEventUUID := uuid.New()
	err = subStore.Renew(ctx, resSub, reNewEventUUID)
	require.Nil(t, err)

	// switch back
	switchEvtUUID := uuid.New()
	switchReq := &types.SubscriptionUpdateReq{
		CurrentUser: userName,
		UserUUID:    userUUID,
		SubID:       resSub.ID,
		SkuType:     int(reqSkuType),
		ResourceID:  resourceID1,
		SkuUnitType: string(reqSkuUnitType),
		EventUUID:   switchEvtUUID,
	}
	resSub, reasion, err = subStore.UpdateResource(ctx, switchReq, resSub)
	require.Nil(t, err)
	require.Equal(t, resSub.NextResourceID, resourceID1)
	require.Equal(t, resSub.NextPriceID, priceRes1.ID)
	require.Equal(t, reasion, string(types.BillingReasionSubscriptionDowngrade))

}

func TestAccountSubscriptionStore_CreateThenReuse(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	reqSkuType := types.SKUStarship
	reqSkuUnitType := types.UnitMonth
	userName := "user1"
	resourceID1 := "resource1"
	resourceID2 := "resource2"
	userUUID := "user-uuid"

	price1 := database.AccountPrice{
		SkuType:     reqSkuType,
		SkuPrice:    int64(100),
		SkuUnit:     int64(1),
		ResourceID:  resourceID1,
		SkuUnitType: reqSkuUnitType,
		SkuKind:     types.SKUTimeSpan,
	}

	price2 := database.AccountPrice{
		SkuType:     reqSkuType,
		SkuPrice:    int64(200),
		SkuUnit:     int64(1),
		ResourceID:  resourceID2,
		SkuUnitType: reqSkuUnitType,
		SkuKind:     types.SKUTimeSpan,
	}

	priceStore := database.NewAccountPriceStoreWithDB(db)
	priceRes1, err := priceStore.Create(ctx, price1)
	require.Nil(t, err)
	priceRes2, err := priceStore.Create(ctx, price2)
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

	createQeq := &types.SubscriptionCreateReq{
		CurrentUser: userName,
		UserUUID:    userUUID,
		SkuType:     reqSkuType,
		ResourceID:  resourceID1,
		SkuUnitType: string(reqSkuUnitType),
		EventUUID:   eventUUID,
	}

	sub, err := subStore.Create(ctx, createQeq)
	require.Nil(t, err)
	require.Equal(t, sub.PriceID, priceRes1.ID)
	require.Equal(t, sub.ResourceID, createQeq.ResourceID)
	require.Equal(t, sub.NextPriceID, priceRes1.ID)
	require.Equal(t, sub.NextResourceID, createQeq.ResourceID)
	require.Equal(t, sub.UserUUID, userUUID)
	require.Equal(t, sub.AmountPaidTotal, float64(price1.SkuPrice))
	require.Equal(t, sub.Status, types.SubscriptionStatusActive)

	resUser, err := userStore.FindUserByID(ctx, userUUID)
	require.Nil(t, err)
	require.Equal(t, float64(200), resUser.Balance)
	require.Equal(t, float64(100), resUser.CashBalance)

	billStore := database.NewAccountSubscriptionBillWithDB(db)
	item, err := billStore.GetByID(ctx, sub.LastBillID)
	require.Nil(t, err)
	require.Equal(t, float64(price1.SkuPrice), item.AmountPaid)
	require.Equal(t, item.UserUUID, userUUID)

	acctST := database.NewAccountStatementStoreWithDB(db)
	stmt, err := acctST.GetByEventID(ctx, eventUUID)
	require.Nil(t, err)
	require.Equal(t, float64(0-price1.SkuPrice), stmt.Value)

	sub.Status = types.SubscriptionStatusClosed
	sub.EndAt = time.Now()
	sub, err = subStore.Update(ctx, *sub)
	require.Nil(t, err)
	require.Equal(t, sub.Status, types.SubscriptionStatusClosed)

	// reuse
	newEventUUID := uuid.New()

	createReuse := &types.SubscriptionCreateReq{
		CurrentUser: userName,
		UserUUID:    userUUID,
		SkuType:     reqSkuType,
		ResourceID:  resourceID2,
		SkuUnitType: string(reqSkuUnitType),
		EventUUID:   newEventUUID,
	}

	reUsesub, err := subStore.Create(ctx, createReuse)
	require.Nil(t, err)
	require.Equal(t, reUsesub.PriceID, priceRes2.ID)
	require.Equal(t, reUsesub.ResourceID, createReuse.ResourceID)
	require.Equal(t, reUsesub.NextPriceID, priceRes2.ID)
	require.Equal(t, reUsesub.NextResourceID, createReuse.ResourceID)
	require.Equal(t, reUsesub.UserUUID, userUUID)
	require.Equal(t, reUsesub.Status, types.SubscriptionStatusActive)
	require.Equal(t, reUsesub.AmountPaidCount, int64(2))
	require.Equal(t, reUsesub.ID, sub.ID)

	newBill, err := billStore.GetByID(ctx, reUsesub.LastBillID)
	require.Nil(t, err)
	require.Equal(t, newBill.UserUUID, userUUID)
	require.Equal(t, newBill.SubID, sub.ID)
	require.Equal(t, newBill.EventUUID, newEventUUID.String())
	require.Equal(t, newBill.ResourceID, resourceID2)

	newStmt, err := acctST.GetByEventID(ctx, newEventUUID)
	require.Nil(t, err)
	require.Equal(t, newStmt.ResourceID, createReuse.ResourceID)
}

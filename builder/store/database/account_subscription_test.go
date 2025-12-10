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

func TestAccountSubscriptionStore_CreateOrUpdate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	reqSkuType := types.SKUStarship
	reqSkuUnitType := types.UnitMonth

	userName := "user1"
	userUUID := "user-uuid"
	resourceID1 := "resource1"
	resourceID2 := "resource2"

	// add price
	priceStore := database.NewAccountPriceStoreWithDB(db)
	price1 := database.AccountPrice{
		SkuType:     reqSkuType,
		SkuPrice:    int64(100),
		SkuUnit:     int64(1),
		ResourceID:  resourceID1,
		SkuUnitType: reqSkuUnitType,
		SkuKind:     types.SKUTimeSpan,
	}
	priceRes1, err := priceStore.Create(ctx, price1)
	require.Nil(t, err)

	price2 := database.AccountPrice{
		SkuType:     reqSkuType,
		SkuPrice:    int64(200),
		SkuUnit:     int64(1),
		ResourceID:  resourceID2,
		SkuUnitType: reqSkuUnitType,
		SkuKind:     types.SKUTimeSpan,
	}
	priceRes2, err := priceStore.Create(ctx, price2)
	require.Nil(t, err)

	// set user balance
	userStore := database.NewAccountUserStoreWithDB(db)
	acctUser := database.AccountUser{
		UserUUID:    userUUID,
		Balance:     float64(200),
		CashBalance: float64(200),
	}
	err = userStore.Create(ctx, acctUser)
	require.Nil(t, err)

	// case 1: Create - new - sub
	subStore := database.NewAccountSubscriptionWithDB(db)
	eventUUID := uuid.New()
	createReq := &types.SubscriptionUpdateReq{
		CurrentUser: userName,
		UserUUID:    userUUID,
		SkuType:     reqSkuType,
		ResourceID:  resourceID1,
		SkuUnitType: reqSkuUnitType,
		EventUUID:   eventUUID,
	}
	sub1, err := subStore.CreateOrUpdate(ctx, createReq)
	require.Nil(t, err)
	require.Equal(t, sub1.PriceID, priceRes1.ID)
	require.Equal(t, sub1.ResourceID, priceRes1.ResourceID)
	require.Equal(t, sub1.NextPriceID, priceRes1.ID)
	require.Equal(t, sub1.NextResourceID, priceRes1.ResourceID)
	require.Equal(t, sub1.UserUUID, userUUID)
	require.Equal(t, sub1.Status, types.SubscriptionStatusActive)

	// check user balance after
	resUser, err := userStore.FindUserByID(ctx, userUUID)
	require.Nil(t, err)
	require.Equal(t, float64(200), resUser.Balance)
	require.Equal(t, float64(100), resUser.CashBalance)

	// check bill
	billStore := database.NewAccountSubscriptionBillWithDB(db)
	item, err := billStore.GetByID(ctx, sub1.LastBillID)
	require.Nil(t, err)
	require.Equal(t, float64(price1.SkuPrice), item.AmountPaid)
	require.Equal(t, item.UserUUID, userUUID)

	// check statement
	acctST := database.NewAccountStatementStoreWithDB(db)
	stmt, err := acctST.GetByEventID(ctx, eventUUID)
	require.Nil(t, err)
	require.Equal(t, float64(0-price1.SkuPrice), stmt.Value)

	// case 2: upgrade - exist - sub
	eventUUID = uuid.New()
	updateReq := &types.SubscriptionUpdateReq{
		CurrentUser: userName,
		UserUUID:    userUUID,
		SkuType:     reqSkuType,
		ResourceID:  resourceID2,
		SkuUnitType: reqSkuUnitType,
		EventUUID:   eventUUID,
	}
	sub2, err := subStore.CreateOrUpdate(ctx, updateReq)
	require.Nil(t, err)
	require.Equal(t, sub2.PriceID, priceRes2.ID)
	require.Equal(t, sub2.ResourceID, priceRes2.ResourceID)
	require.Equal(t, sub2.NextPriceID, priceRes2.ID)
	require.Equal(t, sub2.NextResourceID, priceRes2.ResourceID)
	require.Equal(t, sub2.UserUUID, userUUID)
	require.Equal(t, sub2.Status, types.SubscriptionStatusActive)

	// case 3: close exist sub
	eventUUID = uuid.New()
	closeReq := &types.SubscriptionUpdateReq{
		CurrentUser: userName,
		UserUUID:    userUUID,
		SkuType:     reqSkuType,
		ResourceID:  types.SubscriptionFree,
		SkuUnitType: reqSkuUnitType,
		EventUUID:   eventUUID,
	}
	sub3, err := subStore.CreateOrUpdate(ctx, closeReq)
	require.Nil(t, err)
	require.Equal(t, sub3.PriceID, priceRes2.ID)
	require.Equal(t, sub3.ResourceID, priceRes2.ResourceID)
	require.Equal(t, sub3.Status, types.SubscriptionStatusClosed)
	require.NotNil(t, sub3.EndAt)

	// case 4: switch status back
	updateReq.EventUUID = uuid.New()
	sub2, err = subStore.CreateOrUpdate(ctx, updateReq)
	require.Nil(t, err)
	require.Equal(t, sub2.PriceID, priceRes2.ID)
	require.Equal(t, sub2.ResourceID, priceRes2.ResourceID)
	require.Equal(t, sub2.NextPriceID, priceRes2.ID)
	require.Equal(t, sub2.NextResourceID, priceRes2.ResourceID)
	require.Equal(t, sub2.UserUUID, userUUID)
	require.Equal(t, sub2.Status, types.SubscriptionStatusActive)

	// update sub last period end
	sub3.LastPeriodEnd, err = time.Parse("2006-01-02", "2023-01-01")
	require.Nil(t, err)
	sub3, err = subStore.Update(ctx, *sub3)
	require.Nil(t, err)

	// case 5: refresh exist sub
	eventUUID = uuid.New()
	refreshReq := &types.SubscriptionUpdateReq{
		CurrentUser: userName,
		UserUUID:    userUUID,
		SkuType:     reqSkuType,
		ResourceID:  resourceID2,
		SkuUnitType: reqSkuUnitType,
		EventUUID:   eventUUID,
	}
	sub4, err := subStore.CreateOrUpdate(ctx, refreshReq)
	require.Nil(t, err)
	require.Equal(t, sub4.PriceID, priceRes2.ID)
	require.Equal(t, sub4.ResourceID, priceRes2.ResourceID)
	require.Equal(t, sub4.PriceID, sub4.NextPriceID)
	require.Equal(t, sub4.ResourceID, sub4.NextResourceID)
	require.Equal(t, sub4.Status, types.SubscriptionStatusActive)
	require.Equal(t, sub4.StartAt, sub4.LastPeriodStart)
	require.Equal(t, sub4.AmountPaidCount, sub3.AmountPaidCount+1)

}

func TestAccountSubscriptionStore_CURD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	reqSkuType := types.SKUCSGHub
	reqSkuUnitType := types.UnitMonth
	userName := "user1"
	resourceID1 := "resource1"
	userUUID := "user-uuid"

	// add price
	priceStore := database.NewAccountPriceStoreWithDB(db)
	price1 := database.AccountPrice{
		SkuType:     reqSkuType,
		SkuPrice:    int64(100),
		SkuUnit:     int64(1),
		ResourceID:  resourceID1,
		SkuUnitType: reqSkuUnitType,
		SkuKind:     types.SKUTimeSpan,
	}
	priceRes1, err := priceStore.Create(ctx, price1)
	require.Nil(t, err)

	// add user balance
	userStore := database.NewAccountUserStoreWithDB(db)
	acctUser := database.AccountUser{
		UserUUID:    userUUID,
		Balance:     float64(200),
		CashBalance: float64(200),
	}
	err = userStore.Create(ctx, acctUser)
	require.Nil(t, err)

	// create subscription
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

	// case - list
	listRes, err := subStore.List(ctx, &types.SubscriptionListReq{
		CurrentUser:   userName,
		UserUUID:      userUUID,
		Status:        types.SubscriptionStatusActive,
		SkuType:       reqSkuType,
		QueryUserUUID: userUUID,
		Per:           10,
		Page:          1,
		StartTime:     time.Now().AddDate(0, 0, -1).Format("2006-01-02T15:04:05Z"),
		EndTime:       time.Now().AddDate(0, 0, 1).Format("2006-01-02T15:04:05Z"),
	})
	require.Nil(t, err)
	require.Equal(t, 1, len(listRes.Data))
	require.Equal(t, 1, listRes.Total)

	// case - get by id
	resSub, err := subStore.GetByID(ctx, sub.ID)
	require.Nil(t, err)
	require.Equal(t, sub.ID, resSub.ID)

	// case - status by user uuid
	_, err = subStore.StatusByUserUUID(ctx, userUUID, reqSkuType)
	require.Nil(t, err)
	require.Equal(t, sub.Status, types.SubscriptionStatusActive)
}

func TestAccountSubscriptionStore_Renew(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	reqSkuType := types.SKUCSGHub
	reqSkuUnitType := types.UnitMonth
	userName := "user1"
	resourceID1 := "resource1"
	userUUID := "user-uuid"

	// add price
	priceStore := database.NewAccountPriceStoreWithDB(db)
	price1 := database.AccountPrice{
		SkuType:     reqSkuType,
		SkuPrice:    int64(100),
		SkuUnit:     int64(1),
		ResourceID:  resourceID1,
		SkuUnitType: reqSkuUnitType,
		SkuKind:     types.SKUTimeSpan,
	}
	priceRes1, err := priceStore.Create(ctx, price1)
	require.Nil(t, err)

	// add user balance
	userStore := database.NewAccountUserStoreWithDB(db)
	acctUser := database.AccountUser{
		UserUUID:    userUUID,
		Balance:     float64(200),
		CashBalance: float64(200),
	}
	err = userStore.Create(ctx, acctUser)
	require.Nil(t, err)

	// create subscription
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

	// add a expired sub
	useruuid2 := "user-uuid-2"
	expiredSub := &database.AccountSubscription{
		UserUUID:        useruuid2,
		SkuType:         reqSkuType,
		PriceID:         priceRes1.ID,
		ResourceID:      resourceID1,
		NextPriceID:     priceRes1.ID,
		NextResourceID:  resourceID1,
		StartAt:         time.Now().AddDate(0, -1, 0),
		Status:          types.SubscriptionStatusActive,
		AmountPaidCount: 1,
		LastBillID:      2,
		LastPeriodStart: time.Now().AddDate(0, -1, 0),
		LastPeriodEnd:   time.Now().AddDate(0, 0, -1),
	}
	res, err := db.Core.NewInsert().Model(expiredSub).Exec(ctx, expiredSub)
	require.Nil(t, err)
	c, err := res.RowsAffected()
	require.Nil(t, err)
	require.Equal(t, int64(1), c)

	// case - list renews
	listRes, err := subStore.ListRenews(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(listRes))

	// add user balance for renew
	acctUser2 := database.AccountUser{
		UserUUID:    useruuid2,
		Balance:     float64(200),
		CashBalance: float64(200),
	}
	err = userStore.Create(ctx, acctUser2)
	require.Nil(t, err)

	// case renew success
	newEventUUID := uuid.New()
	err = subStore.Renew(ctx, expiredSub, newEventUUID)
	require.Nil(t, err)
	require.Equal(t, expiredSub.PriceID, expiredSub.NextPriceID)
	require.Equal(t, expiredSub.Status, types.SubscriptionStatusActive)
	require.Greater(t, expiredSub.LastPeriodEnd.Unix(), time.Now().Unix())
	require.Equal(t, expiredSub.AmountPaidCount, int64(2))

	// update user balance for cancel
	acctUser2.Balance = float64(0)
	acctUser2.CashBalance = float64(0)
	res, err = db.Core.NewUpdate().Model(&acctUser2).Where("user_uuid = ?", useruuid2).Exec(ctx)
	require.Nil(t, err)
	c, err = res.RowsAffected()
	require.Nil(t, err)
	require.Equal(t, int64(1), c)

	// update sub to renew again
	expiredSub.LastPeriodEnd = time.Now().AddDate(0, 0, -1)
	res, err = db.Core.NewUpdate().Model(expiredSub).WherePK().Exec(ctx)
	require.Nil(t, err)
	c, err = res.RowsAffected()
	require.Nil(t, err)
	require.Equal(t, int64(1), c)

	// case cancel sub due to not enough balance
	newEventUUID = uuid.New()
	err = subStore.Renew(ctx, expiredSub, newEventUUID)
	require.Nil(t, err)
	require.Equal(t, expiredSub.PriceID, expiredSub.NextPriceID)
	require.Equal(t, types.SubscriptionStatusCanceled, expiredSub.Status)
}

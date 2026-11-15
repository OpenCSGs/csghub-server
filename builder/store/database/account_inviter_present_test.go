package database_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestInviterGiftIsAtomicAndIdempotent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	user := uuid.NewString()
	store := database.NewAccountPresentStoreWithDB(db)
	now := time.Now()
	gift := newNewUserPresentGift(user, types.InviterCredit.ID, 1000, now, now.Add(90*24*time.Hour))
	gift.Present.ParticipantUUID = uuid.NewString()
	gift.Present.Value = 8000
	gift.Vouchers = gift.Vouchers[1:]
	gift.Vouchers[0].Total = 8000
	awarded, err := store.SendInviterPresent(ctx, gift)
	require.NoError(t, err)
	require.True(t, awarded)
	awarded, err = store.SendInviterPresent(ctx, gift)
	require.NoError(t, err)
	require.False(t, awarded)
	var vouchers []database.AccountVoucher
	require.NoError(t, db.Core.NewSelect().Model(&vouchers).Where("target_uuid = ?", user).Scan(ctx))
	require.Len(t, vouchers, 1)
	require.Equal(t, types.VoucherTypeToken, vouchers[0].VoucherType)
	require.Equal(t, float64(8000), vouchers[0].Total)
	var account database.AccountUser
	require.NoError(t, db.Core.NewSelect().Model(&account).Where("user_uuid = ?", user).Scan(ctx))
	require.Zero(t, account.Balance)
	require.Zero(t, account.CashBalance)
	// the grant is recorded as a zero-amount audit statement carrying the
	// voucher face value, and the duplicate grant adds none
	var statement database.AccountStatement
	require.NoError(t, db.Core.NewSelect().Model(&statement).Where("user_uuid = ?", user).Scan(ctx))
	require.Equal(t, gift.Present.EventUUID, statement.EventUUID)
	require.Equal(t, float64(0), statement.Value)
	require.Equal(t, float64(8000), statement.EventValue)
	require.Equal(t, types.ScenePortalCharge, statement.Scene)
	var sub database.AccountSubscription
	require.NoError(t, db.Core.NewSelect().Model(&sub).Where("user_uuid = ? AND sku_type = ?", user, types.SKUCSGLite).Scan(ctx))
	initialEnd := sub.LastPeriodEnd
	gift.Present.EventUUID = uuid.New()
	gift.Present.ParticipantUUID = uuid.NewString()
	for i := range gift.Vouchers {
		gift.Vouchers[i].ID = 0
		gift.Vouchers[i].BatchNo = gift.Present.EventUUID.String()
	}
	awarded, err = store.SendInviterPresent(ctx, gift)
	require.NoError(t, err)
	require.True(t, awarded)
	require.NoError(t, db.Core.NewSelect().Model(&sub).WherePK().Scan(ctx))
	require.WithinDuration(t, initialEnd.AddDate(0, 1, 0), sub.LastPeriodEnd, 24*time.Hour)

	_, err = db.Core.NewUpdate().Model(&sub).Set("status = ?", types.SubscriptionStatusClosed).WherePK().Exec(ctx)
	require.NoError(t, err)
	gift.Present.EventUUID = uuid.New()
	gift.Present.ParticipantUUID = uuid.NewString()
	for i := range gift.Vouchers {
		gift.Vouchers[i].ID = 0
		gift.Vouchers[i].BatchNo = gift.Present.EventUUID.String()
	}
	awarded, err = store.SendInviterPresent(ctx, gift)
	require.NoError(t, err)
	require.True(t, awarded)
	require.NoError(t, db.Core.NewSelect().Model(&sub).WherePK().Scan(ctx))
	require.Equal(t, types.SubscriptionStatusActive, sub.Status)
	require.WithinDuration(t, time.Now().AddDate(0, 1, 0), sub.LastPeriodEnd, 24*time.Hour)
	var license database.License
	require.NoError(t, db.Core.NewSelect().Model(&license).Where("key = ?", fmt.Sprintf("subscription:%d:%d", types.SKUCSGLite, sub.ID)).Scan(ctx))
	require.WithinDuration(t, sub.LastPeriodEnd, license.ExpireTime, time.Second)
}

func TestInviterGiftRollsBackWhenSubscriptionFails(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	user := uuid.NewString()
	now := time.Now()
	gift := newNewUserPresentGift(user, types.InviterCredit.ID, 1000, now, now.Add(90*24*time.Hour))
	gift.Present.ParticipantUUID = uuid.NewString()
	gift.PlanResourceID = "missing-plan"
	_, err := database.NewAccountPresentStoreWithDB(db).SendInviterPresent(ctx, gift)
	require.Error(t, err)
	count, err := db.Core.NewSelect().Model((*database.AccountPresent)(nil)).Where("user_uuid = ?", user).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
	count, err = db.Core.NewSelect().Model((*database.AccountVoucher)(nil)).Where("target_uuid = ?", user).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
	count, err = db.Core.NewSelect().Model((*database.AccountUser)(nil)).Where("user_uuid = ?", user).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestInviterGiftSkipsExistingPaidSubscription(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	user := uuid.NewString()
	_, err := db.Core.NewInsert().Model(&database.AccountUser{UserUUID: user, CashBalance: 10000}).Exec(ctx)
	require.NoError(t, err)
	price, err := database.NewAccountPriceStoreWithDB(db).Create(ctx, database.AccountPrice{
		SkuType: types.SKUCSGLite, SkuPrice: 3900, SkuUnit: 1, ResourceID: uuid.NewString(),
		SkuUnitType: types.UnitMonth, SkuKind: types.SKUTimeSpan, SkuStatus: types.SkuStatusEnabled, Discount: 1,
	})
	require.NoError(t, err)
	license := &types.SubscriptionLicenseTemplate{Company: "customer", Product: types.LicenseProductCSGLite, Edition: "Enterprise", MaxUser: 1}
	sub, err := database.NewAccountSubscriptionWithDB(db).CreateOrUpdate(ctx, &types.SubscriptionUpdateReq{
		UserUUID: user, OpUserUUID: user, SkuType: types.SKUCSGLite, ResourceID: price.ResourceID,
		SkuUnitType: types.UnitMonth, EventUUID: uuid.New(), License: license,
	})
	require.NoError(t, err)
	end := sub.LastPeriodEnd
	var preGiftAccount database.AccountUser
	require.NoError(t, db.Core.NewSelect().Model(&preGiftAccount).Where("user_uuid = ?", user).Scan(ctx))
	now := time.Now()
	gift := newNewUserPresentGift(user, types.InviterCredit.ID, 1000, now, now.Add(90*24*time.Hour))
	gift.Present.ParticipantUUID = uuid.NewString()
	awarded, err := database.NewAccountPresentStoreWithDB(db).SendInviterPresent(ctx, gift)
	require.NoError(t, err)
	require.True(t, awarded)
	var account database.AccountUser
	require.NoError(t, db.Core.NewSelect().Model(&account).Where("user_uuid = ?", user).Scan(ctx))
	require.Equal(t, preGiftAccount.CashBalance, account.CashBalance)
	require.NoError(t, db.Core.NewSelect().Model(sub).WherePK().Scan(ctx))
	require.WithinDuration(t, end, sub.LastPeriodEnd, time.Second)
	count, err := db.Core.NewSelect().Model((*database.AccountVoucher)(nil)).Where("target_uuid = ?", user).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	_, err = db.Core.NewUpdate().Model((*database.AccountSubscriptionBill)(nil)).Set("status = ?", types.BillingStatusUnpaid).Where("id = ?", sub.LastBillID).Exec(ctx)
	require.NoError(t, err)
	gift.Present.EventUUID = uuid.New()
	gift.Present.ParticipantUUID = uuid.NewString()
	for i := range gift.Vouchers {
		gift.Vouchers[i].ID = 0
		gift.Vouchers[i].BatchNo = gift.Present.EventUUID.String()
	}
	awarded, err = database.NewAccountPresentStoreWithDB(db).SendInviterPresent(ctx, gift)
	require.NoError(t, err)
	require.True(t, awarded)
	require.NoError(t, db.Core.NewSelect().Model(sub).WherePK().Scan(ctx))
	require.WithinDuration(t, end.AddDate(0, 1, 0), sub.LastPeriodEnd, 24*time.Hour)
	var giftBill database.AccountSubscriptionBill
	require.NoError(t, db.Core.NewSelect().Model(&giftBill).Where("id = ?", sub.LastBillID).Scan(ctx))
	require.Equal(t, types.BillingStatusPaid, giftBill.Status)
	require.Zero(t, giftBill.AmountPaid)

	_, err = db.Core.NewUpdate().Model(sub).Set("status = ?", types.SubscriptionStatusExpired).WherePK().Exec(ctx)
	require.NoError(t, err)
	gift.Present.EventUUID = uuid.New()
	gift.Present.ParticipantUUID = uuid.NewString()
	for i := range gift.Vouchers {
		gift.Vouchers[i].ID = 0
		gift.Vouchers[i].BatchNo = gift.Present.EventUUID.String()
	}
	awarded, err = database.NewAccountPresentStoreWithDB(db).SendInviterPresent(ctx, gift)
	require.NoError(t, err)
	require.True(t, awarded)
	require.NoError(t, db.Core.NewSelect().Model(sub).WherePK().Scan(ctx))
	require.Equal(t, types.SubscriptionStatusActive, sub.Status)
	require.WithinDuration(t, now.AddDate(0, 1, 0), sub.LastPeriodEnd, 24*time.Hour)
}

func TestInviterGiftConcurrentRetry(t *testing.T) {
	setup := tests.InitTestDB()
	defer setup.Close()
	ctx := context.Background()
	db, err := database.NewDB(ctx, database.DBConfig{Dialect: database.DialectPostgres, DSN: tests.TestDBDSN() + "sslmode=disable"})
	require.NoError(t, err)
	defer db.Close()
	user := uuid.NewString()
	defer func() {
		for _, table := range []string{"account_vouchers", "licenses", "account_subscription_bills", "account_subscriptions", "account_statements", "account_presents", "account_users"} {
			column := "user_uuid"
			if table == "account_vouchers" {
				column = "target_uuid"
			}
			_, cleanupErr := db.Core.NewDelete().TableExpr(table).Where(column+" = ?", user).Exec(ctx)
			require.NoError(t, cleanupErr)
		}
	}()
	_, err = db.Core.NewInsert().Model(&database.AccountUser{UserUUID: user}).Exec(ctx)
	require.NoError(t, err)
	now := time.Now()
	base := newNewUserPresentGift(user, types.InviterCredit.ID, 1000, now, now.Add(90*24*time.Hour))
	base.Present.ParticipantUUID = uuid.NewString()
	store := database.NewAccountPresentStoreWithDB(db)
	results := make(chan bool, 4)
	errs := make(chan error, 4)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gift := base
			gift.Present.EventUUID = uuid.New()
			gift.Vouchers = append([]database.AccountVoucher(nil), base.Vouchers...)
			for i := range gift.Vouchers {
				gift.Vouchers[i].BatchNo = gift.Present.EventUUID.String()
			}
			awarded, err := store.SendInviterPresent(ctx, gift)
			results <- awarded
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	count := 0
	for awarded := range results {
		if awarded {
			count++
		}
	}
	require.Equal(t, 1, count)
	count, err = db.Core.NewSelect().Model((*database.AccountVoucher)(nil)).Where("target_uuid = ?", user).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

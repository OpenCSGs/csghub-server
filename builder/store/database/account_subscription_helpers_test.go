package database_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestCSGLiteSubscriptionPeriodsAndCoexistence(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	prices := database.NewAccountPriceStoreWithDB(db)
	accounts := database.NewAccountUserStoreWithDB(db)
	subs := database.NewAccountSubscriptionWithDB(db)
	license := &types.SubscriptionLicenseTemplate{Company: "customer", Product: types.LicenseProductCSGLite, Edition: "Enterprise", MaxUser: 1}
	for _, months := range []int64{1, 3, 12} {
		t.Run(fmt.Sprintf("%d months", months), func(t *testing.T) {
			user := uuid.NewString()
			require.NoError(t, accounts.Create(ctx, database.AccountUser{UserUUID: user, CashBalance: 50000}))
			var previousID int64
			for _, skuType := range []types.SKUType{types.SKUCSGHub, types.SKUCSGLite} {
				price, err := prices.Create(ctx, database.AccountPrice{SkuType: skuType, SkuPrice: 1000, SkuUnit: months, ResourceID: uuid.NewString(), SkuUnitType: types.UnitMonth, SkuKind: types.SKUTimeSpan, Discount: 1})
				require.NoError(t, err)
				req := &types.SubscriptionUpdateReq{UserUUID: user, OpUserUUID: user, SkuType: skuType, ResourceID: price.ResourceID, SkuUnitType: types.UnitMonth, EventUUID: uuid.New()}
				if skuType == types.SKUCSGLite {
					req.License = license
				}
				sub, err := subs.CreateOrUpdate(ctx, req)
				require.NoError(t, err)
				require.NotEqual(t, previousID, sub.ID)
				previousID = sub.ID
				require.WithinDuration(t, sub.LastPeriodStart.AddDate(0, int(months), 0), sub.LastPeriodEnd, time.Second)
				req.EventUUID = uuid.New()
				repeated, err := subs.CreateOrUpdate(ctx, req)
				require.NoError(t, err)
				require.Equal(t, sub.LastBillID, repeated.LastBillID)
			}
			account, err := accounts.FindUserByID(ctx, user)
			require.NoError(t, err)
			require.Equal(t, float64(48000), account.CashBalance)
			closed, err := subs.CreateOrUpdate(ctx, &types.SubscriptionUpdateReq{UserUUID: user, SkuType: types.SKUCSGLite, ResourceID: types.SubscriptionFree, SkuUnitType: types.UnitMonth, EventUUID: uuid.New(), License: license})
			require.NoError(t, err)
			require.Equal(t, types.SubscriptionStatusClosed, closed.Status)
			stale := *closed
			stale.Status = types.SubscriptionStatusActive
			stale.LastPeriodEnd = time.Now().Add(-time.Hour)
			require.ErrorIs(t, subs.Renew(ctx, &stale, types.SubscriptionRenewReq{EventUUID: uuid.New(), License: license}), database.ErrSubscriptionRenewalSkipped)
			require.Equal(t, types.SubscriptionStatusClosed, stale.Status)
			hub, err := subs.StatusByUserUUID(ctx, user, types.SKUCSGHub)
			require.NoError(t, err)
			require.Equal(t, types.SubscriptionStatusActive, hub.Status)
		})
	}
}

func TestCSGLiteSubscriptionInsufficientBalance(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	user := uuid.NewString()
	price, err := database.NewAccountPriceStoreWithDB(db).Create(ctx, database.AccountPrice{SkuType: types.SKUCSGLite, SkuPrice: 5000, SkuUnit: 1, ResourceID: uuid.NewString(), SkuUnitType: types.UnitMonth, SkuKind: types.SKUTimeSpan, Discount: 1})
	require.NoError(t, err)
	require.NoError(t, database.NewAccountUserStoreWithDB(db).Create(ctx, database.AccountUser{UserUUID: user, CashBalance: 100}))
	subs := database.NewAccountSubscriptionWithDB(db)
	_, err = subs.CreateOrUpdate(ctx, &types.SubscriptionUpdateReq{UserUUID: user, OpUserUUID: user, SkuType: types.SKUCSGLite, ResourceID: price.ResourceID, SkuUnitType: types.UnitMonth, EventUUID: uuid.New()})
	require.ErrorIs(t, err, errorx.ErrInsufficientBalance)
	require.NotErrorIs(t, err, errorx.ErrDatabaseFailure)
	customErr, ok := errorx.GetFirstCustomError(err)
	require.True(t, ok)
	require.Equal(t, "ACT-ERR-0", customErr.(errorx.CustomError).Code())
	var count int
	require.NoError(t, db.Core.NewRaw("SELECT count(*) FROM account_subscriptions WHERE user_uuid = ?", user).Scan(ctx, &count))
	require.Zero(t, count)
}

func TestCSGLiteSubscriptionAdminRepricingWithDefaultDiscount(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	prices := database.NewAccountPriceStoreWithDB(db)
	resourceID := uuid.NewString()
	old, err := prices.Create(ctx, database.AccountPrice{
		SkuType: types.SKUCSGLite, ResourceID: resourceID, SkuKind: types.SKUTimeSpan,
		SkuPrice: 3900, SkuUnit: 1, SkuUnitType: types.UnitMonth, Discount: 1,
	})
	require.NoError(t, err)
	updated, err := prices.Create(ctx, database.AccountPrice{
		SkuType: types.SKUCSGLite, ResourceID: resourceID, SkuKind: types.SKUTimeSpan,
		SkuPrice: 4500, SkuUnit: 2, SkuUnitType: types.UnitMonth,
	})
	require.NoError(t, err)
	disabled, err := prices.GetByID(ctx, old.ID)
	require.NoError(t, err)
	require.Equal(t, types.SkuStatusDisabled, disabled.SkuStatus)
	user := uuid.NewString()
	accounts := database.NewAccountUserStoreWithDB(db)
	require.NoError(t, accounts.Create(ctx, database.AccountUser{UserUUID: user, CashBalance: 10000}))
	sub, err := database.NewAccountSubscriptionWithDB(db).CreateOrUpdate(ctx, &types.SubscriptionUpdateReq{
		UserUUID: user, SkuType: types.SKUCSGLite, ResourceID: resourceID, SkuUnitType: types.UnitMonth, EventUUID: uuid.New(),
	})
	require.NoError(t, err)
	require.Equal(t, updated.ID, sub.PriceID)
	months := (sub.LastPeriodEnd.Year()-sub.LastPeriodStart.Year())*12 + int(sub.LastPeriodEnd.Month()-sub.LastPeriodStart.Month())
	require.Equal(t, 2, months)
	account, err := accounts.FindUserByID(ctx, user)
	require.NoError(t, err)
	require.Equal(t, float64(5500), account.CashBalance)
	bill, err := database.NewAccountSubscriptionBillWithDB(db).GetByID(ctx, sub.LastBillID)
	require.NoError(t, err)
	require.Equal(t, float64(4500), bill.AmountPaid)
	require.Zero(t, bill.Discount)
}

func TestCSGLiteSubscriptionRejectsUnavailablePrices(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	for _, tc := range []struct {
		name          string
		price, months int64
		status        types.SkuStatus
	}{
		{"disabled", 1000, 1, types.SkuStatusDisabled},
		{"zero price", 0, 1, types.SkuStatusEnabled},
		{"negative price", -1000, 1, types.SkuStatusEnabled},
		{"zero period", 1000, 0, types.SkuStatusEnabled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			price, err := database.NewAccountPriceStoreWithDB(db).Create(ctx, database.AccountPrice{
				SkuType: types.SKUCSGLite, ResourceID: uuid.NewString(), SkuKind: types.SKUTimeSpan,
				SkuPrice: tc.price, SkuUnit: tc.months, SkuUnitType: types.UnitMonth, SkuStatus: tc.status,
			})
			require.NoError(t, err)
			user := uuid.NewString()
			accounts := database.NewAccountUserStoreWithDB(db)
			require.NoError(t, accounts.Create(ctx, database.AccountUser{UserUUID: user, CashBalance: 10000}))
			_, err = database.NewAccountSubscriptionWithDB(db).CreateOrUpdate(ctx, &types.SubscriptionUpdateReq{
				UserUUID: user, SkuType: types.SKUCSGLite, ResourceID: price.ResourceID, SkuUnitType: types.UnitMonth, EventUUID: uuid.New(),
			})
			require.Error(t, err)
			account, err := accounts.FindUserByID(ctx, user)
			require.NoError(t, err)
			require.Equal(t, float64(10000), account.CashBalance)
		})
	}
}

func TestCSGLiteSubscriptionRenewal(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	subs := database.NewAccountSubscriptionWithDB(db)
	license := &types.SubscriptionLicenseTemplate{Company: "customer", Product: types.LicenseProductCSGLite, Edition: "Enterprise", MaxUser: 1}
	accounts := database.NewAccountUserStoreWithDB(db)
	price, err := database.NewAccountPriceStoreWithDB(db).Create(ctx, database.AccountPrice{SkuType: types.SKUCSGLite, SkuPrice: 5000, SkuUnit: 3, ResourceID: uuid.NewString(), SkuUnitType: types.UnitMonth, SkuKind: types.SKUTimeSpan, Discount: 0.8})
	require.NoError(t, err)
	for _, balance := range []float64{4000, 8000} {
		user := uuid.NewString()
		require.NoError(t, accounts.Create(ctx, database.AccountUser{UserUUID: user, CashBalance: balance}))
		sub, err := subs.CreateOrUpdate(ctx, &types.SubscriptionUpdateReq{UserUUID: user, SkuType: types.SKUCSGLite, ResourceID: price.ResourceID, SkuUnitType: types.UnitMonth, EventUUID: uuid.New(), License: license})
		require.NoError(t, err)
		require.Equal(t, float64(4000), sub.AmountPaidTotal)
		purchaseBill, err := database.NewAccountSubscriptionBillWithDB(db).GetByID(ctx, sub.LastBillID)
		require.NoError(t, err)
		require.Equal(t, float64(4000), purchaseBill.AmountPaid)
		require.Equal(t, 0.8, purchaseBill.Discount)
		var statement database.AccountStatement
		require.NoError(t, db.Core.NewSelect().Model(&statement).Where("event_uuid = ?", purchaseBill.EventUUID).Scan(ctx))
		require.Equal(t, float64(-4000), statement.Value)
		require.Equal(t, float64(5000), statement.Price)
		require.Equal(t, 0.8, statement.Discount)
		sub.LastPeriodEnd = time.Now().Add(-time.Hour)
		sub.StartAt = sub.LastPeriodEnd.AddDate(0, -3, 0)
		sub.LastPeriodStart = sub.StartAt
		sub, err = subs.Update(ctx, *sub)
		require.NoError(t, err)
		stale := *sub
		require.NoError(t, subs.Renew(ctx, sub, types.SubscriptionRenewReq{EventUUID: uuid.New(), License: license}))
		require.ErrorIs(t, subs.Renew(ctx, &stale, types.SubscriptionRenewReq{EventUUID: uuid.New(), License: license}), database.ErrSubscriptionRenewalSkipped, "a stale renewal must not charge twice")
		account, err := accounts.FindUserByID(ctx, user)
		require.NoError(t, err)
		bill, err := database.NewAccountSubscriptionBillWithDB(db).GetByID(ctx, sub.LastBillID)
		require.NoError(t, err)
		require.Equal(t, float64(4000), bill.AmountPaid)
		if balance == 4000 {
			require.Equal(t, types.SubscriptionStatusCanceled, sub.Status)
			require.Zero(t, account.CashBalance)
		} else {
			require.Equal(t, types.SubscriptionStatusActive, sub.Status)
			require.Equal(t, balance-8000, account.CashBalance)
			require.Equal(t, float64(8000), sub.AmountPaidTotal)
			require.NoError(t, db.Core.NewSelect().Model(&statement).Where("event_uuid = ?", bill.EventUUID).Scan(ctx))
			require.Equal(t, float64(-4000), statement.Value)
			require.Equal(t, 0.8, statement.Discount)
			require.WithinDuration(t, stale.LastPeriodStart.AddDate(0, 3, 0), stale.LastPeriodEnd, time.Second)
		}
	}
}

func TestLicensedSubscriptionConcurrentPurchases(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	for _, tc := range []struct {
		sku     types.SKUType
		product types.LicenseProduct
	}{
		{types.SKUCSGLite, types.LicenseProductCSGLite},
		{types.SKUType(99), "AnotherProduct"},
	} {
		t.Run(string(tc.product), func(t *testing.T) {
			user := uuid.NewString()
			price, err := database.NewAccountPriceStoreWithDB(db).Create(ctx, database.AccountPrice{SkuType: tc.sku, SkuPrice: 5000, SkuUnit: 1, ResourceID: uuid.NewString(), SkuUnitType: types.UnitMonth, SkuKind: types.SKUTimeSpan, Discount: 1})
			require.NoError(t, err)
			require.NoError(t, database.NewAccountUserStoreWithDB(db).Create(ctx, database.AccountUser{UserUUID: user, CashBalance: 50000}))
			subs := database.NewAccountSubscriptionWithDB(db)
			errs := make(chan error, 4)
			var wg sync.WaitGroup
			for i := 0; i < 4; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, err := subs.CreateOrUpdate(ctx, &types.SubscriptionUpdateReq{
						UserUUID: user, SkuType: tc.sku, ResourceID: price.ResourceID, SkuUnitType: types.UnitMonth, EventUUID: uuid.New(),
						License: &types.SubscriptionLicenseTemplate{Company: "customer", Product: tc.product, Edition: "Enterprise", MaxUser: 1},
					})
					errs <- err
				}()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				require.NoError(t, err)
			}
			account, err := database.NewAccountUserStoreWithDB(db).FindUserByID(ctx, user)
			require.NoError(t, err)
			require.Equal(t, float64(45000), account.CashBalance)
			count, err := db.Core.NewSelect().Model((*database.License)(nil)).Where("user_uuid = ?", user).Count(ctx)
			require.NoError(t, err)
			require.Equal(t, 1, count, "concurrent purchases must create one automatic license")
		})
	}
}

func TestCSGLiteSubscriptionMigrationRoundTrip(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	expected := map[string]struct{ price, months int64 }{
		"csglite_sub_bymonth":   {3900, 1},
		"csglite_sub_byquarter": {8900, 3},
		"csglite_sub_byyear":    {33900, 12},
	}
	for _, direction := range []string{"down", "up", "up"} {
		applyCSGLitePricingMigration(t, db, direction)
		prices, err := database.NewAccountPriceStoreWithDB(db).ListBySkuTypeAndKinds(ctx, types.AcctPriceListByKindsReq{
			SkuType: types.SKUCSGLite, SkuKinds: []types.SKUKind{types.SKUTimeSpan},
		})
		require.NoError(t, err)
		if direction == "down" {
			require.Empty(t, prices)
			continue
		}
		require.Len(t, prices, len(expected))
		for _, price := range prices {
			plan, ok := expected[price.ResourceID]
			require.True(t, ok)
			require.Equal(t, plan.price, price.SkuPrice)
			require.Equal(t, plan.months, price.SkuUnit)
			require.Equal(t, types.UnitMonth, price.SkuUnitType)
			require.Equal(t, "CNY", price.SkuPriceCurrency)
			require.Equal(t, 1.0, price.Discount)
		}
	}
}

func TestCSGLitePricingMigrationPreservesConfiguredAndUsedPrices(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	prices := database.NewAccountPriceStoreWithDB(db)
	plans, err := prices.ListBySkuTypeAndKinds(ctx, types.AcctPriceListByKindsReq{
		SkuType: types.SKUCSGLite, SkuKinds: []types.SKUKind{types.SKUTimeSpan}, ResourceID: "csglite_sub_bymonth",
	})
	require.NoError(t, err)
	require.Len(t, plans, 1)
	plan := plans[0]
	plan.SkuPrice = 4000
	_, err = prices.Update(ctx, plan)
	require.NoError(t, err)
	applyCSGLitePricingMigration(t, db, "up")
	configured, err := prices.GetByID(ctx, plan.ID)
	require.NoError(t, err)
	require.Equal(t, int64(4000), configured.SkuPrice)
	plan.SkuPrice = 3900
	_, err = prices.Update(ctx, plan)
	require.NoError(t, err)
	user := uuid.NewString()
	require.NoError(t, database.NewAccountUserStoreWithDB(db).Create(ctx, database.AccountUser{UserUUID: user, CashBalance: 10000}))
	_, err = database.NewAccountSubscriptionWithDB(db).CreateOrUpdate(ctx, &types.SubscriptionUpdateReq{
		UserUUID: user, SkuType: types.SKUCSGLite, ResourceID: plan.ResourceID, SkuUnitType: types.UnitMonth, EventUUID: uuid.New(),
	})
	require.NoError(t, err)
	applyCSGLitePricingMigration(t, db, "down")
	_, err = prices.GetByID(ctx, plan.ID)
	require.NoError(t, err, "rollback must preserve the billed price")
}

func applyCSGLitePricingMigration(t *testing.T, db *database.DB, direction string) {
	t.Helper()
	script, err := os.ReadFile("builder/store/database/migrations/20261027104908_add_csglite_subscription_prices." + direction + ".sql")
	require.NoError(t, err)
	for _, statement := range strings.Split(string(script), "--bun:split") {
		_, err = db.Core.ExecContext(context.Background(), statement)
		require.NoError(t, err)
	}
}

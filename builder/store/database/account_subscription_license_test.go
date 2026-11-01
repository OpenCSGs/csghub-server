package database_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

type subscriptionLicenseFixture struct {
	db    *database.DB
	store database.AccountSubscriptionStore
	req   *types.SubscriptionUpdateReq
}

func newSubscriptionLicenseFixture(t *testing.T, balance float64) subscriptionLicenseFixture {
	t.Helper()
	db := tests.InitTestDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ctx := context.Background()
	user := uuid.NewString()
	_, err := db.Core.NewInsert().Model(&database.User{UUID: user, Username: user, Email: user + "@example.com"}).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, database.NewAccountUserStoreWithDB(db).Create(ctx, database.AccountUser{UserUUID: user, CashBalance: balance}))
	price, err := database.NewAccountPriceStoreWithDB(db).Create(ctx, database.AccountPrice{
		SkuType: types.SKUCSGLite, SkuPrice: 1000, Discount: 1, SkuUnit: 1,
		SkuUnitType: types.UnitMonth, SkuKind: types.SKUTimeSpan, ResourceID: uuid.NewString(),
	})
	require.NoError(t, err)
	return subscriptionLicenseFixture{
		db: db, store: database.NewAccountSubscriptionWithDB(db),
		req: &types.SubscriptionUpdateReq{
			UserUUID: user, SkuType: types.SKUCSGLite, ResourceID: price.ResourceID, SkuUnitType: types.UnitMonth, EventUUID: uuid.New(),
			License: &types.SubscriptionLicenseTemplate{Company: user, Email: user + "@example.com", Product: types.LicenseProductCSGLite, Edition: "Enterprise", MaxUser: 1},
		},
	}
}

func (f subscriptionLicenseFixture) license(t *testing.T, sub *database.AccountSubscription) database.License {
	t.Helper()
	var license database.License
	require.NoError(t, f.db.Core.NewSelect().Model(&license).
		Where("key = ?", fmt.Sprintf("subscription:%d:%d", sub.SkuType, sub.ID)).Scan(context.Background()))
	return license
}

func TestSubscriptionLicensePurchaseAndRepair(t *testing.T) {
	f := newSubscriptionLicenseFixture(t, 5000)
	ctx := context.Background()
	manual := &database.License{Key: uuid.NewString(), UserUUID: f.req.UserUUID, Product: "CSGLite", Edition: "Enterprise", MaxUser: 10,
		Company: "manual", StartTime: time.Now().Add(-time.Hour), ExpireTime: time.Now().AddDate(1, 0, 0)}
	_, err := f.db.Core.NewInsert().Model(manual).Exec(ctx)
	require.NoError(t, err)
	sub, err := f.store.CreateOrUpdate(ctx, f.req)
	require.NoError(t, err)
	license := f.license(t, sub)
	require.Equal(t, f.req.UserUUID, license.UserUUID)
	require.Equal(t, "Enterprise", license.Edition)
	require.Equal(t, "subscription", license.Issuer)
	require.Equal(t, 1, license.MaxUser)
	require.WithinDuration(t, sub.LastPeriodStart, license.StartTime, time.Millisecond)
	require.WithinDuration(t, sub.LastPeriodEnd, license.ExpireTime, time.Millisecond)
	for i := 0; i < 2; i++ {
		f.req.EventUUID = uuid.New()
		repeated, err := f.store.CreateOrUpdate(ctx, f.req)
		require.NoError(t, err)
		require.Equal(t, sub.LastBillID, repeated.LastBillID)
		require.Equal(t, license.ID, f.license(t, sub).ID)
	}
	_, err = f.db.Core.NewDelete().Model(&license).WherePK().ForceDelete().Exec(ctx)
	require.NoError(t, err)
	f.req.EventUUID = uuid.New()
	_, err = f.store.CreateOrUpdate(ctx, f.req)
	require.NoError(t, err)
	repaired := f.license(t, sub)
	require.Equal(t, license.Key, repaired.Key)
	require.NotEqual(t, license.ID, repaired.ID)
	account, err := database.NewAccountUserStoreWithDB(f.db).FindUserByID(ctx, f.req.UserUUID)
	require.NoError(t, err)
	require.Equal(t, float64(4000), account.CashBalance)
	var unchanged database.License
	require.NoError(t, f.db.Core.NewSelect().Model(&unchanged).Where("id = ?", manual.ID).Scan(ctx))
	require.Equal(t, manual.ExpireTime.Unix(), unchanged.ExpireTime.Unix())
	require.Equal(t, 10, unchanged.MaxUser)
}

func TestSubscriptionLicenseProductIsolation(t *testing.T) {
	f := newSubscriptionLicenseFixture(t, 5000)
	ctx := context.Background()
	for _, sku := range []types.SKUType{types.SKUCSGHub, types.SKUType(99)} {
		price, err := database.NewAccountPriceStoreWithDB(f.db).Create(ctx, database.AccountPrice{
			SkuType: sku, SkuPrice: 1000, SkuUnit: 1, SkuUnitType: types.UnitMonth,
			SkuKind: types.SKUTimeSpan, ResourceID: uuid.NewString(),
		})
		require.NoError(t, err)
		req := &types.SubscriptionUpdateReq{UserUUID: f.req.UserUUID, SkuType: sku, ResourceID: price.ResourceID, SkuUnitType: types.UnitMonth, EventUUID: uuid.New()}
		if sku == types.SKUType(99) {
			req.License = &types.SubscriptionLicenseTemplate{Company: "customer", Product: "AnotherProduct", Edition: "Enterprise", MaxUser: 1}
		}
		sub, err := f.store.CreateOrUpdate(ctx, req)
		require.NoError(t, err)
		if req.License != nil {
			license := f.license(t, sub)
			require.Equal(t, "AnotherProduct", license.Product)
			require.Equal(t, fmt.Sprintf("subscription:99:%d", sub.ID), license.Key)
		}
	}
	count, err := f.db.Core.NewSelect().Model((*database.License)(nil)).Where("user_uuid = ?", f.req.UserUUID).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count, "products without a license template must not issue licenses")
}

func TestSubscriptionLicenseRenewalAndCancellation(t *testing.T) {
	for _, balance := range []float64{1000, 5000} {
		t.Run(fmt.Sprint(balance), func(t *testing.T) {
			f := newSubscriptionLicenseFixture(t, balance)
			ctx := context.Background()
			sub, err := f.store.CreateOrUpdate(ctx, f.req)
			require.NoError(t, err)
			original := f.license(t, sub)
			sub.LastPeriodEnd = time.Now().Add(-time.Hour)
			sub.LastPeriodStart = sub.LastPeriodEnd.AddDate(0, -1, 0)
			sub.StartAt = sub.LastPeriodStart
			sub, err = f.store.Update(ctx, *sub)
			require.NoError(t, err)
			stale := *sub
			require.NoError(t, f.store.Renew(ctx, sub, types.SubscriptionRenewReq{EventUUID: uuid.New(), License: f.req.License}))
			license := f.license(t, sub)
			require.Equal(t, original.ID, license.ID)
			if balance == 1000 {
				require.Equal(t, types.SubscriptionStatusCanceled, sub.Status)
				require.Equal(t, original.ExpireTime, license.ExpireTime)
				return
			}
			require.WithinDuration(t, sub.LastPeriodEnd, license.ExpireTime, time.Millisecond)
			require.ErrorIs(t, f.store.Renew(ctx, &stale, types.SubscriptionRenewReq{EventUUID: uuid.New(), License: f.req.License}), database.ErrSubscriptionRenewalSkipped)
			f.req.ResourceID = types.SubscriptionFree
			f.req.EventUUID = uuid.New()
			_, err = f.store.CreateOrUpdate(ctx, f.req)
			require.NoError(t, err)
			require.Equal(t, license.ExpireTime, f.license(t, sub).ExpireTime)
		})
	}
}

func TestSubscriptionLicensePersistenceFailureRollsBack(t *testing.T) {
	for _, renewal := range []bool{false, true} {
		t.Run(fmt.Sprintf("renewal=%t", renewal), func(t *testing.T) {
			f := newSubscriptionLicenseFixture(t, 5000)
			ctx := context.Background()
			var sub *database.AccountSubscription
			var err error
			if renewal {
				sub, err = f.store.CreateOrUpdate(ctx, f.req)
				require.NoError(t, err)
				sub.LastPeriodEnd = time.Now().Add(-time.Hour)
				sub.LastPeriodStart = sub.LastPeriodEnd.AddDate(0, -1, 0)
				sub.StartAt = sub.LastPeriodStart
				sub, err = f.store.Update(ctx, *sub)
				require.NoError(t, err)
			}
			_, err = f.db.Core.ExecContext(ctx, "ALTER TABLE licenses ADD CONSTRAINT reject_subscription_license CHECK (issuer IS DISTINCT FROM 'subscription') NOT VALID")
			require.NoError(t, err)
			if renewal {
				previousBill := sub.LastBillID
				err = f.store.Renew(ctx, sub, types.SubscriptionRenewReq{EventUUID: uuid.New(), License: f.req.License})
				require.Error(t, err)
				persisted, err := f.store.StatusByUserUUID(ctx, f.req.UserUUID, f.req.SkuType)
				require.NoError(t, err)
				require.Equal(t, previousBill, persisted.LastBillID)
			} else {
				_, err = f.store.CreateOrUpdate(ctx, f.req)
				require.Error(t, err)
				count, err := f.db.Core.NewSelect().Model((*database.AccountSubscription)(nil)).Where("user_uuid = ?", f.req.UserUUID).Count(ctx)
				require.NoError(t, err)
				require.Zero(t, count)
			}
			account, err := database.NewAccountUserStoreWithDB(f.db).FindUserByID(ctx, f.req.UserUUID)
			require.NoError(t, err)
			want := float64(5000)
			if renewal {
				want = 4000
			}
			require.Equal(t, want, account.CashBalance)
		})
	}
}

func TestSubscriptionUpgradeInsufficientBalancePreservesPaidState(t *testing.T) {
	f := newSubscriptionLicenseFixture(t, 1000)
	ctx := context.Background()
	sub, err := f.store.CreateOrUpdate(ctx, f.req)
	require.NoError(t, err)
	license := f.license(t, sub)
	price, err := database.NewAccountPriceStoreWithDB(f.db).Create(ctx, database.AccountPrice{
		SkuType: types.SKUCSGLite, SkuPrice: 2000, Discount: 1, SkuUnit: 1,
		SkuUnitType: types.UnitMonth, SkuKind: types.SKUTimeSpan, ResourceID: uuid.NewString(),
	})
	require.NoError(t, err)
	f.req.ResourceID = price.ResourceID
	f.req.EventUUID = uuid.New()
	_, err = f.store.CreateOrUpdate(ctx, f.req)
	require.ErrorIs(t, err, errorx.ErrInsufficientBalance)
	require.NotErrorIs(t, err, errorx.ErrDatabaseFailure)
	persisted, err := f.store.StatusByUserUUID(ctx, f.req.UserUUID, f.req.SkuType)
	require.NoError(t, err)
	require.Equal(t, sub.LastBillID, persisted.LastBillID)
	require.Equal(t, license.ExpireTime, f.license(t, sub).ExpireTime)
	account, err := database.NewAccountUserStoreWithDB(f.db).FindUserByID(ctx, f.req.UserUUID)
	require.NoError(t, err)
	require.Zero(t, account.CashBalance)
}

func TestSubscriptionLicensePlanChangesAndRepurchase(t *testing.T) {
	f := newSubscriptionLicenseFixture(t, 10000)
	ctx := context.Background()
	sub, err := f.store.CreateOrUpdate(ctx, f.req)
	require.NoError(t, err)
	license := f.license(t, sub)
	oldResource := f.req.ResourceID
	price, err := database.NewAccountPriceStoreWithDB(f.db).Create(ctx, database.AccountPrice{
		SkuType: types.SKUCSGLite, SkuPrice: 2000, Discount: 1, SkuUnit: 3,
		SkuUnitType: types.UnitMonth, SkuKind: types.SKUTimeSpan, ResourceID: uuid.NewString(),
	})
	require.NoError(t, err)
	f.req.ResourceID = price.ResourceID
	f.req.EventUUID = uuid.New()
	sub, err = f.store.CreateOrUpdate(ctx, f.req)
	require.NoError(t, err)
	upgraded := f.license(t, sub)
	require.Equal(t, license.ID, upgraded.ID)
	require.WithinDuration(t, sub.LastPeriodEnd, upgraded.ExpireTime, time.Millisecond)
	f.req.ResourceID = oldResource
	f.req.EventUUID = uuid.New()
	sub, err = f.store.CreateOrUpdate(ctx, f.req)
	require.NoError(t, err)
	require.Equal(t, upgraded.ExpireTime, f.license(t, sub).ExpireTime, "scheduled downgrade must not shorten paid validity")
	sub.Status = types.SubscriptionStatusExpired
	sub.LastPeriodEnd = time.Now().Add(-time.Hour)
	_, err = f.store.Update(ctx, *sub)
	require.NoError(t, err)
	f.req.EventUUID = uuid.New()
	sub, err = f.store.CreateOrUpdate(ctx, f.req)
	require.NoError(t, err)
	require.Equal(t, license.ID, f.license(t, sub).ID)
	require.WithinDuration(t, sub.LastPeriodEnd, f.license(t, sub).ExpireTime, time.Millisecond)
}

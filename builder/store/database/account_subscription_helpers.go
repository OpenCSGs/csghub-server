package database

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// subscriptionCharge keeps usage discounts separate from subscription pricing.
// A zero discount is the legacy value for an unspecified discount.
func subscriptionCharge(price *AccountPrice) float64 {
	amount := float64(price.SkuPrice)
	if price.SkuType == types.SKUCSGLite && price.Discount > 0 && price.Discount < 1 {
		return math.Round(amount * price.Discount)
	}
	return amount
}

// lockSubscription takes the per (sku, user) advisory transaction lock that
// serializes every mutation of the user's subscription for that sku. Callers
// must take it before locking account_users: lockSubscriptionPurchase and
// lockSubscriptionRenewal use the same order, and inverting it would deadlock.
func lockSubscription(ctx context.Context, tx bun.Tx, skuType types.SKUType, userUUID string) error {
	key := fmt.Sprintf("subscription:%d:%s", skuType, userUUID)
	_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key)
	return errorx.HandleDBError(err, errorx.Ctx().Set("operation", "lock_subscription"))
}

func lockSubscriptionPurchase(ctx context.Context, tx bun.Tx, req *types.SubscriptionUpdateReq) error {
	if req.License == nil {
		return nil
	}
	return lockSubscription(ctx, tx, req.SkuType, req.UserUUID)
}

func filterSubscriptionPurchasePrice(query *bun.SelectQuery, skuType types.SKUType) *bun.SelectQuery {
	if skuType == types.SKUCSGLite {
		return query.Where("sku_status = ?", types.SkuStatusEnabled).
			Where("sku_price > 0").Where("sku_unit > 0")
	}
	return query
}

func lockSubscriptionRenewal(ctx context.Context, tx bun.Tx, sub *AccountSubscription, license *types.SubscriptionLicenseTemplate) (bool, error) {
	if license == nil {
		return true, nil
	}
	if err := lockSubscriptionPurchase(ctx, tx, &types.SubscriptionUpdateReq{SkuType: sub.SkuType, UserUUID: sub.UserUUID, License: license}); err != nil {
		return false, err
	}
	if err := tx.NewSelect().Model(sub).WherePK().Scan(ctx); err != nil {
		return false, errorx.HandleDBError(err, errorx.Ctx().Set("subscription_id", sub.ID))
	}
	return sub.Status == types.SubscriptionStatusActive && !sub.LastPeriodEnd.After(time.Now()), nil
}

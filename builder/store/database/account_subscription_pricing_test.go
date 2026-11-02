package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
)

func TestSubscriptionCharge(t *testing.T) {
	for _, tc := range []struct {
		name     string
		skuType  types.SKUType
		discount float64
		want     float64
	}{
		{"csglite discount", types.SKUCSGLite, 0.8, 3121},
		{"csglite rounds half fen up", types.SKUCSGLite, 0.5, 1951},
		{"csglite full price", types.SKUCSGLite, 1, 3901},
		{"csglite unspecified discount", types.SKUCSGLite, 0, 3901},
		{"csghub usage discount does not reduce subscription price", types.SKUCSGHub, 0.8, 3901},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, subscriptionCharge(&AccountPrice{SkuType: tc.skuType, SkuPrice: 3901, Discount: tc.discount}))
		})
	}
}

func TestSubscriptionFeeGapUsesProductPricing(t *testing.T) {
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	sub := &AccountSubscription{LastPeriodStart: start, LastPeriodEnd: end}
	store := &accountSubscriptionStoreImpl{}
	for _, tc := range []struct {
		name string
		sku  types.SKUType
		want float64
	}{
		{"csglite uses discounted prices", types.SKUCSGLite, 600},
		{"csghub retains list prices", types.SKUCSGHub, 1000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pre := AccountPrice{SkuType: tc.sku, SkuPrice: 1000, Discount: 0.8}
			post := &AccountPrice{SkuType: tc.sku, SkuPrice: 2000, Discount: 0.7}
			gap, err := store.calculateFeeGap(pre, post, start, sub, end)
			require.NoError(t, err)
			require.Equal(t, tc.want, gap)
		})
	}
	pre := AccountPrice{SkuType: types.SKUCSGLite, SkuPrice: 2000, Discount: 0.5}
	post := &AccountPrice{SkuType: types.SKUCSGLite, SkuPrice: 2000, Discount: 0.8}
	gap, err := store.calculateFeeGap(pre, post, start.Add(end.Sub(start)/2), sub, end)
	require.NoError(t, err)
	require.Equal(t, float64(300), gap, "plan changes compare discounted prices even when list prices match")
}

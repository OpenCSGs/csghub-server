package database_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAccountPriceStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPriceStoreWithDB(db)
	created, err := store.Create(ctx, database.AccountPrice{
		SkuDesc: "sku",
	})
	require.Nil(t, err)
	require.Equal(t, "sku", created.SkuDesc)

	r := &database.AccountPrice{}
	err = db.Core.NewSelect().Model(r).Where("id=?", created.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "sku", r.SkuDesc)

}

func TestAccountPriceStore_Update(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPriceStoreWithDB(db)
	c, err := store.Create(ctx, database.AccountPrice{
		SkuDesc: "sku",
	})
	require.Nil(t, err)

	c.SkuDesc = "new_sku"
	_, err = store.Update(ctx, *c)
	require.Nil(t, err)

	r := &database.AccountPrice{}
	err = db.Core.NewSelect().Model(r).Where("id=?", c.ID).Scan(ctx)
	require.Nil(t, err)
	require.Equal(t, "new_sku", r.SkuDesc)

}

func TestAccountPriceStore_Delete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPriceStoreWithDB(db)
	c, err := store.Create(ctx, database.AccountPrice{
		SkuDesc: "sku",
	})
	require.Nil(t, err)

	err = store.Delete(ctx, *c)
	require.Nil(t, err)

	r := &database.AccountPrice{}
	err = db.Core.NewSelect().Model(r).Where("id=?", c.ID).Scan(ctx)
	require.NotNil(t, err)
}

func TestAccountPriceStore_GetByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPriceStoreWithDB(db)
	c, err := store.Create(ctx, database.AccountPrice{
		SkuDesc: "sku",
	})
	require.Nil(t, err)

	g, err := store.GetByID(ctx, c.ID)
	require.Nil(t, err)
	require.Equal(t, "sku", g.SkuDesc)

}

func TestAccountPriceStore_GetLatestByTime(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPriceStoreWithDB(db)

	prices := []*database.AccountPrice{
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "a", SkuStatus: types.SkuStatusEnabled, Resolution: "low",
		},
		{
			SkuType: types.SKUReserve, SkuKind: types.SKUPackageAddon, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "b", SkuStatus: types.SkuStatusEnabled, Resolution: "low",
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "c", SkuStatus: types.SkuStatusEnabled, Resolution: "low",
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUTimeSpan, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "d", SkuStatus: types.SkuStatusEnabled, Resolution: "low",
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "e", SkuStatus: types.SkuStatusEnabled, Resolution: "low",
		},
	}

	dt := time.Now()
	times := []time.Time{
		dt.Add(-3 * time.Hour), dt.Add(6 * time.Hour),
		dt.Add(2 * time.Hour), dt.Add(1 * time.Hour),
		dt.Add(12 * time.Hour),
	}

	for i, p := range prices {
		p.CreatedAt = times[i]
	}

	for _, p := range prices {
		_, err := db.Core.NewInsert().Model(p).Exec(ctx)
		require.Nil(t, err)
	}

	p, err := store.GetLatestByTime(ctx, types.AcctPriceQueryReq{
		SkuType:     types.SKUCSGHub,
		ResourceID:  "r",
		PriceTime:   dt.Add(8 * time.Hour),
		SkuKind:     types.SKUPackageAddon,
		SkuUnitType: []string{"u"},
	})
	require.Nil(t, err)
	require.Equal(t, "c", p.SkuDesc)

}

func TestAccountPriceStore_ListBySkuType(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPriceStoreWithDB(db)

	prices := []*database.AccountPrice{
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "a", SkuStatus: types.SkuStatusEnabled,
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "c",
			SkuUnitType: "u", SkuDesc: "b", SkuStatus: types.SkuStatusEnabled,
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPayAsYouGo, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "c", SkuStatus: types.SkuStatusEnabled,
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPayAsYouGo, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "d", SkuStatus: types.SkuStatusDisabled,
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPayAsYouGo, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "e", SkuStatus: types.SkuStatusDisabled,
		},
		{
			SkuType: types.SKUReserve, SkuKind: types.SKUPackageAddon, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "f", SkuStatus: types.SkuStatusEnabled,
		},
	}

	namesFunc := func(prices []database.AccountPrice) string {
		names := []string{}
		for _, p := range prices {
			names = append(names, p.SkuDesc)
		}
		return strings.Join(names, "/")
	}

	for _, p := range prices {
		_, err := db.Core.NewInsert().Model(p).Exec(ctx)
		require.Nil(t, err)
	}

	data, count, err := store.ListBySkuType(ctx, types.AcctPriceListDBReq{
		SkuType:   types.SKUCSGHub,
		SkuStatus: types.SkuStatusEnabled,
		Page:      1,
		Per:       10,
	})
	require.Nil(t, err)
	require.Equal(t, 3, count)
	require.Equal(t, 3, len(data))
	require.Equal(t, "c/b/a", namesFunc(data))

	data, count, err = store.ListBySkuType(ctx, types.AcctPriceListDBReq{
		SkuType:   types.SKUCSGHub,
		SkuKind:   types.SKUPackageAddon,
		SkuStatus: types.SkuStatusEnabled,
		Page:      1,
		Per:       10,
	})
	require.Nil(t, err)
	require.Equal(t, 2, count)
	require.Equal(t, 2, len(data))
	require.Equal(t, "b/a", namesFunc(data))

	data, count, err = store.ListBySkuType(ctx, types.AcctPriceListDBReq{
		SkuType:    types.SKUCSGHub,
		SkuKind:    types.SKUPackageAddon,
		ResourceID: []string{"r"},
		SkuStatus:  types.SkuStatusEnabled,
		Page:       1,
		Per:        10,
	})
	require.Nil(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, 1, len(data))
	require.Equal(t, "a", namesFunc(data))

}

func TestAccountPriceStore_ListBySkuType_WithSort(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPriceStoreWithDB(db)

	prices := []*database.AccountPrice{
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r1",
			SkuUnitType: "u", SkuDesc: "a", SkuStatus: types.SkuStatusEnabled,
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r3",
			SkuUnitType: "u", SkuDesc: "b", SkuStatus: types.SkuStatusEnabled,
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r2",
			SkuUnitType: "u", SkuDesc: "c", SkuStatus: types.SkuStatusEnabled,
		},
	}

	for _, p := range prices {
		_, err := store.Create(ctx, *p)
		require.Nil(t, err)
	}

	data, count, err := store.ListBySkuType(ctx, types.AcctPriceListDBReq{
		SkuType:   types.SKUCSGHub,
		Page:      1,
		Per:       10,
		SortBy:    "resource_id",
		SortOrder: "DESC",
	})
	require.Nil(t, err)
	require.Equal(t, 3, count)
	require.Equal(t, 3, len(data))
	require.Equal(t, "b/c/a", func(prices []database.AccountPrice) string {
		names := []string{}
		for _, p := range prices {
			names = append(names, p.SkuDesc)
		}
		return strings.Join(names, "/")
	}(data))
}

func TestAccountPriceStore_ListByIds(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPriceStoreWithDB(db)

	price1 := &database.AccountPrice{
		SkuType:          1,
		SkuPrice:         100,
		SkuUnit:          10,
		SkuDesc:          "sku1",
		ResourceID:       "resource1",
		SkuUnitType:      "unit1",
		SkuPriceCurrency: "USD",
		SkuKind:          1,
		Quota:            "quota1",
		SkuPriceID:       1,
	}
	price2 := &database.AccountPrice{
		SkuType:          2,
		SkuPrice:         200,
		SkuUnit:          20,
		SkuDesc:          "sku2",
		ResourceID:       "resource2",
		SkuUnitType:      "unit2",
		SkuPriceCurrency: "EUR",
		SkuKind:          2,
		Quota:            "quota2",
		SkuPriceID:       2,
	}

	_, err := db.Core.NewInsert().Model(price1).Exec(ctx)
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(price2).Exec(ctx)
	require.Nil(t, err)

	ids := []int64{price1.ID, price2.ID}
	result, err := store.ListByIds(ctx, ids)
	require.Nil(t, err)

	require.Equal(t, 2, len(result))

	for _, price := range result {
		if price.Id == price1.ID {
			require.Equal(t, price1.SkuType, price.SkuType)
			require.Equal(t, price1.SkuKind, price.SkuKind)
		} else if price.Id == price2.ID {
			require.Equal(t, price2.SkuType, price.SkuType)
			require.Equal(t, price2.SkuKind, price.SkuKind)
		}
	}
}

func TestAccountPriceStore_BatchCreate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPriceStoreWithDB(db)

	t.Run("disable old enabled prices and keep latest", func(t *testing.T) {
		old1, err := store.Create(ctx, database.AccountPrice{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r1",
			SkuDesc: "old_1", SkuStatus: types.SkuStatusEnabled,
		})
		require.Nil(t, err)

		old2, err := store.Create(ctx, database.AccountPrice{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r1",
			SkuDesc: "old_2", SkuStatus: types.SkuStatusEnabled,
		})
		require.Nil(t, err)

		old3, err := store.Create(ctx, database.AccountPrice{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r2",
			SkuDesc: "old_3", SkuStatus: types.SkuStatusEnabled,
		})
		require.Nil(t, err)

		prices := []database.AccountPrice{
			{
				SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r1",
				SkuDesc: "new_1", SkuStatus: types.SkuStatusEnabled,
			},
			{
				SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r2",
				SkuDesc: "new_2", SkuStatus: types.SkuStatusEnabled,
			},
		}

		result, err := store.BatchCreate(ctx, prices)
		require.Nil(t, err)
		require.Equal(t, 2, len(result))
		require.Equal(t, "new_1", result[0].SkuDesc)
		require.Equal(t, "new_2", result[1].SkuDesc)

		r1 := &database.AccountPrice{}
		err = db.Core.NewSelect().Model(r1).Where("id=?", old1.ID).Scan(ctx)
		require.Nil(t, err)
		require.Equal(t, types.SkuStatusDisabled, r1.SkuStatus)

		r2 := &database.AccountPrice{}
		err = db.Core.NewSelect().Model(r2).Where("id=?", old2.ID).Scan(ctx)
		require.Nil(t, err)
		require.Equal(t, types.SkuStatusDisabled, r2.SkuStatus)

		r3 := &database.AccountPrice{}
		err = db.Core.NewSelect().Model(r3).Where("id=?", old3.ID).Scan(ctx)
		require.Nil(t, err)
		require.Equal(t, types.SkuStatusDisabled, r3.SkuStatus)
	})

	t.Run("no existing enabled prices", func(t *testing.T) {
		prices := []database.AccountPrice{
			{
				SkuType: types.SKUReserve, SkuKind: types.SKUPayAsYouGo, ResourceID: "r3",
				SkuDesc: "fresh_1", SkuStatus: types.SkuStatusEnabled,
			},
		}

		result, err := store.BatchCreate(ctx, prices)
		require.Nil(t, err)
		require.Equal(t, 1, len(result))
		require.Equal(t, "fresh_1", result[0].SkuDesc)
	})
}

func TestAccountPriceStore_BatchCreate_Empty(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPriceStoreWithDB(db)

	result, err := store.BatchCreate(ctx, nil)
	require.Nil(t, err)
	require.Nil(t, result)
}

func TestAccountPriceStore_ListBySkuTypeAndKinds(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPriceStoreWithDB(db)

	prices := []*database.AccountPrice{
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r1",
			SkuUnitType: "u", SkuDesc: "a", SkuStatus: types.SkuStatusEnabled,
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r2",
			SkuUnitType: "u", SkuDesc: "b", SkuStatus: types.SkuStatusEnabled,
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPayAsYouGo, ResourceID: "r1",
			SkuUnitType: "u", SkuDesc: "c", SkuStatus: types.SkuStatusEnabled,
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPayAsYouGo, ResourceID: "r2",
			SkuUnitType: "u", SkuDesc: "d", SkuStatus: types.SkuStatusEnabled,
		},
		{
			SkuType: types.SKUReserve, SkuKind: types.SKUPackageAddon, ResourceID: "r1",
			SkuUnitType: "u", SkuDesc: "e", SkuStatus: types.SkuStatusEnabled,
		},
	}

	for _, p := range prices {
		_, err := store.Create(ctx, *p)
		require.Nil(t, err)
	}

	namesFunc := func(prices []database.AccountPrice) string {
		names := []string{}
		for _, p := range prices {
			names = append(names, p.SkuDesc)
		}
		return strings.Join(names, "/")
	}

	t.Run("list by sku_type and multiple sku_kinds", func(t *testing.T) {
		result, err := store.ListBySkuTypeAndKinds(ctx, types.AcctPriceListByKindsReq{
			SkuType:  types.SKUCSGHub,
			SkuKinds: []types.SKUKind{types.SKUPackageAddon, types.SKUPayAsYouGo},
		})
		require.Nil(t, err)
		require.Equal(t, 4, len(result))
		require.Equal(t, "c/a/d/b", namesFunc(result))
	})

	t.Run("list by sku_type and single sku_kind", func(t *testing.T) {
		result, err := store.ListBySkuTypeAndKinds(ctx, types.AcctPriceListByKindsReq{
			SkuType:  types.SKUCSGHub,
			SkuKinds: []types.SKUKind{types.SKUPackageAddon},
		})
		require.Nil(t, err)
		require.Equal(t, 2, len(result))
		require.Equal(t, "a/b", namesFunc(result))
	})

	t.Run("list with resource_id filter", func(t *testing.T) {
		result, err := store.ListBySkuTypeAndKinds(ctx, types.AcctPriceListByKindsReq{
			SkuType:    types.SKUCSGHub,
			SkuKinds:   []types.SKUKind{types.SKUPackageAddon, types.SKUPayAsYouGo},
			ResourceID: "r1",
		})
		require.Nil(t, err)
		require.Equal(t, 2, len(result))
		require.Equal(t, "c/a", namesFunc(result))
	})

	t.Run("list by different sku_type", func(t *testing.T) {
		result, err := store.ListBySkuTypeAndKinds(ctx, types.AcctPriceListByKindsReq{
			SkuType:  types.SKUReserve,
			SkuKinds: []types.SKUKind{types.SKUPackageAddon},
		})
		require.Nil(t, err)
		require.Equal(t, 1, len(result))
		require.Equal(t, "e", namesFunc(result))
	})

	t.Run("list with non-existent resource_id", func(t *testing.T) {
		result, err := store.ListBySkuTypeAndKinds(ctx, types.AcctPriceListByKindsReq{
			SkuType:    types.SKUCSGHub,
			SkuKinds:   []types.SKUKind{types.SKUPackageAddon},
			ResourceID: "non_existent",
		})
		require.Nil(t, err)
		require.Equal(t, 0, len(result))
	})
}

func TestAccountPriceStore_OffLineBySkuTypeAndResourceID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPriceStoreWithDB(db)

	t.Run("disable enabled prices matching sku_type and resource_id", func(t *testing.T) {
		p1, err := store.Create(ctx, database.AccountPrice{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPayAsYouGo, ResourceID: "r1",
			SkuDesc: "price_1", SkuStatus: types.SkuStatusEnabled,
		})
		require.Nil(t, err)

		p2, err := store.Create(ctx, database.AccountPrice{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPromptToken, ResourceID: "r1",
			SkuDesc: "price_2", SkuStatus: types.SkuStatusEnabled,
		})
		require.Nil(t, err)

		err = store.OffLineBySkuTypeAndResourceID(ctx, types.AcctPriceOffLineReq{
			SkuType:    types.SKUCSGHub,
			ResourceID: "r1",
		})
		require.Nil(t, err)

		r1 := &database.AccountPrice{}
		err = db.Core.NewSelect().Model(r1).Where("id=?", p1.ID).Scan(ctx)
		require.Nil(t, err)
		require.Equal(t, types.SkuStatusDisabled, r1.SkuStatus)

		r2 := &database.AccountPrice{}
		err = db.Core.NewSelect().Model(r2).Where("id=?", p2.ID).Scan(ctx)
		require.Nil(t, err)
		require.Equal(t, types.SkuStatusDisabled, r2.SkuStatus)
	})

	t.Run("do not affect prices with different sku_type", func(t *testing.T) {
		p1, err := store.Create(ctx, database.AccountPrice{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPayAsYouGo, ResourceID: "r2",
			SkuDesc: "csghub_price", SkuStatus: types.SkuStatusEnabled,
		})
		require.Nil(t, err)

		p2, err := store.Create(ctx, database.AccountPrice{
			SkuType: types.SKUReserve, SkuKind: types.SKUPayAsYouGo, ResourceID: "r2",
			SkuDesc: "reserve_price", SkuStatus: types.SkuStatusEnabled,
		})
		require.Nil(t, err)

		err = store.OffLineBySkuTypeAndResourceID(ctx, types.AcctPriceOffLineReq{
			SkuType:    types.SKUCSGHub,
			ResourceID: "r2",
		})
		require.Nil(t, err)

		r1 := &database.AccountPrice{}
		err = db.Core.NewSelect().Model(r1).Where("id=?", p1.ID).Scan(ctx)
		require.Nil(t, err)
		require.Equal(t, types.SkuStatusDisabled, r1.SkuStatus)

		r2 := &database.AccountPrice{}
		err = db.Core.NewSelect().Model(r2).Where("id=?", p2.ID).Scan(ctx)
		require.Nil(t, err)
		require.Equal(t, types.SkuStatusEnabled, r2.SkuStatus)
	})

	t.Run("do not affect prices with different resource_id", func(t *testing.T) {
		p1, err := store.Create(ctx, database.AccountPrice{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPayAsYouGo, ResourceID: "r3",
			SkuDesc: "target_price", SkuStatus: types.SkuStatusEnabled,
		})
		require.Nil(t, err)

		p2, err := store.Create(ctx, database.AccountPrice{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPayAsYouGo, ResourceID: "r4",
			SkuDesc: "other_price", SkuStatus: types.SkuStatusEnabled,
		})
		require.Nil(t, err)

		err = store.OffLineBySkuTypeAndResourceID(ctx, types.AcctPriceOffLineReq{
			SkuType:    types.SKUCSGHub,
			ResourceID: "r3",
		})
		require.Nil(t, err)

		r1 := &database.AccountPrice{}
		err = db.Core.NewSelect().Model(r1).Where("id=?", p1.ID).Scan(ctx)
		require.Nil(t, err)
		require.Equal(t, types.SkuStatusDisabled, r1.SkuStatus)

		r2 := &database.AccountPrice{}
		err = db.Core.NewSelect().Model(r2).Where("id=?", p2.ID).Scan(ctx)
		require.Nil(t, err)
		require.Equal(t, types.SkuStatusEnabled, r2.SkuStatus)
	})

	t.Run("do not affect already disabled prices", func(t *testing.T) {
		p1, err := store.Create(ctx, database.AccountPrice{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPayAsYouGo, ResourceID: "r5",
			SkuDesc: "enabled_price", SkuStatus: types.SkuStatusEnabled,
		})
		require.Nil(t, err)

		// Manually insert a disabled price (bypassing Create's auto-disable logic)
		p2 := &database.AccountPrice{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPayAsYouGo, ResourceID: "r5",
			SkuDesc: "already_disabled", SkuStatus: types.SkuStatusDisabled,
		}
		_, err = db.Core.NewInsert().Model(p2).Exec(ctx)
		require.Nil(t, err)

		err = store.OffLineBySkuTypeAndResourceID(ctx, types.AcctPriceOffLineReq{
			SkuType:    types.SKUCSGHub,
			ResourceID: "r5",
		})
		require.Nil(t, err)

		r1 := &database.AccountPrice{}
		err = db.Core.NewSelect().Model(r1).Where("id=?", p1.ID).Scan(ctx)
		require.Nil(t, err)
		require.Equal(t, types.SkuStatusDisabled, r1.SkuStatus)

		r2 := &database.AccountPrice{}
		err = db.Core.NewSelect().Model(r2).Where("id=?", p2.ID).Scan(ctx)
		require.Nil(t, err)
		require.Equal(t, types.SkuStatusDisabled, r2.SkuStatus)
	})

	t.Run("no matching enabled prices does not return error", func(t *testing.T) {
		err := store.OffLineBySkuTypeAndResourceID(ctx, types.AcctPriceOffLineReq{
			SkuType:    types.SKUReserve,
			ResourceID: "nonexistent",
		})
		require.Nil(t, err)
	})
}

func TestAccountPriceStore_CountByResourceIDs(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAccountPriceStoreWithDB(db)

	_, err := store.Create(ctx, database.AccountPrice{
		SkuType:    types.SKUReserve,
		SkuPrice:   100,
		SkuUnit:    1,
		SkuDesc:    "test1",
		ResourceID: fmt.Sprintf(types.CSGHubResourceFmt, types.ProviderTypeServerless, "ns1/model-a"),
		SkuKind:    types.SKUPayAsYouGo,
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.AccountPrice{
		SkuType:    types.SKUReserve,
		SkuPrice:   200,
		SkuUnit:    1,
		SkuDesc:    "test2",
		ResourceID: fmt.Sprintf(types.CSGHubResourceFmt, types.ProviderTypeInference, "ns1/model-a"),
		SkuKind:    types.SKUPayAsYouGo,
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.AccountPrice{
		SkuType:    types.SKUReserve,
		SkuPrice:   300,
		SkuUnit:    1,
		SkuDesc:    "test3",
		ResourceID: fmt.Sprintf(types.CSGHubResourceFmt, types.ProviderTypeServerless, "ns2/other"),
		SkuKind:    types.SKUPayAsYouGo,
	})
	require.Nil(t, err)

	// Match both serverless and inference for model-a: 2 prices
	count, err := store.CountByResourceIDs(ctx, []string{
		fmt.Sprintf(types.CSGHubResourceFmt, types.ProviderTypeServerless, "ns1/model-a"),
		fmt.Sprintf(types.CSGHubResourceFmt, types.ProviderTypeInference, "ns1/model-a"),
	})
	require.Nil(t, err)
	require.Equal(t, 2, count)

	// Only serverless for other: 1 price
	count, err = store.CountByResourceIDs(ctx, []string{
		fmt.Sprintf(types.CSGHubResourceFmt, types.ProviderTypeServerless, "ns2/other"),
	})
	require.Nil(t, err)
	require.Equal(t, 1, count)

	// No match
	count, err = store.CountByResourceIDs(ctx, []string{
		fmt.Sprintf(types.CSGHubResourceFmt, types.ProviderTypeServerless, "ns1/nonexistent"),
	})
	require.Nil(t, err)
	require.Equal(t, 0, count)
}

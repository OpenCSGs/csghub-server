package database_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cast"
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
			SkuUnitType: "u", SkuDesc: "a",
		},
		{
			SkuType: types.SKUReserve, SkuKind: types.SKUPackageAddon, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "b",
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "c",
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUTimeSpan, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "d",
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "e",
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
		_, err := store.Create(ctx, *p)
		require.Nil(t, err)
	}

	p, err := store.GetLatestByTime(ctx, types.AcctPriceQueryReq{
		SkuType:     types.SKUCSGHub,
		ResourceID:  "r",
		PriceTime:   dt.Add(8 * time.Hour),
		SkuKind:     types.SKUPackageAddon,
		SkuUnitType: "u",
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
			SkuUnitType: "u", SkuDesc: "a",
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "c",
			SkuUnitType: "u", SkuDesc: "b",
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPayAsYouGo, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "c",
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPayAsYouGo, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "d",
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPayAsYouGo, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "e",
		},
		{
			SkuType: types.SKUReserve, SkuKind: types.SKUPackageAddon, ResourceID: "r",
			SkuUnitType: "u", SkuDesc: "f",
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
		_, err := store.Create(ctx, *p)
		require.Nil(t, err)
	}

	data, count, err := store.ListBySkuType(ctx, types.AcctPriceListDBReq{
		SkuType: types.SKUCSGHub,
		Page:    1,
		Per:     10,
	})
	require.Nil(t, err)
	require.Equal(t, 3, count)
	require.Equal(t, 3, len(data))
	require.Equal(t, "c/b/a", namesFunc(data))

	data, count, err = store.ListBySkuType(ctx, types.AcctPriceListDBReq{
		SkuType: types.SKUCSGHub,
		SkuKind: cast.ToString(int(types.SKUPackageAddon)),
		Page:    1,
		Per:     10,
	})
	require.Nil(t, err)
	require.Equal(t, 2, count)
	require.Equal(t, 2, len(data))
	require.Equal(t, "b/a", namesFunc(data))

	data, count, err = store.ListBySkuType(ctx, types.AcctPriceListDBReq{
		SkuType:    types.SKUCSGHub,
		SkuKind:    cast.ToString(int(types.SKUPackageAddon)),
		ResourceID: []string{"r"},
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
			SkuUnitType: "u", SkuDesc: "a",
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r3",
			SkuUnitType: "u", SkuDesc: "b",
		},
		{
			SkuType: types.SKUCSGHub, SkuKind: types.SKUPackageAddon, ResourceID: "r2",
			SkuUnitType: "u", SkuDesc: "c",
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

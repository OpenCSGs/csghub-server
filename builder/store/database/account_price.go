package database

import (
	"context"
	"fmt"
	"strconv"

	"github.com/uptrace/bun"

	"opencsg.com/csghub-server/common/types"
)

type accountPriceStoreImpl struct {
	db *DB
}

type AccountPriceStore interface {
	Create(ctx context.Context, input AccountPrice) (*AccountPrice, error)
	Update(ctx context.Context, input AccountPrice) (*AccountPrice, error)
	Delete(ctx context.Context, input AccountPrice) error
	GetByID(ctx context.Context, id int64) (*AccountPrice, error)
	GetLatestByTime(ctx context.Context, req types.AcctPriceQueryReq) (*AccountPrice, error)
	ListBySkuType(ctx context.Context, req types.AcctPriceListReq) ([]AccountPrice, int, error)
	ListByIds(ctx context.Context, ids []int64) ([]*types.AcctPriceResp, error)
}

func NewAccountPriceStore() AccountPriceStore {
	return &accountPriceStoreImpl{
		db: defaultDB,
	}
}

func NewAccountPriceStoreWithDB(db *DB) AccountPriceStore {
	return &accountPriceStoreImpl{
		db: db,
	}
}

type AccountPrice struct {
	ID               int64             `bun:",pk,autoincrement" json:"id"`
	SkuType          types.SKUType     `bun:",notnull" json:"sku_type"`
	SkuPrice         int64             `bun:",notnull" json:"sku_price"`
	SkuUnit          int64             `bun:",notnull" json:"sku_unit"`
	SkuDesc          string            `bun:",notnull" json:"sku_desc"`
	ResourceID       string            `bun:",notnull" json:"resource_id"`
	SkuUnitType      types.SkuUnitType `json:"sku_unit_type"`
	SkuPriceCurrency string            `json:"sku_price_currency"`
	SkuKind          types.SKUKind     `json:"sku_kind"`
	Quota            string            `json:"quota"`
	SkuPriceID       int64             `json:"sku_price_id"`
	Discount         float64           `json:"discount"` // discount rate, e.g. 0.9 means 10% discount
	UseLimitPrice    int64             `json:"use_limit_price"`
	times
}

type PriceResp struct {
	Prices []AccountPrice `json:"data"`
	Total  int            `json:"total"`
}

func (a *accountPriceStoreImpl) Create(ctx context.Context, input AccountPrice) (*AccountPrice, error) {
	res, err := a.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("failed to insert price, error:%w", err)
	}
	return &input, nil
}

func (a *accountPriceStoreImpl) Update(ctx context.Context, input AccountPrice) (*AccountPrice, error) {
	_, err := a.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update price, error: %w", err)
	}
	return &input, nil
}

func (a *accountPriceStoreImpl) Delete(ctx context.Context, input AccountPrice) error {
	_, err := a.db.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete price, error: %w", err)
	}
	return nil
}

func (a *accountPriceStoreImpl) GetByID(ctx context.Context, id int64) (*AccountPrice, error) {
	price := &AccountPrice{}
	err := a.db.Core.NewSelect().Model(price).Where("id = ?", id).Scan(ctx, price)
	if err != nil {
		return nil, fmt.Errorf("select price by id %d, error: %w", id, err)
	}
	return price, nil
}

func (a *accountPriceStoreImpl) GetLatestByTime(ctx context.Context, req types.AcctPriceQueryReq) (*AccountPrice, error) {
	price := &AccountPrice{}
	err := a.db.Core.NewSelect().Model(price).
		Where("sku_type = ?", req.SkuType).
		Where("sku_kind = ?", req.SkuKind).
		Where("resource_id = ?", req.ResourceID).
		Where("sku_unit_type = ?", req.SkuUnitType).
		Where("created_at <= ?", req.PriceTime).
		Order("created_at DESC").Limit(1).Scan(ctx, price)
	if err != nil {
		return nil, fmt.Errorf("select price by time, error: %w", err)
	}
	return price, nil
}

func (a *accountPriceStoreImpl) ListBySkuType(ctx context.Context, req types.AcctPriceListReq) ([]AccountPrice, int, error) {
	var result []AccountPrice
	q := a.db.Core.NewSelect().Model(&result).
		DistinctOn("sku_type, sku_kind, resource_id, sku_unit_type").
		Where("sku_type = ?", req.SkuType)

	if len(req.SkuKind) > 0 {
		skuKindInt, err := strconv.Atoi(req.SkuKind)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid sku kind %s, error: %w", req.SkuKind, err)
		}
		q.Where("sku_kind = ?", skuKindInt)
	}

	if len(req.ResourceID) > 0 {
		q.Where("resource_id = ?", req.ResourceID)
	}

	count, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to counting recorder, error: %w", err)
	}
	_, err = q.Order("sku_type ASC").Order("sku_kind ASC").Order("resource_id ASC").
		Order("sku_unit_type ASC").Order("created_at DESC").
		Limit(req.Per).Offset((req.Page-1)*req.Per).Exec(ctx, &result)
	if err != nil {
		return nil, 0, fmt.Errorf("select prices by type and kind and resource, error: %w", err)
	}
	return result, count, nil
}

func (a *accountPriceStoreImpl) ListByIds(ctx context.Context, ids []int64) ([]*types.AcctPriceResp, error) {
	var result []*AccountPrice
	if len(ids) == 0 {
		return nil, nil
	}
	err := a.db.Core.NewSelect().Model(&result).
		Where("id IN (?)", bun.In(ids)).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to select prices by IDs, error: %w", err)
	}
	res := make([]*types.AcctPriceResp, len(result))
	for index, price := range result {
		res[index] = &types.AcctPriceResp{
			Id:       price.ID,
			SkuType:  price.SkuType,
			SkuPrice: price.SkuPrice,
			SkuKind:  price.SkuKind,
			SkuDesc:  price.SkuDesc,
		}
	}

	return res, nil
}

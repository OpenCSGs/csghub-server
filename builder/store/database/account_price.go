package database

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/common/types"
)

type AccountPriceStore struct {
	db *DB
}

func NewAccountPriceStore() *AccountPriceStore {
	return &AccountPriceStore{
		db: defaultDB,
	}
}

type AccountPrice struct {
	ID         int64         `bun:",pk,autoincrement" json:"id"`
	SkuType    types.SKUType `bun:",notnull" json:"sku_type"`
	SkuPrice   int64         `bun:",notnull" json:"sku_price"`
	SkuUnit    int64         `bun:",notnull" json:"sku_unit"`
	SkuDesc    string        `bun:",notnull" json:"sku_desc"`
	ResourceID string        `bun:",notnull" json:"resource_id"`
	times
}

func (a *AccountPriceStore) Create(ctx context.Context, input AccountPrice) (*AccountPrice, error) {
	res, err := a.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create price failed,error:%w", err)
	}
	return &input, nil
}

func (a *AccountPriceStore) Update(ctx context.Context, input AccountPrice) (*AccountPrice, error) {
	_, err := a.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update price, error: %w", err)
	}
	return &input, nil
}

func (a *AccountPriceStore) Delete(ctx context.Context, input AccountPrice) error {
	_, err := a.db.Core.NewDelete().Model(&input).WherePK().Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete price, error: %w", err)
	}
	return nil
}

func (a *AccountPriceStore) GetByID(ctx context.Context, id int64) (*AccountPrice, error) {
	price := &AccountPrice{}
	err := a.db.Core.NewSelect().Model(price).Where("id = ?", id).Scan(ctx, price)
	if err != nil {
		return nil, fmt.Errorf("failed to query price, error: %w", err)
	}
	return price, nil
}

func (a *AccountPriceStore) GetLatestByTime(ctx context.Context, req types.ACCT_PRICE_REQ) (*AccountPrice, error) {
	price := &AccountPrice{}
	err := a.db.Core.NewSelect().Model(price).Where("sku_type = ? and resource_id = ? and created_at <= ?", req.SKUType, req.ResourceID, req.PriceTime).Order("created_at DESC").Limit(1).Scan(ctx, price)
	if err != nil {
		return nil, fmt.Errorf("failed to get price by time, error: %w", err)
	}
	return price, nil
}

func (a *AccountPriceStore) ListBySKUType(ctx context.Context, req types.ACCT_PRICE_REQ) ([]AccountPrice, int, error) {
	var result []AccountPrice
	q := a.db.Core.NewSelect().Model(&result).Where("sku_type = ?", req.SKUType)
	count, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to counting recorder, error: %w", err)
	}
	_, err = q.Order("created_at DESC").Limit(req.Per).Offset((req.Page-1)*req.Per).Exec(ctx, &result)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list price by type, error: %w", err)
	}
	return result, count, nil
}

func (a *AccountPriceStore) ListBySKUTypeAndResourceID(ctx context.Context, req types.ACCT_PRICE_REQ) ([]AccountPrice, int, error) {
	var result []AccountPrice
	q := a.db.Core.NewSelect().Model(&result).Where("sku_type = ? and resource_id = ?", req.SKUType, req.ResourceID)
	count, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to counting recorders, error: %w", err)
	}
	_, err = q.Order("created_at DESC").Limit(req.Per).Offset((req.Page-1)*req.Per).Exec(ctx, &result)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list sku prices, error: %w", err)
	}
	return result, count, nil
}

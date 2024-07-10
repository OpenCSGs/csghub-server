package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type AccountingPriceComponent struct {
	au *database.AccountPriceStore
}

func NewAccountingPrice() *AccountingPriceComponent {
	apc := &AccountingPriceComponent{
		au: database.NewAccountPriceStore(),
	}
	return apc
}

func (a *AccountingPriceComponent) GetPriceByID(ctx context.Context, id int64) (*database.AccountPrice, error) {
	price, err := a.au.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get price by id, %w", err)
	}
	return price, nil
}

func (a *AccountingPriceComponent) CreatePrice(ctx context.Context, req types.ACCT_PRICE) (*database.AccountPrice, error) {
	price := database.AccountPrice{
		SkuType:          types.SKUType(req.SkuType),
		SkuPrice:         req.SkuPrice,
		SkuUnit:          req.SkuUnit,
		SkuDesc:          req.SkuDesc,
		ResourceID:       req.ResourceID,
		SkuUnitType:      req.SkuUnitType,
		SkuPriceCurrency: req.SkuPriceCurrency,
	}
	res, err := a.au.Create(ctx, price)
	if err != nil {
		return nil, fmt.Errorf("failed to add price, error: %w", err)
	}
	return res, nil
}

func (a *AccountingPriceComponent) UpdatePrice(ctx context.Context, req types.ACCT_PRICE, id int64) (*database.AccountPrice, error) {
	price := database.AccountPrice{
		ID:               id,
		SkuType:          types.SKUType(req.SkuType),
		SkuPrice:         req.SkuPrice,
		SkuUnit:          req.SkuUnit,
		SkuDesc:          req.SkuDesc,
		ResourceID:       req.ResourceID,
		SkuUnitType:      req.SkuUnitType,
		SkuPriceCurrency: req.SkuPriceCurrency,
	}
	res, err := a.au.Update(ctx, price)
	if err != nil {
		return nil, fmt.Errorf("failed to modify price, %w", err)
	}
	return res, nil
}

func (a *AccountingPriceComponent) DeletePrice(ctx context.Context, id int64) error {
	price := database.AccountPrice{
		ID: id,
	}
	err := a.au.Delete(ctx, price)
	if err != nil {
		return fmt.Errorf("failed to remove price, %w", err)
	}
	return err
}

func (a *AccountingPriceComponent) GetLatestByTime(ctx context.Context, req types.ACCT_PRICE_REQ) (*database.AccountPrice, error) {
	price, err := a.au.GetLatestByTime(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("fail to get price by time, %w", err)
	}
	return price, nil
}

func (a *AccountingPriceComponent) ListPricesBySKUType(ctx context.Context, req types.ACCT_PRICE_REQ) ([]database.AccountPrice, int, error) {
	prices, total, err := a.au.ListBySKUType(ctx, req)
	if err != nil {
		return nil, 0, fmt.Errorf("fail to list price by sku type, %w", err)
	}
	return prices, total, nil
}

func (a *AccountingPriceComponent) ListPricesBySKUTypeAndResourceID(ctx context.Context, req types.ACCT_PRICE_REQ) ([]database.AccountPrice, int, error) {
	prices, total, err := a.au.ListBySKUTypeAndResourceID(ctx, req)
	if err != nil {
		return nil, 0, fmt.Errorf("fail to list price by sku type and resource, %w", err)
	}
	return prices, total, nil
}

package database

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type accountPriceStoreImpl struct {
	db *DB
}

type AcctPriceGroupKey struct {
	SkuType    types.SKUType   `json:"sku_type"`
	SkuKind    types.SKUKind   `json:"sku_kind"`
	ResourceID string          `json:"resource_id"`
	SkuStatus  types.SkuStatus `json:"sku_status"`
}

type AccountPriceStore interface {
	Create(ctx context.Context, input AccountPrice) (*AccountPrice, error)
	BatchCreate(ctx context.Context, prices []AccountPrice) ([]AccountPrice, error)
	Update(ctx context.Context, input AccountPrice) (*AccountPrice, error)
	Delete(ctx context.Context, input AccountPrice) error
	GetByID(ctx context.Context, id int64) (*AccountPrice, error)
	GetLatestByTime(ctx context.Context, req types.AcctPriceQueryReq) (*AccountPrice, error)
	ListBySkuType(ctx context.Context, req types.AcctPriceListDBReq) ([]AccountPrice, int, error)
	ListBySkuTypeAndKinds(ctx context.Context, req types.AcctPriceListByKindsReq) ([]AccountPrice, error)
	ListByIds(ctx context.Context, ids []int64) ([]*types.AcctPriceResp, error)
	ListDistinctGroupKeys(ctx context.Context, req types.AcctPriceDistinctListReq) ([]AcctPriceGroupKey, int, error)
	ListPricesByGroupKey(ctx context.Context, req types.AcctPriceGroupKeyReq) ([]AccountPrice, error)
	OffLineBySkuTypeAndResourceID(ctx context.Context, req types.AcctPriceOffLineReq) error
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
	Resolution       string            `json:"resolution"`
	SkuStatus        types.SkuStatus   `bun:",default:1" json:"sku_status"`
	times
}

type PriceResp struct {
	Prices []AccountPrice `json:"data"`
	Total  int            `json:"total"`
}

func (a *accountPriceStoreImpl) Create(ctx context.Context, input AccountPrice) (*AccountPrice, error) {
	err := a.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewUpdate().Model(&AccountPrice{}).
			Where("sku_type = ?", input.SkuType).
			Where("sku_kind = ?", input.SkuKind).
			Where("resource_id = ?", input.ResourceID).
			Where("sku_status = ?", types.SkuStatusEnabled).
			Set("sku_status = ?", types.SkuStatusDisabled).
			Exec(ctx)
		if err != nil {
			return errorx.HandleDBError(err, nil)
		}

		res, err := tx.NewInsert().Model(&input).Exec(ctx, &input)
		if err := assertAffectedOneRow(res, err); err != nil {
			return errorx.HandleDBError(err, nil)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &input, nil
}

func (a *accountPriceStoreImpl) BatchCreate(ctx context.Context, prices []AccountPrice) ([]AccountPrice, error) {
	if len(prices) == 0 {
		return nil, nil
	}

	err := a.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		for _, p := range prices {
			_, err := tx.NewUpdate().Model(&AccountPrice{}).
				Where("sku_type = ?", p.SkuType).
				Where("sku_kind = ?", p.SkuKind).
				Where("resource_id = ?", p.ResourceID).
				Where("sku_status = ?", types.SkuStatusEnabled).
				Set("sku_status = ?", types.SkuStatusDisabled).
				Exec(ctx)
			if err != nil {
				return errorx.HandleDBError(err, nil)
			}
		}

		res, err := tx.NewInsert().Model(&prices).Exec(ctx, &prices)
		if err := assertAffectedXRows(int64(len(prices)), res, err); err != nil {
			return errorx.HandleDBError(err, nil)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return prices, nil
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
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("id", id))
	}
	return price, nil
}

func (a *accountPriceStoreImpl) GetLatestByTime(ctx context.Context, req types.AcctPriceQueryReq) (*AccountPrice, error) {
	price := &AccountPrice{}
	err := a.db.Core.NewSelect().Model(price).
		Where("sku_type = ?", req.SkuType).
		Where("sku_kind = ?", req.SkuKind).
		Where("resource_id = ?", req.ResourceID).
		Where("sku_unit_type IN (?)", bun.In(req.SkuUnitType)).
		Where("created_at <= ?", req.PriceTime).
		Where("sku_status = ?", types.SkuStatusEnabled).
		Order("created_at DESC").Limit(1).Scan(ctx, price)
	if err != nil {
		return nil, fmt.Errorf("select price by time, error: %w", err)
	}
	return price, nil
}

func (a *accountPriceStoreImpl) ListBySkuType(ctx context.Context, req types.AcctPriceListDBReq) ([]AccountPrice, int, error) {
	var result []AccountPrice
	q := a.db.Core.NewSelect().Model(&result).Where("sku_type = ?", req.SkuType)

	if req.SkuKind > 0 {
		q.Where("sku_kind = ?", req.SkuKind)
	}

	resourceIDs := []string{}
	for _, rid := range req.ResourceID {
		if rid != "" {
			resourceIDs = append(resourceIDs, rid)
		}
	}
	if len(resourceIDs) > 0 {
		q.Where("resource_id IN (?)", bun.In(resourceIDs))
	}

	if req.SkuStatus > 0 {
		q.Where("sku_status = ?", req.SkuStatus)
	}

	count, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to counting recorder, error: %w", err)
	}
	sortMap := map[string]string{
		"sku_type":      "ASC",
		"sku_kind":      "ASC",
		"resource_id":   "ASC",
		"sku_unit_type": "ASC",
		"sku_status":    "ASC",
		"created_at":    "DESC",
	}

	if len(req.SortBy) > 0 && len(req.SortOrder) > 0 {
		sortMap[req.SortBy] = req.SortOrder
	}
	_, err = q.Order(fmt.Sprintf("%s %s", "sku_type", sortMap["sku_type"])).
		Order(fmt.Sprintf("%s %s", "sku_kind", sortMap["sku_kind"])).
		Order(fmt.Sprintf("%s %s", "resource_id", sortMap["resource_id"])).
		Order(fmt.Sprintf("%s %s", "sku_unit_type", sortMap["sku_unit_type"])).
		Order(fmt.Sprintf("%s %s", "sku_status", sortMap["sku_status"])).
		Order(fmt.Sprintf("%s %s", "created_at", sortMap["created_at"])).
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
			Id:        price.ID,
			SkuType:   price.SkuType,
			SkuPrice:  price.SkuPrice,
			SkuKind:   price.SkuKind,
			SkuDesc:   price.SkuDesc,
			SkuStatus: price.SkuStatus,
		}
	}

	return res, nil
}

func (a *accountPriceStoreImpl) ListBySkuTypeAndKinds(ctx context.Context, req types.AcctPriceListByKindsReq) ([]AccountPrice, error) {
	var result []AccountPrice
	q := a.db.Core.NewSelect().Model(&result).
		Where("sku_type = ?", req.SkuType).
		Where("sku_kind IN (?)", bun.In(req.SkuKinds)).
		Where("sku_status = ?", types.SkuStatusEnabled)

	if req.ResourceID != "" {
		q.Where("resource_id = ?", req.ResourceID)
	}

	q = q.Order("resource_id ASC").Order("sku_kind ASC").Order("created_at DESC")

	err := q.Scan(ctx, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to select prices by sku type and kinds, error: %w", err)
	}
	return result, nil
}

func (a *accountPriceStoreImpl) ListDistinctGroupKeys(ctx context.Context, req types.AcctPriceDistinctListReq) ([]AcctPriceGroupKey, int, error) {
	var groupKeys []AcctPriceGroupKey
	subQ := a.db.Core.NewSelect().
		ColumnExpr("DISTINCT sku_type, sku_kind, resource_id, sku_status").
		Model(&AccountPrice{}).
		Where("sku_type = ?", req.SkuType)

	if req.SkuKind > 0 {
		subQ.Where("sku_kind = ?", req.SkuKind)
	}

	resourceIDs := []string{}
	for _, rid := range req.ResourceID {
		if len(rid) > 0 {
			resourceIDs = append(resourceIDs, rid)
		}
	}
	if len(resourceIDs) > 0 {
		subQ.Where("resource_id IN (?)", bun.In(resourceIDs))
	}

	subQ.Where("sku_status = ?", req.SkuStatus)

	total, err := a.db.Core.NewSelect().
		TableExpr("(?) AS sub", subQ).
		Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, nil)
	}

	err = subQ.Order("sku_type ASC").Order("sku_kind ASC").Order("resource_id ASC").
		Limit(req.Per).Offset((req.Page-1)*req.Per).
		Scan(ctx, &groupKeys)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, nil)
	}

	return groupKeys, total, nil
}

func (a *accountPriceStoreImpl) ListPricesByGroupKey(ctx context.Context, req types.AcctPriceGroupKeyReq) ([]AccountPrice, error) {
	var result []AccountPrice
	err := a.db.Core.NewSelect().Model(&result).
		Where("sku_type = ?", req.SkuType).
		Where("sku_kind = ?", req.SkuKind).
		Where("resource_id = ?", req.ResourceID).
		Where("sku_status = ?", req.SkuStatus).
		Order("created_at DESC").
		Scan(ctx, &result)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return result, nil
}

func (a *accountPriceStoreImpl) OffLineBySkuTypeAndResourceID(ctx context.Context, req types.AcctPriceOffLineReq) error {
	_, err := a.db.Core.NewUpdate().Model(&AccountPrice{}).
		Where("sku_type = ?", req.SkuType).
		Where("resource_id = ?", req.ResourceID).
		Where("sku_status = ?", types.SkuStatusEnabled).
		Set("sku_status = ?", types.SkuStatusDisabled).
		Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, nil)
	}
	return nil
}

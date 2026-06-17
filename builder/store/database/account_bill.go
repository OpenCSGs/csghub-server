package database

import (
	"context"
	"fmt"
	"time"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type accountBillStoreImpl struct {
	db *DB
}

type AccountBillStore interface {
	ListByUserIDAndDate(ctx context.Context, req types.AcctBillsReq) (AccountBillRes, error)
	ListBillsDetailByUserID(ctx context.Context, req types.AcctBillsDetailReq) (AccountBillDetailRes, error)
	SumValueByAPIKey(ctx context.Context, tokenID int64) (float64, error)
	SumValueByAPIKeyBetween(ctx context.Context, tokenID int64, start, end time.Time) (float64, error)
	GetVoucherBills(ctx context.Context, req types.VoucherBillReq) ([]VoucherBillGroupedResult, error)
}

func NewAccountBillStore() AccountBillStore {
	return &accountBillStoreImpl{
		db: defaultDB,
	}
}

func NewAccountBillStoreWithDB(db *DB) AccountBillStore {
	return &accountBillStoreImpl{
		db: db,
	}
}

type AccountBill struct {
	ID              int64             `bun:",pk,autoincrement" json:"id"`
	BillDate        time.Time         `bun:"type:date" json:"bill_date"`
	UserUUID        string            `bun:",notnull" json:"user_uuid"`
	Scene           types.SceneType   `bun:",notnull" json:"scene"`
	CustomerID      string            `bun:",notnull" json:"customer_id"`
	Value           float64           `bun:",notnull" json:"value"`
	Consumption     float64           `bun:",notnull" json:"consumption"`
	PromptToken     float64           `bun:",notnull" json:"prompt_token"`
	CompletionToken float64           `bun:",notnull" json:"completion_token"`
	APIKey          string            `bun:",notnull,default:''" json:"api_key"`
	Count           float64           `bun:",notnull,default:0" json:"count"`
	TokenID         int64             `bun:",notnull,default:0" json:"token_id"`
	DataType        string            `bun:",notnull,default:''" json:"data_type"`
	Resolution      string            `bun:",notnull,default:''" json:"resolution"`
	Duration        float64           `bun:",notnull,default:0" json:"duration"`
	VoucherNo       string            `bun:",notnull,default:''" json:"voucher_no"`
	VoucherValue    float64           `bun:",notnull,default:0" json:"voucher_value"`
	CashValue       float64           `bun:",notnull,default:0" json:"cash_value"`
	UnitType        types.SkuUnitType `bun:",notnull,default:''" json:"unit_type"`
	times
}

type BillValues struct {
	TotalValue   float64 `bun:"total_value"`
	VoucherValue float64 `bun:"voucher_value"`
	CashValue    float64 `bun:"cash_value"`
	Consumption  float64 `bun:"consumption"`
}

type TotalResult struct {
	TotalValue           float64 `bun:"total_value"`
	TotalConsumption     float64 `bun:"total_consumption"`
	TotalPromptToken     float64 `bun:"total_prompt_token"`
	TotalCompletionToken float64 `bun:"total_completion_token"`
	TotalVoucherValue    float64 `bun:"total_voucher_value"`
	TotalCashValue       float64 `bun:"total_cash_value"`
	TotalDuration        float64 `bun:"total_duration"`
	TotalCount           float64 `bun:"total_count"`
}

type AccountBillRes struct {
	Data []types.ITEM `json:"data"`
	types.AcctSummary
}

type AccountBillDetailRes struct {
	Data  []AccountBill `json:"data"`
	Total int           `json:"total"`
}

func (s *accountBillStoreImpl) ListByUserIDAndDate(ctx context.Context, req types.AcctBillsReq) (AccountBillRes, error) {
	var bill []AccountBill
	var res []types.ITEM
	q := s.db.Operator.Core.NewSelect().Model(&bill).
		ColumnExpr("customer_id as instance_name").
		ColumnExpr("data_type").
		ColumnExpr("resolution").
		ColumnExpr("unit_type").
		ColumnExpr("sum(value) as value").
		ColumnExpr("sum(consumption) as consumption").
		ColumnExpr("sum(prompt_token) as prompt_token").
		ColumnExpr("sum(completion_token) as completion_token").
		ColumnExpr("sum(voucher_value) as voucher_value").
		ColumnExpr("sum(cash_value) as cash_value").
		ColumnExpr("sum(duration) as duration").
		ColumnExpr("sum(count) as count").
		Where("bill_date >= ? and bill_date <= ?", req.StartDate, req.EndDate).
		Where("user_uuid = ?", req.TargetUUID).
		Where("scene = ?", req.Scene).
		Group("customer_id", "data_type", "resolution", "unit_type")

	count, err := q.Count(ctx)
	if err != nil {
		return AccountBillRes{}, errorx.HandleDBError(err, nil)
	}

	var totalResult TotalResult

	err = s.db.Operator.Core.NewSelect().With("grouped_items", q).TableExpr("grouped_items").
		ColumnExpr("SUM(value) AS total_value").
		ColumnExpr("SUM(consumption) as total_consumption").
		ColumnExpr("SUM(prompt_token) as total_prompt_token").
		ColumnExpr("SUM(completion_token) as total_completion_token").
		ColumnExpr("SUM(voucher_value) as total_voucher_value").
		ColumnExpr("SUM(cash_value) as total_cash_value").
		ColumnExpr("SUM(duration) as total_duration").
		ColumnExpr("SUM(count) as total_count").
		Scan(ctx, &totalResult)
	if err != nil {
		return AccountBillRes{}, errorx.HandleDBError(err, nil)
	}

	err = q.Order("customer_id").Limit(req.Per).Offset((req.Page-1)*req.Per).Scan(ctx, &res)
	if err != nil {
		return AccountBillRes{}, errorx.HandleDBError(err, nil)
	}
	return AccountBillRes{
		Data: res,
		AcctSummary: types.AcctSummary{
			Total:                count,
			TotalValue:           totalResult.TotalValue,
			TotalConsumption:     totalResult.TotalConsumption,
			TotalPromptToken:     totalResult.TotalPromptToken,
			TotalCompletionToken: totalResult.TotalCompletionToken,
			TotalVoucherValue:    totalResult.TotalVoucherValue,
			TotalCashValue:       totalResult.TotalCashValue,
			TotalDuration:        totalResult.TotalDuration,
			TotalCount:           totalResult.TotalCount,
		},
	}, err
}

func (s *accountBillStoreImpl) ListBillsDetailByUserID(ctx context.Context, req types.AcctBillsDetailReq) (AccountBillDetailRes, error) {
	var bills []AccountBill

	q := s.db.Operator.Core.NewSelect().Model(&bills).
		Where("bill_date >= ? and bill_date <= ? and user_uuid = ? and scene = ?", req.StartDate, req.EndDate, req.TargetUUID, req.Scene)

	if req.InstanceName != "" {
		q = q.Where("customer_id = ?", req.InstanceName)
	}

	count, err := q.Count(ctx)
	if err != nil {
		return AccountBillDetailRes{}, errorx.HandleDBError(err, nil)
	}

	err = q.Order("bill_date ASC").Limit(req.Per).Offset((req.Page-1)*req.Per).Scan(ctx, &bills)
	if err != nil {
		return AccountBillDetailRes{}, errorx.HandleDBError(err, nil)
	}

	return AccountBillDetailRes{
		Data:  bills,
		Total: count,
	}, nil
}

func (s *accountBillStoreImpl) SumValueByAPIKey(ctx context.Context, tokenID int64) (float64, error) {
	var result struct {
		TotalValue float64 `bun:"total_value"`
	}
	err := s.db.Operator.Core.NewSelect().
		Table("account_bills").
		ColumnExpr("SUM(value) AS total_value").
		Where("token_id = ?", tokenID).
		Where("scene = ?", types.SceneModelServerless).
		Scan(ctx, &result)
	if err != nil {
		return 0, errorx.HandleDBError(err, nil)
	}
	return result.TotalValue, nil
}

func (s *accountBillStoreImpl) SumValueByAPIKeyBetween(ctx context.Context, tokenID int64, start, end time.Time) (float64, error) {
	var result struct {
		TotalValue float64 `bun:"total_value"`
	}
	err := s.db.Operator.Core.NewSelect().
		Table("account_bills").
		ColumnExpr("SUM(value) AS total_value").
		Where("token_id = ?", tokenID).
		Where("scene = ?", types.SceneModelServerless).
		Where("bill_date >= ? AND bill_date <= ?", start, end).
		Scan(ctx, &result)
	if err != nil {
		return 0, errorx.HandleDBError(err, nil)
	}
	return result.TotalValue, nil
}

type VoucherBillGroupedResult struct {
	Scene        types.SceneType `json:"scene"`
	InstanceName string          `json:"instance_name"`
	VoucherValue float64         `json:"voucher_value"`
	Consumption  float64         `json:"consumption"`
}

func (s *accountBillStoreImpl) GetVoucherBills(ctx context.Context, req types.VoucherBillReq) ([]VoucherBillGroupedResult, error) {
	var results []VoucherBillGroupedResult
	q := s.db.Core.NewSelect().
		Model((*AccountBill)(nil)).
		ColumnExpr("scene, customer_id as instance_name, SUM(voucher_value) AS voucher_value, SUM(consumption) AS consumption").
		Where("bill_date >= ? and bill_date <= ?", req.StartDate, req.EndDate).
		Where("user_uuid = ?", req.TargetUUID)

	if req.Scene != 0 {
		q = q.Where("scene = ?", req.Scene)
	}

	if req.InstanceName != "" {
		q = q.Where("customer_id = ?", req.InstanceName)
	}

	q = q.Where("voucher_no = ?", req.VoucherNo)

	err := q.Group("scene", "customer_id").Scan(ctx, &results)
	if err != nil {
		return nil, fmt.Errorf("query grouped voucher bills for target %s voucher %s: %w", req.TargetUUID, req.VoucherNo, err)
	}
	return results, nil
}

package database

import (
	"context"
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
	ID              int64           `bun:",pk,autoincrement" json:"id"`
	BillDate        time.Time       `bun:"type:date" json:"bill_date"`
	UserUUID        string          `bun:",notnull" json:"user_uuid"`
	Scene           types.SceneType `bun:",notnull" json:"scene"`
	CustomerID      string          `bun:",notnull" json:"customer_id"`
	Value           float64         `bun:",notnull" json:"value"`
	Consumption     float64         `bun:",notnull" json:"consumption"`
	PromptToken     float64         `bun:",notnull" json:"prompt_token"`
	CompletionToken float64         `bun:",notnull" json:"completion_token"`
	APIKey          string          `bun:",notnull,default:''" json:"api_key"`
	Count           float64         `bun:",notnull,default:0" json:"count"`
	times
}

type TotalResult struct {
	TotalValue           float64 `bun:"total_value"`
	TotalConsumption     float64 `bun:"total_consumption"`
	TotalPromptToken     float64 `bun:"total_prompt_token"`
	TotalCompletionToken float64 `bun:"total_completion_token"`
}

type AccountBillRes struct {
	Data []map[string]interface{} `json:"data"`
	types.AcctSummary
}

type AccountBillDetailRes struct {
	Data  []AccountBill `json:"data"`
	Total int           `json:"total"`
}

func (s *accountBillStoreImpl) ListByUserIDAndDate(ctx context.Context, req types.AcctBillsReq) (AccountBillRes, error) {
	var bill []AccountBill
	var res []map[string]interface{}
	q := s.db.Operator.Core.NewSelect().Model(&bill).ColumnExpr("customer_id as instance_name, sum(value) as value, sum(consumption) as consumption, sum(prompt_token) as prompt_token, sum(completion_token) as completion_token").Where("bill_date >= ? and bill_date <= ? and user_uuid = ? and scene = ?", req.StartDate, req.EndDate, req.TargetUUID, req.Scene).Group("customer_id")

	count, err := q.Count(ctx)
	if err != nil {
		return AccountBillRes{}, errorx.HandleDBError(err, nil)
	}

	var totalResult TotalResult

	err = s.db.Operator.Core.NewSelect().With("grouped_items", q).TableExpr("grouped_items").ColumnExpr("SUM(value) AS total_value, SUM(consumption) as total_consumption, SUM(prompt_token) as total_prompt_token, SUM(completion_token) as total_completion_token").Scan(ctx, &totalResult)
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

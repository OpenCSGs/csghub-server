package database

import (
	"context"
	"time"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type AccountSubscriptionBillStore interface {
	GetByID(ctx context.Context, id int64) (*AccountSubscriptionBill, error)
	List(ctx context.Context, req *types.SubscriptionBillListReq) (*SubBillListResult, error)
}

type SubBillListResult struct {
	Data            []AccountSubscriptionBill `json:"data"`
	Total           int                       `json:"total"`
	PaidTotalAmount float64                   `json:"paid_total_amount"`
}

type accountSubscriptionBillStoreImpl struct {
	db *DB
}

func NewAccountSubscriptionBillStore() AccountSubscriptionBillStore {
	return &accountSubscriptionBillStoreImpl{
		db: defaultDB,
	}
}

func NewAccountSubscriptionBillWithDB(db *DB) AccountSubscriptionBillStore {
	return &accountSubscriptionBillStoreImpl{
		db: db,
	}
}

type AccountSubscriptionBill struct {
	ID          int64                `bun:",pk,autoincrement" json:"id"`
	SubID       int64                `bun:",notnull" json:"sub_id"`
	EventUUID   string               `bun:",notnull,unique" json:"event_uuid"`
	UserUUID    string               `bun:",notnull" json:"user_uuid"`
	AmountPaid  float64              `bun:",notnull" json:"amount_paid"`
	Status      types.BillingStatus  `bun:",notnull" json:"status"`
	Reason      types.BillingReasion `bun:",notnull" json:"reason"`
	PeriodStart time.Time            `bun:",notnull" json:"period_start"`
	PeriodEnd   time.Time            `bun:",notnull" json:"period_end"`
	PriceID     int64                `bun:",notnull" json:"price_id"`
	ResourceID  string               `bun:",notnull" json:"resource_id"`
	Explain     string               `bun:",nullzero" json:"explain"`
	times
}

func (s *accountSubscriptionBillStoreImpl) GetByID(ctx context.Context, id int64) (*AccountSubscriptionBill, error) {
	var bill AccountSubscriptionBill
	if err := s.db.Operator.Core.NewSelect().Model(&bill).Where("id = ?", id).Scan(ctx); err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &bill, nil
}

func (s *accountSubscriptionBillStoreImpl) List(ctx context.Context, req *types.SubscriptionBillListReq) (*SubBillListResult, error) {
	type SumResult struct {
		PaidTotalAmount float64 `bun:"paid_total_amount"`
	}

	var bills []AccountSubscriptionBill
	var sumResult SumResult

	q := s.db.Operator.Core.NewSelect().Model(&bills).
		Where("created_at >= ?", req.StartTime).
		Where("created_at <= ?", req.EndTime)

	sumQuery := s.db.Operator.Core.NewSelect().Model((*AccountSubscriptionBill)(nil)).
		ColumnExpr("COALESCE(SUM(amount_paid), 0) as paid_total_amount").
		Where("created_at >= ?", req.StartTime).
		Where("created_at <= ?", req.EndTime)

	if len(req.QueryUserUUID) > 0 {
		q = q.Where("user_uuid = ?", req.QueryUserUUID)
		sumQuery = sumQuery.Where("user_uuid = ?", req.QueryUserUUID)
	}

	if len(req.Status) > 0 {
		q = q.Where("status = ?", req.Status)
		sumQuery = sumQuery.Where("status = ?", req.Status)
	}

	count, err := q.Count(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	err = sumQuery.Scan(ctx, &sumResult)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	_, err = q.Order("id DESC").Limit(req.Per).Offset((req.Page-1)*req.Per).Exec(ctx, &bills)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	result := &SubBillListResult{
		Data:            bills,
		Total:           count,
		PaidTotalAmount: sumResult.PaidTotalAmount,
	}

	return result, nil

}

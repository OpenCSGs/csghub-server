package database

import (
	"context"
	"time"

	"opencsg.com/csghub-server/common/types"
)

type AccountBillStore struct {
	db *DB
}

func NewAccountBillStore() *AccountBillStore {
	return &AccountBillStore{
		db: defaultDB,
	}
}

type AccountBill struct {
	ID          int64           `bun:",pk,autoincrement" json:"id"`
	BillDate    time.Time       `bun:"type:date" json:"bill_date"`
	UserUUID    string          `bun:",notnull" json:"user_uuid"`
	Scene       types.SceneType `bun:",notnull" json:"scene"`
	CustomerID  string          `bun:",notnull" json:"customer_id"`
	Value       float64         `bun:",notnull" json:"value"`
	Consumption float64         `bun:",notnull" json:"consumption"`
	times
}

type TotalResult struct {
	TotalValue       float64 `bun:"total_value"`
	TotalConsumption float64 `bun:"total_consumption"`
}

type AccountBillRes struct {
	Data []map[string]interface{} `json:"data"`
	types.ACCT_SUMMARY
}

func (s *AccountBillStore) ListByUserIDAndDate(ctx context.Context, req types.ACCT_BILLS_REQ) (AccountBillRes, error) {
	var bill []AccountBill
	var res []map[string]interface{}
	q := s.db.Operator.Core.NewSelect().Model(&bill).ColumnExpr("customer_id as instance_name, sum(value) as value, sum(consumption) as consumption").Where("bill_date >= ? and bill_date <= ? and user_uuid = ? and scene = ?", req.StartDate, req.EndDate, req.UserUUID, req.Scene).Group("customer_id")

	count, err := q.Count(ctx)
	if err != nil {
		return AccountBillRes{}, err
	}

	var totalResult TotalResult

	err = s.db.Operator.Core.NewSelect().With("grouped_items", q).TableExpr("grouped_items").ColumnExpr("SUM(value) AS total_value, SUM(consumption) as total_consumption").Scan(ctx, &totalResult)
	if err != nil {
		return AccountBillRes{}, err
	}

	err = q.Order("customer_id").Limit(req.Per).Offset((req.Page-1)*req.Per).Scan(ctx, &res)
	if err != nil {
		return AccountBillRes{}, err
	}
	return AccountBillRes{
		Data: res,
		ACCT_SUMMARY: types.ACCT_SUMMARY{
			Total:            count,
			TotalValue:       totalResult.TotalValue,
			TotalConsumption: totalResult.TotalConsumption,
		},
	}, err
}

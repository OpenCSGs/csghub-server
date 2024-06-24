package database

import (
	"context"
	"time"
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
	ID          int64     `bun:",pk,autoincrement" json:"id"`
	BillDate    time.Time `bun:"type:date" json:"bill_date"`
	UserID      string    `bun:",notnull" json:"user_id"` // casdoor uuid
	Scene       SceneType `bun:",notnull" json:"scene"`
	CustomerID  string    `bun:",notnull" json:"customer_id"`
	Value       float64   `bun:",notnull" json:"value"`
	Consumption float64   `bun:",notnull" json:"consumption"`
	times
}

func (s *AccountBillStore) ListByUserIDAndDate(ctx context.Context, userID string, startDate, endDate string, scene, per, page int) ([]map[string]interface{}, int, error) {
	var result []AccountBill
	var res []map[string]interface{}
	q := s.db.Operator.Core.NewSelect().Model(&result).ColumnExpr("customer_id as instance_name, sum(value) as value, sum(consumption) as consumption").Where("bill_date >= ? and bill_date <= ? and user_id = ? and scene = ?", startDate, endDate, userID, scene).Group("customer_id")

	count, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	q.Order("customer_id").Limit(per).Offset((page-1)*per).Scan(ctx, &res)
	return res, count, err
}

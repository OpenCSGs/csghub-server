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

func (s *AccountBillStore) ListByUserIDAndDate(ctx context.Context, userID string, startDate, endDate string) ([]AccountBill, error) {
	var result []AccountBill
	_, err := s.db.Operator.Core.NewSelect().Model(&result).Where("bill_date >= ? and bill_date <= ? and user_id = ?", startDate, endDate, userID).Order("scene").Order("bill_date DESC").Exec(ctx, &result)
	return result, err
}

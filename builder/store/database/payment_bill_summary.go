package database

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

type BillSummaryDBStore interface {
	CreateOrGetBillSummary(ctx context.Context, summary *BillSummaryDB) (*BillSummaryDB, error)
}

type BillSummaryDB struct {
	bun.BaseModel `bun:"table:payment_bill_summary,alias:pbs"`

	ID          int64     `bun:",pk,autoincrement" json:"id"`
	GatewayType string    `bun:",notnull" json:"gateway_type"`
	Account     string    `bun:",notnull" json:"account"`
	BillDate    time.Time `bun:",notnull,type:timestamp" json:"bill_date"`
	S3Bucket    string    `bun:",notnull" json:"s3_bucket"`
	S3Key       string    `bun:",notnull" json:"s3_key"`

	CreatedAt time.Time `bun:",nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:",nullzero,default:current_timestamp" json:"updated_at"`
}

type BillSummaryDBStoreImpl struct {
	db *DB
}

func NewBillSummaryDBStoreWithDB(db *DB) BillSummaryDBStore {
	return &BillSummaryDBStoreImpl{db: db}
}

func NewBillSummaryDBStore() BillSummaryDBStore {
	return NewBillSummaryDBStoreWithDB(defaultDB)
}

// CreateOrGetBillSummary retrieves an existing bill summary record or creates a new one.
//
// The function first checks for an existing record based on the unique combination of
// gateway_type, account, and bill_date. If no record is found, it attempts to insert a new one.
// To handle potential race conditions in concurrent environments, the function uses
// "ON CONFLICT DO NOTHING" during the insert operation, ensuring no duplicate records are created.
// Finally, it retrieves the record to return the up-to-date data.
func (bs *BillSummaryDBStoreImpl) CreateOrGetBillSummary(ctx context.Context, summary *BillSummaryDB) (*BillSummaryDB, error) {
	summary.BillDate = summary.BillDate.UTC()

	existing := new(BillSummaryDB)
	err := bs.db.Operator.Core.NewSelect().
		Model(existing).
		Where("gateway_type = ?", summary.GatewayType).
		Where("account = ?", summary.Account).
		Where("bill_date = ?", summary.BillDate).
		Scan(ctx)
	if err == nil {
		return existing, nil
	}

	_, err = bs.db.Operator.Core.NewInsert().
		Model(summary).
		On("CONFLICT (gateway_type, account, bill_date) DO NOTHING").
		Returning("id").
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("create bill summary record, error: %w", err)
	}

	err = bs.db.Operator.Core.NewSelect().
		Model(summary).
		Where("gateway_type = ?", summary.GatewayType).
		Where("account = ?", summary.Account).
		Where("bill_date = ?", summary.BillDate).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created/updated bill summary: %w", err)
	}

	return summary, nil
}

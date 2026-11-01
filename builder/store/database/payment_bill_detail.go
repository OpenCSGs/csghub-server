package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/uptrace/bun"
	"time"
)

type BillDetailDBStore interface {
	CreateBillDetails(ctx context.Context, details []*BillDetailDB) error
}

type BillDetailDB struct {
	bun.BaseModel `bun:"table:payment_bill_detail,alias:pbd"`

	ID              int64  `bun:",pk,autoincrement" json:"id"`
	BillSummaryID   int64  `bun:",notnull" json:"bill_summary_id"`
	PayOrderID      string `bun:",notnull" json:"pay_order_id"`
	MerchantOrderID string `bun:",notnull" json:"merchant_order_id"`
	BusinessType    string `bun:",notnull" json:"business_type"`
	ProductName     string `bun:",notnull" json:"product_name"`

	CreateTime   time.Time `bun:",notnull" json:"create_time"`
	CompleteTime time.Time `bun:",notnull" json:"complete_time"`

	PayUser         string  `bun:",notnull" json:"pay_user"`
	OrderAmount     float64 `bun:",notnull" json:"order_amount"`
	MerchantReceive float64 `bun:",notnull" json:"merchant_receive"`
	ServiceFee      float64 `bun:",notnull" json:"service_fee"`

	Currency  string    `bun:",nullzero,default:'CNY'" json:"currency"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `bun:",nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:",nullzero,default:current_timestamp" json:"updated_at"`
}

type BillDetailDBStoreImpl struct {
	db *DB
}

func NewBillDetailDBStoreWithDB(db *DB) BillDetailDBStore {
	return &BillDetailDBStoreImpl{db: db}
}

func NewBillDetailDBStore() BillDetailDBStore {
	return NewBillDetailDBStoreWithDB(defaultDB)
}

// CreateBillDetails inserts bill detail records into the database.
//
// The function first queries existing records to minimize unnecessary insert operations
// and filters out duplicates from the input list. This reduces database load and improves performance.
//
// However, to ensure data consistency in concurrent environments, the function uses
// "ON CONFLICT DO NOTHING" during the insert operation. This handles cases where records
// might be inserted by other processes between the SELECT and INSERT steps, preventing errors.
func (ps *BillDetailDBStoreImpl) CreateBillDetails(ctx context.Context, details []*BillDetailDB) error {
	if len(details) == 0 {
		return nil
	}

	payOrderIDs := make([]string, 0, len(details))
	for _, d := range details {
		payOrderIDs = append(payOrderIDs, d.PayOrderID)
	}

	var existing []BillDetailDB
	err := ps.db.Operator.Core.NewSelect().
		Model(&existing).
		Where("bill_summary_id = ?", details[0].BillSummaryID).
		Where("pay_order_id IN (?)", bun.In(payOrderIDs)).
		Scan(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf(
			"check existing bill details error: bill_summary_id=%d, pay_order_ids=%v, err: %w",
			details[0].BillSummaryID,
			payOrderIDs,
			err,
		)
	}

	existingSet := make(map[string]struct{}, len(existing))
	for _, e := range existing {
		existingSet[e.PayOrderID] = struct{}{}
	}

	newDetails := make([]*BillDetailDB, 0, len(details))
	for _, d := range details {
		if _, found := existingSet[d.PayOrderID]; !found {
			newDetails = append(newDetails, d)
		}
	}

	if len(newDetails) == 0 {
		return nil
	}

	_, err = ps.db.Operator.Core.NewInsert().
		Model(&newDetails).
		On("CONFLICT (bill_summary_id, pay_order_id) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("create bill details records, error: %w", err)
	}

	return nil
}

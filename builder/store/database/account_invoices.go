package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// ErrRechargeNotInvoicable is returned when some recharge orders are not in a
// state that allows them to be invoiced, e.g. already attached to another
// non-failed invoice.
var ErrRechargeNotInvoicable = errors.New("some recharge orders are not invoicable")

// Invoice status constants
const (
	InvoiceStatusProcessing  = "processing"
	InvoiceStatusIssued      = "issued"
	InvoiceStatusFailed      = "failed"
	InvoiceStatusNotInvoiced = "not_invoiced"
)

// Invoice type constants
const (
	InvoiceTypeOrdinary = "ordinary"
	InvoiceTypeVAT      = "vat"
)

// Invoice title type constants
const (
	TitleTypeEnterpriseOrdinary = "enterprise_ordinary"
)

// InvoiceListParams defines the parameters for listing invoices.
type InvoiceListParams struct {
	UserUUID string `json:"user_uuid"` // Specify the user's UUID
	Page     int    `json:"page"`      // Current page number
	PageSize int    `json:"page_size"` // Number of items per page
	Search   string `json:"search"`    // Search field
	Status   string `json:"status"`    // Filter by status
	Sort     string `json:"sort"`      // e.g., "id ASC" or "apply_time DESC"
}

// AccountInvoiceStore defines the interface for invoice operations.
type AccountInvoiceStore interface {
	// CreateInvoiceWithRecharges creates an invoice record and its recharge-order
	// associations in a single transaction.
	CreateInvoiceWithRecharges(ctx context.Context, invoice *AccountInvoice, recharges []*AccountRecharge) error
	// GetInvoice retrieves a single invoice by its ID.
	GetInvoice(ctx context.Context, id int64) (*AccountInvoice, error)
	// UpdateInvoice updates an existing invoice record.
	UpdateInvoice(ctx context.Context, invoice *AccountInvoice) error
	// UpdateInvoiceNotFailed updates an existing invoice record only if its
	// current status is not failed. It reports whether a row was updated.
	UpdateInvoiceNotFailed(ctx context.Context, invoice *AccountInvoice) (bool, error)
	// DeleteInvoice deletes an invoice record by its ID.
	DeleteInvoice(ctx context.Context, id int64) error
	// ListInvoices lists invoices with pagination.
	ListInvoices(ctx context.Context, params InvoiceListParams) ([]AccountInvoice, int, error)
	// CreateInvoiceTitle creates a new invoice title record.
	CreateInvoiceTitle(ctx context.Context, title *AccountInvoiceTitle) error
	// UpdateInvoiceTitle updates an existing invoice title record.
	UpdateInvoiceTitle(ctx context.Context, title *AccountInvoiceTitle) error
	// UpdateInvoiceTitleNotDefault updates an existing invoice title record to not be the default.
	UpdateInvoiceTitleNotDefault(ctx context.Context, uid string) error
	// ListInvoiceTitles lists invoice titles.
	ListInvoiceTitles(ctx context.Context, params InvoiceListParams) ([]AccountInvoiceTitle, int, error)
	// GetInvoiceTitle retrieves a single invoice title by its ID.
	GetInvoiceTitle(ctx context.Context, titleID int64) (*AccountInvoiceTitle, error)
	// GetInvoiceTitleByTaxID retrieves a single invoice title by its user UUID and tax ID.
	GetInvoiceTitleByTaxID(ctx context.Context, userUUID, taxID string) (*AccountInvoiceTitle, error)
	// DeleteInvoiceTitle deletes an existing invoice title record.
	DeleteInvoiceTitle(ctx context.Context, titleID int64) error
	// GetBillingSummary retrieves the invoice dashboard summary.
	GetBillingSummary(ctx context.Context, params BillingSummaryParams) (*BillingSummary, error)
	// GetInvoicableRecharges retrieves paid recharges of a user that are not
	// attached to any non-failed invoice.
	GetInvoicableRecharges(ctx context.Context, params InvoicableRechargeFilter) ([]*AccountRecharge, int, error)
	// ListInvoiceRecharges retrieves the recharge orders associated with an invoice.
	ListInvoiceRecharges(ctx context.Context, invoiceID int64) ([]AccountInvoiceRecharge, error)
	// ListInvoiceRechargesByInvoiceIDs retrieves the recharge orders associated
	// with the given invoices.
	ListInvoiceRechargesByInvoiceIDs(ctx context.Context, invoiceIDs []int64) ([]AccountInvoiceRecharge, error)
}

type accountInvoiceImpl struct {
	db *DB
}

var _ AccountInvoiceStore = (*accountInvoiceImpl)(nil)

func NewAccountInvoiceStore() AccountInvoiceStore {
	return &accountInvoiceImpl{db: defaultDB}
}

func NewAccountInvoiceStoreWithDB(db *DB) AccountInvoiceStore {
	return &accountInvoiceImpl{db: db}
}

// CreateInvoiceWithRecharges creates an invoice and links it to the given
// recharge orders in one transaction.
func (a *accountInvoiceImpl) CreateInvoiceWithRecharges(ctx context.Context, invoice *AccountInvoice, recharges []*AccountRecharge) error {
	if len(recharges) == 0 {
		return fmt.Errorf("create invoice with recharges: no recharge orders given")
	}
	return a.db.Core.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		// Serialize concurrent invoice applications of the same user. Under
		// READ COMMITTED the invoicability re-check below is a single
		// statement whose snapshot may not see links committed by a
		// concurrent transaction after its row lock wait ends, so locking
		// rows alone cannot prevent double-invoicing. The per-user
		// transaction-scoped advisory lock guarantees the re-check runs only
		// after any earlier invoice creation of this user has committed or
		// rolled back.
		if _, err := tx.NewRaw("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", invoice.UserUUID).Exec(ctx); err != nil {
			return fmt.Errorf("acquire invoice lock of user %s, error: %w", invoice.UserUUID, err)
		}
		// Lock the recharge rows and re-check their invoicability inside the
		// transaction so that concurrent applications for the same orders are
		// serialized instead of double-invoicing them.
		orderNos := make([]string, 0, len(recharges))
		for _, recharge := range recharges {
			orderNos = append(orderNos, recharge.OrderNo)
		}
		var locked []*AccountRecharge
		err := invoicableRechargeQuery(tx, InvoicableRechargeFilter{
			UserUUID: invoice.UserUUID,
			OrderNos: orderNos,
		}).For("UPDATE").Scan(ctx, &locked)
		if err != nil {
			return fmt.Errorf("lock invoicable recharges of user %s, error: %w", invoice.UserUUID, err)
		}
		if len(locked) != len(recharges) {
			return ErrRechargeNotInvoicable
		}

		res, err := tx.NewInsert().Model(invoice).Exec(ctx)
		if assertAffectedOneRow(res, err) != nil {
			return err
		}

		links := make([]AccountInvoiceRecharge, 0, len(recharges))
		for _, recharge := range recharges {
			links = append(links, AccountInvoiceRecharge{
				InvoiceID:      int64(invoice.ID),
				RechargeUUID:   recharge.RechargeUUID,
				UserUUID:       recharge.UserUUID,
				AmountCents:    recharge.Amount,
				Currency:       recharge.Currency,
				OrderNo:        recharge.OrderNo,
				RechargeTime:   bun.NullTime{Time: recharge.TimeSucceeded},
				PaymentChannel: string(recharge.Channel),
			})
		}
		res, err = tx.NewInsert().Model(&links).Exec(ctx)
		if assertAffectedXRows(int64(len(links)), res, err) != nil {
			return err
		}
		return nil
	})
}

// UpdateInvoice implements the method to update an existing invoice.
func (a *accountInvoiceImpl) UpdateInvoice(ctx context.Context, invoice *AccountInvoice) error {
	// Assume using bun for database update operation
	res, err := a.db.Core.NewUpdate().
		Model(invoice).
		Where("id = ?", invoice.ID).
		Exec(ctx)
	if assertAffectedOneRow(res, err) != nil {
		return err
	}
	return nil
}

// UpdateInvoiceNotFailed updates an existing invoice record only when its
// stored status is not failed, and reports whether a row was updated. The
// condition is evaluated atomically inside the UPDATE against the latest
// committed row version, so a concurrent update that moved the invoice to
// the terminal failed state between the caller's read and this write
// reliably yields false instead of reviving it.
func (a *accountInvoiceImpl) UpdateInvoiceNotFailed(ctx context.Context, invoice *AccountInvoice) (bool, error) {
	res, err := a.db.Core.NewUpdate().
		Model(invoice).
		Where("id = ?", invoice.ID).
		Where("status != ?", InvoiceStatusFailed).
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("update invoice %d, error: %w", invoice.ID, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("retrieving affected row count of invoice %d update: %w", invoice.ID, err)
	}
	return affected == 1, nil
}

// ListInvoices implements the method to list invoices with pagination.
func (a *accountInvoiceImpl) ListInvoices(ctx context.Context, params InvoiceListParams) ([]AccountInvoice, int, error) {
	var invoices []AccountInvoice
	query := a.db.Core.NewSelect().
		Model(&invoices)

	if params.UserUUID != "" {
		query = query.Where("user_uuid =?", params.UserUUID)
	}

	if params.Search != "" {
		search := "%" + params.Search + "%"
		query = query.WhereGroup("AND", func(q *bun.SelectQuery) *bun.SelectQuery {
			q = q.Where("invoice_title ILIKE ?", search).
				WhereOr("bank_name ILIKE?", search).
				WhereOr("bank_account ILIKE?", search).
				WhereOr("user_name ILIKE?", search).
				WhereOr("taxpayer_id ILIKE?", search).
				WhereOr("TO_CHAR(apply_time, 'YYYY-MM-DD') ILIKE?", search)
			return q
		})
	}

	if params.Status != "" {
		query = query.Where("status =?", params.Status)
	}

	if params.Sort != "" {
		query = query.Order(params.Sort)
	}

	count, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	err = query.Offset((params.Page-1)*params.PageSize).
		Limit(params.PageSize).
		Scan(ctx, &invoices)
	if err != nil {
		return nil, 0, err
	}

	return invoices, count, nil
}

// CreateInvoiceTitle implements the method to create a new invoice title.
func (a *accountInvoiceImpl) CreateInvoiceTitle(ctx context.Context, title *AccountInvoiceTitle) error {
	res, err := a.db.Core.NewInsert().Model(title).Exec(ctx)
	if assertAffectedOneRow(res, err) != nil {
		return err
	}
	return nil
}

// UpdateInvoiceTitle implements the method to update an existing invoice title.
func (a *accountInvoiceImpl) UpdateInvoiceTitle(ctx context.Context, title *AccountInvoiceTitle) error {
	res, err := a.db.Core.NewUpdate().
		Model(title).
		Where("id = ?", title.ID).
		Exec(ctx)
	if assertAffectedOneRow(res, err) != nil {
		return err
	}
	return nil
}

func (a *accountInvoiceImpl) UpdateInvoiceTitleNotDefault(ctx context.Context, uid string) error {
	_, err := a.db.Core.NewUpdate().
		Model(&AccountInvoiceTitle{}).
		Where("user_uuid =?", uid).
		Set("is_default =?", false).
		Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

// ListInvoiceTitles implements the method to list invoice titles.
func (a *accountInvoiceImpl) ListInvoiceTitles(ctx context.Context, params InvoiceListParams) ([]AccountInvoiceTitle, int, error) {
	var titles []AccountInvoiceTitle
	query := a.db.Core.NewSelect().
		Model(&titles)

	if params.UserUUID != "" {
		query = query.Where("user_uuid =?", params.UserUUID)
	}

	if params.Search != "" {
		search := "%" + params.Search + "%"
		query = query.WhereGroup("AND", func(q *bun.SelectQuery) *bun.SelectQuery {
			q = q.Where("title ILIKE ?", search)
			return q
		})
	}

	query = query.OrderExpr("is_default DESC")
	count, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	err = query.Offset((params.Page-1)*params.PageSize).
		Limit(params.PageSize).
		Scan(ctx, &titles)
	if err != nil {
		return nil, 0, err
	}

	return titles, count, nil
}

// GetInvoiceTitle implements the method to retrieve a single invoice title by its ID.
func (a *accountInvoiceImpl) GetInvoiceTitle(ctx context.Context, titleID int64) (*AccountInvoiceTitle, error) {
	var title AccountInvoiceTitle
	err := a.db.Core.NewSelect().
		Model(&title).
		Where("id = ?", titleID).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &title, nil
}

// DeleteInvoiceTitle implements the method to delete an existing invoice title.
func (a *accountInvoiceImpl) DeleteInvoiceTitle(ctx context.Context, titleID int64) error {
	res, err := a.db.Core.NewDelete().
		Model(&AccountInvoiceTitle{}).
		Where("id =?", titleID).
		Exec(ctx)
	if assertAffectedOneRow(res, err) != nil {
		return err
	}
	return nil
}

// GetInvoiceTitleByTaxID retrieves a single invoice title by its user UUID and tax ID.
func (a *accountInvoiceImpl) GetInvoiceTitleByTaxID(ctx context.Context, userUUID, taxID string) (*AccountInvoiceTitle, error) {
	var title AccountInvoiceTitle
	err := a.db.Core.NewSelect().
		Model(&title).
		Where("user_uuid = ?", userUUID).
		Where("tax_id = ?", taxID).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &title, nil
}

// AccountInvoice represents an invoice record.
type AccountInvoice struct {
	ID            int       `bun:"id,pk,autoincrement"`
	UserUUID      string    `bun:"user_uuid,notnull" json:"user_uuid"`
	UserName      string    `bun:"user_name,notnull" json:"user_name"`              // User name
	TitleType     string    `bun:"title_type,notnull" json:"title_type"`            // Invoice title type
	InvoiceType   string    `bun:"invoice_type,notnull" json:"invoice_type"`        // Invoice type
	BillCycle     string    `bun:"bill_cycle,notnull" json:"bill_cycle"`            // Billing cycle
	InvoiceTitle  string    `bun:"invoice_title,notnull" json:"invoice_title"`      // Invoice title
	ApplyTime     time.Time `bun:"apply_time,type:timestamp" json:"apply_time"`     // Invoice application time
	InvoiceAmount float64   `bun:"invoice_amount,notnull" json:"invoice_amount"`    // Invoice amount
	Status        string    `bun:"status,notnull" json:"status"`                    // Invoice status
	Reason        string    `bun:"reason,notnull" json:"reason"`                    // Reason
	InvoiceDate   time.Time `bun:"invoice_date,type:timestamp" json:"invoice_date"` // Invoice issuance date
	InvoiceURL    string    `bun:"invoice_url,notnull" json:"invoice_url"`          // Invoice URL
	// Redundant fields for list display
	TaxpayerID     string `bun:"taxpayer_id,notnull" json:"taxpayer_id"`         // Taxpayer identification number
	BankName       string `bun:"bank_name,notnull" json:"bank_name"`             // Bank name
	BankAccount    string `bun:"bank_account,notnull" json:"bank_account"`       // Bank account number
	RegisteredAddr string `bun:"registered_addr,notnull" json:"registered_addr"` // Registered address
	ContactPhone   string `bun:"contact_phone,notnull" json:"contact_phone"`     // Contact phone number
	Email          string `bun:"email,notnull" json:"email"`                     // Email address

	times
}

// AccountInvoiceTitle represents an invoice title record.
type AccountInvoiceTitle struct {
	ID           int64  `bun:"id,pk,autoincrement"`
	UserUUID     string `bun:"user_uuid,notnull" json:"user_uuid"` // User
	UserName     string `bun:"user_name,notnull" json:"user_name"`
	Title        string `bun:"title,notnull" json:"title"` // Invoice title name
	TitleType    string `bun:",notnull" json:"title_type"` // Invoice title type
	InvoiceType  string `bun:"invoice_type,notnull" json:"invoice_type"`
	TaxID        string `bun:"tax_id,notnull" json:"tax_id"`       // Taxpayer identification number
	Address      string `bun:"address" json:"address"`             // Registered address
	BankName     string `bun:"bank_name" json:"bank_name"`         // Bank name
	BankAccount  string `bun:"bank_account" json:"bank_account"`   // Bank account number
	ContactPhone string `bun:"contact_phone" json:"contact_phone"` // Contact phone number
	Email        string `bun:"email" json:"email"`                 // Email address
	IsDefault    bool   `bun:"is_default" json:"is_default"`       // Whether it is the default title

	times
}

// AccountInvoiceRecharge is the snapshot of a recharge order included in an invoice.
type AccountInvoiceRecharge struct {
	ID             int64  `bun:",pk,autoincrement"`
	InvoiceID      int64  `bun:"invoice_id,notnull"`
	RechargeUUID   string `bun:"recharge_uuid,notnull"`
	UserUUID       string `bun:"user_uuid,notnull"`
	AmountCents    int64  `bun:"amount_cents,notnull"`
	Currency       string `bun:"currency,notnull,default:'CNY'"`
	OrderNo        string `bun:"order_no,notnull"`
	RechargeTime   bun.NullTime
	PaymentChannel string `bun:"payment_channel,notnull,default:''"`

	times
}

// InvoicableRechargeFilter selects the paid recharges of a user that are not
// invoiced yet.
type InvoicableRechargeFilter struct {
	UserUUID   string
	OrderNos   []string
	StartMonth string // inclusive, format "YYYY-MM"
	EndMonth   string // exclusive, format "YYYY-MM"
	Page       int
	PageSize   int
}

// invoicableRechargeQuery builds the base query for paid recharges that are not
// attached to any non-failed invoice.
func invoicableRechargeQuery(idb bun.IDB, params InvoicableRechargeFilter) *bun.SelectQuery {
	query := idb.NewSelect().
		Model((*AccountRecharge)(nil)).
		Where("account_recharge.user_uuid = ?", params.UserUUID).
		Where("succeeded = ?", true).
		Where("closed = ?", false).
		Where("NOT EXISTS (SELECT 1 FROM account_invoice_recharges AS air JOIN account_invoices AS ai ON ai.id = air.invoice_id WHERE air.recharge_uuid = account_recharge.recharge_uuid AND ai.status != ?)", InvoiceStatusFailed)
	if len(params.OrderNos) > 0 {
		query = query.Where("order_no IN (?)", bun.In(params.OrderNos))
	}
	if params.StartMonth != "" {
		query = query.Where("time_succeeded >= ?", params.StartMonth+"-01")
	}
	if params.EndMonth != "" {
		query = query.Where("time_succeeded < ?", params.EndMonth+"-01")
	}
	return query
}

// GetInvoicableRecharges implements the method to list paid recharges that are
// not invoiced yet.
func (a *accountInvoiceImpl) GetInvoicableRecharges(ctx context.Context, params InvoicableRechargeFilter) ([]*AccountRecharge, int, error) {
	query := invoicableRechargeQuery(a.db.Core, params).Order("time_succeeded DESC")

	count, err := query.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count invoicable recharges for user %s, error: %w", params.UserUUID, err)
	}

	if params.Page > 0 && params.PageSize > 0 {
		query = query.Offset((params.Page - 1) * params.PageSize).Limit(params.PageSize)
	}

	var recharges []*AccountRecharge
	err = query.Scan(ctx, &recharges)
	if err != nil {
		return nil, 0, fmt.Errorf("list invoicable recharges for user %s, error: %w", params.UserUUID, err)
	}

	return recharges, count, nil
}

// ListInvoiceRecharges implements the method to list the recharge orders of an invoice.
func (a *accountInvoiceImpl) ListInvoiceRecharges(ctx context.Context, invoiceID int64) ([]AccountInvoiceRecharge, error) {
	var links []AccountInvoiceRecharge
	err := a.db.Core.NewSelect().
		Model(&links).
		Where("invoice_id = ?", invoiceID).
		Order("id ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recharges of invoice %d, error: %w", invoiceID, err)
	}
	return links, nil
}

// ListInvoiceRechargesByInvoiceIDs implements the method to list the recharge
// orders of the given invoices in one query, ordered by link id.
func (a *accountInvoiceImpl) ListInvoiceRechargesByInvoiceIDs(ctx context.Context, invoiceIDs []int64) ([]AccountInvoiceRecharge, error) {
	if len(invoiceIDs) == 0 {
		return nil, nil
	}
	var links []AccountInvoiceRecharge
	err := a.db.Core.NewSelect().
		Model(&links).
		Where("invoice_id IN (?)", bun.In(invoiceIDs)).
		Order("id ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recharges of invoices %v, error: %w", invoiceIDs, err)
	}
	return links, nil
}

// BillingSummary defines the structure of the billing summary result.
type BillingSummary struct {
	CurrentMonthNonInvoicable float64 `json:"current_month_non_invoicable"`
	InvoicedAmount            float64 `json:"invoiced_amount"`
	UninvoicedAmount          float64 `json:"uninvoiced_amount"`
}

type BillingSummaryParams struct {
	UserUUID   string
	StartMonth string
	EndMonth   string
}

func (a *accountInvoiceImpl) GetBillingSummary(ctx context.Context, params BillingSummaryParams) (*BillingSummary, error) {
	// Sum the paid recharges in the range that are not attached to a
	// non-failed invoice. Current-month recharges are invoicable right away,
	// so there is no non-invoicable amount anymore.
	invoicableFilter := InvoicableRechargeFilter{
		UserUUID:   params.UserUUID,
		StartMonth: params.StartMonth,
		EndMonth:   params.EndMonth,
	}
	var uninvoicedCents int64
	err := invoicableRechargeQuery(a.db.Core, invoicableFilter).
		ColumnExpr("COALESCE(SUM(amount), 0)").
		Scan(ctx, &uninvoicedCents)
	if err != nil {
		return nil, fmt.Errorf("sum uninvoiced recharge amount for user %s, error: %w", params.UserUUID, err)
	}

	// Sum the invoices applied within the range, excluding failed ones.
	// apply_time covers both legacy bill-cycle invoices and recharge-based ones.
	invoiceQuery := a.db.Core.NewSelect().
		Model((*AccountInvoice)(nil)).
		ColumnExpr("COALESCE(SUM(invoice_amount), 0)").
		Where("user_uuid = ?", params.UserUUID).
		Where("status != ?", InvoiceStatusFailed)
	if params.StartMonth != "" {
		invoiceQuery = invoiceQuery.Where("apply_time >= ?", params.StartMonth+"-01")
	}
	if params.EndMonth != "" {
		invoiceQuery = invoiceQuery.Where("apply_time < ?", params.EndMonth+"-01")
	}

	var invoicedAmount float64
	err = invoiceQuery.Scan(ctx, &invoicedAmount)
	if err != nil {
		return nil, fmt.Errorf("sum invoiced amount for user %s, error: %w", params.UserUUID, err)
	}

	return &BillingSummary{
		InvoicedAmount:   invoicedAmount,
		UninvoicedAmount: float64(uninvoicedCents) / 100.0,
	}, nil
}

func (a *accountInvoiceImpl) DeleteInvoice(ctx context.Context, id int64) error {
	res, err := a.db.Core.NewDelete().
		Model(&AccountInvoice{}).
		Where("id =?", id).
		Exec(ctx)
	if assertAffectedOneRow(res, err) != nil {
		return err
	}
	return nil
}

func (a *accountInvoiceImpl) GetInvoice(ctx context.Context, id int64) (*AccountInvoice, error) {
	var invoice AccountInvoice
	err := a.db.Core.NewSelect().
		Model(&invoice).
		Where("id =?", id).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &invoice, nil
}

package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/uptrace/bun"
)

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
	// CreateInvoice creates a new invoice record.
	CreateInvoice(ctx context.Context, invoice *AccountInvoice) error
	// GetInvoice retrieves a single invoice by its ID.
	GetInvoice(ctx context.Context, id int64) (*AccountInvoice, error)
	// GetInvoiceByBillCycle retrieves an invoice by its bill cycle and user UUID.
	GetInvoiceByBillCycle(ctx context.Context, billCycle, userUUID string) (*AccountInvoice, error)
	// UpdateInvoice updates an existing invoice record.
	UpdateInvoice(ctx context.Context, invoice *AccountInvoice) error
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
	// GetBillingSummary retrieves the billing summary.
	GetBillingSummary(ctx context.Context, params BillingSummaryParams) (*BillingSummary, error)
	// GetInvoicableList retrieves the list of invoicable items.
	GetInvoicableList(ctx context.Context, params PagedRequest) ([]Invoicable, int, error)
	// GetBillAmount retrieves the bill amount for a specific user and bill month.
	GetBillAmount(ctx context.Context, uid string, billMonth string) (float64, error)
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

// CreateInvoice implements the method to create a new invoice.
func (a *accountInvoiceImpl) CreateInvoice(ctx context.Context, invoice *AccountInvoice) error {
	res, err := a.db.Core.NewInsert().Model(invoice).Exec(ctx)
	if assertAffectedOneRow(res, err) != nil {
		return err
	}
	return nil
}

// GetInvoiceByBillCycle retrieves an invoice by its bill cycle and user UUID.
func (a *accountInvoiceImpl) GetInvoiceByBillCycle(ctx context.Context, billCycle, userUUID string) (*AccountInvoice, error) {
	var invoice AccountInvoice
	err := a.db.Core.NewSelect().
		Model(&invoice).
		Where("bill_cycle = ?", billCycle).
		Where("user_uuid =?", userUUID).
		Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &invoice, nil
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

type Invoicable struct {
	BillCycle     string
	InvoiceAmount float64
}

type PagedRequest struct {
	UserUUID   string `json:"user_uuid"` // Specify the user's UUID
	Page       int    `json:"page"`      // Current page number
	PageSize   int    `json:"page_size"` // Number of items per page
	Search     string `json:"search"`    // Search field
	Sort       string `json:"sort"`      // e.g., "ASC" or "DESC"
	StartMonth string
	EndMonth   string
}

// GetInvoicableList implements the method to list invoicable items.
func (a *accountInvoiceImpl) GetInvoicableList(ctx context.Context, params PagedRequest) ([]Invoicable, int, error) {
	var invoicables []Invoicable
	subQuery := a.db.Core.NewSelect().
		Model((*AccountBill)(nil)).
		ColumnExpr("TO_CHAR(bill_date, 'YYYY-MM') AS bill_cycle").
		ColumnExpr("COALESCE(SUM(ABS(value)), 0) AS invoice_amount").
		Where("user_uuid = ?", params.UserUUID).
		Where("bill_date >= ?", params.StartMonth+"-01").
		Where("bill_date < ?", params.EndMonth+"-01").
		GroupExpr("TO_CHAR(bill_date, 'YYYY-MM')")

	query := a.db.Core.NewSelect().
		With("mb", subQuery).
		Column("mb.bill_cycle", "mb.invoice_amount").
		Table("mb").
		Where("NOT EXISTS (SELECT 1 FROM account_invoices AS account_invoice WHERE account_invoice.bill_cycle = mb.bill_cycle AND account_invoice.user_uuid = ?)", params.UserUUID).
		Order("mb.bill_cycle DESC")

	if params.Page > 0 && params.PageSize > 0 {
		query = query.Offset((params.Page - 1) * params.PageSize).Limit(params.PageSize)
	}

	count, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	err = query.Scan(ctx, &invoicables)
	if err != nil {
		return nil, 0, err
	}

	return invoicables, count, nil
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
	now := time.Now()
	currentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	nextMonth := currentMonth.AddDate(0, 1, 0)

	// Calculate the total value of the current month, i.e., the temporarily non-invoicable amount
	var currentMonthNonInvoicable float64
	err := a.db.Core.NewSelect().
		Model((*AccountBill)(nil)).
		ColumnExpr("COALESCE(SUM(ABS(value)), 0)").
		Where("user_uuid = ?", params.UserUUID).
		Where("bill_date >= ?", currentMonth.Format("2006-01-02")).
		Where("bill_date < ?", nextMonth.Format("2006-01-02")).
		Scan(ctx, &currentMonthNonInvoicable)
	if err != nil {
		return nil, err
	}

	// Calculate the total bill amount within the specified time range
	var totalBillAmount float64
	billQuery := a.db.Core.NewSelect().
		Model((*AccountBill)(nil)).
		ColumnExpr("COALESCE(SUM(ABS(value)), 0)").
		Where("user_uuid = ?", params.UserUUID)

	if params.StartMonth != "" {
		billQuery = billQuery.Where("date_trunc('month', bill_date) >= date_trunc('month', ?::timestamp)", params.StartMonth+"-01")
	}
	if params.EndMonth != "" {
		billQuery = billQuery.Where("date_trunc('month', bill_date) < date_trunc('month', ?::timestamp) ", params.EndMonth+"-01")
	}

	// Exclude the bills of the current month
	billQuery = billQuery.Where("bill_date < ?", currentMonth)

	err = billQuery.Scan(ctx, &totalBillAmount)
	if err != nil {
		return nil, err
	}

	// Calculate the invoiced amount within the specified time range
	var invoicedAmount float64
	invoiceQuery := a.db.Core.NewSelect().
		Model((*AccountInvoice)(nil)).
		ColumnExpr("SUM(invoice_amount)").
		Where("user_uuid = ?", params.UserUUID)

	if params.StartMonth != "" {
		invoiceQuery = invoiceQuery.Where("bill_cycle >= ?", params.StartMonth)
	}
	if params.EndMonth != "" {
		invoiceQuery = invoiceQuery.Where("bill_cycle < ?", params.EndMonth)
	}

	err = invoiceQuery.Scan(ctx, &invoicedAmount)
	if err != nil {
		return nil, err
	}

	// Calculate the uninvoiced amount
	uninvoicedAmount := totalBillAmount - invoicedAmount

	return &BillingSummary{
		CurrentMonthNonInvoicable: currentMonthNonInvoicable,
		InvoicedAmount:            invoicedAmount,
		UninvoicedAmount:          uninvoicedAmount,
	}, nil
}

func (a *accountInvoiceImpl) GetBillAmount(ctx context.Context, uid string, billMonth string) (float64, error) {
	var totalBillAmount float64
	query := a.db.Core.NewSelect().
		Model((*AccountBill)(nil)).
		ColumnExpr("COALESCE(SUM(ABS(value)), 0)").
		Where("user_uuid =?", uid).
		Where("to_char(bill_date, 'YYYY-MM') = ?", billMonth)

	err := query.Scan(ctx, &totalBillAmount)
	if err != nil {
		return 0, err
	}
	return totalBillAmount, nil
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

package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

var startNo = "000001"

type accountVoucherStoreImpl struct {
	db *DB
}

type AccountVoucherStore interface {
	Create(ctx context.Context, input AccountVoucher) (*AccountVoucher, error)
	Update(ctx context.Context, input AccountVoucher) (*AccountVoucher, error)
	Delete(ctx context.Context, id int64) error
	GetByID(ctx context.Context, id int64) (*AccountVoucher, error)
	GetByVoucherNo(ctx context.Context, voucherNo string) (*AccountVoucher, error)
	GetLast(ctx context.Context) (*AccountVoucher, error)
	List(ctx context.Context, filter types.VoucherFilter) ([]AccountVoucher, int, error)
	ListByTargetUUID(ctx context.Context, filter types.VoucherNamespaceFilter) ([]AccountVoucher, int, error)
	UpdateStatus(ctx context.Context, id int64, status types.VoucherStatus) (*AccountVoucher, error)
	RefreshStatus(ctx context.Context) error
	GetDashboard(ctx context.Context, req types.VoucherDashboardReq) ([]VoucherDashboardResult, error)
}

func NewAccountVoucherStore() AccountVoucherStore {
	return &accountVoucherStoreImpl{
		db: defaultDB,
	}
}

func NewAccountVoucherStoreWithDB(db *DB) AccountVoucherStore {
	return &accountVoucherStoreImpl{
		db: db,
	}
}

type AccountVoucher struct {
	ID         int64                `bun:",pk,autoincrement" json:"id"`
	VoucherNo  string               `bun:",notnull,unique" json:"voucher_no"`
	TargetType NamespaceType        `bun:",notnull" json:"target_type"` // user or org
	TargetUUID string               `bun:",notnull" json:"target_uuid"`
	TargetName string               `bun:",notnull" json:"target_name"`
	Total      float64              `bun:",notnull,default:0" json:"total"`
	Used       float64              `bun:",notnull,default:0" json:"used"`
	BeginDate  time.Time            `bun:",notnull" json:"begin_date"`
	EndDate    time.Time            `bun:",notnull" json:"end_date"`
	Status     types.VoucherStatus  `bun:",notnull" json:"status"`
	Rules      []types.VoucherRules `bun:",type:jsonb,nullzero" json:"rules"`
	Notes      string               `bun:",nullzero" json:"notes"`
	IssueUUID  string               `bun:",notnull" json:"issue_uuid"`
	IssueName  string               `bun:",notnull" json:"issue_name"`
	times
}

func (a *accountVoucherStoreImpl) Create(ctx context.Context, input AccountVoucher) (*AccountVoucher, error) {
	err := a.db.Operator.Core.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		voucherNo, err := genVoucherNo(ctx, tx)
		if err != nil {
			return fmt.Errorf("generate voucher number: %w", err)
		}
		input.VoucherNo = voucherNo
		res, err := tx.NewInsert().Model(&input).Exec(ctx, &input)
		if err := assertAffectedOneRow(res, err); err != nil {
			return fmt.Errorf("insert account voucher: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create account voucher in transaction: %w", err)
	}
	return &input, nil
}

func (a *accountVoucherStoreImpl) Update(ctx context.Context, input AccountVoucher) (*AccountVoucher, error) {
	_, err := a.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("id", input.ID))
	}
	return &input, nil
}

func (a *accountVoucherStoreImpl) Delete(ctx context.Context, id int64) error {
	_, err := a.db.Core.NewDelete().Model(&AccountVoucher{}).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("id", id))
	}
	return nil
}

func (a *accountVoucherStoreImpl) GetByID(ctx context.Context, id int64) (*AccountVoucher, error) {
	voucher := &AccountVoucher{}
	err := a.db.Core.NewSelect().Model(voucher).Where("id = ?", id).Scan(ctx, voucher)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("id", id))
	}
	return voucher, nil
}

func (a *accountVoucherStoreImpl) GetByVoucherNo(ctx context.Context, voucherNo string) (*AccountVoucher, error) {
	voucher := &AccountVoucher{}
	err := a.db.Core.NewSelect().Model(voucher).Where("voucher_no = ?", voucherNo).Scan(ctx, voucher)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("voucher_no", voucherNo))
	}
	return voucher, nil
}

func (a *accountVoucherStoreImpl) GetLast(ctx context.Context) (*AccountVoucher, error) {
	voucher := &AccountVoucher{}
	err := a.db.Core.NewSelect().Model(voucher).Order("id DESC").Limit(1).Scan(ctx, voucher)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("operation", "get_last_voucher"))
	}
	return voucher, nil
}

func (a *accountVoucherStoreImpl) UpdateStatus(ctx context.Context, id int64, status types.VoucherStatus) (*AccountVoucher, error) {
	voucher := &AccountVoucher{}
	_, err := a.db.Core.NewUpdate().Model(voucher).
		Set("status = ?", status).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("id", id))
	}
	return voucher, nil
}

func (a *accountVoucherStoreImpl) List(ctx context.Context, filter types.VoucherFilter) ([]AccountVoucher, int, error) {
	var vouchers []AccountVoucher
	q := a.db.Core.NewSelect().Model(&vouchers)

	if len(filter.TargetType) > 0 {
		q = q.Where("target_type = ?", filter.TargetType)
	}
	if len(filter.Status) > 0 {
		q = q.Where("status = ?", filter.Status)
	}

	if len(strings.TrimSpace(filter.Search)) > 0 {
		search := strings.TrimSpace(filter.Search)
		if search != "" {
			searchPattern := "%" + search + "%"
			q = q.Where("LOWER(voucher_no) LIKE LOWER(?) OR LOWER(target_name) LIKE LOWER(?)", searchPattern, searchPattern)
		}
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, errorx.Ctx().Set("filter", filter))
	}

	if filter.Per > 0 && filter.Page > 0 {
		q = q.Limit(filter.Per).Offset((filter.Page - 1) * filter.Per)
	}

	err = q.Order("id DESC").Scan(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, errorx.Ctx().Set("filter", filter))
	}

	return vouchers, total, nil
}

func (a *accountVoucherStoreImpl) RefreshStatus(ctx context.Context) error {
	err := a.db.Operator.Core.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		return updateVouchersStatus(ctx, tx)
	})
	if err != nil {
		return errorx.HandleDBError(err, nil)
	}
	return nil
}

type VoucherDashboardResult struct {
	Status types.VoucherStatus `json:"status"`
	Total  float64             `json:"total"`
	Used   float64             `json:"used"`
	Count  int                 `json:"count"`
}

func (a *accountVoucherStoreImpl) GetDashboard(ctx context.Context, req types.VoucherDashboardReq) ([]VoucherDashboardResult, error) {
	var results []VoucherDashboardResult
	err := a.db.Core.NewSelect().
		Model((*AccountVoucher)(nil)).
		ColumnExpr("status, SUM(total) AS total, SUM(used) AS used, COUNT(*) AS count").
		Where("target_uuid = ?", req.TargetUUID).
		Group("status").
		Scan(ctx, &results)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("target_uuid", req.TargetUUID))
	}
	return results, nil
}

func updateVouchersStatus(ctx context.Context, tx bun.Tx) error {
	now := time.Now()

	// Pending vouchers whose begin_date has passed → active
	_, err := tx.NewUpdate().Model(&AccountVoucher{}).
		Set("status = ?", types.VoucherStatusActive).
		Set("updated_at = ?", now).
		Where("status = ?", types.VoucherStatusPending).
		Where("begin_date <= ?", now).
		Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("refresh", "pending"))
	}

	// Active vouchers whose end_date has passed → expired
	_, err = tx.NewUpdate().Model(&AccountVoucher{}).
		Set("status = ?", types.VoucherStatusExpired).
		Set("updated_at = ?", now).
		Where("status = ?", types.VoucherStatusActive).
		Where("end_date <= ?", now).
		Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("refresh", "active"))
	}

	return nil
}

func genVoucherNo(ctx context.Context, db bun.IDB) (string, error) {
	var vouchers []AccountVoucher
	err := db.NewSelect().Model(&vouchers).Order("id DESC").Limit(1).Scan(ctx)
	if err != nil {
		return "", fmt.Errorf("get last voucher for generating voucher number: %w", err)
	}
	year := time.Now().Year()
	prefix := fmt.Sprintf("VC-%d-", year)
	if len(vouchers) == 0 {
		return prefix + startNo, nil
	}
	lastYear := extractVoucherYear(vouchers[0].VoucherNo)
	if lastYear != year {
		return prefix + startNo, nil
	}
	parts := strings.Split(vouchers[0].VoucherNo, "-")
	if len(parts) < 3 {
		return prefix + startNo, nil
	}
	numStr := parts[2]
	seq, err := parseVoucherSeq(numStr)
	if err != nil || seq < 1 {
		return prefix + startNo, nil
	}
	return prefix + fmt.Sprintf("%06d", seq+1), nil
}

func extractVoucherYear(voucherNo string) int {
	parts := strings.Split(voucherNo, "-")
	if len(parts) < 3 {
		return 0
	}
	yearStr := parts[1]
	n, err := strconv.Atoi(yearStr)
	if err != nil {
		return 0
	}
	return n
}

func parseVoucherSeq(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func (a *accountVoucherStoreImpl) ListByTargetUUID(ctx context.Context, filter types.VoucherNamespaceFilter) ([]AccountVoucher, int, error) {
	var vouchers []AccountVoucher
	q := a.db.Core.NewSelect().Model(&vouchers)

	q = q.Where("target_uuid = ?", filter.TargetUUID)

	if len(filter.Status) > 0 {
		q = q.Where("status = ?", filter.Status)
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, errorx.Ctx().Set("filter", filter))
	}

	if filter.Per > 0 && filter.Page > 0 {
		q = q.Limit(filter.Per).Offset((filter.Page - 1) * filter.Per)
	}

	err = q.Order("id DESC").Scan(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, errorx.Ctx().Set("filter", filter))
	}

	return vouchers, total, nil
}

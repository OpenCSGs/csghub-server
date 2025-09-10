package database

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/payment/consts"
)

type accountRechargeStoreImpl struct {
	db *DB
}

type AccountRechargeStore interface {
	CreateRecharge(ctx context.Context, recharge *AccountRecharge) error
	GetRecharge(ctx context.Context, rechargeUUID string) (*AccountRecharge, error)
	GetRechargeByOrderNo(ctx context.Context, orderNo string) (*AccountRecharge, error)
	UpdateRecharge(ctx context.Context, recharge *AccountRecharge) error
	ListRechargeByUserUUID(ctx context.Context, userUUID string, limit, offset int) ([]*AccountRecharge, error)
	ListRecharges(ctx context.Context, userUUID string, filter RechargeFilter) ([]*AccountRecharge, error)
	CountRecharges(ctx context.Context, userUUID string, filter RechargeFilter) (*types.RechargeStats, error)
}

func NewAccountRechargeStore() AccountRechargeStore {
	return &accountRechargeStoreImpl{
		db: defaultDB,
	}
}

func NewAccountRechargeStoreWithDB(db *DB) AccountRechargeStore {
	return &accountRechargeStoreImpl{
		db: db,
	}
}

type AccountRecharge struct {
	RechargeUUID  string                `bun:",notnull,pk,skipupdate" json:"uuid"`                // Recharge object ID
	OrderNo       string                `bun:",notnull,unique" json:"order_no"`                   // Order ID allowed by the payment system
	UserUUID      string                `bun:",notnull,skipupdate" json:"user_uuid"`              // Target UserUUID for the recharge
	FromUserUUID  string                `bun:",notnull,skipupdate" json:"from_user_uuid"`         // Source UserUUID for the recharge
	Amount        int64                 `bun:",notnull,skipupdate" json:"amount"`                 // Actual balance received by the user, in cents
	Currency      string                `bun:",notnull,skipupdate,default:'CNY'" json:"currency"` // 3-letter ISO currency code in uppercase letters
	Channel       consts.PaymentChannel `bun:",notnull,skipupdate" json:"channel"`
	PaymentUUID   string                `bun:",notnull,skipupdate,unique" json:"payment_uuid"`
	Succeeded     bool                  `json:"succeeded"`
	Closed        bool                  `json:"closed"`
	TimeSucceeded time.Time             `bun:",nullzero" json:"time_succeeded"`
	CreatedAt     time.Time             `bun:",notnull,skipupdate,default:current_timestamp" json:"created_at"`
	UpdatedAt     time.Time             `bun:",notnull,default:current_timestamp" json:"updated_at"`
	Description   string                `json:"description"`
}

func (rs *accountRechargeStoreImpl) CreateRecharge(ctx context.Context, recharge *AccountRecharge) error {
	_, err := rs.db.Operator.Core.NewInsert().Model(recharge).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create recharge record, error: %w", err)
	}
	return nil
}

func (rs *accountRechargeStoreImpl) GetRecharge(ctx context.Context, rechargeUUID string) (*AccountRecharge, error) {
	var recharge AccountRecharge
	q := rs.db.Operator.Core.NewSelect().
		Model(&recharge).
		Where("recharge_uuid = ?", rechargeUUID)
	err := q.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("get recharge, error: %w", err)
	}
	return &recharge, nil
}

func (rs *accountRechargeStoreImpl) GetRechargeByOrderNo(ctx context.Context, orderNo string) (*AccountRecharge, error) {
	var recharge AccountRecharge
	q := rs.db.Operator.Core.NewSelect().
		Model(&recharge).
		Where("account_recharge.order_no = ?", orderNo)
	err := q.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("get recharge, error: %w", err)
	}
	return &recharge, nil
}

func (rs *accountRechargeStoreImpl) UpdateRecharge(ctx context.Context, recharge *AccountRecharge) error {
	recharge.UpdatedAt = time.Now()
	_, err := rs.db.Operator.Core.NewUpdate().
		Model(recharge).
		Where("recharge_uuid = ?", recharge.RechargeUUID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update payment record, error: %w", err)
	}
	return nil
}

func (rs *accountRechargeStoreImpl) ListRechargeByUserUUID(ctx context.Context, userUUID string, limit, offset int) ([]*AccountRecharge, error) {
	var recharges []*AccountRecharge
	q := rs.db.Operator.Core.NewSelect().
		Model(&recharges).
		Where("user_uuid = ?", userUUID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset)
	err := q.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recharges for user %s, error: %w", userUUID, err)
	}
	return recharges, nil
}

func (rs *accountRechargeStoreImpl) applyRechargesFilters(q *bun.SelectQuery, filter RechargeFilter) *bun.SelectQuery {

	if filter.OrderNoPattern != "" {
		q.Where("account_recharge.order_no LIKE ?", "%"+filter.OrderNoPattern+"%")
	}

	// Filter by creation date
	if filter.StartDate != "" {
		q.Where("account_recharge.created_at >= ?", filter.StartDate)
	}
	if filter.EndDate != "" {
		q.Where("account_recharge.created_at <= ?", filter.EndDate)
	}

	if filter.Succeeded != nil {
		q.Where("succeeded = ?", *filter.Succeeded)
	}

	if filter.Closed != nil {
		q.Where("closed = ?", *filter.Closed)
	}

	// Join Payment table and filter by Channel
	if filter.PaymentChannel != nil {
		q.Where("channel = ?", *filter.PaymentChannel)
	}

	return q
}

func (rs *accountRechargeStoreImpl) ListRecharges(ctx context.Context, userUUID string, filter RechargeFilter) ([]*AccountRecharge, error) {
	var recharges []*AccountRecharge
	q := rs.db.Operator.Core.NewSelect().
		Model(&recharges).
		Order("created_at DESC")

	if userUUID != "" {
		q.Where("user_uuid = ?", userUUID)
	}

	// Pagination
	if filter.Limit > 0 {
		q.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		q.Offset(filter.Offset)
	}

	// Apply common filters
	q = rs.applyRechargesFilters(q, filter)

	err := q.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recharges for user %s, error: %w", userUUID, err)
	}
	return recharges, nil
}

func (rs *accountRechargeStoreImpl) CountRecharges(ctx context.Context, userUUID string, filter RechargeFilter) (*types.RechargeStats, error) {
	q := rs.db.Operator.Core.NewSelect().
		Model((*AccountRecharge)(nil)).
		ColumnExpr("COUNT(*) AS count").
		ColumnExpr("COALESCE(SUM(amount), 0) AS sum")

	if userUUID != "" {
		q.Where("user_uuid = ?", userUUID)
	}

	// Apply common filters
	q = rs.applyRechargesFilters(q, filter)

	stats := types.RechargeStats{}

	err := q.Scan(ctx, &stats)
	if err != nil {
		return nil, fmt.Errorf("count recharges for user %s, error: %w", userUUID, err)
	}
	return &stats, nil
}

type RechargeFilter struct {
	OrderNoPattern string
	StartDate      string
	EndDate        string
	Succeeded      *bool
	Closed         *bool
	PaymentChannel *consts.PaymentChannel
	Limit          int
	Offset         int
}

func (f *RechargeFilter) SetOrderNoPattern(pattern string) *RechargeFilter {
	f.OrderNoPattern = pattern
	return f
}

func (f *RechargeFilter) SetDateRange(start, end string) *RechargeFilter {
	f.StartDate = start
	f.EndDate = end
	return f
}

func (f *RechargeFilter) SetSucceeded(succeeded bool) *RechargeFilter {
	f.Succeeded = &succeeded
	return f
}

func (f *RechargeFilter) SetPaymentChannel(channel consts.PaymentChannel) *RechargeFilter {
	f.PaymentChannel = &channel
	return f
}

func (f *RechargeFilter) SetLimit(limit int) *RechargeFilter {
	f.Limit = limit
	return f
}

func (f *RechargeFilter) SetOffset(offset int) *RechargeFilter {
	f.Offset = offset
	return f
}

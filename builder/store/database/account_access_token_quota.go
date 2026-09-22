package database

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type accountAccessTokenQuotaStoreImpl struct {
	db *DB
}

type AccountAccessTokenQuotaStore interface {
	Create(ctx context.Context, quota *AccountAccessTokenQuota) error
	Update(ctx context.Context, quota *AccountAccessTokenQuota) error
	GetByID(ctx context.Context, id int64) (*AccountAccessTokenQuota, error)
	FindByTokenID(ctx context.Context, tokenID int64) ([]AccountAccessTokenQuota, error)
}

func NewAccountAccessTokenQuotaStore() AccountAccessTokenQuotaStore {
	return &accountAccessTokenQuotaStoreImpl{
		db: defaultDB,
	}
}

func NewAccountAccessTokenQuotaStoreWithDB(db *DB) AccountAccessTokenQuotaStore {
	return &accountAccessTokenQuotaStoreImpl{
		db: db,
	}
}

type AccountAccessTokenQuota struct {
	ID          int64                          `bun:",pk,autoincrement" json:"id"`
	APIKey      string                         `bun:",notnull" json:"api_key"`
	QuotaType   types.AccountingQuotaType      `bun:",notnull" json:"quota_type"`
	ValueType   types.AccountingQuotaValueType `bun:",notnull" json:"value_type"`
	PeriodStart int64                          `bun:",notnull,default:0" json:"period_start"`
	PeriodEnd   int64                          `bun:",notnull,default:0" json:"period_end"`
	Usage       float64                        `bun:",notnull,default:0" json:"usage"`
	Quota       float64                        `bun:",notnull,default:0" json:"quota"`
	LastUsedAt  *time.Time                     `bun:",nullzero" json:"last_used_at"`
	Allocation  string                         `bun:",notnull,default:''" json:"allocation"`
	TokenID     int64                          `bun:",notnull,default:0" json:"token_id"`
	times
}

func (s *accountAccessTokenQuotaStoreImpl) Create(ctx context.Context, quota *AccountAccessTokenQuota) error {
	err := s.db.Operator.Core.NewInsert().Model(quota).Scan(ctx)
	return errorx.HandleDBError(err, nil)
}

func (s *accountAccessTokenQuotaStoreImpl) Update(ctx context.Context, quota *AccountAccessTokenQuota) error {
	_, err := s.db.Operator.Core.NewUpdate().Model(quota).WherePK().Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

func (s *accountAccessTokenQuotaStoreImpl) GetByID(ctx context.Context, id int64) (*AccountAccessTokenQuota, error) {
	var quota AccountAccessTokenQuota
	err := s.db.Operator.Core.
		NewSelect().
		Model(&quota).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &quota, nil
}

func (s *accountAccessTokenQuotaStoreImpl) FindByTokenID(ctx context.Context, tokenID int64) ([]AccountAccessTokenQuota, error) {
	var quotas []AccountAccessTokenQuota
	err := s.db.Operator.Core.
		NewSelect().
		Model(&quotas).
		Where("token_id = ?", tokenID).
		Order("id DESC").
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return quotas, nil
}

func UpdateAPIKeyUsage(ctx context.Context, tx bun.Tx, input AccountStatement) error {
	if input.TokenID <= 0 {
		return nil
	}
	var quotas []AccountAccessTokenQuota
	// token_id is the stable join key (it survives builtin key refreshes that
	// rewrite api_key on quota rows).
	q := tx.NewSelect().Model(&quotas).Where("token_id = ?", input.TokenID)

	err := q.Scan(ctx)
	if err != nil {
		return fmt.Errorf("failed to get account access id %d quota error: %w", input.TokenID, err)
	}
	loc := config.GetGlobalTimeZone()
	now := time.Now().In(loc)
	for _, quota := range quotas {
		switch quota.QuotaType {
		case types.AccountingQuotaTypeMonthly:
			if now.Unix() < quota.PeriodStart || now.Unix() > quota.PeriodEnd {
				quota.PeriodStart, quota.PeriodEnd = CalcCurrentMonthPeriod()
				quota.Usage = 0
			}
		case types.AccountingQuotaTypeDaily:
			if now.Unix() < quota.PeriodStart || now.Unix() > quota.PeriodEnd {
				quota.PeriodStart, quota.PeriodEnd = CalcCurrentDayPeriod()
				quota.Usage = 0
			}
		}

		if quota.QuotaType == types.AccountingQuotaTypeUnlimited ||
			quota.QuotaType == types.AccountingQuotaTypeMonthly ||
			quota.QuotaType == types.AccountingQuotaTypeDaily ||
			quota.QuotaType == types.AccountingQuotaTotal {
			switch quota.ValueType {
			case types.AccountingQuotaValueTypeFee:
				quota.Usage += input.Value
			case types.AccountingQuotaValueTypeToken:
				quota.Usage += input.Consumption
			}
		}
		quota.LastUsedAt = &now
		_, err = tx.NewUpdate().Model(&quota).WherePK().Exec(ctx)
		if err != nil {
			return fmt.Errorf("update account access %s quota, error:%w", input.APIKey, err)
		}
	}
	return nil
}

func CalcCurrentMonthPeriod() (int64, int64) {
	loc := config.GetGlobalTimeZone()
	now := time.Now().In(loc)
	// Start of current month
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	// End of current month (start of next month minus 1 nanosecond)
	end := start.AddDate(0, 1, 0).Add(-1)
	return start.Unix(), end.Unix()
}

func CalcCurrentDayPeriod() (int64, int64) {
	loc := config.GetGlobalTimeZone()
	now := time.Now().In(loc)
	// Start of current day
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	// End of current day (start of next day minus 1 nanosecond)
	end := start.AddDate(0, 0, 1).Add(-1)
	return start.Unix(), end.Unix()
}

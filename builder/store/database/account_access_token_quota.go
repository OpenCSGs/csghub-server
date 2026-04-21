package database

import (
	"context"
	"time"

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
	FindByAPIKey(ctx context.Context, apiKey string) ([]AccountAccessTokenQuota, error)
	DeleteByID(ctx context.Context, id int64) error
	DeleteByAPIKey(ctx context.Context, apiKey string) error
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

func (s *accountAccessTokenQuotaStoreImpl) FindByAPIKey(ctx context.Context, apiKey string) ([]AccountAccessTokenQuota, error) {
	var quotas []AccountAccessTokenQuota
	err := s.db.Operator.Core.
		NewSelect().
		Model(&quotas).
		Where("api_key = ?", apiKey).
		Order("id DESC").
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return quotas, nil
}

func (s *accountAccessTokenQuotaStoreImpl) DeleteByID(ctx context.Context, id int64) error {
	_, err := s.db.Operator.Core.
		NewDelete().
		Model(&AccountAccessTokenQuota{}).
		Where("id = ?", id).
		Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

func (s *accountAccessTokenQuotaStoreImpl) DeleteByAPIKey(ctx context.Context, apiKey string) error {
	_, err := s.db.Operator.Core.
		NewDelete().
		Model(&AccountAccessTokenQuota{}).
		Where("api_key = ?", apiKey).
		Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

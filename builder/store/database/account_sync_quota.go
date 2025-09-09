package database

import (
	"context"
	"fmt"
)

type accountSyncQuotaStoreImpl struct {
	db *DB
}

type AccountSyncQuotaStore interface {
	GetByID(ctx context.Context, userID int64) (*AccountSyncQuota, error)
	Update(ctx context.Context, accountQuota AccountSyncQuota) (*AccountSyncQuota, error)
	Create(ctx context.Context, accountQuota AccountSyncQuota) error
	Delete(ctx context.Context, accountQuota AccountSyncQuota) error
	ListAllByUserID(ctx context.Context, userID int64) ([]AccountSyncQuota, error)
	RefreshAccountSyncQuota(ctx context.Context) error
	IncreaseRepoLimit(ctx context.Context, userID int64, increment int64) error
}

func NewAccountSyncQuotaStore() AccountSyncQuotaStore {
	return &accountSyncQuotaStoreImpl{
		db: defaultDB,
	}
}

func NewAccountSyncQuotaStoreWithDB(db *DB) AccountSyncQuotaStore {
	return &accountSyncQuotaStoreImpl{
		db: db,
	}
}

type AccountSyncQuota struct {
	UserID         int64 `bun:",pk" json:"user_id"`
	RepoCountLimit int64 `bun:",notnull" json:"repo_count_limit"`
	RepoCountUsed  int64 `bun:",notnull" json:"repo_count_used"`
	SpeedLimit     int64 `bun:",notnull" json:"speed_limit"`
	TrafficLimit   int64 `bun:",notnull" json:"traffic_limit"`
	TrafficUsed    int64 `bun:",notnull" json:"traffic_used"`
}

func (s *accountSyncQuotaStoreImpl) GetByID(ctx context.Context, userID int64) (*AccountSyncQuota, error) {
	quota := &AccountSyncQuota{}
	err := s.db.Core.NewSelect().Model(quota).Where("user_id = ?", userID).Scan(ctx, quota)
	return quota, err
}

func (s *accountSyncQuotaStoreImpl) Update(ctx context.Context, accountQuota AccountSyncQuota) (*AccountSyncQuota, error) {
	res, err := s.db.Core.NewUpdate().Model(&accountQuota).WherePK().Column("repo_count_limit", "speed_limit", "traffic_limit").Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("update user quota failed, error:%w", err)
	}
	return &accountQuota, err
}

func (s *accountSyncQuotaStoreImpl) Create(ctx context.Context, accountQuota AccountSyncQuota) error {
	res, err := s.db.Core.NewInsert().Model(&accountQuota).Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("insert user quota failed, error:%w", err)
	}
	return nil
}

func (s *accountSyncQuotaStoreImpl) Delete(ctx context.Context, accountQuota AccountSyncQuota) error {
	_, err := s.db.Core.NewDelete().Model(&accountQuota).WherePK().Exec(ctx)
	return err
}

func (am *accountSyncQuotaStoreImpl) ListAllByUserID(ctx context.Context, userID int64) ([]AccountSyncQuota, error) {
	var accountSyncQuotas []AccountSyncQuota
	err := am.db.Operator.Core.NewSelect().Model(&accountSyncQuotas).Where("user_id = ?", userID).Scan(ctx, &accountSyncQuotas)
	if err != nil {
		return nil, fmt.Errorf("failed to list all account sync quotas by user id: %w", err)
	}
	return accountSyncQuotas, nil
}

func (am *accountSyncQuotaStoreImpl) RefreshAccountSyncQuota(ctx context.Context) error {
	_, err := am.db.Operator.Core.NewUpdate().
		Model(&AccountSyncQuota{}).
		Set("repo_count_limit = ?", 15).
		Where("repo_count_limit < 15").
		Exec(ctx)
	return err
}

func (s *accountSyncQuotaStoreImpl) IncreaseRepoLimit(ctx context.Context, userID int64, increment int64) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model((*AccountSyncQuota)(nil)).
		Set("repo_count_limit = repo_count_limit + ?", increment).
		Where("user_id = ?", userID).
		Exec(ctx)
	return err
}

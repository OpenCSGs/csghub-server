package database

import (
	"context"
	"fmt"
)

type AccountSyncQuotaStore struct {
	db *DB
}

func NewAccountSyncQuotaStore() *AccountSyncQuotaStore {
	return &AccountSyncQuotaStore{
		db: defaultDB,
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

func (s *AccountSyncQuotaStore) GetByID(ctx context.Context, userID int64) (*AccountSyncQuota, error) {
	quota := &AccountSyncQuota{}
	err := s.db.Core.NewSelect().Model(quota).Where("user_id = ?", userID).Scan(ctx, quota)
	return quota, err
}

func (s *AccountSyncQuotaStore) Update(ctx context.Context, accountQuota AccountSyncQuota) (*AccountSyncQuota, error) {
	res, err := s.db.Core.NewUpdate().Model(&accountQuota).WherePK().Column("repo_count_limit", "speed_limit", "traffic_limit").Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("update user quota failed, error:%w", err)
	}
	return &accountQuota, err
}

func (s *AccountSyncQuotaStore) Create(ctx context.Context, accountQuota AccountSyncQuota) error {
	res, err := s.db.Core.NewInsert().Model(&accountQuota).Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("insert user quota failed, error:%w", err)
	}
	return nil
}

func (s *AccountSyncQuotaStore) Delete(ctx context.Context, accountQuota AccountSyncQuota) error {
	_, err := s.db.Core.NewDelete().Model(&accountQuota).WherePK().Exec(ctx)
	return err
}

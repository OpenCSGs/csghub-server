package database

import "context"

type SyncClientSettingStore struct {
	db *DB
}

func NewSyncClientSettingStore() *SyncClientSettingStore {
	return &SyncClientSettingStore{
		db: defaultDB,
	}
}

type SyncClientSetting struct {
	ID              int64  `bun:",pk,autoincrement" json:"id"`
	Token           string `bun:",notnull" json:"token"`
	ConcurrentCount int    `bun:",nullzero" json:"concurrent_count"`
	MaxBandwidth    int    `bun:",nullzero" json:"max_bandwidth"`
	IsDefault       bool   `bun:"," json:"default"`
	times
}

func (s *SyncClientSettingStore) Create(ctx context.Context, setting *SyncClientSetting) (*SyncClientSetting, error) {
	err := s.db.Operator.Core.NewInsert().
		Model(setting).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return setting, nil
}

func (s *SyncClientSettingStore) SyncClientSettingExists(ctx context.Context) (bool, error) {
	return s.db.Operator.Core.NewSelect().
		Model((*SyncClientSetting)(nil)).
		Exists(ctx)
}

func (s *SyncClientSettingStore) DeleteAll(ctx context.Context) error {
	_, err := s.db.Operator.Core.NewDelete().Model((*SyncClientSetting)(nil)).Where("1=1").Exec(ctx)
	return err
}

func (s *SyncClientSettingStore) First(ctx context.Context) (*SyncClientSetting, error) {
	var mt SyncClientSetting
	err := s.db.Operator.Core.NewSelect().
		Model(&mt).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &mt, nil
}

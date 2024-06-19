package database

import (
	"context"
)

type SyncVersionStore struct {
	db *DB
}

type SyncVersionSource int

func NewSyncVersionStore() *SyncVersionStore {
	return &SyncVersionStore{
		db: defaultDB,
	}
}

func (s *SyncVersionStore) Create(ctx context.Context, version *SyncVersion) (err error) {
	err = s.db.Operator.Core.NewInsert().Model(version).Scan(ctx)
	return
}

func (s *SyncVersionStore) BatchCreate(ctx context.Context, versions []SyncVersion) error {
	result, err := s.db.Core.NewInsert().Model(&versions).Exec(ctx)
	return assertAffectedXRows(int64(len(versions)), result, err)
}

func (s *SyncVersionStore) FindByPath(ctx context.Context, path string) (*SyncVersion, error) {
	var syncVersion SyncVersion
	err := s.db.Core.NewSelect().
		Model(&syncVersion).
		Where("repo_path = ?", path).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &syncVersion, nil
}

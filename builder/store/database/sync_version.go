package database

import (
	"context"

	"opencsg.com/csghub-server/common/types"
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
	_, err = s.db.Operator.Core.NewInsert().Model(version).Exec(ctx)
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

func (s *SyncVersionStore) FindByRepoTypeAndPath(ctx context.Context, path string, repoType types.RepositoryType) (*SyncVersion, error) {
	var syncVersion SyncVersion
	err := s.db.Core.NewSelect().
		Model(&syncVersion).
		Where("repo_path = ? and repo_type = ?", path, repoType).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &syncVersion, nil
}

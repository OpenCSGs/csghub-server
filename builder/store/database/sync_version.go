package database

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

type syncVersionStoreImpl struct {
	db *DB
}

type SyncVersionSource int

type SyncVersionStore interface {
	Create(ctx context.Context, version *SyncVersion) (err error)
	BatchCreate(ctx context.Context, versions []SyncVersion) error
	FindByPath(ctx context.Context, path string) (*SyncVersion, error)
	FindByRepoTypeAndPath(ctx context.Context, path string, repoType types.RepositoryType) (*SyncVersion, error)
	// Complete marks all sync versions for the same repository with version <= the given version as completed.
	Complete(ctx context.Context, version SyncVersion) error
}

func NewSyncVersionStore() SyncVersionStore {
	return &syncVersionStoreImpl{
		db: defaultDB,
	}
}

func NewSyncVersionStoreWithDB(db *DB) SyncVersionStore {
	return &syncVersionStoreImpl{
		db: db,
	}
}

func (s *syncVersionStoreImpl) Create(ctx context.Context, version *SyncVersion) (err error) {
	_, err = s.db.Operator.Core.NewInsert().Model(version).Exec(ctx)
	return
}

func (s *syncVersionStoreImpl) BatchCreate(ctx context.Context, versions []SyncVersion) error {
	result, err := s.db.Core.NewInsert().Model(&versions).Exec(ctx)
	return assertAffectedXRows(int64(len(versions)), result, err)
}

func (s *syncVersionStoreImpl) FindByPath(ctx context.Context, path string) (*SyncVersion, error) {
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

func (s *syncVersionStoreImpl) FindByRepoTypeAndPath(ctx context.Context, path string, repoType types.RepositoryType) (*SyncVersion, error) {
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

// Complete marks all sync versions for the same repository with version <= the given version as completed.
func (s *syncVersionStoreImpl) Complete(ctx context.Context, version SyncVersion) error {
	_, err := s.db.Core.NewUpdate().
		Model(&SyncVersion{}).
		Set("completed = ?", true).
		Where("source_id = ? AND repo_path = ? AND repo_type = ? AND version <= ?", version.SourceID, version.RepoPath, version.RepoType, version.Version).
		Exec(ctx)
	return err
}

package database

import (
	"context"

	"github.com/uptrace/bun"
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
	BatchDeleteOthers(ctx context.Context, repoType types.RepositoryType, keepPaths []string) error
	FindWithBatch(ctx context.Context, repoType types.RepositoryType, batchSize, batch int) ([]SyncVersion, error)
	DeleteOldVersions(ctx context.Context) error
	// Complete marks all sync versions for the same repository with version <= the given version as completed.
	Complete(ctx context.Context, version SyncVersion) error
	DeleteAll(ctx context.Context, repoType types.RepositoryType) error
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

func (s *syncVersionStoreImpl) BatchDeleteOthers(ctx context.Context, repoType types.RepositoryType, keepPaths []string) error {
	_, err := s.db.Core.NewDelete().
		Model(&SyncVersion{}).
		Where("repo_path not in (?) and repo_type = ?", bun.In(keepPaths), repoType).
		Exec(ctx)
	return err
}

func (s *syncVersionStoreImpl) FindWithBatch(ctx context.Context, repoType types.RepositoryType, batchSize, batch int) ([]SyncVersion, error) {
	var syncVersions []SyncVersion
	err := s.db.Core.NewSelect().
		Model(&syncVersions).
		Where("repo_type = ?", repoType).
		Order("version").
		Limit(batchSize).
		Offset(batchSize * batch).
		Scan(ctx)
	if err != nil {
		return syncVersions, err
	}
	return syncVersions, nil
}

func (s *syncVersionStoreImpl) DeleteOldVersions(ctx context.Context) error {
	subQuery := s.db.Core.NewSelect().
		DistinctOn("repo_path, repo_type").
		Column("repo_path", "repo_type", "version").
		Table("sync_versions").
		Order("repo_path", "repo_type", "version DESC")

	_, err := s.db.Core.NewDelete().
		Table("sync_versions").
		Where("(repo_path, repo_type, version) NOT IN (?)", subQuery).
		Exec(ctx)

	if err != nil {
		return err
	}
	return nil
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

func (s *syncVersionStoreImpl) DeleteAll(ctx context.Context, repoType types.RepositoryType) error {
	_, err := s.db.Core.NewDelete().
		Table("sync_versions").
		Where("repo_type = ?", repoType).
		Exec(ctx)
	return err
}

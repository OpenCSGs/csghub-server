package database

import (
	"context"
	"fmt"
)

type multiSyncStoreImpl struct {
	db *DB
}

type MultiSyncStore interface {
	Create(ctx context.Context, v SyncVersion) (*SyncVersion, error)
	// GetAfter get N records after version in ASC order
	GetAfter(ctx context.Context, version, limit int64) ([]SyncVersion, error)
	// GetLatest get max sync version
	GetLatest(ctx context.Context) (SyncVersion, error)
	GetAfterDistinct(ctx context.Context, version int64) ([]SyncVersion, error)
	// GetNotCompletedDistinct get sync versions not completed and distinct
	GetNotCompletedDistinct(ctx context.Context) ([]SyncVersion, error)
}

func NewMultiSyncStore() MultiSyncStore {
	return &multiSyncStoreImpl{
		db: defaultDB,
	}
}

func NewMultiSyncStoreWithDB(db *DB) MultiSyncStore {
	return &multiSyncStoreImpl{
		db: db,
	}
}

func (s *multiSyncStoreImpl) Create(ctx context.Context, v SyncVersion) (*SyncVersion, error) {
	res, err := s.db.Core.NewInsert().Model(&v).Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create sync version in db failed,error:%w", err)
	}

	return &v, nil
}

// GetAfter get N records after version in ASC order
func (s *multiSyncStoreImpl) GetAfter(ctx context.Context, version, limit int64) ([]SyncVersion, error) {
	var vs []SyncVersion
	err := s.db.Core.NewSelect().Model(&vs).Where("version > ?", version).
		Order("version asc").
		Limit(int(limit)).
		Scan(ctx, &vs)
	return vs, err
}

// GetLatest get max sync version
func (s *multiSyncStoreImpl) GetLatest(ctx context.Context) (SyncVersion, error) {
	var v SyncVersion
	err := s.db.Core.NewSelect().Model(&v).
		Order("version desc").
		Limit(1).
		Scan(ctx, &v)

	return v, err
}

func (s *multiSyncStoreImpl) GetAfterDistinct(ctx context.Context, version int64) ([]SyncVersion, error) {
	var vs []SyncVersion
	err := s.db.Core.NewSelect().
		ColumnExpr("DISTINCT ON (source_id, repo_path, repo_type) version, source_id, repo_path, repo_type, last_modified_at, change_log").
		Model(&vs).
		Where("version > ?", version).
		Order("source_id", "repo_path", "repo_type", "version DESC").
		Scan(ctx, &vs)
	return vs, err
}


// get not completed sync version distinct
func (s *multiSyncStoreImpl) GetNotCompletedDistinct(ctx context.Context) ([]SyncVersion, error) {
	var vs []SyncVersion
	err := s.db.Core.NewSelect().
		ColumnExpr("DISTINCT ON (source_id, repo_path, repo_type) version, source_id, repo_path, repo_type, last_modified_at, change_log").
		Model(&vs).
		Where("completed = ?", false).
		Order("source_id", "repo_path", "repo_type", "version DESC").
		Scan(ctx, &vs)


	return vs, err
}

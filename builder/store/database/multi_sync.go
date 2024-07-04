package database

import (
	"context"
	"fmt"
)

type MultiSyncStore struct {
	db *DB
}

func NewMultiSyncStore() *MultiSyncStore {
	return &MultiSyncStore{
		db: defaultDB,
	}
}

func (s *MultiSyncStore) Create(ctx context.Context, v SyncVersion) (*SyncVersion, error) {
	res, err := s.db.Core.NewInsert().Model(&v).Exec(ctx, &v)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create sync version in db failed,error:%w", err)
	}

	return &v, nil
}

// GetAfter get N records after version in ASC order
func (s *MultiSyncStore) GetAfter(ctx context.Context, version, limit int64) ([]SyncVersion, error) {
	var vs []SyncVersion
	err := s.db.Core.NewSelect().Model(&vs).Where("version > ?", version).
		Order("version asc").
		Limit(int(limit)).
		Scan(ctx, &vs)
	return vs, err
}

// GetLatest get max sync version
func (s *MultiSyncStore) GetLatest(ctx context.Context) (SyncVersion, error) {
	var v SyncVersion
	err := s.db.Core.NewSelect().Model(&v).
		Order("version desc").
		Limit(1).
		Scan(ctx, &v)

	return v, err
}

func (s *MultiSyncStore) GetAfterDistinct(ctx context.Context, version int64) ([]SyncVersion, error) {
	var vs []SyncVersion
	err := s.db.Core.NewSelect().
		ColumnExpr("DISTINCT ON (source_id, repo_path, repo_type) version, source_id, repo_path, repo_type, last_modified_at, change_log").
		Model(&vs).
		Where("version > ?", version).
		Scan(ctx, &vs)
	return vs, err
}

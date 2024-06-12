package database

import (
	"context"
)

type SyncVersionStore struct {
	db *DB
}

type SyncVersionSource int

const (
	OpenCSGSource = iota
	HFSource
)

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

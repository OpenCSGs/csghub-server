package database

import (
	"context"
	"database/sql"
	"errors"
)

// HistoryArchiveCheckpoint is the store-side model for the
// history_archive_checkpoints table. The migrations package has its own copy
// to avoid an import cycle; the two are intentionally independent.
type HistoryArchiveCheckpoint struct {
	TableName string `bun:"table_name,pk" json:"table_name"`
	LastPK    string `bun:"last_pk,type:text,notnull,default:''" json:"last_pk"`
	times
}

// HistoryArchiveCheckpointStore persists the keyset cursor (last processed
// primary key) for each archived table so backfill resumes across runs without
// re-scanning the full source table.
type HistoryArchiveCheckpointStore interface {
	// GetLastPK returns the last processed primary key for the given table, or
	// the empty string if no checkpoint exists yet (first run).
	GetLastPK(ctx context.Context, tableName string) (string, error)
	// SaveLastPK upserts the checkpoint for the given table.
	SaveLastPK(ctx context.Context, tableName, lastPK string) error
}

type historyArchiveCheckpointStoreImpl struct {
	db *DB
}

func NewHistoryArchiveCheckpointStore() HistoryArchiveCheckpointStore {
	return NewHistoryArchiveCheckpointStoreWithDB(defaultDB)
}

func NewHistoryArchiveCheckpointStoreWithDB(db *DB) HistoryArchiveCheckpointStore {
	return &historyArchiveCheckpointStoreImpl{db: db}
}

func (s *historyArchiveCheckpointStoreImpl) GetLastPK(ctx context.Context, tableName string) (string, error) {
	var cp HistoryArchiveCheckpoint
	err := s.db.Operator.Core.NewSelect().Model(&cp).Where("table_name = ?", tableName).Scan(ctx, &cp)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return cp.LastPK, nil
}

func (s *historyArchiveCheckpointStoreImpl) SaveLastPK(ctx context.Context, tableName, lastPK string) error {
	cp := HistoryArchiveCheckpoint{
		TableName: tableName,
		LastPK:    lastPK,
	}
	_, err := s.db.Operator.Core.NewInsert().Model(&cp).
		On("CONFLICT (table_name) DO UPDATE").
		Set("last_pk = EXCLUDED.last_pk").
		Set("updated_at = current_timestamp").
		Exec(ctx)
	return err
}

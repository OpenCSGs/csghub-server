package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

// HistoryArchiveCheckpoint tracks the resume position (last processed primary
// key) for each table archived by the history-archive backfill. Unlike
// cron_checkpoints (which stores a date), this stores the exact primary-key
// value so backfill can resume with a keyset cursor (WHERE pk > last_pk) and
// avoid re-scanning the full source table every batch.
//
// last_pk is text so it can hold both bigint ids (e.g. account_statements.id)
// and uuid strings (e.g. account_events.event_uuid).
type HistoryArchiveCheckpoint struct {
	TableName string `bun:"table_name,pk" json:"table_name"`
	LastPK    string `bun:"last_pk,type:text,notnull,default:''" json:"last_pk"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, HistoryArchiveCheckpoint{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, HistoryArchiveCheckpoint{})
	})
}

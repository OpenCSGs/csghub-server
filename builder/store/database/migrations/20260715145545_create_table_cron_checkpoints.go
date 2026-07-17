package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// CronCheckpoint is the migrations-side model used only to create the table.
// The database package has its own (fuller) CronCheckpoint for store queries;
// the two are intentionally independent to avoid an import cycle.
type CronCheckpoint struct {
	JobName  string    `bun:"job_name,pk" json:"job_name"`
	LastDate time.Time `bun:"type:date,notnull" json:"last_date"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, CronCheckpoint{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, CronCheckpoint{})
	})
}

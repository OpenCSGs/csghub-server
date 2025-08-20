package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type Trend map[string]int
type StatSnapshot struct {
	ID           int64  `bun:",pk,autoincrement" json:"id"`
	TargetType   string `bun:",notnull" json:"target_type"`
	DateType     string `bun:",notnull" json:"date_type"`
	SnapshotDate string `bun:",notnull" json:"snapshot_date"`
	TrendData    Trend  `bun:",type:jsonb" json:"trend_data"`
	TotalCount   int    `bun:",notnull" json:"total_count"`
	NewCount     int    `bun:",notnull" json:"new_count"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, (*StatSnapshot)(nil)); err != nil {
			return fmt.Errorf("create table stat_snapshot fail: %w", err)
		}
		_, err := db.NewCreateIndex().
			Model((*StatSnapshot)(nil)).
			Index("idx_unique_snapshot").
			Unique().
			Column("target_type", "date_type", "snapshot_date").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create unique index fail: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, (*StatSnapshot)(nil))
	})
}

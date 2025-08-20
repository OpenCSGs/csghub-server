package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type Rule struct {
	ID       int64  `bun:"id,pk,autoincrement"`
	Content  string `bun:",notnull"`
	RuleType string `bun:",notnull,unique"`

	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, &Rule{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, &Rule{})
	})
}

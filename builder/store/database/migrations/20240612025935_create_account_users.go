package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type AccountUser struct {
	ID      int64   `bun:",pk,autoincrement" json:"id"`
	UserID  string  `bun:",notnull" json:"user_id"`
	Balance float64 `bun:",notnull" json:"balance"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AccountUser{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccountUser{})
	})
}

package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, AccessToken{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccessToken{})
	})
}

type AccessToken struct {
	ID     int    `bun:",pk,autoincrement" json:"id"`
	Name   string `bun:",notnull" json:"name"`
	Token  string `bun:",notnull" json:"token"`
	UserID int    `bun:",notnull" json:"user_id"`
	User   User   `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	times
}

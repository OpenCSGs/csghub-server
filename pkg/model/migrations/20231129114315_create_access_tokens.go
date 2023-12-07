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
	ID     int64  `bun:",pk,autoincrement" json:"id"`
	GitID  int64  `bun:",pk" json:"git_id"`
	Name   string `bun:",notnull" json:"name"`
	Token  string `bun:",notnull" json:"token"`
	UserID int64  `bun:",pk" json:"user_id"`
	User   *User  `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	times
}

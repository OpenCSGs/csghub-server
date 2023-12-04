package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, SSHKey{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, SSHKey{})
	})
}

type SSHKey struct {
	ID      int    `bun:",pk,autoincrement" json:"id"`
	GID     int    `bun:",notnull" json:"gid"`
	Name    string `bun:",notnull" json:"name"`
	Content string `bun:",notnull" json:"content"`
	UserID  int    `bun:",notnull" json:"user_id"`
	User    User   `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	times
}

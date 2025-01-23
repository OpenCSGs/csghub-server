package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type Broadcast struct {
	ID      int64  `bun:",pk,autoincrement" json:"id"`
	Content string `bun:"type:text,notnull" json:"content"`
	BcType  string `bun:",notnull" json:"bc_type"`
	Theme   string `bun:",notnull" json:"theme"`
	Status  string `bun:",notnull" json:"status"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, Broadcast{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, Broadcast{})
	})
}

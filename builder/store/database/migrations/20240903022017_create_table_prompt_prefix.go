package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type PromptPrefix struct {
	ID int64  `bun:",pk,autoincrement" json:"id"`
	ZH string `bun:",notnull" json:"zh"`
	EN string `bun:",notnull" json:"en"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, PromptPrefix{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, PromptPrefix{})
	})
}

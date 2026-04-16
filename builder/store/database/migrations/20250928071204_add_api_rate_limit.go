package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	type ApiRateLimit struct {
		ID      int64  `bun:",pk,autoincrement" json:"id"`
		Path    string `bun:",notnull,unique" json:"path"`
		Limit   int64  `bun:",notnull" json:"limit"`
		Window  int64  `bun:",notnull" json:"window"`
		CheckIP bool   `bun:",notnull,default:false" json:"checkIP"`
		times
	}

	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, &ApiRateLimit{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, &ApiRateLimit{})
	})
}

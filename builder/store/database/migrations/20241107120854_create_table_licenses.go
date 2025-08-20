package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type License struct {
	ID         int64     `bun:",pk,autoincrement" json:"id"`
	Key        string    `bun:",notnull" json:"key"`
	Company    string    `bun:",notnull" json:"company"`
	Email      string    `bun:",notnull" json:"email"`
	Product    string    `bun:",notnull" json:"product"`
	Edition    string    `bun:",notnull" json:"edition"`
	Version    string    `bun:",nullzero" json:"version"`
	Status     string    `bun:",nullzero" json:"status"`
	MaxUser    int       `bun:",notnull" json:"max_user"`
	StartTime  time.Time `bun:",notnull" json:"start_time"`
	ExpireTime time.Time `bun:",notnull" json:"expire_time"`
	Extra      string    `bun:",nullzero" json:"extra"`
	Remark     string    `bun:",nullzero" json:"remark"`
	UserUUID   string    `bun:",notnull" json:"user_uuid"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, License{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*License)(nil)).
			Index("idx_license_product_edition").
			Column("product", "edition").
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, License{})
	})
}

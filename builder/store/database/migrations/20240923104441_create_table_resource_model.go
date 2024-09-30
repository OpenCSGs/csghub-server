package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, ResourceModel{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*ResourceModel)(nil)).
			Index("idx_resource_model_name").
			Column("resource_name", "engine_name", "model_name").
			Exec(ctx)
		return err

	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, ResourceModel{})
	})
}

type ResourceModel struct {
	ID           int64  `bun:",pk,autoincrement" json:"id"`
	ResourceName string `bun:",notnull" json:"resource_name"`
	EngineName   string `bun:",notnull" json:"engine_name"`
	ModelName    string `bun:",notnull" json:"model_name"`
	Type         string `bun:",notnull" json:"type"`
	times
}

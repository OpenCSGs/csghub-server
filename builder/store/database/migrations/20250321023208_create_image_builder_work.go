package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type ImageBuilderWork struct {
	ID int64 `bun:",pk,autoincrement" json:"id"`

	WorkName   string `bun:"work_name,notnull,unique" json:"work_name"`
	WorkStatus string `bun:"work_status,notnull" json:"work_status"`
	Message    string `bun:"message" json:"message"`
	PodName    string `bun:"pod_name" json:"pod_name"`
	ClusterID  string `bun:"cluster_id" json:"cluster_id"`
	Namespace  string `bun:"namespace,notnull" json:"namespace"`
	ImagePath  string `bun:"image_path,notnull" json:"image_path"`
	BuildId    string `bun:"build_id,notnull,unique" json:"build_id"`

	InitContainerStatus string `bun:"init_container_status,notnull" json:"init_container_status"`
	InitContainerLog    string `bun:"init_container_log" json:"init_container_log"`
	MainContainerLog    string `bun:"main_container_log" json:"main_container_log"`

	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, ImageBuilderWork{}); err != nil {
			return err
		}
		_, err := db.NewCreateIndex().
			Model(&ImageBuilderWork{}).
			Index("idx_image_builder_work_work_name").
			Column("work_name").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = db.NewCreateIndex().
			Model(&ImageBuilderWork{}).
			Index("idx_image_builder_work_build_id").
			Column("build_id").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, ImageBuilderWork{})
	})
}

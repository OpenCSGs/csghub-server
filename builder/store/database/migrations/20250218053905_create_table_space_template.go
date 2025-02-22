package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type SpaceTemplate struct {
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	Type        string `bun:",notnull" json:"type"`
	Name        string `bun:",notnull" json:"name"`
	ShowName    string `bun:",notnull" json:"show_name"`
	Enable      bool   `bun:",notnull,default:false" json:"enable"`
	Path        string `bun:",notnull" json:"path"`
	DevMode     bool   `bun:",notnull,default:false" json:"dev_mode"`
	Port        int    `bun:",notnull" json:"port"`
	Secrets     string `bun:",nullzero" json:"secrets"`
	Variables   string `bun:",nullzero" json:"variables"`
	Description string `bun:",nullzero" json:"description"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, SpaceTemplate{})
		if err != nil {
			return fmt.Errorf("create table space template fail: %w", err)
		}
		_, err = db.NewCreateIndex().
			Model((*SpaceTemplate)(nil)).
			Index("idx_unique_space_template_type_name").
			Column("type", "name").
			Unique().
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_unique_space_template_type_name fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*SpaceTemplate)(nil)).
			Index("idx_space_template_type_enable_name").
			Column("type", "enable", "name").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_space_template_type_enable_name fail: %w", err)
		}

		err = initSpaceTemplates(ctx, db)
		return err

	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, SpaceTemplate{})
	})
}

func initSpaceTemplates(ctx context.Context, db *bun.DB) error {
	var templates = []SpaceTemplate{
		{
			Type:        "docker",
			Name:        "ChatUI",
			ShowName:    "ChatUI",
			Enable:      true,
			Path:        "model_chatui",
			Port:        8080,
			Variables:   "[{ \"name\": \"MODEL_NAME\", \"value\": \"Qwen/Qwen2-0.5B-Instruct\", \"type\": \"string\" }]",
			Description: "A web-based chat UI that supports model.",
		},
	}

	_, err := db.NewInsert().Model(&templates).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to insert default space template to db: %w", err)
	}
	return nil
}

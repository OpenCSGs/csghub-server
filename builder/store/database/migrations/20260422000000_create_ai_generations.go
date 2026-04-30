package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type AIGeneration struct {
	ID                 int64          `bun:"id,pk,autoincrement"`
	ResourceType       string         `bun:"resource_type,notnull"`
	ResourceID         string         `bun:"resource_id,notnull"`
	ProviderResourceID string         `bun:"provider_resource_id,notnull"`
	ProviderMetadata   map[string]any `bun:"provider_metadata,type:jsonb,nullzero"`
	OwnerUUID          string         `bun:"owner_uuid,notnull"`
	ModelID            string         `bun:"model_id,notnull"`
	Status             string         `bun:"status,notnull"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] create_ai_generations")
		if _, err := db.NewCreateTable().Model((*AIGeneration)(nil)).IfNotExists().Exec(ctx); err != nil {
			return err
		}
		_, err := db.NewCreateIndex().
			Model((*AIGeneration)(nil)).
			Index("idx_ai_generations_resource").
			Column("resource_type", "resource_id").
			Unique().
			IfNotExists().
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] create_ai_generations")
		_, err := db.NewDropTable().Model((*AIGeneration)(nil)).IfExists().Exec(ctx)
		return err
	})
}

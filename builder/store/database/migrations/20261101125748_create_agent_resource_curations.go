package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type AgentResourceCuration struct {
	bun.BaseModel `bun:"table:agent_resource_curations"`
	ID            int64  `bun:",pk,autoincrement"`
	EntityType    string `bun:",notnull"`
	EntityID      string `bun:",notnull,type:text"`
	Weight        int64  `bun:",notnull,default:0"`
	Recommended   bool   `bun:",notnull,default:false"`
	Unlisted      bool   `bun:",notnull,default:false"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, AgentResourceCuration{}); err != nil {
			return err
		}
		_, err := db.NewCreateIndex().
			Model((*AgentResourceCuration)(nil)).
			Index("idx_agent_resource_curations_entity").
			Column("entity_type", "entity_id").
			Unique().
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AgentResourceCuration{})
	})
}

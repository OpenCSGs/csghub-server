package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// AgentUserPreference represents a user's preference for an agent-related entity
type AgentUserPreference struct {
	ID         int64  `bun:",pk,autoincrement" json:"id"`
	UserUUID   string `bun:",notnull" json:"user_uuid"`
	EntityType string `bun:",notnull" json:"entity_type"`
	EntityID   string `bun:",notnull,type:text" json:"entity_id"` // TEXT to support both integer IDs (as strings) and string IDs
	Action     string `bun:",notnull" json:"action"`              // pin etc.
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, AgentUserPreference{}); err != nil {
			return fmt.Errorf("create table agent_user_preferences fail: %w", err)
		}

		_, err := db.ExecContext(ctx, `
			ALTER TABLE agent_user_preferences 
			ADD CONSTRAINT idx_agent_user_preferences_user_action_entity UNIQUE (user_uuid, action, entity_type, entity_id);
		`)
		if err != nil {
			return fmt.Errorf("create unique constraint idx_agent_user_preferences_user_action_entity fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*AgentUserPreference)(nil)).
			Index("idx_agent_user_preferences_user_action_entity_type_created_at").
			Column("user_uuid", "action", "entity_type", "created_at").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_agent_user_preferences_user_action_entity_type_created_at fail: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AgentUserPreference{})
	})
}

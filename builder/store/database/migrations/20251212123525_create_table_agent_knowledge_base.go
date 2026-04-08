package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type AgentKnowledgeBase struct {
	ID          int64          `bun:",pk,autoincrement" json:"id"`
	UserUUID    string         `bun:",notnull" json:"user_uuid"`
	Name        string         `bun:",notnull" json:"name"`
	Description string         `bun:",nullzero" json:"description"`
	ContentID   string         `bun:",notnull,unique" json:"content_id"`    // Used to specify the unique id of the knowledge base resource
	Public      bool           `bun:",notnull" json:"public"`               // Whether the knowledge base is public
	Metadata    map[string]any `bun:",type:jsonb,nullzero" json:"metadata"` // Knowledge base metadata
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, AgentKnowledgeBase{}); err != nil {
			return fmt.Errorf("create table agent_knowledge_bases fail: %w", err)
		}

		_, err := db.ExecContext(ctx, `
			ALTER TABLE agent_knowledge_bases 
			ADD CONSTRAINT idx_agent_knowledge_bases_user_uuid_name UNIQUE (user_uuid, name);
		`)
		if err != nil {
			return fmt.Errorf("create unique constraint idx_agent_knowledge_bases_user_uuid_name fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*AgentKnowledgeBase)(nil)).
			Index("idx_agent_knowledge_bases_updated_at").
			Column("updated_at").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_agent_knowledge_bases_updated_at fail: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*AgentKnowledgeBase)(nil)).
			Index("idx_agent_knowledge_bases_public").
			Column("public").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index idx_agent_knowledge_bases_public fail: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AgentKnowledgeBase{})
	})
}

package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, &AgentTemplate{}, &AgentInstance{})
		if err != nil {
			return err
		}
		// create index for AgentTemplate
		_, err = db.NewCreateIndex().Model(&AgentTemplate{}).
			Index("idx_agent_template_user_uuid").
			Column("user_uuid").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}
		// create index for AgentInstance
		_, err = db.NewCreateIndex().Model(&AgentInstance{}).
			Index("idx_agent_instance_user_uuid_template_id").
			Column("user_uuid", "template_id").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		err := dropTables(ctx, db, &AgentTemplate{}, &AgentInstance{})
		if err != nil {
			return err
		}
		return nil
	})
}

// AgentTemplate represents the template for an agent
type AgentTemplate struct {
	ID       int64  `bun:",pk,autoincrement" json:"id"`
	Type     string `bun:",notnull" json:"type"`      // Possible values: langflow, agno, code, etc.
	UserUUID string `bun:",notnull" json:"user_uuid"` // Associated with the corresponding field in the User table
	Content  string `bun:",type:text" json:"content"` // Used to store the complete content of the template
	Public   bool   `bun:",notnull" json:"public"`    // Whether the template is public
	times
}

// AgentInstance represents an instance created from an agent template
type AgentInstance struct {
	ID         int64  `bun:",pk,autoincrement" json:"id"`
	TemplateID int64  `bun:"" json:"template_id"`        // Associated with the id in the template table
	UserUUID   string `bun:",notnull" json:"user_uuid"`  // Associated with the corresponding field in the User table
	Type       string `bun:",notnull" json:"type"`       // Possible values: langflow, agno, code, etc.
	ContentID  string `bun:",notnull" json:"content_id"` // Used to specify the unique id of the instance resource
	Public     bool   `bun:",notnull" json:"public"`     // Whether the instance is public
	times
}

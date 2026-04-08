package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type AgentInstanceSession struct {
	ID         int64  `bun:",pk,autoincrement" json:"id"`
	UUID       string `bun:",notnull,unique" json:"uuid"`
	Name       string `bun:",nullzero" json:"name"`
	InstanceID int64  `bun:",notnull" json:"instance_id"`
	UserUUID   string `bun:",notnull" json:"user_uuid"`
	Type       string `bun:",notnull" json:"type"` // Possible values: langflow, agno, code, etc.
	times
}

type AgentInstanceSessionHistory struct {
	ID        int64  `bun:",pk,autoincrement" json:"id"`
	SessionID int64  `bun:",notnull" json:"session_id"`
	Request   bool   `bun:",notnull" json:"request"` // true: request, false: response
	Content   string `bun:",type:text" json:"content"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, &AgentInstanceSession{}, &AgentInstanceSessionHistory{})
		if err != nil {
			return err
		}

		// create index for agent_instance_sessions on name
		_, err = db.NewCreateIndex().Model(&AgentInstanceSession{}).
			Index("idx_agent_instance_session_name").
			Column("name").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}

		// create index for agent_instance_session_histories on session_id
		_, err = db.NewCreateIndex().Model(&AgentInstanceSessionHistory{}).
			Index("idx_agent_instance_session_history_session_id").
			Column("session_id").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		err := dropTables(ctx, db, &AgentInstanceSession{}, &AgentInstanceSessionHistory{})
		if err != nil {
			return err
		}
		return nil
	})
}

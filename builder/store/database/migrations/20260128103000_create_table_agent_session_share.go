package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type AgentInstanceSessionShare struct {
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	ShareUUID   string `bun:",notnull,unique" json:"share_uuid"`
	UserUUID    string `bun:",notnull" json:"user_uuid"`
	InstanceID  int64  `bun:",notnull" json:"instance_id"`
	SessionUUID string `bun:",notnull" json:"session_uuid"`
	MaxTurn     int64  `bun:",notnull" json:"max_turn"`
	ExpiresAt   int64  `bun:",notnull" json:"expires_at"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, &AgentInstanceSessionShare{})
		if err != nil {
			return err
		}

		// create index for agent_instance_session_shares on session_uuid
		_, err = db.NewCreateIndex().Model(&AgentInstanceSessionShare{}).
			Index("idx_agent_instance_session_share_session_uuid").
			Column("session_uuid").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}

		// create index for agent_instance_session_shares on user_uuid and expires_at
		_, err = db.NewCreateIndex().Model(&AgentInstanceSessionShare{}).
			Index("idx_agent_instance_session_share_user_uuid_expires_at").
			Column("user_uuid").
			Column("expires_at").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return err
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, &AgentInstanceSessionShare{})
	})
}

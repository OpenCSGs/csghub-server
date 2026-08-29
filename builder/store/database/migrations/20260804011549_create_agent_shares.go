package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type AgentShare struct {
	ID         int64  `bun:",pk,autoincrement" json:"id"`
	ShareUUID  string `bun:",notnull,unique" json:"share_uuid"`
	ShareName  string `bun:",notnull,unique" json:"share_name"`
	Type       string `bun:",notnull" json:"type"`
	UserUUID   string `bun:",notnull" json:"user_uuid"`
	InstanceID int64  `bun:",notnull" json:"instance_id"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, &AgentShare{})
		if err != nil {
			return err
		}

		_, err = db.NewCreateIndex().Model(&AgentShare{}).
			Index("idx_agent_shares_instance_id").
			Column("instance_id").
			IfNotExists().
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, &AgentShare{})
	})
}

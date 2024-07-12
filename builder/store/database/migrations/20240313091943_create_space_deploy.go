package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

type Deploy struct {
	ID int64 `bun:",pk,autoincrement" json:"id"`
	// space_id to deploy, it's 0 if deploy model
	SpaceID   int64  `bun:",notnull" json:"space_id"`
	Status    int    `bun:",notnull" json:"status"`
	GitPath   string `bun:",notnull" json:"git_path"`
	GitBranch string `bun:",notnull" json:"git_branch"`
	Env       string `bun:",nullzero" json:"env"`
	Secret    string `bun:",nullzero" json:"secret"`
	Template  string `bun:",notnull" json:"tmeplate"`
	Hardware  string `bun:",notnull" json:"hardware"`
	// for image run task, aka task_type = 1
	// running image of cluster, comes from builder or pre-define
	ImageID string `bun:",nullzero" json:"image_id"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		dropTables(ctx, db, database.Space{})
		return createTables(ctx, db, database.Space{}, Deploy{}, database.DeployTask{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, Deploy{}, database.DeployTask{})
	})
}

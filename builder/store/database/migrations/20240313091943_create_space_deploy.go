package migrations

import (
	"context"

	"github.com/uptrace/bun"
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

type DeployTask struct {
	ID int64 `bun:",pk,autoincrement" json:"id"`
	// 0: build, 1: run
	TaskType int     `bun:",notnull" json:"task_type"`
	Status   int     `bun:",notnull" json:"status"`
	Message  string  `bun:",nullzero" json:"message"`
	DeployID int64   `bun:",notnull" json:"deploy_id"`
	Deploy   *Deploy `bun:"rel:belongs-to,join:deploy_id=id" json:"deploy"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_ = dropTables(ctx, db, Space{})
		return createTables(ctx, db, Space{}, Deploy{}, DeployTask{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, Deploy{}, DeployTask{})
	})
}

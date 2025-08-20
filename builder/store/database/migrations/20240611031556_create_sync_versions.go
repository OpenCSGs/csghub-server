package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type SyncVersion struct {
	Version        int64                `bun:",pk,autoincrement" json:"version"`
	SourceID       int64                `bun:",notnull" json:"source_id"`
	RepoPath       string               `bun:",notnull" json:"repo_path"`
	RepoType       types.RepositoryType `bun:",notnull" json:"repo_type"`
	LastModifiedAt time.Time            `bun:",notnull" json:"last_modified_at"`
	ChangeLog      string               `bun:"," json:"change_log"`
	// true if CE,EE complete the sync process successfully, e.g the repo created.
	Completed bool `bun:",notnull" json:"completed"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, SyncVersion{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, SyncVersion{})
	})
}

package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

type RepositoriesRuntimeFramework struct {
	ID                 int64             `bun:",pk,autoincrement" json:"id"`
	RuntimeFrameworkID int64             `bun:",notnull" json:"runtime_framework_id"`
	RuntimeFramework   *RuntimeFramework `bun:"rel:belongs-to,join:runtime_framework_id=id" json:"runtime_framework"`
	RepoID             int64             `bun:",notnull" json:"repo_id"`
	Type               int               `bun:",notnull" json:"type"` // 0-space, 1-inference, 2-finetune
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, RepositoriesRuntimeFramework{})
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, RepositoriesRuntimeFramework{})
	})
}

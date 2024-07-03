package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

type File struct {
	ID                int64                `bun:",pk,autoincrement" json:"id"`
	Name              string               `json:"name"`
	Path              string               `json:"path"`
	ParentPath        string               `json:"parent_path"`
	Size              int64                `json:"size"`
	LastCommitMessage string               `json:"last_commit_message"`
	LastCommitDate    string               `json:"last_commit_date"`
	RepositoryID      int64                `json:"repository_id"`
	Repository        *database.Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, File{})

	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, File{})
	})
}

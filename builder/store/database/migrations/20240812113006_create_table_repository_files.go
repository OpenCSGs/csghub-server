package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, RepositoryFile{}, RepositoryFileCheck{})
		if err != nil {
			return fmt.Errorf("create table repository_files: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*RepositoryFile)(nil)).
			Index("repository_files_path_idx").
			Column("repository_id", "branch").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create index repository_files_path_idx: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*RepositoryFileCheck)(nil)).
			Index("repository_file_checks_repo_file_id_idx").
			Column("repo_file_id").
			Unique().
			IfNotExists().
			Exec(ctx)

		if err != nil {
			return fmt.Errorf("create index repository_file_checks_repo_file_id_idx: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, RepositoryFile{}, RepositoryFileCheck{})
	})

}

type RepositoryFile struct {
	ID              int64       `bun:",pk,autoincrement" `
	RepositoryID    int64       `bun:",notnull" `
	Path            string      `bun:",notnull" `
	FileType        string      `bun:",notnull" `
	Size            int64       `bun:",nullzero" `
	LastModify      time.Time   `bun:",nullzero" `
	CommitSha       string      `bun:",nullzero" `
	LfsRelativePath string      `bun:",nullzero" `
	Branch          string      `bun:",nullzero" `
	Repository      *Repository `bun:"rel:belongs-to,join:repository_id=id"`
}

type RepositoryFileCheck struct {
	ID         int64                      `bun:",pk,autoincrement" json:"id"`
	RepoFileID int64                      `bun:"," json:"repo_file_id"`
	Status     types.SensitiveCheckStatus `bun:",notnull" json:"status"`
	Message    string                     `bun:",nullzero" json:"message"`
	CreatedAt  time.Time                  `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	//uuid for async check task
	TaskID string `bun:",nullzero" json:"task_id"`
}

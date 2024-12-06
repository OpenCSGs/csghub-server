package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type TagRule struct {
	ID        int64     `bun:",pk,autoincrement" json:"id"`
	RepoName  string    `bun:",notnull" json:"repo_name"`
	RepoType  string    `bun:",notnull" json:"repo_type"`
	Category  string    `bun:",notnull" json:"category"`
	TagName   string    `bun:",notnull" json:"tag_name"`
	Tag       Tag       `bun:",rel:has-one,join:tag_name=name"`
	CreatedAt time.Time `bun:",nullzero,notnull,skipupdate,default:current_timestamp" json:"created_at"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, TagRule{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*TagRule)(nil)).
			Index("idx_dataset_tag_name_type").
			Column("repo_name", "repo_type").
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, TagRule{})
	})
}

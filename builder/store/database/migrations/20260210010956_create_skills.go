package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type Skill struct {
	ID            int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID  int64       `bun:",notnull" json:"repository_id"`
	Repository    *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time   `bun:",notnull" json:"last_updated_at"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, Skill{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, Skill{})
	})
}

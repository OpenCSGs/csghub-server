package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

type LfsLock struct {
	ID           int64               `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64               `bun:",notnull" json:"repository_id"`
	Repository   database.Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	UserID       int64               `bun:",notnull" json:"user_id"`
	User         database.User       `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	Path         string              `bun:",notnull" json:"path"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, LfsLock{})
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] ")
		return dropTables(ctx, db, LfsLock{})
	})
}


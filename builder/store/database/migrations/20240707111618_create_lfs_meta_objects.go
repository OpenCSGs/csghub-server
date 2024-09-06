package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

type LfsMetaObject struct {
	ID           int64               `bun:",pk,autoincrement" json:"user_id"`
	Oid          string              `bun:",notnull" json:"oid"`
	Size         int64               `bun:",notnull" json:"size"`
	RepositoryID int64               `bun:",notnull" json:"repository_id"`
	Repository   database.Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	Existing     bool                `bun:",notnull" json:"existing"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, LfsMetaObject{})
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, LfsMetaObject{})
	})
}


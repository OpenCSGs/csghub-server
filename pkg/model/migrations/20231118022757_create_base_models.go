package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

var baseModelTables = []any{
	Repository{},
	LfsFile{},
	PublicKey{},
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, baseModelTables...)
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, baseModelTables...)
	})
}

type Repository struct {
	ID        int        `bun:",pk,autoincrement" json:"id"`
	UserID    string     `bun:",notnull" json:"user_id"`
	Path      string     `bun:",notnull" json:"path"`
	Name      string     `bun:",notnull" json:"name"`
	OwnerName string     `bun:",notnull" json:"owner_name"`
	LfsFiles  []*LfsFile `bun:"rel:has-many,join:id=repository_id"`
	times
}

type LfsFile struct {
	ID           int        `bun:",pk,autoincrement" json:"id"`
	RepositoryID int        `bun:",notnull" json:"repository_id"`
	Repository   Repository `bun:"rel:belongs-to,join:repository_id=id"`
	OriginPath   string     `bun:",notnull" json:"orgin_path"`
	times
}

type PublicKey struct {
	ID     int    `bun:",pk,autoincrement" json:"id"`
	UserID string `bun:",notnull" json:"user_id"`
	Value  string `bun:",notnull" json:"value"`
	times
}

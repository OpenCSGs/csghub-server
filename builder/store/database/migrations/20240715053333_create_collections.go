package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

type CollectionRepository struct {
	ID           int64       `bun:",autoincrement" json:"id"`
	CollectionID int64       `bun:",pk" json:"collection_id"`
	RepositoryID int64       `bun:",pk" json:"repository_id"`
	Collection   *Collection `bun:"rel:belongs-to,join:collection_id=id"`
	Repository   *Repository `bun:"rel:belongs-to,join:repository_id=id"`
}

type Collection struct {
	ID           int64        `bun:",pk,autoincrement" json:"id"`
	Namespace    string       `bun:",notnull" json:"namespace"`
	Username     string       `bun:",notnull" json:"username"`
	UserID       int64        `bun:",notnull" json:"user_id"`
	Name         string       `bun:",notnull" json:"name"`
	Theme        string       `bun:",notnull" json:"theme"`
	Nickname     string       `bun:",notnull" json:"nickname"`
	Description  string       `bun:",nullzero" json:"description"`
	Private      bool         `bun:",notnull" json:"private"`
	Repositories []Repository `bun:"m2m:collection_repositories,join:Collection=Repository" json:"repositories"`
	Likes        int64        `bun:",nullzero" json:"likes"`
	DeletedAt    time.Time    `bun:",soft_delete,nullzero"`
	times
}

var collectionTables = []any{
	CollectionRepository{},
	Collection{},
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, collectionTables...)
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, collectionTables...)
	})
}

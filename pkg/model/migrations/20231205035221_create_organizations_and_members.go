package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

var orgTables = []any{
	Organization{},
	Member{},
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, orgTables...)
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, orgTables...)
	})
}

type Organization struct {
	ID          int64      `bun:",pk,autoincrement" json:"id"`
	Name        string     `bun:",notnull" json:"name"`
	Path        string     `bun:",notnull" json:"path"`
	GitPath     string     `bun:",notnull" json:"git_path"`
	Description string     `json:"description"`
	UserID      int64      `bun:",notnull" json:"user_id"`
	User        *User      `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	NamespaceID int64      `bun:",pk" json:"namespace_id"`
	Namespace   *Namespace `bun:"rel:has-one,join:namespace_id=id" json:"namespace"`
	times
}

type Member struct {
	ID             int64         `bun:",pk,autoincrement" json:"id"`
	OrganizationID int64         `bun:",pk" json:"organization_id"`
	UserID         int64         `bun:",pk" json:"user_id"`
	Organization   *Organization `bun:"rel:belongs-to,join:organization_id=id" json:"organization"`
	User           *User         `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	Role           string        `bun:",notnull" json:"role"`
	times
}

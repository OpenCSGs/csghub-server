package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

var baseModelTables = []any{
	User{},
	Repository{},
	LfsFile{},
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, baseModelTables...)
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, baseModelTables...)
	})
}

type User struct {
	ID           int            `bun:",pk,autoincrement" json:"id"`
	GitID        int            `bun:",notnull" json:"git_id"`
	Name         string         `bun:",notnull" json:"name"`
	Username     string         `bun:",notnull,unique" json:"username"`
	Email        string         `bun:",notnull,unique" json:"email"`
	Password     string         `bun:",notnull" json:"password"`
	AccessTokens []*AccessToken `bun:"rel:has-many,join:id=user_id"`
	times
}

type RepositoryType string

const (
	Model   RepositoryType = "model"
	Dataset RepositoryType = "dataset"
)

type Repository struct {
	ID             int        `bun:",pk,autoincrement" json:"id"`
	UserID         int        `bun:",notnull" json:"user_id"`
	Path           string     `bun:",notnull" json:"path"`
	Name           string     `bun:",notnull" json:"name"`
	Description    string     `bun:",notnull" json:"description"`
	Private        bool       `bun:",notnull" json:"private"`
	Labels         string     `bun:",notnull" json:"labels"`
	License        string     `bun:",notnull" json:"license"`
	Readme         string     `bun:"," json:"readme"`
	DefaultBranch  string     `bun:"," json:"default_branch"`
	LfsFiles       []*LfsFile `bun:"rel:has-many,join:id=repository_id"`
	RepositoryType string     `bun:",notnull" json:"repository_type"`
	times
}

type LfsFile struct {
	ID           int        `bun:",pk,autoincrement" json:"id"`
	RepositoryID int        `bun:",notnull" json:"repository_id"`
	Repository   Repository `bun:"rel:belongs-to,join:repository_id=id"`
	OriginPath   string     `bun:",notnull" json:"orgin_path"`
	times
}

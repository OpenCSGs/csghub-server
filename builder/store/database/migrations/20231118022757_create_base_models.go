package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

type Dataset struct {
	ID            int64                `bun:",pk,autoincrement" json:"id"`
	Name          string               `bun:",notnull" json:"name"`
	UrlSlug       string               `bun:",notnull" json:"nickname"`
	Description   string               `bun:",nullzero" json:"description"`
	Likes         int64                `bun:",notnull" json:"likes"`
	Downloads     int64                `bun:",notnull" json:"downloads"`
	Path          string               `bun:",notnull" json:"path"`
	GitPath       string               `bun:",notnull" json:"git_path"`
	RepositoryID  int64                `bun:",notnull" json:"repository_id"`
	Repository    *database.Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time            `bun:",notnull" json:"last"`
	Private       bool                 `bun:",notnull" json:"private"`
	UserID        int64                `bun:",notnull" json:"user_id"`
	User          *database.User       `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	times
}

type Model struct {
	ID            int64                `bun:",pk,autoincrement" json:"id"`
	Name          string               `bun:",notnull" json:"name"`
	UrlSlug       string               `bun:",notnull" json:"nickname"`
	Description   string               `bun:",nullzero" json:"description"`
	Likes         int64                `bun:",notnull" json:"likes"`
	Downloads     int64                `bun:",notnull" json:"downloads"`
	Path          string               `bun:",notnull" json:"path"`
	GitPath       string               `bun:",notnull" json:"git_path"`
	RepositoryID  int64                `bun:",notnull" json:"repository_id"`
	Repository    *database.Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time            `bun:",notnull" json:"last_updated_at"`
	Private       bool                 `bun:",notnull" json:"private"`
	UserID        int64                `bun:",notnull" json:"user_id"`
	User          *database.User       `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	times
}

var baseModelTables = []any{
	database.User{},
	database.RepositoryTag{},
	database.Repository{},
	database.Namespace{},
	database.Tag{},
	database.TagCategory{},
	Model{},
	Dataset{},
	database.LfsFile{},
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return createTables(ctx, db, baseModelTables...)
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, baseModelTables...)
	})
}

type NamespaceType string

const (
	UserNamespace NamespaceType = "user"
	OrgNamespace  NamespaceType = "organization"
)

type RepositoryType string

const (
	ModelType   RepositoryType = "model"
	DatasetType RepositoryType = "dataset"
)

type TagScope string

const (
	ModelTagScope   TagScope = "model"
	DatasetTagScope TagScope = "dataset"
)

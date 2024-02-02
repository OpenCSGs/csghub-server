package migrations

import (
	"context"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

var baseModelTables = []any{
	database.User{},
	database.RepositoryTag{},
	database.Repository{},
	database.Namespace{},
	database.Tag{},
	database.TagCategory{},
	database.Model{},
	database.Dataset{},
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

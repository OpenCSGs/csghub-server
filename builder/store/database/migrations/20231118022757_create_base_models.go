package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

var baseModelTables = []any{
	User{},
	RepositoryTag{},
	Repository{},
	Namespace{},
	Tag{},
	Model{},
	Dataset{},
	LfsFile{},
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
	ModelTagScope    TagScope = "model"
	DatabaseTagScope TagScope = "database"
)

type User struct {
	ID           int64         `bun:",pk,autoincrement" json:"id"`
	GitID        int64         `bun:",pk" json:"git_id"`
	Name         string        `bun:",notnull" json:"name"`
	Username     string        `bun:",notnull,unique" json:"username"`
	Email        string        `bun:",notnull,unique" json:"email"`
	Password     string        `bun:",notnull" json:"password"`
	AccessTokens []AccessToken `bun:"rel:has-many,join:id=user_id"`
	Namespaces   []Namespace   `bun:"rel:has-many,join:id=user_id" json:"namespace"`
	times
}

type Namespace struct {
	ID            int64         `bun:",pk,autoincrement" json:"id"`
	Path          string        `bun:",notnull" json:"path"`
	UserID        int64         `bun:",pk" json:"user_id"`
	User          User          `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	NamespaceType NamespaceType `bun:",notnull" json:"namespace_type"`
	times
}

type Repository struct {
	ID             int64          `bun:",pk,autoincrement" json:"id"`
	UserID         int64          `bun:",pk" json:"user_id"`
	Path           string         `bun:",notnull" json:"path"`
	GitPath        string         `bun:",notnull" json:"git_path"`
	Name           string         `bun:",notnull" json:"name"`
	Description    string         `bun:",nullzero" json:"description"`
	Private        bool           `bun:",notnull" json:"private"`
	Labels         string         `bun:",nullzero" json:"labels"`
	License        string         `bun:",nullzero" json:"license"`
	Readme         string         `bun:",nullzero" json:"readme"`
	DefaultBranch  string         `bun:",notnull" json:"default_branch"`
	LfsFiles       []LfsFile      `bun:"rel:has-many,join:id=repository_id"`
	Tags           []Tag          `bun:"m2m:repository_tags,join:Repository=Tag" json:"tags"`
	RepositoryType RepositoryType `bun:",notnull" json:"repository_type"`
	times
}

type Model struct {
	ID            int64       `bun:",pk,autoincrement" json:"id"`
	Name          string      `bun:",notnull" json:"name"`
	UrlSlug       string      `bun:",notnull" json:"url_slug"`
	Description   string      `bun:",nullzero" json:"description"`
	Likes         int64       `bun:",notnull" json:"likes"`
	Downloads     int64       `bun:",notnull" json:"downloads"`
	Path          string      `bun:",notnull" json:"path"`
	GitPath       string      `bun:",notnull" json:"git_path"`
	RepositoryID  int64       `bun:",notnull" json:"repository_id"`
	Repository    *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time   `bun:",notnull" json:"last"`
	Private       bool        `bun:",notnull" json:"private"`
	UserID        int64       `bun:",notnull" json:"user_id"`
	User          *User       `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	times
}

type Dataset struct {
	ID            int64       `bun:",pk,autoincrement" json:"id"`
	Name          string      `bun:",notnull" json:"name"`
	UrlSlug       string      `bun:",notnull" json:"url_slug"`
	Description   string      `bun:",nullzero" json:"description"`
	Likes         int64       `bun:",notnull" json:"likes"`
	Downloads     int64       `bun:",notnull" json:"downloads"`
	Path          string      `bun:",notnull" json:"path"`
	GitPath       string      `bun:",notnull" json:"git_path"`
	RepositoryID  int64       `bun:",notnull" json:"repository_id"`
	Repository    *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time   `bun:",notnull" json:"last"`
	Private       bool        `bun:",notnull" json:"private"`
	UserID        int64       `bun:",notnull" json:"user_id"`
	User          *User       `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	times
}

type RepositoryTag struct {
	ID           int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64       `bun:",pk" json:"repository_id"`
	TagID        int64       `bun:",pk" json:"tag_id"`
	Repository   *Repository `bun:"rel:belongs-to,join:repository_id=id"`
	Tag          *Tag        `bun:"rel:belongs-to,join:tag_id=id"`
}

type TagStatus int8

type Tag struct {
	ID       int64    `bun:",pk,autoincrement" json:"id"`
	Name     string   `bun:",notnull" json:"name" yaml:"name"`
	Category string   `bun:",notnull" json:"category" yaml:"category"`
	Group    string   `bun:",notnull" json:"group" yaml:"group"`
	Scope    TagScope `bun:",notnull" json:"scope" yaml:"scope"`
	times
}

type LfsFile struct {
	ID           int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64       `bun:",pk" json:"repository_id"`
	Repository   *Repository `bun:"rel:belongs-to,join:repository_id=id"`
	OriginPath   string      `bun:",notnull" json:"orgin_path"`
	times
}

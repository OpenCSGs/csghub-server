package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type Tag struct {
	ID       int64    `bun:",pk,autoincrement" json:"id"`
	Name     string   `bun:",notnull" json:"name" yaml:"name"`
	Category string   `bun:",notnull" json:"category" yaml:"category"`
	Group    string   `bun:",notnull" json:"group" yaml:"group"`
	Scope    TagScope `bun:",notnull" json:"scope" yaml:"scope"`
	BuiltIn  bool     `bun:",notnull" json:"built_in" yaml:"built_in"`
	ShowName string   `bun:"" json:"show_name" yaml:"show_name"`
	times
}

type User struct {
	ID           int64         `bun:",pk,autoincrement" json:"id"`
	GitID        int64         `bun:",notnull" json:"git_id"`
	Name         string        `bun:",notnull" json:"name"`
	Username     string        `bun:",notnull,unique" json:"username"`
	Email        string        `bun:",notnull,unique" json:"email"`
	Password     string        `bun:",notnull" json:"-"`
	AccessTokens []AccessToken `bun:"rel:has-many,join:id=user_id"`
	Namespaces   []Namespace   `bun:"rel:has-many,join:id=user_id" json:"namespace"`
	times
}

type Dataset struct {
	ID            int64       `bun:",pk,autoincrement" json:"id"`
	Name          string      `bun:",notnull" json:"name"`
	UrlSlug       string      `bun:",notnull" json:"nickname"`
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

type Model struct {
	ID            int64       `bun:",pk,autoincrement" json:"id"`
	Name          string      `bun:",notnull" json:"name"`
	UrlSlug       string      `bun:",notnull" json:"nickname"`
	Description   string      `bun:",nullzero" json:"description"`
	Likes         int64       `bun:",notnull" json:"likes"`
	Downloads     int64       `bun:",notnull" json:"downloads"`
	Path          string      `bun:",notnull" json:"path"`
	GitPath       string      `bun:",notnull" json:"git_path"`
	RepositoryID  int64       `bun:",notnull" json:"repository_id"`
	Repository    *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt time.Time   `bun:",notnull" json:"last_updated_at"`
	Private       bool        `bun:",notnull" json:"private"`
	UserID        int64       `bun:",notnull" json:"user_id"`
	User          *User       `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	times
}

type RepositoryTag struct {
	ID           int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64       `bun:",notnull" json:"repository_id"`
	TagID        int64       `bun:",notnull" json:"tag_id"`
	Repository   *Repository `bun:"rel:belongs-to,join:repository_id=id"`
	Tag          *Tag        `bun:"rel:belongs-to,join:tag_id=id"`
	/*
		for meta tags parsed from README.md file, count is alway 1

		for Library tags, count means how many a kind of library file (e.g. *.ONNX file) exists in the repository
	*/
	Count int32 `bun:",default:1" json:"count"`
}

type Repository struct {
	ID            int64     `bun:",pk,autoincrement" json:"id"`
	UserID        int64     `bun:",notnull" json:"user_id"`
	User          User      `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	Path          string    `bun:",notnull" json:"path"`
	GitPath       string    `bun:",notnull" json:"git_path"`
	Name          string    `bun:",notnull" json:"name"`
	Nickname      string    `bun:",notnull" json:"nickname"`
	Description   string    `bun:",nullzero" json:"description"`
	Private       bool      `bun:",notnull" json:"private"`
	Labels        string    `bun:",nullzero" json:"labels"`
	License       string    `bun:",nullzero" json:"license"`
	Readme        string    `bun:",nullzero" json:"readme"`
	DefaultBranch string    `bun:",notnull" json:"default_branch"`
	LfsFiles      []LfsFile `bun:"rel:has-many,join:id=repository_id" json:"-"`

	Likes          int64                `bun:",nullzero" json:"likes"`
	DownloadCount  int64                `bun:",nullzero" json:"download_count"`
	Tags           []Tag                `bun:"m2m:repository_tags,join:Repository=Tag" json:"tags"`
	Metadata       Metadata             `bun:"rel:has-one,join:id=repository_id" json:"metadata"`
	RepositoryType types.RepositoryType `bun:",notnull" json:"repository_type"`
	HTTPCloneURL   string               `bun:",nullzero" json:"http_clone_url"`
	SSHCloneURL    string               `bun:",nullzero" json:"ssh_clone_url"`

	StarCount int `bun:",nullzero" json:"star_count"`
	times
}

type Namespace struct {
	ID            int64         `bun:",pk,autoincrement" json:"id"`
	Path          string        `bun:",notnull" json:"path"`
	UserID        int64         `bun:",notnull" json:"user_id"`
	User          User          `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	NamespaceType NamespaceType `bun:",notnull" json:"namespace_type"`
	times
}

type TagCategory struct {
	ID       int64          `bun:",pk,autoincrement" json:"id"`
	Name     string         `bun:",notnull" json:"name" yaml:"name"`
	ShowName string         `bun:"" json:"show_name" yaml:"show_name"`
	Scope    types.TagScope `bun:",notnull" json:"scope" yaml:"scope"`
	Enabled  bool           `bun:"default:true" json:"enabled" yaml:"enabled"`
}

type LfsFile struct {
	ID           int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64       `bun:",notnull" json:"repository_id"`
	Repository   *Repository `bun:"rel:belongs-to,join:repository_id=id"`
	OriginPath   string      `bun:",notnull" json:"origin_path"`
	times
}

type AccessToken struct {
	ID     int64  `bun:",pk,autoincrement" json:"id"`
	GitID  int64  `bun:",notnull" json:"git_id"`
	Name   string `bun:",notnull" json:"name"`
	Token  string `bun:",notnull" json:"token"`
	UserID int64  `bun:",notnull" json:"user_id"`
	User   *User  `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	times
}

var baseModelTables = []any{
	User{},
	RepositoryTag{},
	Repository{},
	Namespace{},
	Tag{},
	TagCategory{},
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
	ModelTagScope   TagScope = "model"
	DatasetTagScope TagScope = "dataset"
)

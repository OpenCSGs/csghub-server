package database

import (
	"context"
	"fmt"
)

type RepositoryType string

const (
	ModelRepo   RepositoryType = "model"
	DatasetRepo RepositoryType = "dataset"
)

type RepoStore struct {
	db *DB
}

func NewRepoStore(db *DB) *RepoStore {
	return &RepoStore{
		db: db,
	}
}

type Repository struct {
	ID             int64                `bun:",pk,autoincrement" json:"id"`
	UserID         int64                `bun:",notnull" json:"user_id"`
	Path           string               `bun:",notnull" json:"path"`
	GitPath        string               `bun:",notnull" json:"git_path"`
	Name           string               `bun:",notnull" json:"name"`
	Description    string               `bun:",nullzero" json:"description"`
	Private        bool                 `bun:",notnull" json:"private"`
	Labels         string               `bun:",nullzero" json:"labels"`
	License        string               `bun:",nullzero" json:"license"`
	Readme         string               `bun:",nullzero" json:"readme"`
	DefaultBranch  string               `bun:",notnull" json:"default_branch"`
	LfsFiles       []LfsFile            `bun:"rel:has-many,join:id=repository_id" json:"-"`
	Downloads      []RepositoryDownload `bun:"rel:has-many,join:id=repository_id" json:"downloads"`
	Tags           []Tag                `bun:"m2m:repository_tags,join:Repository=Tag" json:"tags"`
	RepositoryType RepositoryType       `bun:",notnull" json:"repository_type"`
	HTTPCloneURL   string               `bun:",nullzero" json:"http_clone_url"`
	SSHCloneURL    string               `bun:",nullzero" json:"ssh_clone_url"`
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

func (s *RepoStore) CreateRepo(ctx context.Context, repo Repository) (err error) {
	err = s.db.Operator.Core.NewInsert().Model(repo).Scan(ctx)
	return
}

func (s *RepoStore) FindyByPath(ctx context.Context, owner string, repoPath string) (repo *Repository, err error) {
	err = s.db.Operator.Core.
		NewSelect().
		Model(&repo).
		Relation("Repository").
		Where("path =?", fmt.Sprintf("%s/%s", owner, repoPath)).
		Where("name =?", repoPath).
		Limit(1).
		Scan(ctx)
	return
}

func (s *RepoStore) FindById(ctx context.Context, id int64) (*Repository, error) {
	resRepo := new(Repository)
	err := s.db.Operator.Core.
		NewSelect().
		Model(resRepo).
		Where("id =?", id).
		Scan(ctx)
	return resRepo, err
}

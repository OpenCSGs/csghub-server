package database

import (
	"context"
	"fmt"

	"opencsg.com/starhub-server/pkg/model"
)

type RepositoryType string

const (
	ModelRepo   RepositoryType = "model"
	DatasetRepo RepositoryType = "dataset"
)

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
	RepositoryType RepositoryType `bun:",notnull" json:"repository_type"`
	times
}

type RepoStore struct {
	db *model.DB
}

func NewRepoStore(db *model.DB) *RepoStore {
	return &RepoStore{
		db: db,
	}
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

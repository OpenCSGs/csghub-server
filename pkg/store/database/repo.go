package database

import (
	"context"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/model"
)

type RepositoryType string

const (
	ModelRepo   RepositoryType = "model"
	DatasetRepo RepositoryType = "dataset"
)

type Repository struct {
	ID             int            `bun:",pk,autoincrement" json:"id"`
	UserID         int            `bun:",notnull" json:"user_id"`
	Path           string         `bun:",notnull" json:"path"`
	Name           string         `bun:",notnull" json:"name"`
	Description    string         `bun:",notnull" json:"description"`
	Private        bool           `bun:",notnull" json:"private"`
	Labels         string         `bun:",notnull" json:"labels"`
	License        string         `bun:",notnull" json:"license"`
	Readme         string         `bun:"," json:"readme"`
	DefaultBranch  string         `bun:"," json:"default_branch"`
	LfsFiles       []*LfsFile     `bun:"rel:has-many,join:id=repository_id"`
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

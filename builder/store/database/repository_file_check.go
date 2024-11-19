package database

import (
	"context"
	"time"

	"opencsg.com/csghub-server/common/types"
)

// RepositoryFileCheck is the sensitive check record for a repository file
type RepositoryFileCheck struct {
	ID         int64                      `bun:",pk,autoincrement" json:"id"`
	RepoFileID int64                      `bun:"," json:"repo_file_id"`
	Status     types.SensitiveCheckStatus `bun:",nullzero" json:"status"`
	Message    string                     `bun:",nullzero" json:"message"`
	CreatedAt  time.Time                  `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	//uuid for async check task
	TaskID string `bun:",nullzero" json:"task_id"`
}

type repoFileCheckStoreImpl struct {
	db *DB
}

type RepoFileCheckStore interface {
	Create(ctx context.Context, history RepositoryFileCheck) error
	Upsert(ctx context.Context, history RepositoryFileCheck) error
}

func NewRepoFileCheckStore() RepoFileCheckStore {
	return &repoFileCheckStoreImpl{
		db: defaultDB,
	}
}

func (s *repoFileCheckStoreImpl) Create(ctx context.Context, history RepositoryFileCheck) error {
	_, err := s.db.Operator.Core.NewInsert().Model(&history).Exec(ctx)
	return err
}

func (s *repoFileCheckStoreImpl) Upsert(ctx context.Context, history RepositoryFileCheck) error {
	_, err := s.db.Operator.Core.NewInsert().Model(&history).
		On("CONFLICT (repo_file_id) DO UPDATE").
		Exec(ctx)
	return err
}

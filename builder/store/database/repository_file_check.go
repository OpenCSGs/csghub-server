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

type RepositoryFileCheckDetail struct {
	Path    string
	Status  types.SensitiveCheckStatus
	Message string
}

type repoFileCheckStoreImpl struct {
	db *DB
}

type RepoFileCheckStore interface {
	Create(ctx context.Context, history RepositoryFileCheck) error
	Upsert(ctx context.Context, history RepositoryFileCheck) error
	ListSensitiveCheckDetails(ctx context.Context, repoID int64, branch string) ([]RepositoryFileCheckDetail, error)
}

func NewRepoFileCheckStore() RepoFileCheckStore {
	return &repoFileCheckStoreImpl{
		db: defaultDB,
	}
}

func NewRepoFileCheckStoreWithDB(db *DB) RepoFileCheckStore {
	return &repoFileCheckStoreImpl{
		db: db,
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

func (s *repoFileCheckStoreImpl) ListSensitiveCheckDetails(ctx context.Context, repoID int64, branch string) ([]RepositoryFileCheckDetail, error) {
	var details []RepositoryFileCheckDetail
	err := s.db.Operator.Core.NewSelect().
		ColumnExpr("rf.path AS path").
		ColumnExpr("rfc.status AS status").
		ColumnExpr("rfc.message AS message").
		TableExpr("repository_file_checks AS rfc").
		Join("INNER JOIN repository_files AS rf ON rf.id = rfc.repo_file_id").
		Where("rf.repository_id = ? AND rf.branch = ?", repoID, branch).
		Where("rfc.status IN (?, ?)", types.SensitiveCheckFail, types.SensitiveCheckException).
		Where("rfc.message <> ''").
		OrderExpr("rf.path ASC").
		Scan(ctx, &details)
	return details, err
}

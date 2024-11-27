package database

import (
	"context"
	"log/slog"
	"time"

	"opencsg.com/csghub-server/common/types"
)

type RepositoryFile struct {
	ID              int64       `bun:",pk,autoincrement" `
	RepositoryID    int64       `bun:",notnull" `
	Path            string      `bun:",notnull" `
	FileType        string      `bun:",notnull" `
	Size            int64       `bun:",nullzero" `
	LastModify      time.Time   `bun:",nullzero" `
	CommitSha       string      `bun:",nullzero" `
	LfsRelativePath string      `bun:",nullzero" `
	Branch          string      `bun:",nullzero" `
	Repository      *Repository `bun:"rel:belongs-to,join:repository_id=id"`
}

type repoFileStoreImpl struct {
	db *DB
}

type RepoFileStore interface {
	Create(ctx context.Context, file *RepositoryFile) error
	BatchGet(ctx context.Context, repoID, lastRepoFileID, batch int64) ([]*RepositoryFile, error)
	BatchGetUnchcked(ctx context.Context, repoID, lastRepoFileID, batch int64) ([]*RepositoryFile, error)
	Exists(ctx context.Context, file RepositoryFile) (bool, error)
	ExistsSensitiveCheckRecord(ctx context.Context, repoID int64, branch string, status types.SensitiveCheckStatus) (bool, error)
}

func NewRepoFileStore() RepoFileStore {
	return &repoFileStoreImpl{
		db: defaultDB,
	}
}

func NewRepoFileStoreWithDB(db *DB) RepoFileStore {
	return &repoFileStoreImpl{
		db: db,
	}
}

func (s *repoFileStoreImpl) Create(ctx context.Context, file *RepositoryFile) error {
	_, err := s.db.Operator.Core.NewInsert().Model(file).Exec(ctx)
	return err
}

func (s *repoFileStoreImpl) BatchGet(ctx context.Context, repoID, lastRepoFileID, batch int64) ([]*RepositoryFile, error) {
	files := make([]*RepositoryFile, 0, batch)
	err := s.db.Operator.Core.NewSelect().
		Model(&files).
		Relation("Repository").
		Where("repository.id = ?", repoID).
		Where("repository_file.id > ? ", lastRepoFileID).
		Order("repository_file.id ASC").
		Limit(int(batch)).
		Scan(ctx)
	return files, err
}

func (s *repoFileStoreImpl) BatchGetUnchcked(ctx context.Context, repoID, lastRepoFileID, batch int64) ([]*RepositoryFile, error) {
	files := make([]*RepositoryFile, 0, batch)
	err := s.db.Operator.Core.NewSelect().
		Model(&files).
		Relation("Repository").
		Join("LEFT JOIN repository_file_checks rfc ON rfc.repo_file_id = repository_file.id").
		Where("repository.id = ?", repoID).
		Where("repository_file.id > ? and (rfc.status is null or rfc.status = 0)", lastRepoFileID).
		Order("repository_file.id ASC").
		Limit(int(batch)).
		Scan(ctx)
	return files, err
}

func (s *repoFileStoreImpl) Exists(ctx context.Context, file RepositoryFile) (bool, error) {
	slog.Debug("file", slog.Any("file", file))
	return s.db.Operator.Core.NewSelect().Model(&file).
		Where("path = ? and repository_id = ? and branch = ? and COALESCE(commit_sha, '') = ?", file.Path, file.RepositoryID, file.Branch, file.CommitSha).
		Exists(ctx)
}

func (s *repoFileStoreImpl) ExistsSensitiveCheckRecord(ctx context.Context, repoID int64, branch string, status types.SensitiveCheckStatus) (bool, error) {
	return s.db.Operator.Core.NewSelect().Model(&RepositoryFileCheck{}).
		Join("INNER JOIN repository_files rf ON rf.id = repository_file_check.repo_file_id").
		Where("rf.repository_id = ? and rf.branch = ? and repository_file_check.status = ?", repoID, branch, status).
		Exists(ctx)
}

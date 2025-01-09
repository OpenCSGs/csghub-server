package database

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

type fileStoreImpl struct {
	db *DB
}

type FileStore interface {
	FindByParentPath(ctx context.Context, repoID int64, path string, pagination *types.OffsetPagination) ([]File, error)
	BatchCreate(ctx context.Context, files []File) error
}

func NewFileStoreWithDB(db *DB) FileStore {
	return &fileStoreImpl{
		db: db,
	}
}

func NewFileStore() FileStore {
	return &fileStoreImpl{
		db: defaultDB,
	}
}

// File represents a file in a repository, *only used by multi-sync client*
type File struct {
	ID                int64       `bun:",pk,autoincrement" json:"id"`
	Name              string      `json:"name"`
	Path              string      `json:"path"`
	ParentPath        string      `json:"parent_path"`
	Size              int64       `json:"size"`
	LastCommitMessage string      `json:"last_commit_message"`
	LastCommitDate    string      `json:"last_commit_date"`
	RepositoryID      int64       `json:"repository_id"`
	Repository        *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	times
}

func (s *fileStoreImpl) FindByParentPath(ctx context.Context, repoID int64, path string, pagination *types.OffsetPagination) ([]File, error) {
	var files []File
	query := s.db.Operator.Core.NewSelect().
		Model(&files).
		Where("parent_path = ? and repository_id = ?", path, repoID)
	if pagination != nil {
		query = query.Limit(pagination.Limit).Offset(pagination.Offset)
	}
	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (s *fileStoreImpl) BatchCreate(ctx context.Context, files []File) error {
	result, err := s.db.Operator.Core.NewInsert().
		Model(&files).
		Exec(ctx)
	if err != nil {
		return err
	}

	return assertAffectedXRows(int64(len(files)), result, err)
}

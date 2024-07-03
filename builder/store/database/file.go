package database

import (
	"context"
)

type FileStore struct {
	db *DB
}

func NewFileStore() *FileStore {
	return &FileStore{
		db: defaultDB,
	}
}

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

func (s *FileStore) FindByParentPath(ctx context.Context, repoID int64, path string) ([]File, error) {
	var files []File
	err := s.db.Operator.Core.NewSelect().
		Model(&files).
		Where("parent_path = ? and repository_id = ?", path, repoID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (s *FileStore) BatchCreate(ctx context.Context, files []File) error {
	result, err := s.db.Operator.Core.NewInsert().
		Model(&files).
		Exec(ctx)
	if err != nil {
		return err
	}

	return assertAffectedXRows(int64(len(files)), result, err)
}

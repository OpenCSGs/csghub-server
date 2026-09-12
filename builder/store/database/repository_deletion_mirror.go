package database

import (
	"context"
	"database/sql"
	"fmt"
)

// RepositoryDeletionMirrorTaskStore resolves the mirror task that must stop
// before asynchronous repository cleanup touches storage or Git data.
type RepositoryDeletionMirrorTaskStore interface {
	FindCurrentMirrorTaskID(ctx context.Context, repositoryID int64) (int64, error)
}

type repositoryDeletionMirrorTaskStore struct{ db *DB }

func NewRepositoryDeletionMirrorTaskStore() RepositoryDeletionMirrorTaskStore {
	return NewRepositoryDeletionMirrorTaskStoreWithDB(defaultDB)
}

func NewRepositoryDeletionMirrorTaskStoreWithDB(db *DB) RepositoryDeletionMirrorTaskStore {
	return &repositoryDeletionMirrorTaskStore{db: db}
}

func (s *repositoryDeletionMirrorTaskStore) FindCurrentMirrorTaskID(ctx context.Context, repositoryID int64) (int64, error) {
	var taskID int64
	err := s.db.Operator.Core.NewSelect().Table("mirrors").Column("current_task_id").
		Where("repository_id = ?", repositoryID).Scan(ctx, &taskID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("find current mirror task: %w", err)
	}
	return taskID, nil
}

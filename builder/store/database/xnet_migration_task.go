package database

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"opencsg.com/csghub-server/common/types"
)

type XnetMigrationTaskStore interface {
	CreateXnetMigrationTask(ctx context.Context, repoID int64, lastMessage string) (*XnetMigrationTask, error)
	EnsureActiveXnetMigrationTask(ctx context.Context, repoID int64) (*XnetMigrationTask, error)
	UpdateXnetMigrationTask(ctx context.Context, id int64, lastMessage string, status types.XnetMigrationTaskStatus) error
	UpdateXnetMigrationTaskIfNonTerminal(ctx context.Context, id int64, lastMessage string, status types.XnetMigrationTaskStatus) error
	CompleteCurrentXnetMigrationTask(ctx context.Context, repoID int64, lastMessage string) error
	GetXnetMigrationTaskByID(ctx context.Context, id int64) (*XnetMigrationTask, error)
	ListXnetMigrationTasksByStatus(ctx context.Context, status types.XnetMigrationTaskStatus) ([]*XnetMigrationTask, error)
	ListXnetMigrationTasksByRepoID(ctx context.Context, repoID int64) ([]*XnetMigrationTask, error)
}

// EnsureActiveXnetMigrationTask returns the repository's current non-terminal
// migration task, or atomically creates and attaches a replacement. Locking the
// repository row prevents concurrent global dispatchers from creating competing
// tasks for the same repository after an explicit retry.
func (s *XnetMigrationTaskStoreImpl) EnsureActiveXnetMigrationTask(ctx context.Context, repoID int64) (*XnetMigrationTask, error) {
	var task XnetMigrationTask
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var repo Repository
		if err := tx.NewSelect().Model(&repo).Column("id", "current_xnet_migration_task_id").
			Where("id = ?", repoID).For("UPDATE").Scan(ctx); err != nil {
			return fmt.Errorf("lock repository %d: %w", repoID, err)
		}
		if repo.CurrentXnetMigrationTaskID != 0 {
			if err := tx.NewSelect().Model(&task).Where("id = ?", repo.CurrentXnetMigrationTaskID).Scan(ctx); err != nil {
				return fmt.Errorf("get current xnet migration task %d: %w", repo.CurrentXnetMigrationTaskID, err)
			}
			if task.Status != types.XnetMigrationTaskStatusCompleted && task.Status != types.XnetMigrationTaskStatusFailed {
				return nil
			}
		}
		task = XnetMigrationTask{RepositoryID: repoID, Status: types.XnetMigrationTaskStatusPending}
		if _, err := tx.NewInsert().Model(&task).Exec(ctx); err != nil {
			return fmt.Errorf("create xnet migration task: %w", err)
		}
		if _, err := tx.NewUpdate().Model((*Repository)(nil)).
			Set("current_xnet_migration_task_id = ?", task.ID).Where("id = ?", repoID).Exec(ctx); err != nil {
			return fmt.Errorf("attach xnet migration task %d: %w", task.ID, err)
		}
		return nil
	})
	return &task, err
}

type XnetMigrationTaskStoreImpl struct {
	db *DB
}

func NewXnetMigrationTaskStore() XnetMigrationTaskStore {
	return &XnetMigrationTaskStoreImpl{
		db: defaultDB,
	}
}

func NewXnetMigrationTaskStoreWithDB(db *DB) XnetMigrationTaskStore {
	return &XnetMigrationTaskStoreImpl{
		db: db,
	}
}

type XnetMigrationTask struct {
	ID           int64                         `bun:"id,pk,autoincrement"`
	RepositoryID int64                         `bun:"repository_id,notnull"`
	LastMessage  string                        `bun:"last_message"`
	Status       types.XnetMigrationTaskStatus `bun:"status,notnull"`

	times
}

func (s *XnetMigrationTaskStoreImpl) CreateXnetMigrationTask(ctx context.Context, repoID int64, lastMessage string) (*XnetMigrationTask, error) {
	task := &XnetMigrationTask{
		RepositoryID: repoID,
		LastMessage:  lastMessage,
		Status:       types.XnetMigrationTaskStatusPending,
	}
	_, err := s.db.Operator.Core.NewInsert().Model(task).Exec(ctx)
	return task, err
}
func (s *XnetMigrationTaskStoreImpl) UpdateXnetMigrationTask(ctx context.Context, id int64, lastMessage string, status types.XnetMigrationTaskStatus) error {
	_, err := s.db.Operator.Core.NewUpdate().Model(&XnetMigrationTask{
		ID:          id,
		LastMessage: lastMessage,
		Status:      status,
	}).WherePK().Exec(ctx)
	return err
}
func (s *XnetMigrationTaskStoreImpl) UpdateXnetMigrationTaskIfNonTerminal(ctx context.Context, id int64, lastMessage string, status types.XnetMigrationTaskStatus) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model((*XnetMigrationTask)(nil)).
		Set("last_message = ?", lastMessage).
		Set("status = ?", status).
		Where("id = ?", id).
		Where("status NOT IN (?, ?)", types.XnetMigrationTaskStatusCompleted, types.XnetMigrationTaskStatusFailed).
		Exec(ctx)
	return err
}

func (s *XnetMigrationTaskStoreImpl) CompleteCurrentXnetMigrationTask(ctx context.Context, repoID int64, lastMessage string) error {
	return s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var repo Repository
		query := tx.NewSelect().
			Model(&repo).
			Column("id", "current_xnet_migration_task_id").
			Where("id = ?", repoID)
		if tx.Dialect().Name() == dialect.PG {
			query = query.For("UPDATE")
		}
		if err := query.Scan(ctx); err != nil {
			return fmt.Errorf("lock repository %d: %w", repoID, err)
		}
		if _, err := tx.NewUpdate().
			Model((*Repository)(nil)).
			Set("xnet_enabled = ?", true).
			Where("id = ?", repoID).
			Exec(ctx); err != nil {
			return fmt.Errorf("enable xnet for repository %d: %w", repoID, err)
		}
		if repo.CurrentXnetMigrationTaskID == 0 {
			return nil
		}
		_, err := tx.NewUpdate().
			Model((*XnetMigrationTask)(nil)).
			Set("last_message = ?", lastMessage).
			Set("status = ?", types.XnetMigrationTaskStatusCompleted).
			Where("id = ?", repo.CurrentXnetMigrationTaskID).
			Where("status IN (?, ?, ?)",
				types.XnetMigrationTaskStatusPending,
				types.XnetMigrationTaskStatusRunning,
				types.XnetMigrationTaskStatusFailed).
			Exec(ctx)
		return err
	})
}
func (s *XnetMigrationTaskStoreImpl) GetXnetMigrationTaskByID(ctx context.Context, id int64) (*XnetMigrationTask, error) {
	var task XnetMigrationTask
	err := s.db.Operator.Core.NewSelect().Model(&task).Where("id = ?", id).Scan(ctx)
	return &task, err
}
func (s *XnetMigrationTaskStoreImpl) ListXnetMigrationTasksByStatus(ctx context.Context, status types.XnetMigrationTaskStatus) ([]*XnetMigrationTask, error) {
	var tasks []*XnetMigrationTask
	err := s.db.Operator.Core.NewSelect().Model(&tasks).Where("status = ?", status).Scan(ctx)
	return tasks, err
}

func (s *XnetMigrationTaskStoreImpl) ListXnetMigrationTasksByRepoID(ctx context.Context, repoID int64) ([]*XnetMigrationTask, error) {
	var tasks []*XnetMigrationTask
	err := s.db.Operator.Core.NewSelect().Model(&tasks).Where("repository_id = ?", repoID).Scan(ctx)
	return tasks, err
}

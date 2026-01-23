package database

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

type XnetMigrationTaskStore interface {
	CreateXnetMigrationTask(ctx context.Context, repoID int64, lastMessage string) (*XnetMigrationTask, error)
	UpdateXnetMigrationTask(ctx context.Context, id int64, lastMessage string, status types.XnetMigrationTaskStatus) error
	GetXnetMigrationTaskByID(ctx context.Context, id int64) (*XnetMigrationTask, error)
	ListXnetMigrationTasksByStatus(ctx context.Context, status types.XnetMigrationTaskStatus) ([]*XnetMigrationTask, error)
	ListXnetMigrationTasksByRepoID(ctx context.Context, repoID int64) ([]*XnetMigrationTask, error)
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

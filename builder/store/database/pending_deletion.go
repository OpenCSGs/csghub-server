package database

import (
	"context"
)

type pendingDeletionStoreImpl struct {
	db *DB
}

type PendingDeletionStore interface {
	Create(ctx context.Context, pendingDeletion *PendingDeletion) (err error)
	FindByTableName(ctx context.Context, tableName string) (pendingDeletions []*PendingDeletion, err error)
}

func NewPendingDeletionStore() PendingDeletionStore {
	return &pendingDeletionStoreImpl{
		db: defaultDB,
	}
}

func NewPendingDeletionStoreWithDB(db *DB) PendingDeletionStore {
	return &pendingDeletionStoreImpl{
		db: db,
	}
}

type PendingDeletion struct {
	ID        int64  `bun:",pk,autoincrement"`
	TableName string `bun:",notnull"`
	Value     string `bun:",notnull"`

	times
}

func (s *pendingDeletionStoreImpl) Create(ctx context.Context, pendingDeletion *PendingDeletion) (err error) {
	err = s.db.Operator.Core.NewInsert().
		Model(pendingDeletion).
		Scan(ctx)
	return
}

func (s *pendingDeletionStoreImpl) FindByTableName(ctx context.Context, tableName string) (pendingDeletions []*PendingDeletion, err error) {
	err = s.db.Operator.Core.NewSelect().
		Model(&pendingDeletions).
		Where("table_name = ?", tableName).
		Scan(ctx)
	return
}

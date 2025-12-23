package database

import (
	"context"

	"opencsg.com/csghub-server/common/errorx"
)

type pendingDeletionStoreImpl struct {
	db *DB
}

type PendingDeletionStore interface {
	Create(ctx context.Context, pendingDeletion *PendingDeletion) (err error)
	FindByTableNameWithBatch(ctx context.Context, tableName PendingDeletionTableName, batchSize, batch int) (pendingDeletions []*PendingDeletion, err error)
	Delete(ctx context.Context, pendingDeletion *PendingDeletion) (err error)
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
	ID        int64                    `bun:",pk,autoincrement"`
	TableName PendingDeletionTableName `bun:",notnull"`
	Value     string                   `bun:",notnull"`

	times
}

type PendingDeletionTableName string

const (
	PendingDeletionTableNameRepository PendingDeletionTableName = "repositories"
)

func (s *pendingDeletionStoreImpl) Create(ctx context.Context, pendingDeletion *PendingDeletion) (err error) {
	err = s.db.Operator.Core.NewInsert().
		Model(pendingDeletion).
		Scan(ctx)
	err = errorx.HandleDBError(err, nil)
	return
}

func (s *pendingDeletionStoreImpl) FindByTableNameWithBatch(
	ctx context.Context,
	tableName PendingDeletionTableName,
	batchSize, batch int,
) (pendingDeletions []*PendingDeletion, err error) {
	err = s.db.Operator.Core.NewSelect().
		Model(&pendingDeletions).
		Where("table_name = ?", tableName).
		Limit(batchSize).
		Offset(batchSize * batch).
		Order("id ASC").
		Scan(ctx)
	err = errorx.HandleDBError(err, nil)
	return
}

func (s *pendingDeletionStoreImpl) Delete(ctx context.Context, pendingDeletion *PendingDeletion) (err error) {
	_, err = s.db.Operator.Core.NewDelete().
		Model(pendingDeletion).
		WherePK().
		Exec(ctx)
	err = errorx.HandleDBError(err, nil)
	return
}

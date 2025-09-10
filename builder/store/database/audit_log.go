package database

import (
	"context"

	"opencsg.com/csghub-server/common/types/enum"
)

type auditLogStoreImpl struct {
	db *DB
}

type AuditLogStore interface {
	Create(ctx context.Context, log *AuditLog) error
}

func NewAuditLogStore() AuditLogStore {
	return &auditLogStoreImpl{db: defaultDB}
}

func NewAuditLogStoreWithDB(db *DB) AuditLogStore {
	return &auditLogStoreImpl{
		db: db,
	}
}

type AuditLog struct {
	ID         int64            `bun:",pk,autoincrement" json:"id"`
	TableName  string           `bun:",notnull" json:"table_name"`
	Action     enum.AuditAction `bun:",notnull" json:"action"`
	Operator   User             `bun:"rel:belongs-to,join:operator_id=id" json:"operator"`
	OperatorID int64            `bun:",notnull" json:"operator_id"`
	Before     string           `bun:",nullzero" json:"before"`
	After      string           `bun:",nullzero" json:"after"`

	times
}

func (s *auditLogStoreImpl) Create(ctx context.Context, log *AuditLog) error {
	_, err := s.db.Operator.Core.NewInsert().Model(log).Exec(ctx)
	return err
}

package database

import (
	"context"

	"opencsg.com/csghub-server/common/errorx"
)

type CredentialAuditLog struct {
	ID            int64  `bun:",pk,autoincrement" json:"id"`
	NamespaceUUID string `bun:"namespace_uuid,nullzero" json:"namespace_uuid"`
	AgentID       string `bun:",nullzero" json:"agent_id"`
	TaskID        string `bun:",nullzero" json:"task_id"`
	SessionID     string `bun:",nullzero" json:"session_id"`
	CredentialID  int64  `bun:",nullzero" json:"credential_id"`
	Provider      string `bun:",nullzero" json:"provider"`
	Action        string `bun:",notnull" json:"action"`
	Result        string `bun:",notnull" json:"result"`
	Reason        string `bun:",nullzero" json:"reason"`
	TraceID       string `bun:",nullzero" json:"trace_id"`
	times
}

type CredentialAuditLogStore interface {
	Create(ctx context.Context, log *CredentialAuditLog) error
}

type credentialAuditLogStoreImpl struct {
	db *DB
}

func NewCredentialAuditLogStore() CredentialAuditLogStore {
	return &credentialAuditLogStoreImpl{db: defaultDB}
}

func NewCredentialAuditLogStoreWithDB(db *DB) CredentialAuditLogStore {
	return &credentialAuditLogStoreImpl{db: db}
}

func (s *credentialAuditLogStoreImpl) Create(ctx context.Context, log *CredentialAuditLog) error {
	_, err := s.db.Operator.Core.NewInsert().Model(log).Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

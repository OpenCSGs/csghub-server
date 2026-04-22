package database

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
)

type TaskCredentialGrant struct {
	ID           int64     `bun:",pk,autoincrement" json:"id"`
	TaskID       string    `bun:",notnull" json:"task_id"`
	SessionID    string    `bun:",notnull" json:"session_id"`
	AgentID      string    `bun:",notnull" json:"agent_id"`
	CredentialID int64     `bun:",notnull" json:"credential_id"`
	ExpiresAt    time.Time `bun:",notnull" json:"expires_at"`
	times
}

type TaskCredentialGrantStore interface {
	Create(ctx context.Context, grant *TaskCredentialGrant) (*TaskCredentialGrant, error)
	CreateBatchWithAuditLogs(ctx context.Context, grants []*TaskCredentialGrant, auditLogs []*CredentialAuditLog) ([]TaskCredentialGrant, error)
	ListBySessionID(ctx context.Context, sessionID string) ([]TaskCredentialGrant, error)
	ListValidBySession(ctx context.Context, sessionID string) ([]TaskCredentialGrant, error)
	FindValidBySessionAndCredentialID(ctx context.Context, sessionID string, credentialID int64) (*TaskCredentialGrant, *Credential, error)
	RevokeBySessionID(ctx context.Context, sessionID string) (int64, error)
}

type taskCredentialGrantStoreImpl struct {
	db *DB
}

func NewTaskCredentialGrantStore() TaskCredentialGrantStore {
	return &taskCredentialGrantStoreImpl{db: defaultDB}
}

func NewTaskCredentialGrantStoreWithDB(db *DB) TaskCredentialGrantStore {
	return &taskCredentialGrantStoreImpl{db: db}
}

func (s *taskCredentialGrantStoreImpl) Create(ctx context.Context, grant *TaskCredentialGrant) (*TaskCredentialGrant, error) {
	_, err := s.db.Operator.Core.NewInsert().Model(grant).Exec(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return grant, nil
}

func (s *taskCredentialGrantStoreImpl) CreateBatchWithAuditLogs(ctx context.Context, grants []*TaskCredentialGrant, auditLogs []*CredentialAuditLog) ([]TaskCredentialGrant, error) {
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		for _, grant := range grants {
			if _, err := tx.NewInsert().Model(grant).Exec(ctx); err != nil {
				return err
			}
		}
		for _, auditLog := range auditLogs {
			if _, err := tx.NewInsert().Model(auditLog).Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	created := make([]TaskCredentialGrant, 0, len(grants))
	for _, grant := range grants {
		created = append(created, *grant)
	}
	return created, nil
}

func (s *taskCredentialGrantStoreImpl) ListBySessionID(ctx context.Context, sessionID string) ([]TaskCredentialGrant, error) {
	var grants []TaskCredentialGrant
	err := s.db.Operator.Core.NewSelect().
		Model(&grants).
		Where("session_id = ?", sessionID).
		Order("id DESC").
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return grants, nil
}

func (s *taskCredentialGrantStoreImpl) ListValidBySession(ctx context.Context, sessionID string) ([]TaskCredentialGrant, error) {
	var grants []TaskCredentialGrant
	err := s.db.Operator.Core.NewSelect().
		Model(&grants).
		Where("session_id = ?", sessionID).
		Where("expires_at > NOW()").
		Order("id DESC").
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return grants, nil
}

func (s *taskCredentialGrantStoreImpl) FindValidBySessionAndCredentialID(ctx context.Context, sessionID string, credentialID int64) (*TaskCredentialGrant, *Credential, error) {
	grants, err := s.ListValidBySession(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	credentialStore := NewCredentialStoreWithDB(s.db)
	for _, grant := range grants {
		if grant.CredentialID != credentialID {
			continue
		}
		g := grant
		credential, err := credentialStore.FindByID(ctx, g.CredentialID)
		if err != nil {
			continue
		}
		if credential.Status == "active" && credential.ArchivedAt.IsZero() {
			return &g, credential, nil
		}
	}
	return nil, nil, errorx.ErrNotFound
}

func (s *taskCredentialGrantStoreImpl) RevokeBySessionID(ctx context.Context, sessionID string) (int64, error) {
	result, err := s.db.Operator.Core.NewUpdate().
		Model((*TaskCredentialGrant)(nil)).
		Set("expires_at = NOW()").
		Where("session_id = ?", sessionID).
		Where("expires_at > NOW()").
		Exec(ctx)
	if err != nil {
		return 0, errorx.HandleDBError(err, nil)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

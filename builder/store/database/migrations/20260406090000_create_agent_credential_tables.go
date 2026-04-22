package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

type CredentialSecret struct {
	ID               int64  `bun:",pk,autoincrement" json:"id"`
	SecretCiphertext []byte `bun:",notnull" json:"secret_ciphertext"`
	SecretNonce      []byte `bun:",notnull" json:"secret_nonce"`
	SecretVersion    int    `bun:",notnull,default:1" json:"secret_version"`
	KMSKeyID         string `bun:",nullzero" json:"kms_key_id"`
	times
}

type Credential struct {
	ID             int64          `bun:",pk,autoincrement" json:"id"`
	NamespaceUUID  string         `bun:"namespace_uuid,notnull" json:"namespace_uuid"`
	CredentialName string         `bun:",notnull" json:"credential_name"`
	Provider       string         `bun:",notnull" json:"provider"`
	AuthType       string         `bun:",notnull" json:"auth_type"`
	Description    string         `bun:",nullzero" json:"description"`
	SecretBackend  string         `bun:",notnull,default:'postgres_encrypted'" json:"secret_backend"`
	SecretRef      string         `bun:",notnull" json:"secret_ref"`
	Metadata       map[string]any `bun:",type:jsonb,nullzero" json:"metadata"`
	Status         string         `bun:",notnull" json:"status"`
	ExpiresAt      time.Time      `bun:",nullzero" json:"expires_at"`
	LastUsedAt     time.Time      `bun:",nullzero" json:"last_used_at"`
	ArchivedAt     time.Time      `bun:",nullzero" json:"archived_at"`
	times
}

type TaskCredentialGrant struct {
	ID           int64     `bun:",pk,autoincrement" json:"id"`
	TaskID       string    `bun:",notnull" json:"task_id"`
	SessionID    string    `bun:",notnull" json:"session_id"`
	AgentID      string    `bun:",notnull" json:"agent_id"`
	CredentialID int64     `bun:",notnull" json:"credential_id"`
	ExpiresAt    time.Time `bun:",notnull" json:"expires_at"`
	times
}

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

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, CredentialSecret{}, Credential{}, TaskCredentialGrant{}, CredentialAuditLog{})
		if err != nil {
			return err
		}

		_, err = db.NewCreateIndex().
			Model((*Credential)(nil)).
			Index("idx_credentials_namespace_provider_status").
			Column("namespace_uuid", "provider", "status").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create idx_credentials_namespace_provider_status failed: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*Credential)(nil)).
			Index("idx_credentials_namespace_name_unique").
			Column("namespace_uuid", "credential_name").
			Unique().
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create idx_credentials_namespace_name_unique failed: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*TaskCredentialGrant)(nil)).
			Index("idx_task_credential_grants_task_session_agent_expires").
			Column("task_id", "session_id", "agent_id", "expires_at").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create idx_task_credential_grants_task_session_agent_expires failed: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model((*CredentialAuditLog)(nil)).
			Index("idx_credential_audit_logs_task_session_provider").
			Column("task_id", "session_id", "provider").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create idx_credential_audit_logs_task_session_provider failed: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, CredentialAuditLog{}, TaskCredentialGrant{}, Credential{}, CredentialSecret{})
	})
}

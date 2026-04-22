package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

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

type CredentialStore interface {
	Create(ctx context.Context, credential *Credential) (*Credential, error)
	ListByUser(ctx context.Context, userUUID string, filter types.CredentialFilter, per int, page int) ([]Credential, int, error)
	FindByID(ctx context.Context, id int64) (*Credential, error)
	FindByName(ctx context.Context, userUUID string, credentialName string) (*Credential, error)
	Update(ctx context.Context, credential *Credential) error
	MarkRevoked(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error
}

type credentialStoreImpl struct {
	db *DB
}

func NewCredentialStore() CredentialStore {
	return &credentialStoreImpl{db: defaultDB}
}

func NewCredentialStoreWithDB(db *DB) CredentialStore {
	return &credentialStoreImpl{db: db}
}

func (s *credentialStoreImpl) Create(ctx context.Context, credential *Credential) (*Credential, error) {
	_, err := s.db.Operator.Core.NewInsert().Model(credential).Exec(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return credential, nil
}

func (s *credentialStoreImpl) ListByUser(ctx context.Context, namespaceUUID string, filter types.CredentialFilter, per int, page int) ([]Credential, int, error) {
	var credentials []Credential
	query := s.db.Operator.Core.NewSelect().
		Model(&credentials).
		Where("namespace_uuid = ?", namespaceUUID).
		Where("status = ?", "active")
	search := strings.TrimSpace(filter.Search)
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("LOWER(credential_name) LIKE LOWER(?)", searchPattern)
	}

	total, err := query.Order("id DESC").
		Limit(per).
		Offset((page - 1) * per).
		ScanAndCount(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, nil)
	}
	return credentials, total, nil
}

func (s *credentialStoreImpl) FindByID(ctx context.Context, id int64) (*Credential, error) {
	var credential Credential
	err := s.db.Operator.Core.NewSelect().Model(&credential).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errorx.ErrNotFound
		}
		return nil, errorx.HandleDBError(err, nil)
	}
	return &credential, nil
}

func (s *credentialStoreImpl) FindByName(ctx context.Context, namespaceUUID string, credentialName string) (*Credential, error) {
	var credential Credential
	err := s.db.Operator.Core.NewSelect().
		Model(&credential).
		Where("namespace_uuid = ?", namespaceUUID).
		Where("credential_name = ?", credentialName).
		Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errorx.ErrNotFound
		}
		return nil, errorx.HandleDBError(err, nil)
	}
	return &credential, nil
}

func (s *credentialStoreImpl) Update(ctx context.Context, credential *Credential) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model(credential).
		Column("credential_name", "description", "metadata", "status", "secret_backend", "secret_ref", "expires_at", "last_used_at", "archived_at", "updated_at").
		WherePK().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update credential failed: %w", errorx.HandleDBError(err, nil))
	}
	return nil
}

func (s *credentialStoreImpl) MarkRevoked(ctx context.Context, id int64) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model((*Credential)(nil)).
		Set("status = ?", "revoked").
		Set("updated_at = NOW()").
		Where("id = ?", id).
		Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

func (s *credentialStoreImpl) Delete(ctx context.Context, id int64) error {
	_, err := s.db.Operator.Core.NewDelete().Model((*Credential)(nil)).Where("id = ?", id).Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

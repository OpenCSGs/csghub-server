package database

import (
	"context"
	"database/sql"
	"fmt"

	"opencsg.com/csghub-server/common/errorx"
)

type CredentialSecret struct {
	ID               int64  `bun:",pk,autoincrement" json:"id"`
	SecretCiphertext []byte `bun:",notnull" json:"secret_ciphertext"`
	SecretNonce      []byte `bun:",notnull" json:"secret_nonce"`
	SecretVersion    int    `bun:",notnull,default:1" json:"secret_version"`
	KMSKeyID         string `bun:",nullzero" json:"kms_key_id"`
	times
}

type CredentialSecretStore interface {
	Create(ctx context.Context, secret *CredentialSecret) (*CredentialSecret, error)
	FindByID(ctx context.Context, id int64) (*CredentialSecret, error)
	UpdateCipher(ctx context.Context, id int64, ciphertext []byte, nonce []byte, version int, kmsKeyID string) error
	Delete(ctx context.Context, id int64) error
}

type credentialSecretStoreImpl struct {
	db *DB
}

func NewCredentialSecretStore() CredentialSecretStore {
	return &credentialSecretStoreImpl{db: defaultDB}
}

func NewCredentialSecretStoreWithDB(db *DB) CredentialSecretStore {
	return &credentialSecretStoreImpl{db: db}
}

func (s *credentialSecretStoreImpl) Create(ctx context.Context, secret *CredentialSecret) (*CredentialSecret, error) {
	_, err := s.db.Operator.Core.NewInsert().Model(secret).Exec(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return secret, nil
}

func (s *credentialSecretStoreImpl) FindByID(ctx context.Context, id int64) (*CredentialSecret, error) {
	var secret CredentialSecret
	err := s.db.Operator.Core.NewSelect().Model(&secret).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errorx.ErrNotFound
		}
		return nil, errorx.HandleDBError(err, nil)
	}
	return &secret, nil
}

func (s *credentialSecretStoreImpl) UpdateCipher(ctx context.Context, id int64, ciphertext []byte, nonce []byte, version int, kmsKeyID string) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model((*CredentialSecret)(nil)).
		Set("secret_ciphertext = ?", ciphertext).
		Set("secret_nonce = ?", nonce).
		Set("secret_version = ?", version).
		Set("kms_key_id = ?", kmsKeyID).
		Set("updated_at = NOW()").
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update credential secret failed: %w", errorx.HandleDBError(err, nil))
	}
	return nil
}

func (s *credentialSecretStoreImpl) Delete(ctx context.Context, id int64) error {
	_, err := s.db.Operator.Core.NewDelete().Model((*CredentialSecret)(nil)).Where("id = ?", id).Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

package database

import (
	"context"
	"time"
)

type sSHKeyStoreImpl struct {
	db *DB
}

type SSHKeyStore interface {
	Index(ctx context.Context, username string, per, page int) (sshKeys []SSHKey, err error)
	Create(ctx context.Context, sshKey *SSHKey) (*SSHKey, error)
	FindByID(ctx context.Context, id int64) (*SSHKey, error)
	FindByFingerpringSHA256(ctx context.Context, fingerprint string) (*SSHKey, error)
	Delete(ctx context.Context, id int64) (err error)
	IsExist(ctx context.Context, username, keyName string) (exists bool, err error)
	FindByUsernameAndName(ctx context.Context, username, keyName string) (sshKey SSHKey, err error)
	FindByKeyContent(ctx context.Context, key string) (*SSHKey, error)
	FindByNameAndUserID(ctx context.Context, name string, userID int64) (*SSHKey, error)
}

func NewSSHKeyStore() SSHKeyStore {
	return &sSHKeyStoreImpl{
		db: defaultDB,
	}
}

func NewSSHKeyStoreWithDB(db *DB) SSHKeyStore {
	return &sSHKeyStoreImpl{
		db: db,
	}
}

type SSHKey struct {
	ID                int64     `bun:",pk,autoincrement" json:"id"`
	GitID             int64     `bun:",notnull" json:"git_id"`
	Name              string    `bun:",notnull" json:"name"`
	Content           string    `bun:",notnull" json:"content"`
	UserID            int64     `bun:",notnull" json:"user_id"`
	User              *User     `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	FingerprintSHA256 string    `bun:"," json:"fingerprint_sha256"`
	DeletedAt         time.Time `bun:",soft_delete,nullzero"`
	times
}

func (s *sSHKeyStoreImpl) Index(ctx context.Context, username string, per, page int) (sshKeys []SSHKey, err error) {
	err = s.db.Operator.Core.
		NewSelect().
		Model(&sshKeys).
		Relation("User").
		Join("JOIN users AS u ON u.id = ssh_key.user_id").
		Where("u.username = ?", username).
		Order("created_at DESC").
		Limit(per).
		Offset((page - 1) * per).
		Scan(ctx)
	return
}

func (s *sSHKeyStoreImpl) Create(ctx context.Context, sshKey *SSHKey) (*SSHKey, error) {
	err := s.db.Operator.Core.
		NewInsert().
		Model(sshKey).
		Returning("*").
		Scan(ctx)
	return sshKey, err
}

func (s *sSHKeyStoreImpl) FindByID(ctx context.Context, id int64) (*SSHKey, error) {
	var sshKey SSHKey
	err := s.db.Operator.Core.
		NewSelect().
		Model(&sshKey).
		Relation("User").
		Where("ssh_key.id = ?", id).
		Scan(ctx)
	return &sshKey, err
}

func (s *sSHKeyStoreImpl) FindByFingerpringSHA256(ctx context.Context, fingerprint string) (*SSHKey, error) {
	var sshKey SSHKey
	err := s.db.Operator.Core.
		NewSelect().
		Model(&sshKey).
		Relation("User").
		Where("ssh_key.fingerprint_sha256 = ?", fingerprint).
		Scan(ctx)
	return &sshKey, err
}

func (s *sSHKeyStoreImpl) Delete(ctx context.Context, id int64) (err error) {
	var sshKey SSHKey
	_, err = s.db.Operator.Core.
		NewDelete().
		Model(&sshKey).
		Where("id = ?", id).
		ForceDelete().
		Exec(ctx)
	return
}

func (s *sSHKeyStoreImpl) IsExist(ctx context.Context, username, keyName string) (exists bool, err error) {
	var sshKey SSHKey
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model(&sshKey).
		Join("JOIN users AS u ON u.id = ssh_key.user_id").
		Where("u.username = ?", username).
		Where("ssh_key.name = ?", keyName).
		Exists(ctx)
	return
}

func (s *sSHKeyStoreImpl) FindByUsernameAndName(ctx context.Context, username, keyName string) (sshKey SSHKey, err error) {
	sshKey.Name = keyName
	err = s.db.Operator.Core.
		NewSelect().
		Model(&sshKey).
		Join("JOIN users AS u ON u.id = ssh_key.user_id").
		Where("u.username = ?", username).
		Where("ssh_key.name = ?", keyName).
		Scan(ctx)
	return sshKey, err
}

func (s *sSHKeyStoreImpl) FindByKeyContent(ctx context.Context, key string) (*SSHKey, error) {
	sshKey := new(SSHKey)
	err := s.db.Operator.Core.
		NewSelect().
		Model(sshKey).
		Where("content = ?", key).
		Scan(ctx)
	return sshKey, err
}

func (s *sSHKeyStoreImpl) FindByNameAndUserID(ctx context.Context, name string, userID int64) (*SSHKey, error) {
	sshKey := new(SSHKey)
	err := s.db.Operator.Core.
		NewSelect().
		Model(sshKey).
		Where("name = ? and user_id = ?", name, userID).
		Scan(ctx)
	return sshKey, err
}

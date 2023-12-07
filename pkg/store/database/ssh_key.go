package database

import (
	"context"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/model"
)

type SSHKeyStore struct {
	db *model.DB
}

func NewSSHKeyStore(db *model.DB) *SSHKeyStore {
	return &SSHKeyStore{
		db: db,
	}
}

type SSHKey struct {
	ID      int64  `bun:",pk,autoincrement" json:"id"`
	GitID   int64  `bun:",notnull" json:"git_id"`
	Name    string `bun:",notnull" json:"name"`
	Content string `bun:",notnull" json:"content"`
	UserID  int64  `bun:",pk" json:"user_id"`
	User    *User  `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	times
}

func (s *SSHKeyStore) Index(ctx context.Context, username string, per, page int) (sshKeys []*SSHKey, err error) {
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

func (s *SSHKeyStore) Create(ctx context.Context, sshKey *SSHKey, user User) (key *SSHKey, err error) {
	sshKey.UserID = user.ID
	err = s.db.Operator.Core.
		NewInsert().
		Model(sshKey).
		Returning("*").
		Scan(ctx)
	return
}

func (s *SSHKeyStore) FindByID(ctx context.Context, id int) (sshKey *SSHKey, err error) {
	var sshKeys []SSHKey
	err = s.db.Operator.Core.
		NewSelect().
		Model(&sshKeys).
		Relation("User").
		Where("ssh_key.id = ?", id).
		Scan(ctx)
	sshKey = &sshKeys[0]
	return
}

func (s *SSHKeyStore) Delete(ctx context.Context, gid int) (err error) {
	var sshKey SSHKey
	_, err = s.db.Operator.Core.
		NewDelete().
		Model(&sshKey).
		Where("gid = ?", gid).
		Exec(ctx)
	return
}

func (s *SSHKeyStore) IsExist(ctx context.Context, username, keyName string) (exists bool, err error) {
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

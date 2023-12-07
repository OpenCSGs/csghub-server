package database

import (
	"context"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/model"
)

type NamespaceStore struct {
	db *model.DB
}

func NewNamespaceStore(db *model.DB) *NamespaceStore {
	return &NamespaceStore{
		db: db,
	}
}

type NamespaceType string

const (
	UserNamespace NamespaceType = "user"
	OrgNamespace  NamespaceType = "organization"
)

type Namespace struct {
	ID            int64         `bun:",pk,autoincrement" json:"id"`
	Path          string        `bun:",notnull" json:"path"`
	UserID        int64         `bun:",pk" json:"user_id"`
	User          User          `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	NamespaceType NamespaceType `bun:",notnull" json:"namespace_type"`
	times
}

func (s *NamespaceStore) FindByPath(ctx context.Context, path string) (namespace Namespace, err error) {
	namespace.Path = path
	err = s.db.Operator.Core.NewSelect().Model(&namespace).Where("path = ?", path).Scan(ctx)
	return
}

func (s *NamespaceStore) Exists(ctx context.Context, path string) (exists bool, err error) {
	var namespace Namespace
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model(&namespace).
		Where("path =?", path).
		Exists(ctx)
	if err != nil {
		return
	}
	return
}

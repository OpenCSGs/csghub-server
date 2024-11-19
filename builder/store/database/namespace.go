package database

import (
	"context"
)

type namespaceStoreImpl struct {
	db *DB
}

type NamespaceStore interface {
	FindByPath(ctx context.Context, path string) (namespace Namespace, err error)
	Exists(ctx context.Context, path string) (exists bool, err error)
}

func NewNamespaceStore() NamespaceStore {
	return &namespaceStoreImpl{db: defaultDB}
}

type NamespaceType string

const (
	UserNamespace NamespaceType = "user"
	OrgNamespace  NamespaceType = "organization"
)

type Namespace struct {
	ID            int64         `bun:",pk,autoincrement" json:"id"`
	Path          string        `bun:",notnull" json:"path"`
	UserID        int64         `bun:",notnull" json:"user_id"`
	User          User          `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	NamespaceType NamespaceType `bun:",notnull" json:"namespace_type"`
	Mirrored      bool          `bun:",notnull" json:"mirrored"`
	times
}

func (s *namespaceStoreImpl) FindByPath(ctx context.Context, path string) (namespace Namespace, err error) {
	namespace.Path = path
	err = s.db.Operator.Core.NewSelect().Model(&namespace).Relation("User").Where("path = ?", path).Scan(ctx)
	return
}

func (s *namespaceStoreImpl) Exists(ctx context.Context, path string) (exists bool, err error) {
	var namespace Namespace
	return s.db.Operator.Core.
		NewSelect().
		Model(&namespace).
		Where("path =?", path).
		Exists(ctx)
}

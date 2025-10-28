package database

import (
	"context"
	"time"

	"opencsg.com/csghub-server/common/errorx"
)

// Define the NamespaceStore interface
type NamespaceStore interface {
	FindByPath(ctx context.Context, path string) (Namespace, error)
	Exists(ctx context.Context, path string) (bool, error)
}

type NamespaceStoreImpl struct {
	db *DB
}

func NewNamespaceStore() NamespaceStore {
	return &NamespaceStoreImpl{db: defaultDB}
}

func NewNamespaceStoreWithDB(db *DB) NamespaceStore {
	return &NamespaceStoreImpl{db: db}
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
	DeletedAt     time.Time     `bun:",soft_delete,nullzero"`
	times
}

func (s *NamespaceStoreImpl) FindByPath(ctx context.Context, path string) (namespace Namespace, err error) {
	namespace.Path = path
	err = s.db.Operator.Core.NewSelect().Model(&namespace).Relation("User").Where("path = ?", path).Scan(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().Set("namespace", path))
	return
}

func (s *NamespaceStoreImpl) Exists(ctx context.Context, path string) (exists bool, err error) {
	var namespace Namespace
	return s.db.Operator.Core.
		NewSelect().
		Model(&namespace).
		Where("path =?", path).
		Exists(ctx)
}
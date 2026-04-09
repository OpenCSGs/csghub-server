package database

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
)

// Define the NamespaceStore interface
type NamespaceStore interface {
	FindByPath(ctx context.Context, path string) (Namespace, error)
	FindByUUID(ctx context.Context, uuid string) (Namespace, error)
	Exists(ctx context.Context, path string) (bool, error)
	ExistsByUUID(ctx context.Context, uuid string) (bool, error)
	FindByUUIDs(ctx context.Context, uuids []string) ([]Namespace, error)
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
	UUID          string        `bun:",nullzero" json:"uuid"`
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

func (s *NamespaceStoreImpl) ExistsByUUID(ctx context.Context, uuid string) (exists bool, err error) {
	var namespace Namespace
	return s.db.Operator.Core.
		NewSelect().
		Model(&namespace).
		Where("uuid =?", uuid).
		Exists(ctx)
}

func (s *NamespaceStoreImpl) FindByUUID(ctx context.Context, uuid string) (namespace Namespace, err error) {
	err = s.db.Operator.Core.
		NewSelect().
		Model(&namespace).
		Where("uuid =?", uuid).
		Scan(ctx, &namespace)
	if err != nil {
		return namespace, errorx.HandleDBError(err, errorx.Ctx().Set("uuid", uuid))
	}
	return namespace, nil
}

func (s *NamespaceStoreImpl) FindByUUIDs(ctx context.Context, uuids []string) (namespaces []Namespace, err error) {
	if len(uuids) == 0 {
		return namespaces, nil
	}

	err = s.db.Operator.Core.
		NewSelect().
		Model(&namespaces).
		Where("uuid IN (?)", bun.In(uuids)).
		Scan(ctx)
	if err != nil {
		return namespaces, errorx.HandleDBError(err, errorx.Ctx().Set("uuids", uuids))
	}

	return namespaces, nil
}

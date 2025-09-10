package database

import (
	"context"

	"opencsg.com/csghub-server/common/errorx"
)

type mirrorNamespaceMappingStoreImpl struct {
	db *DB
}

type MirrorNamespaceMappingStore interface {
	Create(ctx context.Context, mirrorNamespaceMapping *MirrorNamespaceMapping) (*MirrorNamespaceMapping, error)
	Index(ctx context.Context) ([]MirrorNamespaceMapping, error)
	Get(ctx context.Context, id int64) (*MirrorNamespaceMapping, error)
	FindBySourceNamespace(ctx context.Context, name string) (*MirrorNamespaceMapping, error)
	Update(ctx context.Context, mirrorNamespaceMapping *MirrorNamespaceMapping) (MirrorNamespaceMapping, error)
	Delete(ctx context.Context, mirrorNamespaceMapping *MirrorNamespaceMapping) (err error)
}

func NewMirrorNamespaceMappingStore() MirrorNamespaceMappingStore {
	return &mirrorNamespaceMappingStoreImpl{
		db: defaultDB,
	}
}

func NewMirrorNamespaceMappingStoreWithDB(db *DB) MirrorNamespaceMappingStore {
	return &mirrorNamespaceMappingStoreImpl{
		db: db,
	}
}

type MirrorNamespaceMapping struct {
	ID              int64  `bun:",pk,autoincrement" json:"id"`
	TargetNamespace string `bun:",notnull" json:"target_namespace"`
	SourceNamespace string `bun:",notnull,unique" json:"source_namespace"`
	Enabled         *bool  `bun:",notnull,default:true" json:"enabled"`

	times
}

func (s *mirrorNamespaceMappingStoreImpl) Create(ctx context.Context, mirrorNamespaceMapping *MirrorNamespaceMapping) (*MirrorNamespaceMapping, error) {
	if mirrorNamespaceMapping.Enabled == nil {
		var enabled bool = true
		mirrorNamespaceMapping.Enabled = &enabled
	}
	err := s.db.Operator.Core.NewInsert().
		Model(mirrorNamespaceMapping).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx())
	}
	return mirrorNamespaceMapping, nil
}

func (s *mirrorNamespaceMappingStoreImpl) Index(ctx context.Context) ([]MirrorNamespaceMapping, error) {
	var mirrorNamespaceMappings []MirrorNamespaceMapping
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrorNamespaceMappings).
		Order("id desc").
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx())
	}
	return mirrorNamespaceMappings, nil
}

func (s *mirrorNamespaceMappingStoreImpl) Get(ctx context.Context, id int64) (*MirrorNamespaceMapping, error) {
	var mirrorNamespaceMapping MirrorNamespaceMapping
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrorNamespaceMapping).
		Where("id=?", id).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx())
	}
	return &mirrorNamespaceMapping, nil
}

func (s *mirrorNamespaceMappingStoreImpl) FindBySourceNamespace(ctx context.Context, name string) (*MirrorNamespaceMapping, error) {
	var mirrorNamespaceMapping MirrorNamespaceMapping
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrorNamespaceMapping).
		Where("source_namespace=? and enabled=?", name, true).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx())
	}
	return &mirrorNamespaceMapping, nil
}

func (s *mirrorNamespaceMappingStoreImpl) Update(ctx context.Context, mirrorNamespaceMapping *MirrorNamespaceMapping) (m MirrorNamespaceMapping, err error) {
	m.ID = mirrorNamespaceMapping.ID
	query := s.db.Operator.Core.NewUpdate().
		Model(&m).
		WherePK()
	if mirrorNamespaceMapping.Enabled != nil {
		query.Set("enabled = ?", mirrorNamespaceMapping.Enabled)
	}

	if mirrorNamespaceMapping.SourceNamespace != "" {
		query.Set("source_namespace = ?", mirrorNamespaceMapping.SourceNamespace)
	}

	if mirrorNamespaceMapping.TargetNamespace != "" {
		query.Set("target_namespace = ?", mirrorNamespaceMapping.TargetNamespace)
	}

	err = assertAffectedOneRow(query.Exec(ctx))
	if err != nil {
		return m, errorx.HandleDBError(err, errorx.Ctx())
	}

	err = s.db.Core.NewSelect().Model(&m).WherePK().Scan(ctx)

	return m, errorx.HandleDBError(err, errorx.Ctx())
}

func (s *mirrorNamespaceMappingStoreImpl) Delete(ctx context.Context, mirrorNamespaceMapping *MirrorNamespaceMapping) (err error) {
	_, err = s.db.Operator.Core.
		NewDelete().
		Model(mirrorNamespaceMapping).
		WherePK().
		Exec(ctx)
	return errorx.HandleDBError(err, errorx.Ctx())
}

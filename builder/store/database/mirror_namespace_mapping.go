package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/common/errorx"
)

type mirrorNamespaceMappingStoreImpl struct {
	db *DB
}

type MirrorNamespaceMappingStore interface {
	Create(ctx context.Context, mirrorNamespaceMapping *MirrorNamespaceMapping) (*MirrorNamespaceMapping, error)
	Index(ctx context.Context, search string) ([]MirrorNamespaceMapping, error)
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
		var enabled = true
		mirrorNamespaceMapping.Enabled = &enabled
	}
	err := s.db.RunInTx(ctx, func(ctx context.Context, tx Operator) error {
		// Lock the case-insensitive source key even when no mapping row exists yet.
		_, err := tx.Core.NewRaw(
			"SELECT pg_advisory_xact_lock(hashtextextended('mirror_namespace_mapping:' || LOWER(?), 0))",
			mirrorNamespaceMapping.SourceNamespace,
		).Exec(ctx)
		if err != nil {
			return err
		}

		var existing MirrorNamespaceMapping
		exists, err := tx.Core.NewSelect().
			Model(&existing).
			Where("LOWER(source_namespace) = LOWER(?)", mirrorNamespaceMapping.SourceNamespace).
			Exists(ctx)
		if err != nil {
			return err
		}
		if exists {
			return errorx.SourceNamespaceMappingExists(
				fmt.Errorf("source namespace mapping exists: %s", mirrorNamespaceMapping.SourceNamespace),
				errorx.Ctx().Set("source_namespace", mirrorNamespaceMapping.SourceNamespace),
			)
		}

		return tx.Core.NewInsert().Model(mirrorNamespaceMapping).Scan(ctx)
	})
	if err != nil {
		if errors.Is(err, errorx.ErrSourceNamespaceMappingExists) {
			return nil, err
		}
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("source_namespace", mirrorNamespaceMapping.SourceNamespace))
	}
	return mirrorNamespaceMapping, nil
}

func (s *mirrorNamespaceMappingStoreImpl) Index(ctx context.Context, search string) ([]MirrorNamespaceMapping, error) {
	var mirrorNamespaceMappings []MirrorNamespaceMapping
	query := s.db.Operator.Core.NewSelect().
		Model(&mirrorNamespaceMappings)

	if search != "" {
		query = query.Where("LOWER(source_namespace) LIKE ? OR LOWER(target_namespace) LIKE ?", "%"+strings.ToLower(search)+"%", "%"+strings.ToLower(search)+"%")
	}

	err := query.Order("id desc").
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

// FindBySourceNamespace returns an enabled mapping without treating namespace casing as identity.
func (s *mirrorNamespaceMappingStoreImpl) FindBySourceNamespace(ctx context.Context, name string) (*MirrorNamespaceMapping, error) {
	var mirrorNamespaceMapping MirrorNamespaceMapping
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrorNamespaceMapping).
		Where("LOWER(source_namespace) = LOWER(?) and enabled = ?", name, true).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx())
	}
	return &mirrorNamespaceMapping, nil
}

func (s *mirrorNamespaceMappingStoreImpl) Update(ctx context.Context, mirrorNamespaceMapping *MirrorNamespaceMapping) (m MirrorNamespaceMapping, err error) {
	m.ID = mirrorNamespaceMapping.ID
	err = s.db.RunInTx(ctx, func(ctx context.Context, tx Operator) error {
		var stored MirrorNamespaceMapping
		err := tx.Core.NewSelect().
			Model(&stored).
			Where("id = ?", mirrorNamespaceMapping.ID).
			For("UPDATE").
			Scan(ctx)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errorx.SourceNamespaceMappingNotFound(
					fmt.Errorf("source namespace mapping does not exist: id %d", mirrorNamespaceMapping.ID),
					errorx.Ctx().Set("id", mirrorNamespaceMapping.ID),
				)
			}
			return err
		}
		if !strings.EqualFold(strings.TrimSpace(stored.SourceNamespace), strings.TrimSpace(mirrorNamespaceMapping.SourceNamespace)) {
			return errorx.SourceNamespaceMappingNotFound(
				fmt.Errorf("source namespace mapping does not exist: id %d, source namespace %s", mirrorNamespaceMapping.ID, mirrorNamespaceMapping.SourceNamespace),
				errorx.Ctx().
					Set("id", mirrorNamespaceMapping.ID).
					Set("source_namespace", mirrorNamespaceMapping.SourceNamespace),
			)
		}
		if mirrorNamespaceMapping.Enabled == nil && mirrorNamespaceMapping.TargetNamespace == "" {
			m = stored
			return nil
		}

		query := tx.Core.NewUpdate().
			Model(&m).
			WherePK()
		if mirrorNamespaceMapping.Enabled != nil {
			query.Set("enabled = ?", mirrorNamespaceMapping.Enabled)
		}

		if mirrorNamespaceMapping.TargetNamespace != "" {
			query.Set("target_namespace = ?", mirrorNamespaceMapping.TargetNamespace)
		}

		if err := assertAffectedOneRow(query.Exec(ctx)); err != nil {
			return err
		}
		return tx.Core.NewSelect().Model(&m).WherePK().Scan(ctx)
	})
	if err != nil {
		if errors.Is(err, errorx.ErrSourceNamespaceMappingNotFound) {
			return m, err
		}
		return m, errorx.HandleDBError(err, errorx.Ctx())
	}
	return m, nil
}

func (s *mirrorNamespaceMappingStoreImpl) Delete(ctx context.Context, mirrorNamespaceMapping *MirrorNamespaceMapping) (err error) {
	_, err = s.db.Operator.Core.
		NewDelete().
		Model(mirrorNamespaceMapping).
		WherePK().
		Exec(ctx)
	return errorx.HandleDBError(err, errorx.Ctx())
}

package database

import (
	"context"
	"database/sql"
	"errors"

	"opencsg.com/csghub-server/common/errorx"
)

// AgentResourceCuration holds admin-managed curation state for a public
// agent-domain resource: ordering weight, recommended badge, and unlisted
// (hidden from public lists) flag. A missing row means the defaults: listed,
// not recommended, ordered by the resource's own updated_at.
type AgentResourceCuration struct {
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	EntityType  string `bun:",notnull" json:"entity_type"`         // 'agent_instance', etc.
	EntityID    string `bun:",notnull,type:text" json:"entity_id"` // TEXT to support both integer IDs (as strings) and string IDs
	Weight      int64  `bun:",notnull,default:0" json:"weight"`    // Ordering weight within the visible public list, higher first
	Recommended bool   `bun:",notnull,default:false" json:"recommended"`
	Unlisted    bool   `bun:",notnull,default:false" json:"unlisted"`
	times
}

// AgentResourceCurationFields carries the curation columns to upsert; nil
// fields keep their stored values on conflict.
type AgentResourceCurationFields struct {
	Weight      *int64
	Recommended *bool
	Unlisted    *bool
}

// AgentResourceCurationStore provides database operations for AgentResourceCuration
type AgentResourceCurationStore interface {
	UpsertFields(ctx context.Context, entityType string, entityID string, fields AgentResourceCurationFields) error
	GetByEntity(ctx context.Context, entityType string, entityID string) (*AgentResourceCuration, error)
	BatchUpsertWeights(ctx context.Context, entityType string, weights map[string]int64) error
	DeleteByEntity(ctx context.Context, entityType string, entityID string) error
	DeleteAll(ctx context.Context, entityType string) error
}

var _ AgentResourceCurationStore = (*agentResourceCurationStoreImpl)(nil)

type agentResourceCurationStoreImpl struct {
	db *DB
}

// NewAgentResourceCurationStore creates a new AgentResourceCurationStore
func NewAgentResourceCurationStore() AgentResourceCurationStore {
	return &agentResourceCurationStoreImpl{
		db: defaultDB,
	}
}

// NewAgentResourceCurationStoreWithDB creates a new AgentResourceCurationStore with a specific DB
func NewAgentResourceCurationStoreWithDB(db *DB) AgentResourceCurationStore {
	return &agentResourceCurationStoreImpl{
		db: db,
	}
}

// UpsertFields inserts a curation row or updates only the provided columns.
func (s *agentResourceCurationStoreImpl) UpsertFields(ctx context.Context, entityType string, entityID string, fields AgentResourceCurationFields) error {
	curation := &AgentResourceCuration{
		EntityType: entityType,
		EntityID:   normalizeEntityID(entityID),
	}
	q := s.db.Core.NewInsert().Model(curation).
		On("CONFLICT (entity_type, entity_id) DO UPDATE")
	if fields.Weight != nil {
		curation.Weight = *fields.Weight
		q = q.Set("weight = EXCLUDED.weight")
	}
	if fields.Recommended != nil {
		curation.Recommended = *fields.Recommended
		q = q.Set("recommended = EXCLUDED.recommended")
	}
	if fields.Unlisted != nil {
		curation.Unlisted = *fields.Unlisted
		q = q.Set("unlisted = EXCLUDED.unlisted")
	}
	_, err := q.Set("updated_at = NOW()").Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"entity_type": entityType,
			"entity_id":   curation.EntityID,
		})
	}
	return nil
}

// GetByEntity returns the curation row for an entity, or nil when absent.
func (s *agentResourceCurationStoreImpl) GetByEntity(ctx context.Context, entityType string, entityID string) (*AgentResourceCuration, error) {
	normalizedID := normalizeEntityID(entityID)
	curation := &AgentResourceCuration{}
	err := s.db.Core.NewSelect().Model(curation).
		Where("entity_type = ?", entityType).
		Where("entity_id = ?", normalizedID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errorx.HandleDBError(err, map[string]any{
			"entity_type": entityType,
			"entity_id":   normalizedID,
		})
	}
	return curation, nil
}

// BatchUpsertWeights rewrites ordering weights for the given entities in one
// atomic statement, leaving recommended/unlisted untouched.
func (s *agentResourceCurationStoreImpl) BatchUpsertWeights(ctx context.Context, entityType string, weights map[string]int64) error {
	if len(weights) == 0 {
		return nil
	}
	rows := make([]AgentResourceCuration, 0, len(weights))
	for entityID, weight := range weights {
		rows = append(rows, AgentResourceCuration{
			EntityType: entityType,
			EntityID:   normalizeEntityID(entityID),
			Weight:     weight,
		})
	}
	_, err := s.db.Core.NewInsert().Model(&rows).
		On("CONFLICT (entity_type, entity_id) DO UPDATE").
		Set("weight = EXCLUDED.weight").
		Set("updated_at = NOW()").
		Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"entity_type": entityType,
			"rows":        len(rows),
		})
	}
	return nil
}

// DeleteByEntity removes the curation row for a specific entity.
func (s *agentResourceCurationStoreImpl) DeleteByEntity(ctx context.Context, entityType string, entityID string) error {
	_, err := s.db.Core.NewDelete().
		Model((*AgentResourceCuration)(nil)).
		Where("entity_type = ?", entityType).
		Where("entity_id = ?", normalizeEntityID(entityID)).
		Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"entity_type": entityType,
			"entity_id":   entityID,
		})
	}
	return nil
}

// DeleteAll resets curations for one entity type, or for all types when
// entityType is empty.
func (s *agentResourceCurationStoreImpl) DeleteAll(ctx context.Context, entityType string) error {
	// bun requires a Where on deletes; 1=1 keeps the reset-all form valid
	q := s.db.Core.NewDelete().Model((*AgentResourceCuration)(nil)).Where("1 = 1")
	if entityType != "" {
		q = q.Where("entity_type = ?", entityType)
	}
	_, err := q.Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, map[string]any{
			"entity_type": entityType,
		})
	}
	return nil
}

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// Upstream represents a single upstream endpoint linked to an LLMConfig.
type Upstream struct {
	bun.BaseModel `bun:"table:ai_gateway_upstreams"`

	ID                    int64                          `bun:",pk,autoincrement" json:"id"`
	LLMConfigID           int64                          `bun:",notnull" json:"llm_config_id"`
	URL                   string                         `bun:",notnull" json:"url"`
	Weight                int                            `bun:",notnull,default:1" json:"weight"`
	Enabled               bool                           `bun:",notnull,default:true" json:"enabled"`
	ModelName             string                         `bun:",nullzero" json:"model_name"`
	AuthHeader            string                         `bun:",nullzero" json:"auth_header"`
	Provider              string                         `bun:",nullzero" json:"provider"`
	HealthCheckEnabled    bool                           `bun:",notnull,default:false" json:"health_check_enabled"`
	CircuitBreakerEnabled bool                           `bun:",notnull,default:false" json:"circuit_breaker_enabled"`
	Source                types.UpstreamSource           `bun:",notnull,default:external" json:"source"`
	SourceID              int64                          `bun:",notnull,default:0" json:"source_id"`
	Tags                  map[string]string              `bun:",type:jsonb,nullzero" json:"tags,omitempty"`
	Metadata              *types.UpstreamMetadata        `bun:",type:jsonb,nullzero" json:"metadata,omitempty"`
	LimitPolicy           *types.UsageLimitPolicy        `bun:",type:jsonb,nullzero" json:"limit_policy,omitempty"`
	HealthState           *AIGatewayUpstreamHealthState  `bun:"rel:has-one,join:id=upstream_id" json:"health_state,omitempty"`
	CircuitState          *AIGatewayUpstreamCircuitState `bun:"rel:has-one,join:id=upstream_id" json:"circuit_state,omitempty"`
	times
}

// UpstreamStore is the data access interface for upstreams.
type UpstreamStore interface {
	// Create creates a new upstream.
	Create(ctx context.Context, upstream *Upstream) error
	// Update updates an existing upstream.
	Update(ctx context.Context, upstream *Upstream) error
	// Delete deletes an upstream by ID.
	Delete(ctx context.Context, id int64) error
	// GetByID returns an upstream by ID.
	GetByID(ctx context.Context, id int64) (*Upstream, error)
	// ListByLLMConfigID returns all upstreams for a given llm_config.
	ListByLLMConfigID(ctx context.Context, llmConfigID int64) ([]*Upstream, error)
	// ListAllEnabled returns all enabled upstreams across all llm_configs.
	ListAllEnabled(ctx context.Context) ([]*Upstream, error)
	// ListHealthCheckEnabled returns all upstreams with health_check_enabled=true.
	ListHealthCheckEnabled(ctx context.Context) ([]*Upstream, error)
	// DeleteByLLMConfigID deletes all upstreams for a given llm_config.
	DeleteByLLMConfigID(ctx context.Context, llmConfigID int64) error
	// GetBySourceID returns the upstream for a given source and source_id (e.g. csghub deploy).
	// Returns (nil, nil) when no matching upstream exists.
	GetBySourceID(ctx context.Context, source types.UpstreamSource, sourceID int64) (*Upstream, error)
	// UpsertInternalDeployTarget creates or updates an internal deploy upstream.
	// It looks up an existing upstream by source + source_id; if found, only
	// source-owned fields (URL, ModelName, Metadata, Enabled, etc.) are updated.
	// If not found, a new upstream is inserted.
	UpsertInternalDeployTarget(ctx context.Context, upstream *Upstream) error
	// ListEnabledBySource returns all enabled upstreams for a given source.
	ListEnabledBySource(ctx context.Context, source types.UpstreamSource) ([]*Upstream, error)
	// ListEnabledByLogicalModelIDs returns all enabled upstreams whose
	// llm_config_id is in the given set of logical model IDs.
	ListEnabledByLogicalModelIDs(ctx context.Context, modelIDs []int64) ([]*Upstream, error)
}

type upstreamStoreImpl struct {
	db *DB
}

// NewUpstreamStore creates a new UpstreamStore with the default DB.
func NewUpstreamStore(cfg *config.Config) UpstreamStore {
	return &upstreamStoreImpl{db: defaultDB}
}

// NewUpstreamStoreWithDB creates a new UpstreamStore with a given DB.
func NewUpstreamStoreWithDB(db *DB, _ *config.Config) UpstreamStore {
	return &upstreamStoreImpl{db: db}
}

func (s *upstreamStoreImpl) Create(ctx context.Context, upstream *Upstream) error {
	res, err := s.db.Core.NewInsert().Model(upstream).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create upstream: %w", err)
	}
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("create upstream: %w", err)
	}
	return nil
}

func (s *upstreamStoreImpl) Update(ctx context.Context, upstream *Upstream) error {
	_, err := s.db.Core.NewUpdate().Model(upstream).WherePK().Exec(ctx)
	if err != nil {
		return fmt.Errorf("update upstream: %w", err)
	}
	return nil
}

func (s *upstreamStoreImpl) Delete(ctx context.Context, id int64) error {
	_, err := s.db.Core.NewDelete().Model((*Upstream)(nil)).Where("upstream.id = ?", id).Exec(ctx)
	return err
}

func (s *upstreamStoreImpl) GetByID(ctx context.Context, id int64) (*Upstream, error) {
	var u Upstream
	err := s.db.Core.NewSelect().Model(&u).Where("upstream.id = ?", id).Relation("HealthState").Relation("CircuitState").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("get upstream by id: %w", err)
	}
	return &u, nil
}

func (s *upstreamStoreImpl) ListByLLMConfigID(ctx context.Context, llmConfigID int64) ([]*Upstream, error) {
	var upstreams []*Upstream
	err := s.db.Core.NewSelect().Model(&upstreams).Where("upstream.llm_config_id = ?", llmConfigID).Relation("HealthState").Relation("CircuitState").Order("upstream.id ASC").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list upstreams by llm_config_id: %w", err)
	}
	return upstreams, nil
}

func (s *upstreamStoreImpl) ListAllEnabled(ctx context.Context) ([]*Upstream, error) {
	var upstreams []*Upstream
	err := s.db.Core.NewSelect().Model(&upstreams).Where("enabled = true").Order("id ASC").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all enabled upstreams: %w", err)
	}
	return upstreams, nil
}

func (s *upstreamStoreImpl) ListHealthCheckEnabled(ctx context.Context) ([]*Upstream, error) {
	var upstreams []*Upstream
	err := s.db.Core.NewSelect().Model(&upstreams).
		Where("enabled = true").
		Where("health_check_enabled = true").
		Order("id ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list health check enabled upstreams: %w", err)
	}
	return upstreams, nil
}

func (s *upstreamStoreImpl) DeleteByLLMConfigID(ctx context.Context, llmConfigID int64) error {
	_, err := s.db.Core.NewDelete().Model((*Upstream)(nil)).Where("llm_config_id = ?", llmConfigID).Exec(ctx)
	return err
}

func (s *upstreamStoreImpl) GetBySourceID(ctx context.Context, source types.UpstreamSource, sourceID int64) (*Upstream, error) {
	var u Upstream
	err := s.db.Core.NewSelect().Model(&u).
		Where("source = ?", source).
		Where("source_id = ?", sourceID).
		Relation("HealthState").
		Relation("CircuitState").
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get upstream by source %q source_id %d: %w", source, sourceID, err)
	}
	return &u, nil
}

// sourceOwnedColumns are the columns that the deploy sync is allowed to
// overwrite when updating an existing internal upstream. UI-configured fields
// (health_check_enabled, circuit_breaker_enabled, limit_policy, tags, etc.)
// are deliberately excluded so that admin settings survive re-syncs.
var sourceOwnedColumns = []string{
	"url", "model_name", "provider", "enabled",
	"metadata", "source", "source_id", "llm_config_id", "updated_at",
}

func (s *upstreamStoreImpl) UpsertInternalDeployTarget(ctx context.Context, upstream *Upstream) error {
	existing, err := s.GetBySourceID(ctx, upstream.Source, upstream.SourceID)
	if err != nil {
		return fmt.Errorf("upsert internal deploy target: lookup existing: %w", err)
	}
	if existing != nil {
		// Update only source-owned fields on the existing row. Source and
		// SourceID are copied too — they match the lookup key today, but
		// copying them keeps the update correct if the lookup logic changes
		// and avoids silently persisting stale values.
		existing.URL = upstream.URL
		existing.ModelName = upstream.ModelName
		existing.Provider = upstream.Provider
		existing.Enabled = upstream.Enabled
		existing.Metadata = upstream.Metadata
		existing.Source = upstream.Source
		existing.SourceID = upstream.SourceID
		existing.LLMConfigID = upstream.LLMConfigID
		_, err := s.db.Core.NewUpdate().
			Model(existing).
			Column(sourceOwnedColumns...).
			WherePK().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("upsert internal deploy target: update: %w", err)
		}
		// Copy the ID back so the caller has the persisted ID.
		upstream.ID = existing.ID
		return nil
	}
	// Create new upstream.
	res, err := s.db.Core.NewInsert().Model(upstream).Exec(ctx)
	if err != nil {
		return fmt.Errorf("upsert internal deploy target: create: %w", err)
	}
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("upsert internal deploy target: create: %w", err)
	}
	return nil
}

func (s *upstreamStoreImpl) ListEnabledBySource(ctx context.Context, source types.UpstreamSource) ([]*Upstream, error) {
	var upstreams []*Upstream
	err := s.db.Core.NewSelect().Model(&upstreams).
		Where("source = ?", source).
		Where("enabled = true").
		Order("id ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list enabled upstreams by source %q: %w", source, err)
	}
	return upstreams, nil
}

func (s *upstreamStoreImpl) ListEnabledByLogicalModelIDs(ctx context.Context, modelIDs []int64) ([]*Upstream, error) {
	if len(modelIDs) == 0 {
		return nil, nil
	}
	var upstreams []*Upstream
	err := s.db.Core.NewSelect().Model(&upstreams).
		Where("llm_config_id IN (?)", bun.In(modelIDs)).
		Where("enabled = true").
		Order("llm_config_id").
		Order("id").
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return upstreams, nil
}

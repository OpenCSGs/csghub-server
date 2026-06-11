package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	commontypes "opencsg.com/csghub-server/common/types"
)

const AIGenerationResourceTypeVideo = "video"

type AIGeneration struct {
	ID                 int64                      `bun:",pk,autoincrement" json:"id"`
	ResourceType       string                     `bun:",notnull" json:"resource_type"`
	ResourceID         string                     `bun:",notnull" json:"resource_id"`
	ProviderResourceID string                     `bun:",notnull" json:"provider_resource_id"`
	ProviderMetadata   map[string]any             `bun:",type:jsonb,nullzero" json:"provider_metadata"`
	OwnerUUID          string                     `bun:",notnull" json:"owner_uuid"`
	ModelID            string                     `bun:",notnull" json:"model_id"`
	Status             string                     `bun:",notnull" json:"status"`
	FailReason         string                     `bun:",nullzero" json:"fail_reason"`
	Progress           string                     `bun:",nullzero" json:"progress"`
	StartedAt          *time.Time                 `bun:",nullzero" json:"started_at"`
	FinishedAt         *time.Time                 `bun:",nullzero" json:"finished_at"`
	UpstreamID         int64                      `bun:",nullzero" json:"upstream_id"`
	EventUUID          uuid.UUID                  `bun:",type:uuid,nullzero" json:"event_uuid"`
	MeteringMetadata   *commontypes.MeteringEvent `bun:",type:jsonb,nullzero" json:"metering_metadata"`
	EventPublishedAt   *time.Time                 `bun:",nullzero" json:"event_published_at"`
	times
}

type AIGenerationStore interface {
	Create(ctx context.Context, input AIGeneration) (*AIGeneration, error)
	FindByResourceID(ctx context.Context, resourceType, resourceID string) (*AIGeneration, error)
	Update(ctx context.Context, input AIGeneration) (*AIGeneration, error)
	UpdateWithStatus(ctx context.Context, input AIGeneration, fromStatus string) (bool, error)
	UpdateProviderMetadata(ctx context.Context, id int64, providerMetadata map[string]any) error
	PublishMeteringEventInTx(ctx context.Context, id int64, publishFn func(AIGeneration) error) error
}

type AIGenerationMeteringStore interface {
	ListMeteringCandidates(ctx context.Context, staleBefore time.Time, limit int) ([]AIGeneration, error)
}

type aiGenerationStoreImpl struct {
	db *DB
}

func NewAIGenerationStore() AIGenerationStore {
	return &aiGenerationStoreImpl{db: defaultDB}
}

func NewAIGenerationStoreWithDB(db *DB) AIGenerationStore {
	return &aiGenerationStoreImpl{db: db}
}

func NewAIGenerationMeteringStore() AIGenerationMeteringStore {
	return &aiGenerationStoreImpl{db: defaultDB}
}

func NewAIGenerationMeteringStoreWithDB(db *DB) AIGenerationMeteringStore {
	return &aiGenerationStoreImpl{db: db}
}

func (s *aiGenerationStoreImpl) Create(ctx context.Context, input AIGeneration) (*AIGeneration, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("resource_type", input.ResourceType).Set("resource_id", input.ResourceID))
	}
	return &input, nil
}

func (s *aiGenerationStoreImpl) FindByResourceID(ctx context.Context, resourceType, resourceID string) (*AIGeneration, error) {
	var generation AIGeneration
	err := s.db.Core.NewSelect().Model(&generation).
		Where("resource_type = ?", resourceType).
		Where("resource_id = ?", resourceID).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("resource_type", resourceType).Set("resource_id", resourceID))
	}
	return &generation, nil
}

func (s *aiGenerationStoreImpl) Update(ctx context.Context, input AIGeneration) (*AIGeneration, error) {
	res, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("id", input.ID))
	}
	return &input, nil
}

func (s *aiGenerationStoreImpl) UpdateWithStatus(ctx context.Context, input AIGeneration, fromStatus string) (bool, error) {
	res, err := s.db.Core.NewUpdate().Model(&input).
		WherePK().
		Where("status = ?", fromStatus).
		Exec(ctx)
	if err != nil {
		return false, errorx.HandleDBError(err, errorx.Ctx().Set("id", input.ID).Set("from_status", fromStatus))
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, errorx.HandleDBError(err, errorx.Ctx().Set("id", input.ID).Set("from_status", fromStatus))
	}
	return rowsAffected > 0, nil
}

func (s *aiGenerationStoreImpl) UpdateProviderMetadata(ctx context.Context, id int64, providerMetadata map[string]any) error {
	res, err := s.db.Core.NewUpdate().Model((*AIGeneration)(nil)).
		Where("id = ?", id).
		Set("provider_metadata = ?", providerMetadata).
		Set("updated_at = ?", time.Now()).
		Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return errorx.HandleDBError(err, errorx.Ctx().Set("id", id))
	}
	return nil
}

func (s *aiGenerationStoreImpl) PublishMeteringEventInTx(ctx context.Context, id int64, publishFn func(AIGeneration) error) error {
	if publishFn == nil {
		return fmt.Errorf("publish metering event function is nil")
	}
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var generation AIGeneration
		if err := tx.NewSelect().Model(&generation).Where("id = ?", id).For("UPDATE").Scan(ctx); err != nil {
			return errorx.HandleDBError(err, errorx.Ctx().Set("id", id))
		}
		if generation.EventPublishedAt != nil || generation.Status != string(commontypes.AIGatewayAsyncGenerationStatusCompleted) {
			return nil
		}
		if generation.EventUUID == uuid.Nil {
			return fmt.Errorf("ai generation %d event uuid is empty", id)
		}
		if err := publishFn(generation); err != nil {
			return err
		}
		now := time.Now()
		generation.EventPublishedAt = &now
		res, err := tx.NewUpdate().Model(&generation).
			WherePK().
			Column("event_published_at").
			Exec(ctx)
		if err := assertAffectedOneRow(res, err); err != nil {
			return errorx.HandleDBError(err, errorx.Ctx().Set("id", id))
		}
		return nil
	})
}

func (s *aiGenerationStoreImpl) ListMeteringCandidates(ctx context.Context, staleBefore time.Time, limit int) ([]AIGeneration, error) {
	if limit <= 0 {
		limit = 100
	}
	var generations []AIGeneration
	err := s.db.Core.NewSelect().Model(&generations).
		Where("event_published_at IS NULL").
		WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.
				WhereOr("status = ?", commontypes.AIGatewayAsyncGenerationStatusCompleted).
				WhereOr("status IN (?) AND updated_at < ?", bun.In([]commontypes.AIGatewayAsyncGenerationStatus{
					commontypes.AIGatewayAsyncGenerationStatusQueued,
					commontypes.AIGatewayAsyncGenerationStatusInProgress,
				}), staleBefore)
		}).
		Order("updated_at ASC").
		Limit(limit).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("stale_before", staleBefore).Set("limit", limit))
	}
	return generations, nil
}

package database

import (
	"context"
	"fmt"
)

const AIGenerationResourceTypeVideo = "video"

type AIGeneration struct {
	ID                 int64          `bun:",pk,autoincrement" json:"id"`
	ResourceType       string         `bun:",notnull" json:"resource_type"`
	ResourceID         string         `bun:",notnull" json:"resource_id"`
	ProviderResourceID string         `bun:",notnull" json:"provider_resource_id"`
	ProviderMetadata   map[string]any `bun:",type:jsonb,nullzero" json:"provider_metadata"`
	OwnerUUID          string         `bun:",notnull" json:"owner_uuid"`
	ModelID            string         `bun:",notnull" json:"model_id"`
	Status             string         `bun:",notnull" json:"status"`
	times
}

type AIGenerationStore interface {
	Create(ctx context.Context, input AIGeneration) (*AIGeneration, error)
	FindByResourceID(ctx context.Context, resourceType, resourceID string) (*AIGeneration, error)
	Update(ctx context.Context, input AIGeneration) (*AIGeneration, error)
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

func (s *aiGenerationStoreImpl) Create(ctx context.Context, input AIGeneration) (*AIGeneration, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("insert ai generation in db error: %w", err)
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
		return nil, fmt.Errorf("find ai generation by resource %s/%s error: %w", resourceType, resourceID, err)
	}
	return &generation, nil
}

func (s *aiGenerationStoreImpl) Update(ctx context.Context, input AIGeneration) (*AIGeneration, error) {
	res, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("update ai generation %d error: %w", input.ID, err)
	}
	return &input, nil
}

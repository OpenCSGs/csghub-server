package task

import (
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

func generationToTarget(generation database.AIGeneration) commontypes.AIGatewayAsyncGenerationTarget {
	return commontypes.AIGatewayAsyncGenerationTarget{
		ID:                 generation.ID,
		ResourceType:       generation.ResourceType,
		ResourceID:         generation.ResourceID,
		ProviderResourceID: generation.ProviderResourceID,
		ProviderMetadata:   generation.ProviderMetadata,
		UpstreamID:         generation.UpstreamID,
		MeteringMetadata:   generation.MeteringMetadata,
		OwnerUUID:          generation.OwnerUUID,
		ModelID:            generation.ModelID,
		Status:             generation.Status,
		FailReason:         generation.FailReason,
		Progress:           generation.Progress,
		CreatedAt:          generation.CreatedAt,
		StartedAt:          generation.StartedAt,
		FinishedAt:         generation.FinishedAt,
		EventUUID:          generation.EventUUID,
		EventPublishedAt:   generation.EventPublishedAt,
	}
}

func generationFromTarget(target commontypes.AIGatewayAsyncGenerationTarget) database.AIGeneration {
	return database.AIGeneration{
		ID:                 target.ID,
		ResourceType:       target.ResourceType,
		ResourceID:         target.ResourceID,
		ProviderResourceID: target.ProviderResourceID,
		ProviderMetadata:   target.ProviderMetadata,
		UpstreamID:         target.UpstreamID,
		MeteringMetadata:   target.MeteringMetadata,
		OwnerUUID:          target.OwnerUUID,
		ModelID:            target.ModelID,
		Status:             target.Status,
		FailReason:         target.FailReason,
		Progress:           target.Progress,
		StartedAt:          target.StartedAt,
		FinishedAt:         target.FinishedAt,
		EventUUID:          target.EventUUID,
		EventPublishedAt:   target.EventPublishedAt,
	}
}

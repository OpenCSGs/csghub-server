package task

import (
	"strings"

	taskprocessor "opencsg.com/csghub-server/aigateway/task/processor"
	"opencsg.com/csghub-server/builder/store/database"
)

func buildProcessorMap(processors []taskprocessor.ResourceProcessor) map[string]taskprocessor.ResourceProcessor {
	processorMap := make(map[string]taskprocessor.ResourceProcessor, len(processors))
	for _, processor := range processors {
		if processor == nil || strings.TrimSpace(processor.ResourceType()) == "" {
			continue
		}
		processorMap[processor.ResourceType()] = processor
	}
	return processorMap
}

func applyGenerationStatus(generation *database.AIGeneration, status *taskprocessor.GenerationStatus) {
	if generation == nil || status == nil {
		return
	}
	if status.Status != "" {
		generation.Status = status.Status
	}
	if status.Progress != "" {
		generation.Progress = status.Progress
	}
	if status.FailReason != "" {
		generation.FailReason = status.FailReason
	}
	if status.StartedAt != nil {
		generation.StartedAt = status.StartedAt
	}
	if status.FinishedAt != nil {
		generation.FinishedAt = status.FinishedAt
	}
}

func generationRefFromGeneration(generation database.AIGeneration) taskprocessor.GenerationRef {
	return taskprocessor.GenerationRef{
		ID:                 generation.ID,
		ResourceID:         generation.ResourceID,
		ProviderResourceID: generation.ProviderResourceID,
		ProviderMetadata:   generation.ProviderMetadata,
		UpstreamID:         generation.UpstreamID,
		ModelID:            generation.ModelID,
		Status:             generation.Status,
		StartedAt:          generation.StartedAt,
		FinishedAt:         generation.FinishedAt,
	}
}

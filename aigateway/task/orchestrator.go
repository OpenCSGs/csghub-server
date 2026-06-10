package task

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	taskprocessor "opencsg.com/csghub-server/aigateway/task/processor"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
	commonutils "opencsg.com/csghub-server/common/utils/common"
)

func (s *asyncGenerationService) ListPendingGenerations(ctx context.Context) ([]commontypes.AIGatewayAsyncGenerationTarget, error) {
	staleBefore := time.Now().Add(-s.refreshInterval)
	generations, err := s.meteringStore.ListMeteringCandidates(ctx, staleBefore, s.batchSize)
	if err != nil {
		return nil, err
	}

	targets := make([]commontypes.AIGatewayAsyncGenerationTarget, 0, len(generations))
	for _, generation := range generations {
		targets = append(targets, generationToTarget(generation))
	}
	return targets, nil
}

func (s *asyncGenerationService) InspectAndMeter(ctx context.Context, target commontypes.AIGatewayAsyncGenerationTarget) error {
	if s.maxAge > 0 && time.Since(target.CreatedAt) > s.maxAge {
		return s.markGenerationTimedOut(ctx, target)
	}
	generation := generationFromTarget(target)
	processor := s.processors[generation.ResourceType]
	if processor == nil {
		return fmt.Errorf("async generation processor not found for resource type %q", generation.ResourceType)
	}

	if !isTerminalStatus(generation.Status) {
		if err := s.refreshGeneration(ctx, processor, &generation); err != nil {
			return err
		}
	}
	if !isCompletedStatus(generation.Status) || generation.EventPublishedAt != nil {
		return nil
	}

	return s.store.PublishMeteringEventInTx(ctx, generation.ID, func(locked database.AIGeneration) error {
		return s.publishMeteringEvent(ctx, &locked)
	})
}

func (s *asyncGenerationService) markGenerationTimedOut(ctx context.Context, target commontypes.AIGatewayAsyncGenerationTarget) error {
	generation := generationFromTarget(target)
	oldStatus := generation.Status
	if isTerminalStatus(oldStatus) {
		return nil
	}
	now := time.Now()
	generation.Status = string(commontypes.AIGatewayAsyncGenerationStatusFailed)
	generation.FailReason = fmt.Sprintf("async generation exceeded max age of %s without reaching a terminal status", s.maxAge)
	generation.FinishedAt = &now

	won, err := s.store.UpdateWithStatus(ctx, generation, oldStatus)
	if err != nil {
		return err
	}
	if !won {
		slog.WarnContext(ctx,
			"async generation already transitioned before timeout",
			slog.Int64("generation_id", target.ID),
			slog.String("old_status", oldStatus),
		)
		return nil
	}
	slog.WarnContext(ctx,
		"async generation timed out and marked failed",
		slog.Int64("generation_id", target.ID),
		slog.String("old_status", oldStatus),
		slog.String("fail_reason", generation.FailReason),
		slog.Duration("max_age", s.maxAge),
	)
	return nil
}

func (s *asyncGenerationService) refreshGeneration(ctx context.Context, processor taskprocessor.ResourceProcessor, generation *database.AIGeneration) error {
	oldStatus := generation.Status
	status, err := processor.Refresh(ctx, generationRefFromGeneration(*generation))
	if err != nil {
		return err
	}
	if status == nil {
		return nil
	}
	applyGenerationStatus(generation, status)
	commonutils.MergeMapWithDeletion(&generation.ProviderMetadata, status.ProviderMetadata)

	won, err := s.store.UpdateWithStatus(ctx, *generation, oldStatus)
	if err != nil {
		return err
	}
	if !won {
		slog.WarnContext(
			ctx,
			"async generation already transitioned by another process",
			slog.Int64("generation_id", generation.ID),
			slog.String("old_status", oldStatus),
			slog.String("new_status", generation.Status),
		)
	}
	return nil
}

package task

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/store/database"
)

func (s *asyncGenerationService) publishMeteringEvent(ctx context.Context, generation *database.AIGeneration) error {
	if s.eventPub == nil {
		return fmt.Errorf("aigateway async generation publisher is not configured")
	}
	if generation.MeteringMetadata == nil {
		return fmt.Errorf("missing async generation metering event metadata")
	}
	meteringEvent := *generation.MeteringMetadata
	meteringEvent.Uuid = generation.EventUUID
	eventData, err := json.Marshal(meteringEvent)
	if err != nil {
		return fmt.Errorf("marshal async generation metering event: %w", err)
	}
	if err := s.eventPub.PublishMeteringEvent(eventData); err != nil {
		return fmt.Errorf("publish async generation metering event: %w", err)
	}
	slog.InfoContext(ctx, "published aigateway async generation metering event", slog.String("resource_type", generation.ResourceType), slog.String("resource_id", generation.ResourceID), slog.String("event_uuid", generation.EventUUID.String()))
	return nil
}

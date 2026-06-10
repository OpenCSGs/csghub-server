package types

import (
	"time"

	"github.com/google/uuid"
)

type AIGatewayAsyncGenerationStatus string

const (
	AIGatewayAsyncGenerationStatusQueued     AIGatewayAsyncGenerationStatus = "queued"
	AIGatewayAsyncGenerationStatusInProgress AIGatewayAsyncGenerationStatus = "in_progress"
	AIGatewayAsyncGenerationStatusCompleted  AIGatewayAsyncGenerationStatus = "completed"
	AIGatewayAsyncGenerationStatusFailed     AIGatewayAsyncGenerationStatus = "failed"
	AIGatewayAsyncGenerationStatusCancelled  AIGatewayAsyncGenerationStatus = "cancelled"
)

type AIGatewayAsyncGenerationTarget struct {
	ID                 int64
	ResourceType       string
	ResourceID         string
	ProviderResourceID string
	ProviderMetadata   map[string]any
	UpstreamID         int64
	MeteringMetadata   *MeteringEvent
	OwnerUUID          string
	ModelID            string
	Status             string
	FailReason         string
	Progress           string
	CreatedAt          time.Time
	StartedAt          *time.Time
	FinishedAt         *time.Time
	EventUUID          uuid.UUID
	EventPublishedAt   *time.Time
}

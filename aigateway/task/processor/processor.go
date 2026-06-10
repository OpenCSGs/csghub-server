package processor

import (
	"context"
	"time"
)

type ResourceProcessor interface {
	ResourceType() string
	Refresh(ctx context.Context, ref GenerationRef) (*GenerationStatus, error)
}

type GenerationRef struct {
	ID                 int64
	ResourceID         string
	ProviderResourceID string
	ProviderMetadata   map[string]any
	UpstreamID         int64
	ModelID            string
	Status             string
	StartedAt          *time.Time
	FinishedAt         *time.Time
}

type GenerationStatus struct {
	Status           string
	FailReason       string
	Progress         string
	StartedAt        *time.Time
	FinishedAt       *time.Time
	ProviderMetadata map[string]any
}

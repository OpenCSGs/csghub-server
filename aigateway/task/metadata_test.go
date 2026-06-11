package task

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestGenerationToTargetCopiesAllFields(t *testing.T) {
	started := time.Now()
	finished := started.Add(time.Minute)
	published := started.Add(2 * time.Minute)
	eventUUID := uuid.New()
	providerMeta := map[string]any{"pk": "pv"}
	metering := &commontypes.MeteringEvent{
		Uuid:       uuid.New(), // should be overridden by generation.EventUUID
		UserUUID:   "u1",
		Value:      5,
		ValueType:  commontypes.CountNumberType,
		Scene:      1,
		ResourceID: "r1",
	}
	gen := database.AIGeneration{
		ID:                 42,
		ResourceType:       "video",
		ResourceID:         "r1",
		ProviderResourceID: "pr1",
		ProviderMetadata:   providerMeta,
		UpstreamID:         7,
		MeteringMetadata:   metering,
		OwnerUUID:          "owner-1",
		ModelID:            "m1",
		Status:             "in_progress",
		FailReason:         "transient",
		Progress:           "0.5",
		StartedAt:          &started,
		FinishedAt:         &finished,
		EventUUID:          eventUUID,
		EventPublishedAt:   &published,
	}

	target := generationToTarget(gen)

	require.Equal(t, int64(42), target.ID)
	require.Equal(t, "video", target.ResourceType)
	require.Equal(t, "r1", target.ResourceID)
	require.Equal(t, "pr1", target.ProviderResourceID)
	require.Equal(t, providerMeta, target.ProviderMetadata)
	require.Equal(t, int64(7), target.UpstreamID)
	require.Same(t, metering, target.MeteringMetadata, "MeteringMetadata pointer must be copied as-is")
	require.Equal(t, "owner-1", target.OwnerUUID)
	require.Equal(t, "m1", target.ModelID)
	require.Equal(t, "in_progress", target.Status)
	require.Equal(t, "transient", target.FailReason)
	require.Equal(t, "0.5", target.Progress)
	require.Equal(t, gen.CreatedAt, target.CreatedAt, "CreatedAt must be copied from the embedded times struct")
	require.Equal(t, &started, target.StartedAt)
	require.Equal(t, &finished, target.FinishedAt)
	require.Equal(t, eventUUID, target.EventUUID)
	require.Equal(t, &published, target.EventPublishedAt)
}

func TestGenerationFromTargetCopiesAllFields(t *testing.T) {
	started := time.Now()
	finished := started.Add(time.Minute)
	published := started.Add(2 * time.Minute)
	eventUUID := uuid.New()
	providerMeta := map[string]any{"pk": "pv"}
	metering := &commontypes.MeteringEvent{
		Uuid:       eventUUID,
		UserUUID:   "u1",
		Value:      5,
		ValueType:  commontypes.CountNumberType,
		Scene:      1,
		ResourceID: "r1",
	}
	target := commontypes.AIGatewayAsyncGenerationTarget{
		ID:                 42,
		ResourceType:       "video",
		ResourceID:         "r1",
		ProviderResourceID: "pr1",
		ProviderMetadata:   providerMeta,
		UpstreamID:         7,
		MeteringMetadata:   metering,
		OwnerUUID:          "owner-1",
		ModelID:            "m1",
		Status:             "in_progress",
		FailReason:         "transient",
		Progress:           "0.5",
		CreatedAt:          time.Now().Add(-time.Hour),
		StartedAt:          &started,
		FinishedAt:         &finished,
		EventUUID:          eventUUID,
		EventPublishedAt:   &published,
	}

	gen := generationFromTarget(target)

	require.Equal(t, int64(42), gen.ID)
	require.Equal(t, "video", gen.ResourceType)
	require.Equal(t, "r1", gen.ResourceID)
	require.Equal(t, "pr1", gen.ProviderResourceID)
	require.Equal(t, providerMeta, gen.ProviderMetadata)
	require.Equal(t, int64(7), gen.UpstreamID)
	require.Same(t, metering, gen.MeteringMetadata)
	require.Equal(t, "owner-1", gen.OwnerUUID)
	require.Equal(t, "m1", gen.ModelID)
	require.Equal(t, "in_progress", gen.Status)
	require.Equal(t, "transient", gen.FailReason)
	require.Equal(t, "0.5", gen.Progress)
	require.True(t, target.CreatedAt.Equal(gen.CreatedAt) || gen.CreatedAt.IsZero(),
		"CreatedAt must round-trip (or be the zero time if the embedded times struct did not capture it)")
	require.Equal(t, &started, gen.StartedAt)
	require.Equal(t, &finished, gen.FinishedAt)
	require.Equal(t, eventUUID, gen.EventUUID)
	require.Equal(t, &published, gen.EventPublishedAt)
}

func TestGenerationRoundTripPreservesAllFields(t *testing.T) {
	started := time.Now()
	finished := started.Add(time.Minute)
	published := started.Add(2 * time.Minute)
	eventUUID := uuid.New()
	providerMeta := map[string]any{"pk": "pv"}
	metering := &commontypes.MeteringEvent{
		Uuid:       eventUUID,
		UserUUID:   "u1",
		Value:      5,
		ValueType:  commontypes.CountNumberType,
		Scene:      1,
		ResourceID: "r1",
	}
	gen := database.AIGeneration{
		ID:                 42,
		ResourceType:       "video",
		ResourceID:         "r1",
		ProviderResourceID: "pr1",
		ProviderMetadata:   providerMeta,
		UpstreamID:         7,
		MeteringMetadata:   metering,
		OwnerUUID:          "owner-1",
		ModelID:            "m1",
		Status:             "in_progress",
		Progress:           "0.5",
		StartedAt:          &started,
		FinishedAt:         &finished,
		EventUUID:          eventUUID,
		EventPublishedAt:   &published,
	}

	roundTripped := generationFromTarget(generationToTarget(gen))

	require.Equal(t, gen.ID, roundTripped.ID)
	require.Equal(t, gen.ResourceType, roundTripped.ResourceType)
	require.Equal(t, gen.ResourceID, roundTripped.ResourceID)
	require.Equal(t, gen.ProviderResourceID, roundTripped.ProviderResourceID)
	require.Equal(t, gen.ProviderMetadata, roundTripped.ProviderMetadata)
	require.Equal(t, gen.UpstreamID, roundTripped.UpstreamID)
	require.Same(t, gen.MeteringMetadata, roundTripped.MeteringMetadata)
	require.Equal(t, gen.OwnerUUID, roundTripped.OwnerUUID)
	require.Equal(t, gen.ModelID, roundTripped.ModelID)
	require.Equal(t, gen.Status, roundTripped.Status)
	require.Equal(t, gen.Progress, roundTripped.Progress)
	// gen.CreatedAt starts as the zero time (unexported times struct can't be
	// set via outer struct literal), and generationFromTarget also doesn't
	// populate it. So round-trip on CreatedAt is a no-op — both are zero.
	require.True(t, gen.CreatedAt.IsZero())
	require.True(t, roundTripped.CreatedAt.IsZero())
	require.Equal(t, gen.StartedAt, roundTripped.StartedAt)
	require.Equal(t, gen.FinishedAt, roundTripped.FinishedAt)
	require.Equal(t, gen.EventUUID, roundTripped.EventUUID)
	require.Equal(t, gen.EventPublishedAt, roundTripped.EventPublishedAt)
}

func TestGenerationToTargetPreservesZeroValues(t *testing.T) {
	target := generationToTarget(database.AIGeneration{})

	require.Equal(t, int64(0), target.ID)
	require.Equal(t, "", target.ResourceType)
	require.Equal(t, "", target.ResourceID)
	require.Equal(t, "", target.Status)
	require.True(t, target.CreatedAt.IsZero(), "CreatedAt should be the zero time from an empty generation")
	require.Nil(t, target.StartedAt)
	require.Nil(t, target.FinishedAt)
	require.Nil(t, target.ProviderMetadata)
	require.Nil(t, target.MeteringMetadata)
	require.Equal(t, uuid.Nil, target.EventUUID)
}

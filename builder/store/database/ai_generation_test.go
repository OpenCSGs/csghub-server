package database_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestAIGenerationStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAIGenerationStoreWithDB(db)

	generation, err := store.Create(ctx, database.AIGeneration{
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_gateway_1",
		ProviderResourceID: "vid_123",
		ProviderMetadata:   map[string]any{"file_id": "file_123"},
		OwnerUUID:          "user-1",
		ModelID:            "openai/gpt-video",
		Status:             string(commontypes.AIGatewayAsyncGenerationStatusQueued),
		Progress:           "0.5",
		UpstreamID:         123,
		MeteringMetadata:   &commontypes.MeteringEvent{Value: 1},
		EventUUID:          uuid.New(),
	})
	require.NoError(t, err)
	require.NotZero(t, generation.ID)

	_, err = store.Create(ctx, database.AIGeneration{
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_gateway_2",
		ProviderResourceID: "vid_123",
		OwnerUUID:          "user-2",
		ModelID:            "other/gpt-video",
		Status:             string(commontypes.AIGatewayAsyncGenerationStatusQueued),
	})
	require.NoError(t, err)

	generation, err = store.FindByResourceID(ctx, database.AIGenerationResourceTypeVideo, "video_gateway_1")
	require.NoError(t, err)
	require.Equal(t, "user-1", generation.OwnerUUID)
	require.Equal(t, "vid_123", generation.ProviderResourceID)
	require.Equal(t, "file_123", generation.ProviderMetadata["file_id"])
	require.Equal(t, "0.5", generation.Progress)
	require.Equal(t, int64(123), generation.UpstreamID)
	require.NotEqual(t, uuid.Nil, generation.EventUUID)

	generation.Status = string(commontypes.AIGatewayAsyncGenerationStatusCompleted)
	now := time.Now()
	generation.StartedAt = &now
	generation.FinishedAt = &now
	generation.EventPublishedAt = &now
	generation, err = store.Update(ctx, *generation)
	require.NoError(t, err)
	require.Equal(t, string(commontypes.AIGatewayAsyncGenerationStatusCompleted), generation.Status)
	require.NotNil(t, generation.EventPublishedAt)

	generation, err = store.FindByResourceID(ctx, database.AIGenerationResourceTypeVideo, "video_gateway_1")
	require.NoError(t, err)
	require.Equal(t, string(commontypes.AIGatewayAsyncGenerationStatusCompleted), generation.Status)
	require.NotNil(t, generation.StartedAt)
	require.NotNil(t, generation.FinishedAt)
	require.NotNil(t, generation.EventPublishedAt)
}

func TestAIGenerationMeteringStore_ListMeteringCandidates(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAIGenerationStoreWithDB(db)
	meteringStore := database.NewAIGenerationMeteringStoreWithDB(db)

	completed, err := store.Create(ctx, database.AIGeneration{
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_completed",
		ProviderResourceID: "vid_completed",
		OwnerUUID:          "user-1",
		ModelID:            "openai/gpt-video",
		Status:             string(commontypes.AIGatewayAsyncGenerationStatusCompleted),
		EventUUID:          uuid.New(),
	})
	require.NoError(t, err)

	staleQueued, err := store.Create(ctx, database.AIGeneration{
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_stale",
		ProviderResourceID: "vid_stale",
		OwnerUUID:          "user-1",
		ModelID:            "openai/gpt-video",
		Status:             string(commontypes.AIGatewayAsyncGenerationStatusQueued),
		EventUUID:          uuid.New(),
	})
	require.NoError(t, err)

	staleImage, err := store.Create(ctx, database.AIGeneration{
		ResourceType:       "image",
		ResourceID:         "image_stale",
		ProviderResourceID: "img_stale",
		OwnerUUID:          "user-1",
		ModelID:            "openai/gpt-image",
		Status:             string(commontypes.AIGatewayAsyncGenerationStatusQueued),
		EventUUID:          uuid.New(),
	})
	require.NoError(t, err)

	_, err = store.Create(ctx, database.AIGeneration{
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_fresh",
		ProviderResourceID: "vid_fresh",
		OwnerUUID:          "user-1",
		ModelID:            "openai/gpt-video",
		Status:             string(commontypes.AIGatewayAsyncGenerationStatusInProgress),
		EventUUID:          uuid.New(),
	})
	require.NoError(t, err)

	meteredAt := time.Now()
	_, err = store.Create(ctx, database.AIGeneration{
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_metered",
		ProviderResourceID: "vid_metered",
		OwnerUUID:          "user-1",
		ModelID:            "openai/gpt-video",
		Status:             string(commontypes.AIGatewayAsyncGenerationStatusCompleted),
		EventUUID:          uuid.New(),
		EventPublishedAt:   &meteredAt,
	})
	require.NoError(t, err)

	staleTime := time.Now().Add(-2 * time.Minute)
	_, err = db.Core.NewUpdate().
		Table("ai_generations").
		Set("updated_at = ?", staleTime).
		Where("id = ?", staleQueued.ID).
		Exec(ctx)
	require.NoError(t, err)
	_, err = db.Core.NewUpdate().
		Table("ai_generations").
		Set("updated_at = ?", staleTime).
		Where("id = ?", staleImage.ID).
		Exec(ctx)
	require.NoError(t, err)

	rows, err := meteringStore.ListMeteringCandidates(ctx, time.Now().Add(-30*time.Second), 100)
	require.NoError(t, err)

	got := map[string]bool{}
	for _, row := range rows {
		got[row.ResourceID] = true
	}
	require.True(t, got[completed.ResourceID])
	require.True(t, got[staleQueued.ResourceID])
	require.True(t, got[staleImage.ResourceID])
	require.False(t, got["video_fresh"])
	require.False(t, got["video_metered"])
}

func TestAIGenerationStore_UpdateWithStatus(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAIGenerationStoreWithDB(db)
	generation, err := store.Create(ctx, database.AIGeneration{
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_cas",
		ProviderResourceID: "vid_cas",
		OwnerUUID:          "user-1",
		ModelID:            "openai/gpt-video",
		Status:             string(commontypes.AIGatewayAsyncGenerationStatusInProgress),
		EventUUID:          uuid.New(),
	})
	require.NoError(t, err)

	generation.Status = string(commontypes.AIGatewayAsyncGenerationStatusCompleted)
	won, err := store.UpdateWithStatus(ctx, *generation, string(commontypes.AIGatewayAsyncGenerationStatusQueued))
	require.NoError(t, err)
	require.False(t, won)

	won, err = store.UpdateWithStatus(ctx, *generation, string(commontypes.AIGatewayAsyncGenerationStatusInProgress))
	require.NoError(t, err)
	require.True(t, won)

	updated, err := store.FindByResourceID(ctx, database.AIGenerationResourceTypeVideo, "video_cas")
	require.NoError(t, err)
	require.Equal(t, string(commontypes.AIGatewayAsyncGenerationStatusCompleted), updated.Status)
}

func TestAIGenerationStore_PublishMeteringEventInTx(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAIGenerationStoreWithDB(db)
	eventUUID := uuid.New()
	generation, err := store.Create(ctx, database.AIGeneration{
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_publish",
		ProviderResourceID: "vid_publish",
		OwnerUUID:          "user-1",
		ModelID:            "openai/gpt-video",
		Status:             string(commontypes.AIGatewayAsyncGenerationStatusCompleted),
		EventUUID:          eventUUID,
	})
	require.NoError(t, err)

	publishCount := 0
	err = store.PublishMeteringEventInTx(ctx, generation.ID, func(input database.AIGeneration) error {
		publishCount++
		require.Equal(t, eventUUID, input.EventUUID)
		require.Equal(t, string(commontypes.AIGatewayAsyncGenerationStatusCompleted), input.Status)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, publishCount)

	updated, err := store.FindByResourceID(ctx, database.AIGenerationResourceTypeVideo, "video_publish")
	require.NoError(t, err)
	require.NotNil(t, updated.EventPublishedAt)

	err = store.PublishMeteringEventInTx(ctx, generation.ID, func(input database.AIGeneration) error {
		publishCount++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, publishCount)
}

func TestAIGenerationStore_PublishMeteringEventInTxSkipsNonCompleted(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAIGenerationStoreWithDB(db)
	generation, err := store.Create(ctx, database.AIGeneration{
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_publish_in_progress",
		ProviderResourceID: "vid_publish_in_progress",
		OwnerUUID:          "user-1",
		ModelID:            "openai/gpt-video",
		Status:             string(commontypes.AIGatewayAsyncGenerationStatusInProgress),
		EventUUID:          uuid.New(),
	})
	require.NoError(t, err)

	publishCount := 0
	err = store.PublishMeteringEventInTx(ctx, generation.ID, func(input database.AIGeneration) error {
		publishCount++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 0, publishCount)
}

func TestAIGenerationStore_PublishMeteringEventInTxRequiresEventUUID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAIGenerationStoreWithDB(db)
	generation, err := store.Create(ctx, database.AIGeneration{
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_publish_no_uuid",
		ProviderResourceID: "vid_publish_no_uuid",
		OwnerUUID:          "user-1",
		ModelID:            "openai/gpt-video",
		Status:             string(commontypes.AIGatewayAsyncGenerationStatusCompleted),
	})
	require.NoError(t, err)

	publishCount := 0
	err = store.PublishMeteringEventInTx(ctx, generation.ID, func(input database.AIGeneration) error {
		publishCount++
		return nil
	})
	require.Error(t, err)
	require.Equal(t, 0, publishCount)
}

func TestAIGenerationStore_PublishMeteringEventInTxDoesNotMarkOnPublishFailure(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAIGenerationStoreWithDB(db)
	generation, err := store.Create(ctx, database.AIGeneration{
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_publish_failure",
		ProviderResourceID: "vid_publish_failure",
		OwnerUUID:          "user-1",
		ModelID:            "openai/gpt-video",
		Status:             string(commontypes.AIGatewayAsyncGenerationStatusCompleted),
		EventUUID:          uuid.New(),
	})
	require.NoError(t, err)

	expectedErr := errors.New("publish failed")
	err = store.PublishMeteringEventInTx(ctx, generation.ID, func(input database.AIGeneration) error {
		return expectedErr
	})
	require.ErrorIs(t, err, expectedErr)

	updated, err := store.FindByResourceID(ctx, database.AIGenerationResourceTypeVideo, "video_publish_failure")
	require.NoError(t, err)
	require.Nil(t, updated.EventPublishedAt)
}

func TestAIGenerationStore_UpdateProviderMetadataOnly(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAIGenerationStoreWithDB(db)
	publishedAt := time.Now()
	generation, err := store.Create(ctx, database.AIGeneration{
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_metadata_only",
		ProviderResourceID: "vid_metadata_only",
		ProviderMetadata:   map[string]any{"old": "value"},
		OwnerUUID:          "user-1",
		ModelID:            "openai/gpt-video",
		Status:             string(commontypes.AIGatewayAsyncGenerationStatusCompleted),
		EventUUID:          uuid.New(),
		EventPublishedAt:   &publishedAt,
	})
	require.NoError(t, err)

	err = store.UpdateProviderMetadata(ctx, generation.ID, map[string]any{"file_id": "file_123"})
	require.NoError(t, err)

	updated, err := store.FindByResourceID(ctx, database.AIGenerationResourceTypeVideo, "video_metadata_only")
	require.NoError(t, err)
	require.Equal(t, string(commontypes.AIGatewayAsyncGenerationStatusCompleted), updated.Status)
	require.NotNil(t, updated.EventPublishedAt)
	require.Equal(t, "file_123", updated.ProviderMetadata["file_id"])
}

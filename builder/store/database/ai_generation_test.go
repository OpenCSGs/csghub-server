package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
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
		Status:             "queued",
	})
	require.NoError(t, err)
	require.NotZero(t, generation.ID)

	_, err = store.Create(ctx, database.AIGeneration{
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         "video_gateway_2",
		ProviderResourceID: "vid_123",
		OwnerUUID:          "user-2",
		ModelID:            "other/gpt-video",
		Status:             "queued",
	})
	require.NoError(t, err)

	generation, err = store.FindByResourceID(ctx, database.AIGenerationResourceTypeVideo, "video_gateway_1")
	require.NoError(t, err)
	require.Equal(t, "user-1", generation.OwnerUUID)
	require.Equal(t, "vid_123", generation.ProviderResourceID)
	require.Equal(t, "file_123", generation.ProviderMetadata["file_id"])

	generation.Status = "completed"
	generation, err = store.Update(ctx, *generation)
	require.NoError(t, err)
	require.Equal(t, "completed", generation.Status)

	generation, err = store.FindByResourceID(ctx, database.AIGenerationResourceTypeVideo, "video_gateway_1")
	require.NoError(t, err)
	require.Equal(t, "completed", generation.Status)
}

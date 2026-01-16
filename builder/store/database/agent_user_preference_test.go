package database_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAgentUserPreferenceStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentUserPreferenceStoreWithDB(db)

	// Test Create
	userUUID := uuid.New().String()
	preference := &database.AgentUserPreference{
		UserUUID:   userUUID,
		EntityType: types.AgentUserPreferenceEntityTypeAgentInstance,
		EntityID:   "123",
		Action:     types.AgentUserPreferenceActionPin,
	}

	err := store.Create(ctx, preference)
	require.NoError(t, err)
	require.NotZero(t, preference.ID)
	require.NotZero(t, preference.CreatedAt)

	// Test FindByUserAndEntity
	foundPreference, err := store.FindByUserAndEntity(ctx, userUUID, types.AgentUserPreferenceEntityTypeAgentInstance, "123", types.AgentUserPreferenceActionPin)
	require.NoError(t, err)
	require.Equal(t, preference.ID, foundPreference.ID)
	require.Equal(t, userUUID, foundPreference.UserUUID)
	require.Equal(t, types.AgentUserPreferenceEntityTypeAgentInstance, foundPreference.EntityType)
	require.Equal(t, "123", foundPreference.EntityID)
	require.Equal(t, types.AgentUserPreferenceActionPin, foundPreference.Action)

	// Test CountByUserAndType
	count, err := store.CountByUserAndType(ctx, userUUID, types.AgentUserPreferenceEntityTypeAgentInstance, types.AgentUserPreferenceActionPin)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Test Delete
	err = store.Delete(ctx, userUUID, types.AgentUserPreferenceEntityTypeAgentInstance, "123", types.AgentUserPreferenceActionPin)
	require.NoError(t, err)

	// Verify deletion
	_, err = store.FindByUserAndEntity(ctx, userUUID, types.AgentUserPreferenceEntityTypeAgentInstance, "123", types.AgentUserPreferenceActionPin)
	require.Error(t, err)
	require.True(t, errors.Is(err, errorx.ErrNotFound))
}

func TestAgentUserPreferenceStore_Create_Idempotent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentUserPreferenceStoreWithDB(db)

	userUUID := uuid.New().String()
	preference := &database.AgentUserPreference{
		UserUUID:   userUUID,
		EntityType: types.AgentUserPreferenceEntityTypeAgentInstance,
		EntityID:   "123",
		Action:     types.AgentUserPreferenceActionPin,
	}

	// Create first preference
	err := store.Create(ctx, preference)
	require.NoError(t, err)
	firstID := preference.ID

	// Create duplicate preference (should not error due to ON CONFLICT DO NOTHING)
	preference2 := &database.AgentUserPreference{
		UserUUID:   userUUID,
		EntityType: types.AgentUserPreferenceEntityTypeAgentInstance,
		EntityID:   "123",
		Action:     types.AgentUserPreferenceActionPin,
	}
	err = store.Create(ctx, preference2)
	require.NoError(t, err)

	// Verify only one preference exists
	count, err := store.CountByUserAndType(ctx, userUUID, types.AgentUserPreferenceEntityTypeAgentInstance, types.AgentUserPreferenceActionPin)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Verify the original preference still exists
	foundPreference, err := store.FindByUserAndEntity(ctx, userUUID, types.AgentUserPreferenceEntityTypeAgentInstance, "123", types.AgentUserPreferenceActionPin)
	require.NoError(t, err)
	require.Equal(t, firstID, foundPreference.ID)
}

func TestAgentUserPreferenceStore_Delete_Idempotent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentUserPreferenceStoreWithDB(db)

	userUUID := uuid.New().String()

	// Delete non-existent preference (should not error)
	err := store.Delete(ctx, userUUID, types.AgentUserPreferenceEntityTypeAgentInstance, "999", types.AgentUserPreferenceActionPin)
	require.NoError(t, err)
}

func TestAgentUserPreferenceStore_NormalizeEntityID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentUserPreferenceStoreWithDB(db)

	userUUID := uuid.New().String()

	// Test with leading zeros (should be normalized)
	preference1 := &database.AgentUserPreference{
		UserUUID:   userUUID,
		EntityType: types.AgentUserPreferenceEntityTypeAgentInstance,
		EntityID:   "042",
		Action:     types.AgentUserPreferenceActionPin,
	}
	err := store.Create(ctx, preference1)
	require.NoError(t, err)

	// Find using normalized ID (without leading zeros)
	foundPreference, err := store.FindByUserAndEntity(ctx, userUUID, types.AgentUserPreferenceEntityTypeAgentInstance, "42", types.AgentUserPreferenceActionPin)
	require.NoError(t, err)
	require.Equal(t, "42", foundPreference.EntityID)

	// Find using original ID with leading zeros (should also work after normalization)
	foundPreference2, err := store.FindByUserAndEntity(ctx, userUUID, types.AgentUserPreferenceEntityTypeAgentInstance, "042", types.AgentUserPreferenceActionPin)
	require.NoError(t, err)
	require.Equal(t, "42", foundPreference2.EntityID)
}

func TestAgentUserPreferenceStore_StringEntityID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentUserPreferenceStoreWithDB(db)

	userUUID := uuid.New().String()

	// Test with string ID (for MCP servers)
	preference := &database.AgentUserPreference{
		UserUUID:   userUUID,
		EntityType: types.AgentUserPreferenceEntityTypeAgentMCPServer,
		EntityID:   "builtin:1",
		Action:     types.AgentUserPreferenceActionPin,
	}
	err := store.Create(ctx, preference)
	require.NoError(t, err)

	// Find using string ID
	foundPreference, err := store.FindByUserAndEntity(ctx, userUUID, types.AgentUserPreferenceEntityTypeAgentMCPServer, "builtin:1", types.AgentUserPreferenceActionPin)
	require.NoError(t, err)
	require.Equal(t, "builtin:1", foundPreference.EntityID)
}

func TestAgentUserPreferenceStore_CountByUserAndType(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentUserPreferenceStoreWithDB(db)

	userUUID1 := uuid.New().String()
	userUUID2 := uuid.New().String()

	// Create preferences for user1
	for i := 1; i <= 3; i++ {
		preference := &database.AgentUserPreference{
			UserUUID:   userUUID1,
			EntityType: types.AgentUserPreferenceEntityTypeAgentInstance,
			EntityID:   strconv.Itoa(i),
			Action:     types.AgentUserPreferenceActionPin,
		}
		err := store.Create(ctx, preference)
		require.NoError(t, err)
	}

	// Create preferences for user2
	for i := 1; i <= 2; i++ {
		preference := &database.AgentUserPreference{
			UserUUID:   userUUID2,
			EntityType: types.AgentUserPreferenceEntityTypeAgentInstance,
			EntityID:   strconv.Itoa(i),
			Action:     types.AgentUserPreferenceActionPin,
		}
		err := store.Create(ctx, preference)
		require.NoError(t, err)
	}

	// Create knowledge base preferences for user1
	for i := 1; i <= 2; i++ {
		preference := &database.AgentUserPreference{
			UserUUID:   userUUID1,
			EntityType: types.AgentUserPreferenceEntityTypeAgentKnowledgeBase,
			EntityID:   strconv.Itoa(i),
			Action:     types.AgentUserPreferenceActionPin,
		}
		err := store.Create(ctx, preference)
		require.NoError(t, err)
	}

	// Test counts
	count1, err := store.CountByUserAndType(ctx, userUUID1, types.AgentUserPreferenceEntityTypeAgentInstance, types.AgentUserPreferenceActionPin)
	require.NoError(t, err)
	require.Equal(t, 3, count1)

	count2, err := store.CountByUserAndType(ctx, userUUID2, types.AgentUserPreferenceEntityTypeAgentInstance, types.AgentUserPreferenceActionPin)
	require.NoError(t, err)
	require.Equal(t, 2, count2)

	count3, err := store.CountByUserAndType(ctx, userUUID1, types.AgentUserPreferenceEntityTypeAgentKnowledgeBase, types.AgentUserPreferenceActionPin)
	require.NoError(t, err)
	require.Equal(t, 2, count3)
}

func TestAgentUserPreferenceStore_DeleteByEntity(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentUserPreferenceStoreWithDB(db)

	userUUID1 := uuid.New().String()
	userUUID2 := uuid.New().String()
	entityID := "123"

	// Create preferences for same entity but different users
	preference1 := &database.AgentUserPreference{
		UserUUID:   userUUID1,
		EntityType: types.AgentUserPreferenceEntityTypeAgentInstance,
		EntityID:   entityID,
		Action:     types.AgentUserPreferenceActionPin,
	}
	err := store.Create(ctx, preference1)
	require.NoError(t, err)

	preference2 := &database.AgentUserPreference{
		UserUUID:   userUUID2,
		EntityType: types.AgentUserPreferenceEntityTypeAgentInstance,
		EntityID:   entityID,
		Action:     types.AgentUserPreferenceActionPin,
	}
	err = store.Create(ctx, preference2)
	require.NoError(t, err)

	// Delete all preferences for this entity
	err = store.DeleteByEntity(ctx, types.AgentUserPreferenceEntityTypeAgentInstance, entityID)
	require.NoError(t, err)

	// Verify both preferences are deleted
	_, err = store.FindByUserAndEntity(ctx, userUUID1, types.AgentUserPreferenceEntityTypeAgentInstance, entityID, types.AgentUserPreferenceActionPin)
	require.Error(t, err)
	require.True(t, errors.Is(err, errorx.ErrNotFound))

	_, err = store.FindByUserAndEntity(ctx, userUUID2, types.AgentUserPreferenceEntityTypeAgentInstance, entityID, types.AgentUserPreferenceActionPin)
	require.Error(t, err)
	require.True(t, errors.Is(err, errorx.ErrNotFound))
}

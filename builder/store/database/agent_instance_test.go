package database_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestAgentInstanceStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	// Test Create
	userUUID := uuid.New().String()
	instance := &database.AgentInstance{
		TemplateID: 123,
		UserUUID:   userUUID,
		Type:       "langflow",
		ContentID:  "instance-123",
		Public:     false,
	}

	createdInstance, err := store.Create(ctx, instance)
	require.NoError(t, err)
	require.NotZero(t, createdInstance.ID)
	// Update the original instance with the created one for further tests
	*instance = *createdInstance

	// Test FindByID
	foundInstance, err := store.FindByID(ctx, instance.ID)
	require.NoError(t, err)
	require.Equal(t, instance.ID, foundInstance.ID)
	require.Equal(t, instance.TemplateID, foundInstance.TemplateID)
	require.Equal(t, instance.UserUUID, foundInstance.UserUUID)
	require.Equal(t, instance.Type, foundInstance.Type)
	require.Equal(t, instance.ContentID, foundInstance.ContentID)
	require.Equal(t, instance.Public, foundInstance.Public)

	// Test ListByUserUUID
	instances, err := store.ListByUserUUID(ctx, userUUID)
	require.NoError(t, err)
	require.Len(t, instances, 1)
	require.Equal(t, instance.ID, instances[0].ID)

	// Test ListByTemplateID
	instances, err = store.ListByTemplateID(ctx, instance.TemplateID, userUUID)
	require.NoError(t, err)
	require.Len(t, instances, 1)
	require.Equal(t, instance.ID, instances[0].ID)

	// Test Update
	instance.ContentID = "updated-instance-123"
	instance.Public = true
	err = store.Update(ctx, instance)
	require.NoError(t, err)

	// Verify update
	updatedInstance, err := store.FindByID(ctx, instance.ID)
	require.NoError(t, err)
	require.Equal(t, "updated-instance-123", updatedInstance.ContentID)
	require.True(t, updatedInstance.Public)

	// Test Delete
	err = store.Delete(ctx, instance.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = store.FindByID(ctx, instance.ID)
	require.Error(t, err)
}

func TestAgentInstanceStore_ListByUserUUID_WithPublicInstances(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	userUUID1 := uuid.New().String()
	userUUID2 := uuid.New().String()

	// Create private instance for user1
	privateInstance := &database.AgentInstance{
		TemplateID: 1,
		UserUUID:   userUUID1,
		Type:       "langflow",
		ContentID:  "private-instance",
		Public:     false,
	}
	_, err := store.Create(ctx, privateInstance)
	require.NoError(t, err)

	// Create public instance for user1
	publicInstance := &database.AgentInstance{
		TemplateID: 2,
		UserUUID:   userUUID1,
		Type:       "agno",
		ContentID:  "public-instance",
		Public:     true,
	}
	_, err = store.Create(ctx, publicInstance)
	require.NoError(t, err)

	// Create private instance for user2
	user2Instance := &database.AgentInstance{
		TemplateID: 3,
		UserUUID:   userUUID2,
		Type:       "code",
		ContentID:  "user2-instance",
		Public:     false,
	}
	_, err = store.Create(ctx, user2Instance)
	require.NoError(t, err)

	// Test ListByUserUUID for user1 - should return both private and public instances
	instances, err := store.ListByUserUUID(ctx, userUUID1)
	require.NoError(t, err)
	require.Len(t, instances, 2)

	// Test ListByUserUUID for user2 - should return only public instance from user1 and private instance from user2
	instances, err = store.ListByUserUUID(ctx, userUUID2)
	require.NoError(t, err)
	require.Len(t, instances, 2) // public instance from user1 + private instance from user2
}

func TestAgentInstanceStore_ListByTemplateID_WithPublicInstances(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	userUUID1 := uuid.New().String()
	userUUID2 := uuid.New().String()
	templateID := int64(100)

	// Create private instance for user1 from template
	privateInstance := &database.AgentInstance{
		TemplateID: templateID,
		UserUUID:   userUUID1,
		Type:       "langflow",
		ContentID:  "private-instance",
		Public:     false,
	}
	_, err := store.Create(ctx, privateInstance)
	require.NoError(t, err)

	// Create public instance for user1 from template
	publicInstance := &database.AgentInstance{
		TemplateID: templateID,
		UserUUID:   userUUID1,
		Type:       "agno",
		ContentID:  "public-instance",
		Public:     true,
	}
	_, err = store.Create(ctx, publicInstance)
	require.NoError(t, err)

	// Create private instance for user2 from different template
	user2Instance := &database.AgentInstance{
		TemplateID: templateID + 1,
		UserUUID:   userUUID2,
		Type:       "code",
		ContentID:  "user2-instance",
		Public:     false,
	}
	_, err = store.Create(ctx, user2Instance)
	require.NoError(t, err)

	// Test ListByTemplateID for user1 - should return both private and public instances from template
	instances, err := store.ListByTemplateID(ctx, templateID, userUUID1)
	require.NoError(t, err)
	require.Len(t, instances, 2)

	// Test ListByTemplateID for user2 - should return only public instance from user1
	instances, err = store.ListByTemplateID(ctx, templateID, userUUID2)
	require.NoError(t, err)
	require.Len(t, instances, 1)
	require.Equal(t, publicInstance.ID, instances[0].ID)
}

func TestAgentInstanceStore_NotFound(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	// Test FindByID with non-existent ID
	_, err := store.FindByID(ctx, 99999)
	require.Error(t, err)

	// Test ListByUserUUID with non-existent user
	instances, err := store.ListByUserUUID(ctx, "non-existent-user")
	require.NoError(t, err)
	require.Len(t, instances, 0)

	// Test ListByTemplateID with non-existent template
	instances, err = store.ListByTemplateID(ctx, 99999, "non-existent-user")
	require.NoError(t, err)
	require.Len(t, instances, 0)
}

func TestAgentInstanceStore_Update_NonExistent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	// Test Update with non-existent instance
	nonExistentInstance := &database.AgentInstance{
		ID:         99999,
		TemplateID: 123,
		UserUUID:   uuid.New().String(),
		Type:       "langflow",
		ContentID:  "test-content",
		Public:     false,
	}

	err := store.Update(ctx, nonExistentInstance)
	require.Error(t, err)
}

func TestAgentInstanceStore_Delete_NonExistent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	// Test Delete with non-existent ID
	err := store.Delete(ctx, 99999)
	require.Error(t, err)
}

func TestAgentInstanceStore_MultipleInstancesFromSameTemplate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	userUUID := uuid.New().String()
	templateID := int64(200)

	// Create multiple instances from the same template
	instance1 := &database.AgentInstance{
		TemplateID: templateID,
		UserUUID:   userUUID,
		Type:       "langflow",
		ContentID:  "instance-1",
		Public:     false,
	}
	_, err := store.Create(ctx, instance1)
	require.NoError(t, err)

	instance2 := &database.AgentInstance{
		TemplateID: templateID,
		UserUUID:   userUUID,
		Type:       "langflow",
		ContentID:  "instance-2",
		Public:     true,
	}
	_, err = store.Create(ctx, instance2)
	require.NoError(t, err)

	// Test ListByTemplateID - should return both instances
	instances, err := store.ListByTemplateID(ctx, templateID, userUUID)
	require.NoError(t, err)
	require.Len(t, instances, 2)

	// Test ListByUserUUID - should return both instances
	instances, err = store.ListByUserUUID(ctx, userUUID)
	require.NoError(t, err)
	require.Len(t, instances, 2)
}

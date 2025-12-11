package database_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
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
	instances, total, err := store.ListByUserUUID(ctx, userUUID, types.AgentInstanceFilter{}, 10, 1)
	require.NoError(t, err)
	require.Len(t, instances, 5) // 1 user instance + 4 system instances (all public)
	require.Equal(t, 5, total)
	// Find our instance in the results
	found := false
	for _, inst := range instances {
		if inst.ID == instance.ID {
			found = true
			break
		}
	}
	require.True(t, found, "User instance should be found in results")

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
	instances, total, err := store.ListByUserUUID(ctx, userUUID1, types.AgentInstanceFilter{}, 10, 1)
	require.NoError(t, err)
	require.Len(t, instances, 6) // 2 user instances + 4 system instances (all public)
	require.Equal(t, 6, total)

	// Test ListByUserUUID for user2 - should return only public instance from user1 and private instance from user2
	instances, total, err = store.ListByUserUUID(ctx, userUUID2, types.AgentInstanceFilter{}, 10, 1)
	require.NoError(t, err)
	require.Len(t, instances, 6) // public instance from user1 + private instance from user2 + 4 system instances
	require.Equal(t, 6, total)
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
	instances, total, err := store.ListByUserUUID(ctx, "non-existent-user", types.AgentInstanceFilter{}, 10, 1)
	require.NoError(t, err)
	require.Len(t, instances, 4) // Only system instances (all public)
	require.Equal(t, 4, total)

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

	// Test ListByUserUUID - should return both instances
	instances, total, err := store.ListByUserUUID(ctx, userUUID, types.AgentInstanceFilter{}, 10, 1)
	require.NoError(t, err)
	require.Len(t, instances, 6) // 2 user instances + 4 system instances (all public)
	require.Equal(t, 6, total)
}

func TestAgentInstanceStore_ListByUserUUID_WithFilters(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	userUUID := uuid.New().String()

	// Create instances with different types and names
	instance1 := &database.AgentInstance{
		TemplateID:  1,
		UserUUID:    userUUID,
		Type:        "langflow",
		ContentID:   "langflow-instance",
		Name:        "Langflow Agent",
		Description: "A langflow agent for automation",
		Public:      false,
	}
	_, err := store.Create(ctx, instance1)
	require.NoError(t, err)

	instance2 := &database.AgentInstance{
		TemplateID:  2,
		UserUUID:    userUUID,
		Type:        "agno",
		ContentID:   "agno-instance",
		Name:        "Agno Assistant",
		Description: "An agno assistant for help",
		Public:      false,
	}
	_, err = store.Create(ctx, instance2)
	require.NoError(t, err)

	instance3 := &database.AgentInstance{
		TemplateID:  3,
		UserUUID:    userUUID,
		Type:        "langflow",
		ContentID:   "another-langflow",
		Name:        "Another Langflow",
		Description: "Another langflow instance",
		Public:      false,
	}
	_, err = store.Create(ctx, instance3)
	require.NoError(t, err)

	// Test search filter
	instances, total, err := store.ListByUserUUID(ctx, userUUID, types.AgentInstanceFilter{Search: "langflow"}, 10, 1)
	require.NoError(t, err)
	require.Len(t, instances, 2) // Should find both langflow instances (system instances don't match "langflow")
	require.Equal(t, 2, total)

	// Test type filter
	instances, total, err = store.ListByUserUUID(ctx, userUUID, types.AgentInstanceFilter{Type: "agno"}, 10, 1)
	require.NoError(t, err)
	require.Len(t, instances, 1) // Should find only agno instance (system instances are type "code")
	require.Equal(t, 1, total)

	// Test combined filters
	instances, total, err = store.ListByUserUUID(ctx, userUUID, types.AgentInstanceFilter{Search: "another", Type: "langflow"}, 10, 1)
	require.NoError(t, err)
	require.Len(t, instances, 1) // Should find only the "Another Langflow" instance
	require.Equal(t, 1, total)

	// Test pagination
	instances, total, err = store.ListByUserUUID(ctx, userUUID, types.AgentInstanceFilter{}, 2, 1)
	require.NoError(t, err)
	require.Len(t, instances, 2) // Should return only 2 instances due to limit
	require.Equal(t, 7, total)   // But total should be 7 (3 user + 4 system)

	// Test second page
	instances, total, err = store.ListByUserUUID(ctx, userUUID, types.AgentInstanceFilter{}, 2, 2)
	require.NoError(t, err)
	require.Len(t, instances, 2) // Should return 2 instances on second page
	require.Equal(t, 7, total)   // Total should still be 7 (3 user + 4 system)
}

func TestAgentInstanceStore_ListByUserUUID_WithTemplateFilter(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	userUUID := uuid.New().String()
	templateID1 := int64(1)
	templateID2 := int64(2)

	// Create instances with different template IDs
	instance1 := &database.AgentInstance{
		TemplateID:  templateID1,
		UserUUID:    userUUID,
		Type:        "langflow",
		ContentID:   "langflow-instance-1",
		Name:        "Langflow Agent 1",
		Description: "A langflow agent for automation",
		Public:      false,
	}
	_, err := store.Create(ctx, instance1)
	require.NoError(t, err)

	instance2 := &database.AgentInstance{
		TemplateID:  templateID2,
		UserUUID:    userUUID,
		Type:        "agno",
		ContentID:   "agno-instance-1",
		Name:        "Agno Assistant 1",
		Description: "An agno assistant for help",
		Public:      false,
	}
	_, err = store.Create(ctx, instance2)
	require.NoError(t, err)

	instance3 := &database.AgentInstance{
		TemplateID:  templateID1,
		UserUUID:    userUUID,
		Type:        "langflow",
		ContentID:   "langflow-instance-2",
		Name:        "Langflow Agent 2",
		Description: "Another langflow agent",
		Public:      false,
	}
	_, err = store.Create(ctx, instance3)
	require.NoError(t, err)

	// Test template ID filter
	instances, total, err := store.ListByUserUUID(ctx, userUUID, types.AgentInstanceFilter{TemplateID: &templateID1}, 10, 1)
	require.NoError(t, err)
	require.Len(t, instances, 2) // Should find both instances from template 1
	require.Equal(t, 2, total)

	// Test template ID filter with different template
	instances, total, err = store.ListByUserUUID(ctx, userUUID, types.AgentInstanceFilter{TemplateID: &templateID2}, 10, 1)
	require.NoError(t, err)
	require.Len(t, instances, 1) // Should find only instance from template 2
	require.Equal(t, 1, total)

	// Test combined filters (template + type)
	instances, total, err = store.ListByUserUUID(ctx, userUUID, types.AgentInstanceFilter{TemplateID: &templateID1, Type: "langflow"}, 10, 1)
	require.NoError(t, err)
	require.Len(t, instances, 2) // Should find both langflow instances from template 1
	require.Equal(t, 2, total)

	// Test combined filters (template + search)
	instances, total, err = store.ListByUserUUID(ctx, userUUID, types.AgentInstanceFilter{TemplateID: &templateID1, Search: "Agent 1"}, 10, 1)
	require.NoError(t, err)
	require.Len(t, instances, 1) // Should find only "Langflow Agent 1"
	require.Equal(t, 1, total)
}

func TestAgentInstanceStore_ListByUserUUID_WithPublicFilter(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	userUUID := uuid.New().String()

	// Create public instance
	publicInstance := &database.AgentInstance{
		TemplateID:  1,
		UserUUID:    userUUID,
		Type:        "langflow",
		ContentID:   "public-instance",
		Name:        "Public Instance",
		Description: "A public instance",
		Public:      true,
	}
	_, err := store.Create(ctx, publicInstance)
	require.NoError(t, err)

	// Create private instance
	privateInstance := &database.AgentInstance{
		TemplateID:  2,
		UserUUID:    userUUID,
		Type:        "agno",
		ContentID:   "private-instance",
		Name:        "Private Instance",
		Description: "A private instance",
		Public:      false,
	}
	_, err = store.Create(ctx, privateInstance)
	require.NoError(t, err)

	// Test public filter - true
	publicTrue := true
	instances, total, err := store.ListByUserUUID(ctx, userUUID, types.AgentInstanceFilter{Public: &publicTrue}, 10, 1)
	require.NoError(t, err)
	// Should find public instance + system instances (all public)
	require.GreaterOrEqual(t, len(instances), 1, "Should find at least the public instance")
	require.GreaterOrEqual(t, total, 1)
	// Verify the public instance is in results
	found := false
	for _, inst := range instances {
		if inst.ID == publicInstance.ID {
			found = true
			require.True(t, inst.Public, "Instance should be public")
			break
		}
	}
	require.True(t, found, "Public instance should be found in results")

	// Test public filter - false
	publicFalse := false
	instances, total, err = store.ListByUserUUID(ctx, userUUID, types.AgentInstanceFilter{Public: &publicFalse}, 10, 1)
	require.NoError(t, err)
	// Should find only private instance (user's own private instance)
	require.GreaterOrEqual(t, len(instances), 1, "Should find at least the private instance")
	require.GreaterOrEqual(t, total, 1)
	// Verify the private instance is in results
	found = false
	for _, inst := range instances {
		if inst.ID == privateInstance.ID {
			found = true
			require.False(t, inst.Public, "Instance should be private")
			break
		}
	}
	require.True(t, found, "Private instance should be found in results")

	// Test combined filters (public + type)
	publicTrue = true
	instances, total, err = store.ListByUserUUID(ctx, userUUID, types.AgentInstanceFilter{Public: &publicTrue, Type: "langflow"}, 10, 1)
	require.NoError(t, err)
	// Should find public langflow instance
	require.GreaterOrEqual(t, len(instances), 1)
	require.GreaterOrEqual(t, total, 1)
	found = false
	for _, inst := range instances {
		if inst.ID == publicInstance.ID {
			found = true
			require.True(t, inst.Public)
			require.Equal(t, "langflow", inst.Type)
			break
		}
	}
	require.True(t, found, "Public langflow instance should be found in results")
}

func TestAgentInstanceStore_IsInstanceExistsByContentID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	userUUID := uuid.New().String()
	instanceType := "langflow"
	contentID := "test-content-123"

	// Test case 1: Instance does not exist
	exists, err := store.IsInstanceExistsByContentID(ctx, instanceType, contentID)
	require.NoError(t, err)
	require.False(t, exists)

	// Test case 2: Create an instance and verify it exists
	instance := &database.AgentInstance{
		TemplateID: 123,
		UserUUID:   userUUID,
		Type:       instanceType,
		ContentID:  contentID,
		Public:     false,
	}

	createdInstance, err := store.Create(ctx, instance)
	require.NoError(t, err)
	require.NotZero(t, createdInstance.ID)

	// Verify the instance exists
	exists, err = store.IsInstanceExistsByContentID(ctx, instanceType, contentID)
	require.NoError(t, err)
	require.True(t, exists)

	// Test case 3: Different type, same content_id
	exists, err = store.IsInstanceExistsByContentID(ctx, "agno", contentID)
	require.NoError(t, err)
	require.False(t, exists)

	// Test case 4: Same type, different content_id
	exists, err = store.IsInstanceExistsByContentID(ctx, instanceType, "different-content-id")
	require.NoError(t, err)
	require.False(t, exists)

	// Test case 5: Both type and content_id different
	exists, err = store.IsInstanceExistsByContentID(ctx, "agno", "different-content-id")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestAgentInstanceStore_IsInstanceExistsByContentID_MultipleInstances(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	userUUID := uuid.New().String()

	// Create multiple instances with different types and content IDs
	instances := []*database.AgentInstance{
		{
			TemplateID: 1,
			UserUUID:   userUUID,
			Type:       "langflow",
			ContentID:  "content-1",
			Public:     false,
		},
		{
			TemplateID: 2,
			UserUUID:   userUUID,
			Type:       "agno",
			ContentID:  "content-2",
			Public:     false,
		},
		{
			TemplateID: 3,
			UserUUID:   userUUID,
			Type:       "langflow",
			ContentID:  "content-3",
			Public:     false,
		},
	}

	// Create all instances
	for _, instance := range instances {
		_, err := store.Create(ctx, instance)
		require.NoError(t, err)
	}

	// Test each instance exists
	for _, instance := range instances {
		exists, err := store.IsInstanceExistsByContentID(ctx, instance.Type, instance.ContentID)
		require.NoError(t, err)
		require.True(t, exists, "Instance with type %s and content_id %s should exist", instance.Type, instance.ContentID)
	}

	// Test non-existent combinations
	testCases := []struct {
		instanceType string
		contentID    string
		description  string
	}{
		{"langflow", "non-existent", "langflow type with non-existent content_id"},
		{"agno", "content-1", "agno type with langflow content_id"},
		{"code", "content-2", "code type with agno content_id"},
		{"unknown", "content-3", "unknown type with langflow content_id"},
	}

	for _, tc := range testCases {
		exists, err := store.IsInstanceExistsByContentID(ctx, tc.instanceType, tc.contentID)
		require.NoError(t, err)
		require.False(t, exists, "Should not exist: %s", tc.description)
	}
}

func TestAgentInstanceStore_IsInstanceExistsByContentID_EmptyParameters(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	// Test with empty type
	exists, err := store.IsInstanceExistsByContentID(ctx, "", "some-content-id")
	require.NoError(t, err)
	require.False(t, exists)

	// Test with empty content_id
	exists, err = store.IsInstanceExistsByContentID(ctx, "langflow", "")
	require.NoError(t, err)
	require.False(t, exists)

	// Test with both empty
	exists, err = store.IsInstanceExistsByContentID(ctx, "", "")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestAgentInstanceStore_CountByUserAndType(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	userUUID1 := uuid.New().String()
	userUUID2 := uuid.New().String()
	instanceType1 := "langflow"
	instanceType2 := "agno"

	// Test case 1: Count with no instances (should return 0)
	count, err := store.CountByUserAndType(ctx, userUUID1, instanceType1)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// Test case 2: Create instances for user1 with type1
	instance1 := &database.AgentInstance{
		TemplateID: 1,
		UserUUID:   userUUID1,
		Type:       instanceType1,
		ContentID:  "content-1",
		Public:     false,
	}
	_, err = store.Create(ctx, instance1)
	require.NoError(t, err)

	instance2 := &database.AgentInstance{
		TemplateID: 2,
		UserUUID:   userUUID1,
		Type:       instanceType1,
		ContentID:  "content-2",
		Public:     false,
	}
	_, err = store.Create(ctx, instance2)
	require.NoError(t, err)

	instance3 := &database.AgentInstance{
		TemplateID: 3,
		UserUUID:   userUUID1,
		Type:       instanceType1,
		ContentID:  "content-3",
		Public:     true,
	}
	_, err = store.Create(ctx, instance3)
	require.NoError(t, err)

	// Count should return 3 for user1 with type1
	count, err = store.CountByUserAndType(ctx, userUUID1, instanceType1)
	require.NoError(t, err)
	require.Equal(t, 3, count)

	// Test case 3: Create instances for user1 with type2
	instance4 := &database.AgentInstance{
		TemplateID: 4,
		UserUUID:   userUUID1,
		Type:       instanceType2,
		ContentID:  "content-4",
		Public:     false,
	}
	_, err = store.Create(ctx, instance4)
	require.NoError(t, err)

	// Count for user1 with type1 should still be 3
	count, err = store.CountByUserAndType(ctx, userUUID1, instanceType1)
	require.NoError(t, err)
	require.Equal(t, 3, count)

	// Count for user1 with type2 should be 1
	count, err = store.CountByUserAndType(ctx, userUUID1, instanceType2)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Test case 4: Create instances for user2 with type1
	instance5 := &database.AgentInstance{
		TemplateID: 5,
		UserUUID:   userUUID2,
		Type:       instanceType1,
		ContentID:  "content-5",
		Public:     false,
	}
	_, err = store.Create(ctx, instance5)
	require.NoError(t, err)

	// Count for user1 with type1 should still be 3 (not affected by user2's instances)
	count, err = store.CountByUserAndType(ctx, userUUID1, instanceType1)
	require.NoError(t, err)
	require.Equal(t, 3, count)

	// Count for user2 with type1 should be 1
	count, err = store.CountByUserAndType(ctx, userUUID2, instanceType1)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Test case 5: Count with non-existent user (should return 0)
	nonExistentUser := uuid.New().String()
	count, err = store.CountByUserAndType(ctx, nonExistentUser, instanceType1)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// Test case 6: Count with non-existent type (should return 0)
	count, err = store.CountByUserAndType(ctx, userUUID1, "non-existent-type")
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestAgentInstanceStore_Delete_WithCascadingSoftDelete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	instanceStore := database.NewAgentInstanceStoreWithDB(db)
	sessionStore := database.NewAgentInstanceSessionStoreWithDB(db)
	historyStore := database.NewAgentInstanceSessionHistoryStoreWithDB(db)
	taskStore := database.NewAgentInstanceTaskStoreWithDB(db)

	// Create test data
	userUUID := uuid.New().String()
	instance := &database.AgentInstance{
		TemplateID: 1,
		UserUUID:   userUUID,
		Type:       "langflow",
		ContentID:  "test-instance",
		Public:     false,
	}
	createdInstance, err := instanceStore.Create(ctx, instance)
	require.NoError(t, err)
	require.NotZero(t, createdInstance.ID)

	// Create sessions for this instance
	session1 := &database.AgentInstanceSession{
		UUID:       uuid.New().String(),
		Name:       "Session 1",
		InstanceID: createdInstance.ID,
		UserUUID:   userUUID,
		Type:       "langflow",
	}
	createdSession1, err := sessionStore.Create(ctx, session1)
	require.NoError(t, err)

	session2 := &database.AgentInstanceSession{
		UUID:       uuid.New().String(),
		Name:       "Session 2",
		InstanceID: createdInstance.ID,
		UserUUID:   userUUID,
		Type:       "langflow",
	}
	createdSession2, err := sessionStore.Create(ctx, session2)
	require.NoError(t, err)

	// Create session histories for session1
	history1 := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession1.ID,
		Request:   true,
		Turn:      1,
		Content:   "User message 1",
	}
	err = historyStore.Create(ctx, history1)
	require.NoError(t, err)

	history2 := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession1.ID,
		Request:   false,
		Turn:      1,
		Content:   "Assistant response 1",
	}
	err = historyStore.Create(ctx, history2)
	require.NoError(t, err)

	// Create session history for session2
	history3 := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession2.ID,
		Request:   true,
		Turn:      1,
		Content:   "User message 2",
	}
	err = historyStore.Create(ctx, history3)
	require.NoError(t, err)

	// Create tasks for this instance
	task1 := &database.AgentInstanceTask{
		InstanceID:  createdInstance.ID,
		TaskType:    types.AgentTaskTypeFinetuneJob,
		TaskID:      "task-1",
		SessionUUID: createdSession1.UUID,
		UserUUID:    userUUID,
	}
	_, err = taskStore.Create(ctx, task1)
	require.NoError(t, err)

	task2 := &database.AgentInstanceTask{
		InstanceID:  createdInstance.ID,
		TaskType:    types.AgentTaskTypeInference,
		TaskID:      "task-2",
		SessionUUID: createdSession2.UUID,
		UserUUID:    userUUID,
	}
	_, err = taskStore.Create(ctx, task2)
	require.NoError(t, err)

	// Verify all records exist before deletion
	foundInstance, err := instanceStore.FindByID(ctx, createdInstance.ID)
	require.NoError(t, err)
	require.NotNil(t, foundInstance)

	sessions, _, err := sessionStore.ListByInstanceID(ctx, createdInstance.ID)
	require.NoError(t, err)
	require.Len(t, sessions, 2)

	histories1, err := historyStore.ListBySessionID(ctx, createdSession1.ID)
	require.NoError(t, err)
	require.Len(t, histories1, 2)

	histories2, err := historyStore.ListBySessionID(ctx, createdSession2.ID)
	require.NoError(t, err)
	require.Len(t, histories2, 1)

	// Delete the instance (should cascade soft-delete)
	err = instanceStore.Delete(ctx, createdInstance.ID)
	require.NoError(t, err)

	// Verify instance is deleted (not found in normal queries)
	_, err = instanceStore.FindByID(ctx, createdInstance.ID)
	require.Error(t, err, "Instance should not be found after delete")

	// Verify sessions are deleted (not found in normal queries)
	sessions, _, err = sessionStore.ListByInstanceID(ctx, createdInstance.ID)
	require.NoError(t, err)
	require.Len(t, sessions, 0, "Sessions should not be found after delete")

	// Verify sessions are deleted
	_, err = sessionStore.FindByID(ctx, createdSession1.ID)
	require.Error(t, err, "Session 1 should not be found after delete")

	_, err = sessionStore.FindByID(ctx, createdSession2.ID)
	require.Error(t, err, "Session 2 should not be found after delete")

	// Verify session histories are deleted (not found in normal queries)
	histories1, err = historyStore.ListBySessionID(ctx, createdSession1.ID)
	require.NoError(t, err)
	require.Len(t, histories1, 0, "Session histories should not be found after delete")

	histories2, err = historyStore.ListBySessionID(ctx, createdSession2.ID)
	require.NoError(t, err)
	require.Len(t, histories2, 0, "Session histories should not be found after delete")

	// Verify tasks are deleted (not found in normal queries)
	tasks, _, err := taskStore.ListTasks(ctx, userUUID, types.AgentTaskFilter{}, 10, 1)
	require.NoError(t, err)
	// Filter out tasks for our instance
	taskCount := 0
	for _, task := range tasks {
		if task.InstanceID == createdInstance.ID {
			taskCount++
		}
	}
	require.Equal(t, 0, taskCount, "Tasks should not be found after delete")
}

func TestAgentInstanceStore_Delete_WithNoRelatedRecords(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	// Create an instance with no related records
	userUUID := uuid.New().String()
	instance := &database.AgentInstance{
		TemplateID: 1,
		UserUUID:   userUUID,
		Type:       "langflow",
		ContentID:  "isolated-instance",
		Public:     false,
	}
	createdInstance, err := store.Create(ctx, instance)
	require.NoError(t, err)

	// Delete the instance
	err = store.Delete(ctx, createdInstance.ID)
	require.NoError(t, err)

	// Verify instance is deleted (not found in normal queries)
	_, err = store.FindByID(ctx, createdInstance.ID)
	require.Error(t, err, "Instance should not be found after delete")
}

func TestAgentInstanceStore_IsInstanceNameExists(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	userUUID1 := uuid.New().String()
	userUUID2 := uuid.New().String()
	instanceName := "test-instance-name"

	// Test case 1: Instance name does not exist
	exists, err := store.IsInstanceNameExists(ctx, userUUID1, instanceName)
	require.NoError(t, err)
	require.False(t, exists)

	// Test case 2: Create an instance with the name and verify it exists
	instance := &database.AgentInstance{
		TemplateID: 123,
		UserUUID:   userUUID1,
		Type:       "langflow",
		ContentID:  "content-123",
		Name:       instanceName,
		Public:     false,
	}

	createdInstance, err := store.Create(ctx, instance)
	require.NoError(t, err)
	require.NotZero(t, createdInstance.ID)

	// Verify the instance name exists for user1
	exists, err = store.IsInstanceNameExists(ctx, userUUID1, instanceName)
	require.NoError(t, err)
	require.True(t, exists)

	// Test case 3: Different user, same instance name
	exists, err = store.IsInstanceNameExists(ctx, userUUID2, instanceName)
	require.NoError(t, err)
	require.False(t, exists)

	// Test case 4: Same user, different instance name
	exists, err = store.IsInstanceNameExists(ctx, userUUID1, "different-instance-name")
	require.NoError(t, err)
	require.False(t, exists)

	// Test case 5: Both user and instance name different
	exists, err = store.IsInstanceNameExists(ctx, userUUID2, "different-instance-name")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestAgentInstanceStore_IsInstanceNameExists_MultipleInstances(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	userUUID1 := uuid.New().String()
	userUUID2 := uuid.New().String()

	// Create multiple instances with different names for user1
	instances := []*database.AgentInstance{
		{
			TemplateID: 1,
			UserUUID:   userUUID1,
			Type:       "langflow",
			ContentID:  "content-1",
			Name:       "instance-name-1",
			Public:     false,
		},
		{
			TemplateID: 2,
			UserUUID:   userUUID1,
			Type:       "agno",
			ContentID:  "content-2",
			Name:       "instance-name-2",
			Public:     false,
		},
		{
			TemplateID: 3,
			UserUUID:   userUUID1,
			Type:       "langflow",
			ContentID:  "content-3",
			Name:       "instance-name-3",
			Public:     false,
		},
	}

	// Create all instances
	for _, instance := range instances {
		_, err := store.Create(ctx, instance)
		require.NoError(t, err)
	}

	// Test each instance name exists for user1
	for _, instance := range instances {
		exists, err := store.IsInstanceNameExists(ctx, userUUID1, instance.Name)
		require.NoError(t, err)
		require.True(t, exists, "Instance name %s should exist for user %s", instance.Name, userUUID1)
	}

	// Test instance names don't exist for user2
	for _, instance := range instances {
		exists, err := store.IsInstanceNameExists(ctx, userUUID2, instance.Name)
		require.NoError(t, err)
		require.False(t, exists, "Instance name %s should not exist for user %s", instance.Name, userUUID2)
	}

	// Create an instance for user2 with a different name
	user2Instance := &database.AgentInstance{
		TemplateID: 4,
		UserUUID:   userUUID2,
		Type:       "code",
		ContentID:  "content-4",
		Name:       "user2-instance-name",
		Public:     false,
	}
	_, err := store.Create(ctx, user2Instance)
	require.NoError(t, err)

	// Verify user2's instance name exists for user2
	exists, err := store.IsInstanceNameExists(ctx, userUUID2, user2Instance.Name)
	require.NoError(t, err)
	require.True(t, exists)

	// Verify user2's instance name doesn't exist for user1
	exists, err = store.IsInstanceNameExists(ctx, userUUID1, user2Instance.Name)
	require.NoError(t, err)
	require.False(t, exists)

	// Test non-existent combinations
	testCases := []struct {
		userUUID     string
		instanceName string
		description  string
	}{
		{userUUID1, "non-existent-name", "user1 with non-existent name"},
		{userUUID2, "instance-name-1", "user2 with user1's instance name"},
		{uuid.New().String(), "instance-name-1", "non-existent user with existing name"},
		{uuid.New().String(), "non-existent-name", "non-existent user with non-existent name"},
	}

	for _, tc := range testCases {
		exists, err := store.IsInstanceNameExists(ctx, tc.userUUID, tc.instanceName)
		require.NoError(t, err)
		require.False(t, exists, "Should not exist: %s", tc.description)
	}
}

func TestAgentInstanceStore_IsInstanceNameExists_EmptyParameters(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	userUUID := uuid.New().String()

	// Test with empty user UUID
	exists, err := store.IsInstanceNameExists(ctx, "", "some-instance-name")
	require.NoError(t, err)
	require.False(t, exists)

	// Test with empty instance name
	exists, err = store.IsInstanceNameExists(ctx, userUUID, "")
	require.NoError(t, err)
	require.False(t, exists)

	// Test with both empty
	exists, err = store.IsInstanceNameExists(ctx, "", "")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestAgentInstanceStore_IsInstanceNameExists_SameNameDifferentUsers(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceStoreWithDB(db)

	userUUID1 := uuid.New().String()
	userUUID2 := uuid.New().String()
	sharedName := "shared-instance-name"

	// Create instance for user1 with the name
	instance1 := &database.AgentInstance{
		TemplateID: 1,
		UserUUID:   userUUID1,
		Type:       "langflow",
		ContentID:  "content-1",
		Name:       sharedName,
		Public:     false,
	}
	_, err := store.Create(ctx, instance1)
	require.NoError(t, err)

	// Create instance for user2 with the same name
	instance2 := &database.AgentInstance{
		TemplateID: 2,
		UserUUID:   userUUID2,
		Type:       "agno",
		ContentID:  "content-2",
		Name:       sharedName,
		Public:     false,
	}
	_, err = store.Create(ctx, instance2)
	require.NoError(t, err)

	// Verify both users can have instances with the same name
	exists, err := store.IsInstanceNameExists(ctx, userUUID1, sharedName)
	require.NoError(t, err)
	require.True(t, exists, "Instance name should exist for user1")

	exists, err = store.IsInstanceNameExists(ctx, userUUID2, sharedName)
	require.NoError(t, err)
	require.True(t, exists, "Instance name should exist for user2")

	// This confirms the query correctly filters by both user_uuid and name
	// Both users can independently have instances with the same name
}

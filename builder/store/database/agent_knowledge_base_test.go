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

func TestAgentKnowledgeBaseStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentKnowledgeBaseStoreWithDB(db)

	// Test Create
	userUUID := uuid.New().String()
	kb := &database.AgentKnowledgeBase{
		UserUUID:    userUUID,
		Name:        "Test Knowledge Base",
		Description: "Test knowledge base description",
		ContentID:   uuid.New().String(),
		Public:      false,
		Metadata:    map[string]any{"key": "value"},
	}

	createdKB, err := store.Create(ctx, kb)
	require.NoError(t, err)
	require.NotZero(t, createdKB.ID)
	require.Equal(t, kb.UserUUID, createdKB.UserUUID)
	require.Equal(t, kb.Name, createdKB.Name)
	require.Equal(t, kb.Description, createdKB.Description)
	require.Equal(t, kb.ContentID, createdKB.ContentID)
	require.Equal(t, kb.Public, createdKB.Public)
	require.Equal(t, kb.Metadata, createdKB.Metadata)
	// Update the original kb with the created one for further tests
	*kb = *createdKB

	// Test FindByID
	foundKB, err := store.FindByID(ctx, kb.ID)
	require.NoError(t, err)
	require.Equal(t, kb.ID, foundKB.ID)
	require.Equal(t, kb.UserUUID, foundKB.UserUUID)
	require.Equal(t, kb.Name, foundKB.Name)
	require.Equal(t, kb.Description, foundKB.Description)
	require.Equal(t, kb.ContentID, foundKB.ContentID)
	require.Equal(t, kb.Public, foundKB.Public)
	require.Equal(t, kb.Metadata, foundKB.Metadata)

	// Test FindByContentID
	foundKBByContentID, err := store.FindByContentID(ctx, kb.ContentID)
	require.NoError(t, err)
	require.Equal(t, kb.ID, foundKBByContentID.ID)
	require.Equal(t, kb.ContentID, foundKBByContentID.ContentID)

	// Test Update
	kb.Name = "Updated Knowledge Base Name"
	kb.Description = "Updated knowledge base description"
	kb.Public = true
	kb.Metadata = map[string]any{"updated_key": "updated_value"}
	err = store.Update(ctx, kb)
	require.NoError(t, err)

	// Verify update
	updatedKB, err := store.FindByID(ctx, kb.ID)
	require.NoError(t, err)
	require.Equal(t, "Updated Knowledge Base Name", updatedKB.Name)
	require.Equal(t, "Updated knowledge base description", updatedKB.Description)
	require.True(t, updatedKB.Public)
	require.Equal(t, map[string]any{"updated_key": "updated_value"}, updatedKB.Metadata)

	// Test Delete
	err = store.Delete(ctx, kb.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = store.FindByID(ctx, kb.ID)
	require.Error(t, err)
}

func TestAgentKnowledgeBaseStore_List_WithPublicAndPrivate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentKnowledgeBaseStoreWithDB(db)

	userUUID1 := uuid.New().String()
	userUUID2 := uuid.New().String()

	// Create private knowledge base for user1
	privateKB := &database.AgentKnowledgeBase{
		UserUUID:    userUUID1,
		Name:        "Private Knowledge Base",
		Description: "Private knowledge base description",
		ContentID:   uuid.New().String(),
		Public:      false,
	}
	_, err := store.Create(ctx, privateKB)
	require.NoError(t, err)

	// Create public knowledge base for user1
	publicKB := &database.AgentKnowledgeBase{
		UserUUID:    userUUID1,
		Name:        "Public Knowledge Base",
		Description: "Public knowledge base description",
		ContentID:   uuid.New().String(),
		Public:      true,
	}
	_, err = store.Create(ctx, publicKB)
	require.NoError(t, err)

	// Create private knowledge base for user2
	user2KB := &database.AgentKnowledgeBase{
		UserUUID:    userUUID2,
		Name:        "User2 Knowledge Base",
		Description: "User2 knowledge base description",
		ContentID:   uuid.New().String(),
		Public:      false,
	}
	_, err = store.Create(ctx, user2KB)
	require.NoError(t, err)

	// Test List for user1 - should return both private and public knowledge bases from user1
	knowledgeBases, total, err := store.List(ctx, types.AgentKnowledgeBaseFilter{UserUUID: userUUID1}, 10, 1)
	require.NoError(t, err)
	require.Len(t, knowledgeBases, 2)
	require.Equal(t, 2, total)

	// Test List for user2 - should return public knowledge base from user1 and private knowledge base from user2
	knowledgeBases, total, err = store.List(ctx, types.AgentKnowledgeBaseFilter{UserUUID: userUUID2}, 10, 1)
	require.NoError(t, err)
	require.Len(t, knowledgeBases, 2) // public KB from user1 + private KB from user2
	require.Equal(t, 2, total)
}

func TestAgentKnowledgeBaseStore_NotFound(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentKnowledgeBaseStoreWithDB(db)

	// Test FindByID with non-existent ID
	_, err := store.FindByID(ctx, 99999)
	require.Error(t, err)

	// Test FindByContentID with non-existent content ID
	_, err = store.FindByContentID(ctx, "non-existent-content-id")
	require.Error(t, err)

	// Test List with non-existent user
	knowledgeBases, total, err := store.List(ctx, types.AgentKnowledgeBaseFilter{UserUUID: "non-existent-user"}, 10, 1)
	require.NoError(t, err)
	require.Len(t, knowledgeBases, 0)
	require.Equal(t, 0, total)
}

func TestAgentKnowledgeBaseStore_Update_NonExistent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentKnowledgeBaseStoreWithDB(db)

	// Test Update with non-existent knowledge base
	nonExistentKB := &database.AgentKnowledgeBase{
		ID:          99999,
		UserUUID:    uuid.New().String(),
		Name:        "Non-existent Knowledge Base",
		Description: "Non-existent knowledge base description",
		ContentID:   uuid.New().String(),
		Public:      false,
	}

	err := store.Update(ctx, nonExistentKB)
	require.Error(t, err)
}

func TestAgentKnowledgeBaseStore_Delete_NonExistent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentKnowledgeBaseStoreWithDB(db)

	// Test Delete with non-existent ID
	err := store.Delete(ctx, 99999)
	require.Error(t, err)
}

func TestAgentKnowledgeBaseStore_List_WithFilters(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentKnowledgeBaseStoreWithDB(db)

	userUUID := uuid.New().String()

	// Create knowledge bases with different names
	kb1 := &database.AgentKnowledgeBase{
		UserUUID:    userUUID,
		Name:        "Python Knowledge Base",
		Description: "A knowledge base for Python programming",
		ContentID:   uuid.New().String(),
		Public:      false,
	}
	_, err := store.Create(ctx, kb1)
	require.NoError(t, err)

	kb2 := &database.AgentKnowledgeBase{
		UserUUID:    userUUID,
		Name:        "Go Knowledge Base",
		Description: "A knowledge base for Go programming",
		ContentID:   uuid.New().String(),
		Public:      true,
	}
	_, err = store.Create(ctx, kb2)
	require.NoError(t, err)

	kb3 := &database.AgentKnowledgeBase{
		UserUUID:    userUUID,
		Name:        "Another Python KB",
		Description: "Another Python knowledge base",
		ContentID:   uuid.New().String(),
		Public:      false,
	}
	_, err = store.Create(ctx, kb3)
	require.NoError(t, err)

	// Test search filter
	publicTrue := true
	knowledgeBases, total, err := store.List(ctx, types.AgentKnowledgeBaseFilter{
		UserUUID: userUUID,
		Search:   "Python",
	}, 10, 1)
	require.NoError(t, err)
	require.Len(t, knowledgeBases, 2) // Should find both Python knowledge bases
	require.Equal(t, 2, total)

	// Test public filter
	knowledgeBases, total, err = store.List(ctx, types.AgentKnowledgeBaseFilter{
		UserUUID: userUUID,
		Public:   &publicTrue,
	}, 10, 1)
	require.NoError(t, err)
	require.Len(t, knowledgeBases, 1) // Should find only the public knowledge base
	require.Equal(t, 1, total)

	// Test editable filter (true = owned by user)
	editableTrue := true
	knowledgeBases, total, err = store.List(ctx, types.AgentKnowledgeBaseFilter{
		UserUUID: userUUID,
		Editable: &editableTrue,
	}, 10, 1)
	require.NoError(t, err)
	require.Len(t, knowledgeBases, 3) // Should find all knowledge bases owned by user
	require.Equal(t, 3, total)

	// Test editable filter (false = not owned by user)
	editableFalse := false
	knowledgeBases, total, err = store.List(ctx, types.AgentKnowledgeBaseFilter{
		UserUUID: userUUID,
		Editable: &editableFalse,
	}, 10, 1)
	require.NoError(t, err)
	require.Len(t, knowledgeBases, 0) // Should find no knowledge bases not owned by user
	require.Equal(t, 0, total)

	// Test combined filters
	knowledgeBases, total, err = store.List(ctx, types.AgentKnowledgeBaseFilter{
		UserUUID: userUUID,
		Search:   "Go",
		Public:   &publicTrue,
	}, 10, 1)
	require.NoError(t, err)
	require.Len(t, knowledgeBases, 1) // Should find only "Go Knowledge Base" (public and name contains "Go")
	require.Equal(t, 1, total)

	// Test pagination
	knowledgeBases, total, err = store.List(ctx, types.AgentKnowledgeBaseFilter{
		UserUUID: userUUID,
	}, 2, 1)
	require.NoError(t, err)
	require.Len(t, knowledgeBases, 2) // Should return only 2 knowledge bases due to limit
	require.Equal(t, 3, total)        // But total should be 3

	// Test second page
	knowledgeBases, total, err = store.List(ctx, types.AgentKnowledgeBaseFilter{
		UserUUID: userUUID,
	}, 2, 2)
	require.NoError(t, err)
	require.Len(t, knowledgeBases, 1) // Should return 1 knowledge base on second page
	require.Equal(t, 3, total)        // Total should still be 3
}

func TestAgentKnowledgeBaseStore_Create_WithEmptyDescription(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentKnowledgeBaseStoreWithDB(db)

	userUUID := uuid.New().String()
	kb := &database.AgentKnowledgeBase{
		UserUUID:  userUUID,
		Name:      "Knowledge Base Without Description",
		ContentID: uuid.New().String(),
		Public:    false,
	}

	createdKB, err := store.Create(ctx, kb)
	require.NoError(t, err)
	require.NotZero(t, createdKB.ID)
	require.Empty(t, createdKB.Description)
}

func TestAgentKnowledgeBaseStore_Create_WithEmptyMetadata(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentKnowledgeBaseStoreWithDB(db)

	userUUID := uuid.New().String()
	kb := &database.AgentKnowledgeBase{
		UserUUID:    userUUID,
		Name:        "Knowledge Base Without Metadata",
		Description: "Test description",
		ContentID:   uuid.New().String(),
		Public:      false,
	}

	createdKB, err := store.Create(ctx, kb)
	require.NoError(t, err)
	require.NotZero(t, createdKB.ID)
	require.Nil(t, createdKB.Metadata)
}

func TestAgentKnowledgeBaseStore_UniqueContentID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentKnowledgeBaseStoreWithDB(db)

	userUUID := uuid.New().String()
	contentID := uuid.New().String()

	// Create first knowledge base
	kb1 := &database.AgentKnowledgeBase{
		UserUUID:    userUUID,
		Name:        "First Knowledge Base",
		Description: "First description",
		ContentID:   contentID,
		Public:      false,
	}
	_, err := store.Create(ctx, kb1)
	require.NoError(t, err)

	// Try to create second knowledge base with same content ID
	kb2 := &database.AgentKnowledgeBase{
		UserUUID:    uuid.New().String(),
		Name:        "Second Knowledge Base",
		Description: "Second description",
		ContentID:   contentID, // Same content ID
		Public:      false,
	}
	_, err = store.Create(ctx, kb2)
	require.Error(t, err) // Should fail due to unique constraint
}

func TestAgentKnowledgeBaseStore_List_OrderByUpdatedAt(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentKnowledgeBaseStoreWithDB(db)

	userUUID := uuid.New().String()

	// Create knowledge bases
	kb1 := &database.AgentKnowledgeBase{
		UserUUID:    userUUID,
		Name:        "First Knowledge Base",
		Description: "First description",
		ContentID:   uuid.New().String(),
		Public:      false,
	}
	createdKB1, err := store.Create(ctx, kb1)
	require.NoError(t, err)

	kb2 := &database.AgentKnowledgeBase{
		UserUUID:    userUUID,
		Name:        "Second Knowledge Base",
		Description: "Second description",
		ContentID:   uuid.New().String(),
		Public:      false,
	}
	createdKB2, err := store.Create(ctx, kb2)
	require.NoError(t, err)

	// Update first knowledge base to change updated_at
	createdKB1.Name = "Updated First Knowledge Base"
	err = store.Update(ctx, createdKB1)
	require.NoError(t, err)

	// List should return in order of updated_at DESC (most recently updated first)
	knowledgeBases, total, err := store.List(ctx, types.AgentKnowledgeBaseFilter{
		UserUUID: userUUID,
	}, 10, 1)
	require.NoError(t, err)
	require.Len(t, knowledgeBases, 2)
	require.Equal(t, 2, total)
	// First item should be the most recently updated (kb1)
	require.Equal(t, createdKB1.ID, knowledgeBases[0].ID)
	require.Equal(t, "Updated First Knowledge Base", knowledgeBases[0].Name)
	// Second item should be kb2
	require.Equal(t, createdKB2.ID, knowledgeBases[1].ID)
	require.Equal(t, "Second Knowledge Base", knowledgeBases[1].Name)
}

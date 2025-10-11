package database_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestAgentTemplateStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentTemplateStoreWithDB(db)

	// Test Create
	userUUID := uuid.New().String()
	template := &database.AgentTemplate{
		Type:     "langflow",
		UserUUID: userUUID,
		Content:  "test template content",
		Public:   false,
	}

	err := store.Create(ctx, template)
	require.NoError(t, err)
	require.NotZero(t, template.ID)

	// Test FindByID
	foundTemplate, err := store.FindByID(ctx, template.ID)
	require.NoError(t, err)
	require.Equal(t, template.ID, foundTemplate.ID)
	require.Equal(t, template.Type, foundTemplate.Type)
	require.Equal(t, template.UserUUID, foundTemplate.UserUUID)
	require.Equal(t, template.Content, foundTemplate.Content)
	require.Equal(t, template.Public, foundTemplate.Public)

	// Test ListByUserUUID
	templates, err := store.ListByUserUUID(ctx, userUUID)
	require.NoError(t, err)
	require.Len(t, templates, 1)
	require.Equal(t, template.ID, templates[0].ID)

	// Test Update
	template.Content = "updated template content"
	template.Public = true
	err = store.Update(ctx, template)
	require.NoError(t, err)

	// Verify update
	updatedTemplate, err := store.FindByID(ctx, template.ID)
	require.NoError(t, err)
	require.Equal(t, "updated template content", updatedTemplate.Content)
	require.True(t, updatedTemplate.Public)

	// Test Delete
	err = store.Delete(ctx, template.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = store.FindByID(ctx, template.ID)
	require.Error(t, err)
}

func TestAgentTemplateStore_ListByUserUUID_WithPublicTemplates(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentTemplateStoreWithDB(db)

	userUUID1 := uuid.New().String()
	userUUID2 := uuid.New().String()

	// Create private template for user1
	privateTemplate := &database.AgentTemplate{
		Type:     "langflow",
		UserUUID: userUUID1,
		Content:  "private template",
		Public:   false,
	}
	err := store.Create(ctx, privateTemplate)
	require.NoError(t, err)

	// Create public template for user1
	publicTemplate := &database.AgentTemplate{
		Type:     "agno",
		UserUUID: userUUID1,
		Content:  "public template",
		Public:   true,
	}
	err = store.Create(ctx, publicTemplate)
	require.NoError(t, err)

	// Create private template for user2
	user2Template := &database.AgentTemplate{
		Type:     "code",
		UserUUID: userUUID2,
		Content:  "user2 template",
		Public:   false,
	}
	err = store.Create(ctx, user2Template)
	require.NoError(t, err)

	// Test ListByUserUUID for user1 - should return both private and public templates
	templates, err := store.ListByUserUUID(ctx, userUUID1)
	require.NoError(t, err)
	require.Len(t, templates, 2)

	// Test ListByUserUUID for user2 - should return only public template from user1 and private template from user2
	templates, err = store.ListByUserUUID(ctx, userUUID2)
	require.NoError(t, err)
	require.Len(t, templates, 2) // public template from user1 + private template from user2
}

func TestAgentTemplateStore_NotFound(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentTemplateStoreWithDB(db)

	// Test FindByID with non-existent ID
	_, err := store.FindByID(ctx, 99999)
	require.Error(t, err)

	// Test ListByUserUUID with non-existent user
	templates, err := store.ListByUserUUID(ctx, "non-existent-user")
	require.NoError(t, err)
	require.Len(t, templates, 0)
}

func TestAgentTemplateStore_Update_NonExistent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentTemplateStoreWithDB(db)

	// Test Update with non-existent template
	nonExistentTemplate := &database.AgentTemplate{
		ID:       99999,
		Type:     "langflow",
		UserUUID: uuid.New().String(),
		Content:  "test content",
		Public:   false,
	}

	err := store.Update(ctx, nonExistentTemplate)
	require.Error(t, err)
}

func TestAgentTemplateStore_Delete_NonExistent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentTemplateStoreWithDB(db)

	// Test Delete with non-existent ID
	err := store.Delete(ctx, 99999)
	require.Error(t, err)
}

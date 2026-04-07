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

func TestAgentTemplateStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentTemplateStoreWithDB(db)

	// Test Create
	userUUID := uuid.New().String()
	template := &database.AgentTemplate{
		Type:        "langflow",
		UserUUID:    userUUID,
		Name:        "Test Template",
		Description: "Test template description",
		Content:     "test template content",
		Public:      false,
	}

	createdTemplate, err := store.Create(ctx, template)
	require.NoError(t, err)
	require.NotZero(t, createdTemplate.ID)
	// Update the original template with the created one for further tests
	*template = *createdTemplate

	// Test FindByID
	foundTemplate, err := store.FindByID(ctx, template.ID)
	require.NoError(t, err)
	require.Equal(t, template.ID, foundTemplate.ID)
	require.Equal(t, template.Type, foundTemplate.Type)
	require.Equal(t, template.UserUUID, foundTemplate.UserUUID)
	require.Equal(t, template.Name, foundTemplate.Name)
	require.Equal(t, template.Description, foundTemplate.Description)
	require.Equal(t, template.Content, foundTemplate.Content)
	require.Equal(t, template.Public, foundTemplate.Public)

	// Test ListByUserUUID
	templates, total, err := store.ListByUserUUID(ctx, userUUID, types.AgentTemplateFilter{}, 10, 1)
	require.NoError(t, err)
	require.Len(t, templates, 1)
	require.Equal(t, 1, total)
	require.Equal(t, template.ID, templates[0].ID)

	// Test Update
	template.Name = "Updated Template Name"
	template.Description = "Updated template description"
	template.Content = "updated template content"
	template.Public = true
	err = store.Update(ctx, template)
	require.NoError(t, err)

	// Verify update
	updatedTemplate, err := store.FindByID(ctx, template.ID)
	require.NoError(t, err)
	require.Equal(t, "Updated Template Name", updatedTemplate.Name)
	require.Equal(t, "Updated template description", updatedTemplate.Description)
	require.Equal(t, "updated template content", updatedTemplate.Content)
	require.True(t, updatedTemplate.Public)

	// Test Delete
	err = store.Delete(ctx, template.ID)
	require.NoError(t, err)

	// Verify deletion - FindByID should not find soft-deleted template
	_, err = store.FindByID(ctx, template.ID)
	require.Error(t, err)

	// Verify deletion - ListByUserUUID should not include soft-deleted template
	templatesAfterDelete, _, err := store.ListByUserUUID(ctx, userUUID, types.AgentTemplateFilter{}, 10, 1)
	require.NoError(t, err)
	// Verify deleted template is not in results
	found := false
	for _, tmpl := range templatesAfterDelete {
		if tmpl.ID == template.ID {
			found = true
			break
		}
	}
	require.False(t, found, "Deleted template should not be found in list results")
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
		Type:        "langflow",
		UserUUID:    userUUID1,
		Name:        "Private Template",
		Description: "Private template description",
		Content:     "private template",
		Public:      false,
	}
	_, err := store.Create(ctx, privateTemplate)
	require.NoError(t, err)

	// Create public template for user1
	publicTemplate := &database.AgentTemplate{
		Type:        "agno",
		UserUUID:    userUUID1,
		Name:        "Public Template",
		Description: "Public template description",
		Content:     "public template",
		Public:      true,
	}
	_, err = store.Create(ctx, publicTemplate)
	require.NoError(t, err)

	// Create private template for user2
	user2Template := &database.AgentTemplate{
		Type:        "code",
		UserUUID:    userUUID2,
		Name:        "User2 Template",
		Description: "User2 template description",
		Content:     "user2 template",
		Public:      false,
	}
	_, err = store.Create(ctx, user2Template)
	require.NoError(t, err)

	// Test ListByUserUUID for user1 - should return both private and public templates
	templates, total, err := store.ListByUserUUID(ctx, userUUID1, types.AgentTemplateFilter{}, 10, 1)
	require.NoError(t, err)
	require.Len(t, templates, 2)
	require.Equal(t, 2, total)

	// Test ListByUserUUID for user2 - should return only public template from user1 and private template from user2
	templates, total, err = store.ListByUserUUID(ctx, userUUID2, types.AgentTemplateFilter{}, 10, 1)
	require.NoError(t, err)
	require.Len(t, templates, 2) // public template from user1 + private template from user2
	require.Equal(t, 2, total)
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
	templates, total, err := store.ListByUserUUID(ctx, "non-existent-user", types.AgentTemplateFilter{}, 10, 1)
	require.NoError(t, err)
	require.Len(t, templates, 0)
	require.Equal(t, 0, total)
}

func TestAgentTemplateStore_Update_NonExistent(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentTemplateStoreWithDB(db)

	// Test Update with non-existent template
	nonExistentTemplate := &database.AgentTemplate{
		ID:          99999,
		Type:        "langflow",
		UserUUID:    uuid.New().String(),
		Name:        "Non-existent Template",
		Description: "Non-existent template description",
		Content:     "test content",
		Public:      false,
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

func TestAgentTemplateStore_ListByUserUUID_WithFilters(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentTemplateStoreWithDB(db)

	userUUID := uuid.New().String()

	// Create templates with different types and names
	template1 := &database.AgentTemplate{
		Type:        "langflow",
		UserUUID:    userUUID,
		Name:        "Langflow Agent",
		Description: "A langflow agent for automation",
		Content:     "langflow content",
		Public:      false,
	}
	_, err := store.Create(ctx, template1)
	require.NoError(t, err)

	template2 := &database.AgentTemplate{
		Type:        "agno",
		UserUUID:    userUUID,
		Name:        "Agno Assistant",
		Description: "An agno assistant for help",
		Content:     "agno content",
		Public:      false,
	}
	_, err = store.Create(ctx, template2)
	require.NoError(t, err)

	template3 := &database.AgentTemplate{
		Type:        "langflow",
		UserUUID:    userUUID,
		Name:        "Another Langflow",
		Description: "Another langflow agent",
		Content:     "another langflow content",
		Public:      false,
	}
	_, err = store.Create(ctx, template3)
	require.NoError(t, err)

	// Test search filter
	templates, total, err := store.ListByUserUUID(ctx, userUUID, types.AgentTemplateFilter{Search: "Langflow"}, 10, 1)
	require.NoError(t, err)
	require.Len(t, templates, 2) // Should find both langflow templates
	require.Equal(t, 2, total)

	// Test type filter
	templates, total, err = store.ListByUserUUID(ctx, userUUID, types.AgentTemplateFilter{Type: "agno"}, 10, 1)
	require.NoError(t, err)
	require.Len(t, templates, 1) // Should find only the agno template
	require.Equal(t, 1, total)

	// Test combined filters
	templates, total, err = store.ListByUserUUID(ctx, userUUID, types.AgentTemplateFilter{Search: "automation", Type: "langflow"}, 10, 1)
	require.NoError(t, err)
	require.Len(t, templates, 1) // Should find only "Langflow Agent" (has "automation" in description)
	require.Equal(t, 1, total)

	// Test pagination
	templates, total, err = store.ListByUserUUID(ctx, userUUID, types.AgentTemplateFilter{}, 2, 1)
	require.NoError(t, err)
	require.Len(t, templates, 2) // Should return only 2 templates due to limit
	require.Equal(t, 3, total)   // But total should be 3

	// Test second page
	templates, total, err = store.ListByUserUUID(ctx, userUUID, types.AgentTemplateFilter{}, 2, 2)
	require.NoError(t, err)
	require.Len(t, templates, 1) // Should return 1 template on second page
	require.Equal(t, 3, total)   // Total should still be 3
}

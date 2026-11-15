package database_test

import (
	"context"
	"strconv"
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
	_, err = store.Create(ctx, &database.AgentTemplate{Type: "csgclaw", UserUUID: userUUID1, Name: "hidden", IsInternal: true})
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

	// Anonymous callers receive public templates only.
	templates, total, err = store.ListByUserUUID(ctx, "", types.AgentTemplateFilter{}, 10, 1)
	require.NoError(t, err)
	require.Len(t, templates, 1)
	require.Equal(t, 1, total)
	require.Equal(t, publicTemplate.ID, templates[0].ID)
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

func TestAgentTemplateStore_ListByUserUUID_WithEditableFilter(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentTemplateStoreWithDB(db)

	userUUID1 := uuid.New().String()
	userUUID2 := uuid.New().String()

	// Create user1's private template (owned by user1)
	user1Private := &database.AgentTemplate{
		Type:     "langflow",
		UserUUID: userUUID1,
		Name:     "User1 Private Template",
		Content:  "private",
		Public:   false,
	}
	_, err := store.Create(ctx, user1Private)
	require.NoError(t, err)

	// Create user1's public template (owned by user1)
	user1Public := &database.AgentTemplate{
		Type:     "code",
		UserUUID: userUUID1,
		Name:     "User1 Public Template",
		Content:  "public",
		Public:   true,
	}
	_, err = store.Create(ctx, user1Public)
	require.NoError(t, err)

	// Create user2's public template (not owned by user1)
	user2Public := &database.AgentTemplate{
		Type:     "agno",
		UserUUID: userUUID2,
		Name:     "User2 Public Template",
		Content:  "public",
		Public:   true,
	}
	_, err = store.Create(ctx, user2Public)
	require.NoError(t, err)

	// Create user2's private template (not owned by user1, excluded by base query)
	user2Private := &database.AgentTemplate{
		Type:     "langflow",
		UserUUID: userUUID2,
		Name:     "User2 Private Template",
		Content:  "private",
		Public:   false,
	}
	_, err = store.Create(ctx, user2Private)
	require.NoError(t, err)

	// Editable=true should return only templates owned by user1
	editableTrue := true
	templates, total, err := store.ListByUserUUID(ctx, userUUID1, types.AgentTemplateFilter{Editable: &editableTrue}, 10, 1)
	require.NoError(t, err)
	require.Len(t, templates, 2)
	require.Equal(t, 2, total)
	for _, tmpl := range templates {
		require.Equal(t, userUUID1, tmpl.UserUUID, "All templates should belong to user1 when Editable=true")
	}

	// Editable=false should return only public templates not owned by user1
	editableFalse := false
	templates, total, err = store.ListByUserUUID(ctx, userUUID1, types.AgentTemplateFilter{Editable: &editableFalse}, 10, 1)
	require.NoError(t, err)
	require.Len(t, templates, 1)
	require.Equal(t, 1, total)
	require.Equal(t, user2Public.ID, templates[0].ID, "Only user2's public template should be returned when Editable=false")

	// Combined filter (Editable=true + Type)
	templates, total, err = store.ListByUserUUID(ctx, userUUID1, types.AgentTemplateFilter{Editable: &editableTrue, Type: "langflow"}, 10, 1)
	require.NoError(t, err)
	require.Len(t, templates, 1)
	require.Equal(t, 1, total)
	require.Equal(t, user1Private.ID, templates[0].ID, "Only user1's private langflow template should be returned")

	// Combined filter (Editable=false + Type)
	templates, total, err = store.ListByUserUUID(ctx, userUUID1, types.AgentTemplateFilter{Editable: &editableFalse, Type: "agno"}, 10, 1)
	require.NoError(t, err)
	require.Len(t, templates, 1)
	require.Equal(t, 1, total)
	require.Equal(t, user2Public.ID, templates[0].ID, "Only user2's public agno template should be returned")

	// Combined filter (Editable=true + Search)
	templates, total, err = store.ListByUserUUID(ctx, userUUID1, types.AgentTemplateFilter{Editable: &editableTrue, Search: "Public"}, 10, 1)
	require.NoError(t, err)
	require.Len(t, templates, 1)
	require.Equal(t, 1, total)
	require.Equal(t, user1Public.ID, templates[0].ID, "Only user1's public template should match the search")
}

func TestAgentTemplateStore_Curation(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentTemplateStoreWithDB(db)
	curationStore := database.NewAgentResourceCurationStoreWithDB(db)

	ownerUUID := uuid.New().String()
	otherUUID := uuid.New().String()

	mkPublic := func(name string) *database.AgentTemplate {
		tmpl := &database.AgentTemplate{
			Type:     "langflow",
			UserUUID: ownerUUID,
			Name:     name,
			Public:   true,
		}
		_, err := store.Create(ctx, tmpl)
		require.NoError(t, err)
		return tmpl
	}
	tmplA := mkPublic("curation-tmpl-a")
	tmplB := mkPublic("curation-tmpl-b")
	internal := &database.AgentTemplate{Type: "csgclaw", UserUUID: ownerUUID, Name: "curation-internal", Public: true, IsInternal: true}
	_, err := store.Create(ctx, internal)
	require.NoError(t, err)
	idA := strconv.FormatInt(tmplA.ID, 10)
	idB := strconv.FormatInt(tmplB.ID, 10)

	listedByID := func(userUUID string) map[int64]database.AgentTemplate {
		templates, _, err := store.ListByUserUUID(ctx, userUUID, types.AgentTemplateFilter{}, 100, 1)
		require.NoError(t, err)
		byID := make(map[int64]database.AgentTemplate, len(templates))
		for _, tmpl := range templates {
			byID[tmpl.ID] = tmpl
		}
		return byID
	}

	// Unlisted hides the template from non-owners but not from the owner
	unlisted := true
	require.NoError(t, curationStore.UpsertFields(ctx, "agent_template", idA, database.AgentResourceCurationFields{Unlisted: &unlisted}))
	require.NotContains(t, listedByID(otherUUID), tmplA.ID, "unlisted template must be hidden from non-owners")
	require.Contains(t, listedByID(otherUUID), tmplB.ID)
	require.Contains(t, listedByID(ownerUUID), tmplA.ID, "owner still sees own unlisted template")

	// Admin list includes unlisted templates with curation state
	adminItems, adminTotal, err := store.ListPublicWithCuration(ctx, "", 100, 1)
	require.NoError(t, err)
	require.GreaterOrEqual(t, adminTotal, 2)
	adminByID := make(map[int64]database.AgentTemplate, len(adminItems))
	for _, tmpl := range adminItems {
		adminByID[tmpl.ID] = tmpl
	}
	require.True(t, adminByID[tmplA.ID].Unlisted)
	require.False(t, adminByID[tmplB.ID].Unlisted)
	require.NotContains(t, adminByID, internal.ID)

	// Visible order excludes unlisted templates
	visible, err := store.ListVisiblePublicOrder(ctx)
	require.NoError(t, err)
	visibleIDs := make(map[int64]bool, len(visible))
	for _, item := range visible {
		visibleIDs[item.ID] = true
	}
	require.False(t, visibleIDs[tmplA.ID])
	require.True(t, visibleIDs[tmplB.ID])
	require.False(t, visibleIDs[internal.ID])

	// Recommended badge shows on the user list
	recommended := true
	require.NoError(t, curationStore.UpsertFields(ctx, "agent_template", idB, database.AgentResourceCurationFields{Recommended: &recommended}))
	require.True(t, listedByID(otherUUID)[tmplB.ID].Recommended)

	// Deleting a template removes its curation row
	require.NoError(t, store.Delete(ctx, tmplA.ID))
	curation, err := curationStore.GetByEntity(ctx, "agent_template", idA)
	require.NoError(t, err)
	require.Nil(t, curation)
}

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

func TestAgentPromptStore_ListByUsername(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentPromptStoreWithDB(db)
	userStore := database.NewUserStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)
	promptStore := database.NewPromptStoreWithDB(db)
	preferenceStore := database.NewAgentUserPreferenceStoreWithDB(db)

	// Create test user
	userUUID := uuid.New().String()
	err := userStore.Create(ctx, &database.User{
		Username: "testuser",
		UUID:     userUUID,
	}, &database.Namespace{})
	require.NoError(t, err)

	user, err := userStore.FindByUsername(ctx, "testuser")
	require.NoError(t, err)

	// Create test repositories
	repo1, err := repoStore.CreateRepo(ctx, database.Repository{
		Name:           "prompt1",
		UserID:         user.ID,
		GitPath:        "prompts_testuser/prompt1",
		Path:           "testuser/prompt1",
		RepositoryType: types.PromptRepo,
		Description:    "First prompt",
		Private:        false,
	})
	require.NoError(t, err)

	repo2, err := repoStore.CreateRepo(ctx, database.Repository{
		Name:           "prompt2",
		UserID:         user.ID,
		GitPath:        "prompts_testuser/prompt2",
		Path:           "testuser/prompt2",
		RepositoryType: types.PromptRepo,
		Description:    "Second prompt",
		Private:        false,
	})
	require.NoError(t, err)

	repo3, err := repoStore.CreateRepo(ctx, database.Repository{
		Name:           "prompt3",
		UserID:         user.ID,
		GitPath:        "prompts_testuser/prompt3",
		Path:           "testuser/prompt3",
		RepositoryType: types.PromptRepo,
		Description:    "Third prompt",
		Private:        false,
	})
	require.NoError(t, err)

	// Create prompts
	prompt1, err := promptStore.Create(ctx, database.Prompt{
		RepositoryID: repo1.ID,
	})
	require.NoError(t, err)

	prompt2, err := promptStore.Create(ctx, database.Prompt{
		RepositoryID: repo2.ID,
	})
	require.NoError(t, err)

	prompt3, err := promptStore.Create(ctx, database.Prompt{
		RepositoryID: repo3.ID,
	})
	require.NoError(t, err)

	t.Run("list all prompts", func(t *testing.T) {
		prompts, total, err := store.ListByUsername(ctx, "testuser", userUUID, "", 10, 1)
		require.NoError(t, err)
		require.Equal(t, 3, total)
		require.Equal(t, 3, len(prompts))
		// Verify all prompts are returned
		promptIDs := make(map[int64]bool)
		for _, p := range prompts {
			promptIDs[p.ID] = true
			require.False(t, p.IsPinned) // None should be pinned yet
			require.Nil(t, p.PinnedAt)
		}
		require.True(t, promptIDs[prompt1.ID])
		require.True(t, promptIDs[prompt2.ID])
		require.True(t, promptIDs[prompt3.ID])
	})

	t.Run("list with search filter", func(t *testing.T) {
		prompts, total, err := store.ListByUsername(ctx, "testuser", userUUID, "prompt1", 10, 1)
		require.NoError(t, err)
		require.Equal(t, 1, total)
		require.Equal(t, 1, len(prompts))
		require.Equal(t, prompt1.ID, prompts[0].ID)
		require.Equal(t, "testuser/prompt1", prompts[0].Path)
		require.Equal(t, "prompt1", prompts[0].Name)
		require.Equal(t, "First prompt", prompts[0].Description)
	})

	t.Run("list with case insensitive search", func(t *testing.T) {
		prompts, total, err := store.ListByUsername(ctx, "testuser", userUUID, "PROMPT2", 10, 1)
		require.NoError(t, err)
		require.Equal(t, 1, total)
		require.Equal(t, 1, len(prompts))
		require.Equal(t, prompt2.ID, prompts[0].ID)
	})

	t.Run("list with pagination", func(t *testing.T) {
		prompts, total, err := store.ListByUsername(ctx, "testuser", userUUID, "", 2, 1)
		require.NoError(t, err)
		require.Equal(t, 3, total)
		require.Equal(t, 2, len(prompts))

		prompts2, total2, err := store.ListByUsername(ctx, "testuser", userUUID, "", 2, 2)
		require.NoError(t, err)
		require.Equal(t, 3, total2)
		require.Equal(t, 1, len(prompts2))
	})

	t.Run("list with pinned prompt", func(t *testing.T) {
		// Pin prompt2 - use the prompt ID as string (will be normalized by the store)
		err := preferenceStore.Create(ctx, &database.AgentUserPreference{
			UserUUID:   userUUID,
			EntityType: types.AgentUserPreferenceEntityTypePrompt,
			EntityID:   strconv.FormatInt(prompt2.ID, 10), // Convert to string, will be normalized
			Action:     types.AgentUserPreferenceActionPin,
		})
		require.NoError(t, err)

		prompts, total, err := store.ListByUsername(ctx, "testuser", userUUID, "", 10, 1)
		require.NoError(t, err)
		require.Equal(t, 3, total)
		require.Equal(t, 3, len(prompts))

		// Find prompt2 in the results
		var foundPrompt2 *database.AgentPrompt
		for i := range prompts {
			if prompts[i].ID == prompt2.ID {
				foundPrompt2 = &prompts[i]
				break
			}
		}
		require.NotNil(t, foundPrompt2, "prompt2 should be in results")
		require.True(t, foundPrompt2.IsPinned, "prompt2 should be pinned")
		require.NotNil(t, foundPrompt2.PinnedAt, "pinned_at should not be nil")

		// The pinned prompt should be first (ordered by pinned_at DESC NULLS LAST)
		require.True(t, prompts[0].IsPinned, "first prompt should be pinned")
		require.Equal(t, prompt2.ID, prompts[0].ID, "pinned prompt should be first")
	})

	t.Run("list with empty search returns all", func(t *testing.T) {
		prompts, total, err := store.ListByUsername(ctx, "testuser", userUUID, "   ", 10, 1)
		require.NoError(t, err)
		require.Equal(t, 3, total)
		require.Equal(t, 3, len(prompts))
	})

	t.Run("list with non-existent username", func(t *testing.T) {
		prompts, total, err := store.ListByUsername(ctx, "nonexistent", userUUID, "", 10, 1)
		require.NoError(t, err)
		require.Equal(t, 0, total)
		require.Equal(t, 0, len(prompts))
	})
}

func TestAgentPromptStore_FindByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentPromptStoreWithDB(db)
	userStore := database.NewUserStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)
	promptStore := database.NewPromptStoreWithDB(db)

	// Create test user
	userUUID := uuid.New().String()
	err := userStore.Create(ctx, &database.User{
		Username: "testuser2",
		UUID:     userUUID,
	}, &database.Namespace{})
	require.NoError(t, err)

	user, err := userStore.FindByUsername(ctx, "testuser2")
	require.NoError(t, err)

	// Create test repository
	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		Name:           "testprompt",
		UserID:         user.ID,
		GitPath:        "prompts_testuser2/testprompt",
		Path:           "testuser2/testprompt",
		RepositoryType: types.PromptRepo,
		Description:    "Test prompt description",
		Private:        false,
	})
	require.NoError(t, err)

	// Create prompt
	prompt, err := promptStore.Create(ctx, database.Prompt{
		RepositoryID: repo.ID,
	})
	require.NoError(t, err)

	t.Run("find existing prompt", func(t *testing.T) {
		found, err := store.FindByID(ctx, prompt.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		require.Equal(t, prompt.ID, found.ID)
		require.Equal(t, repo.ID, found.RepositoryID)
		require.Equal(t, "testuser2/testprompt", found.Path)
		require.Equal(t, "testprompt", found.Name)
		require.Equal(t, "Test prompt description", found.Description)
		require.Equal(t, false, found.Private)
		require.Equal(t, userUUID, found.UserUUID)
	})

	t.Run("find non-existent prompt", func(t *testing.T) {
		_, err := store.FindByID(ctx, 99999)
		require.Error(t, err)
		require.True(t, errors.Is(err, errorx.ErrNotFound))
	})

	t.Run("find prompt with private repository", func(t *testing.T) {
		// Create private repository
		privateRepo, err := repoStore.CreateRepo(ctx, database.Repository{
			Name:           "privateprompt",
			UserID:         user.ID,
			GitPath:        "prompts_testuser2/privateprompt",
			Path:           "testuser2/privateprompt",
			RepositoryType: types.PromptRepo,
			Description:    "Private prompt",
			Private:        true,
		})
		require.NoError(t, err)

		privatePrompt, err := promptStore.Create(ctx, database.Prompt{
			RepositoryID: privateRepo.ID,
		})
		require.NoError(t, err)

		found, err := store.FindByID(ctx, privatePrompt.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		require.Equal(t, true, found.Private)
	})
}

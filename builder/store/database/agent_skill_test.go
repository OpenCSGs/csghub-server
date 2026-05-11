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

func TestAgentSkillStore_ListForAgent_SyncStatusFilter(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentSkillStoreWithDB(db)
	userStore := database.NewUserStoreWithDB(db)
	repoStore := database.NewRepoStoreWithDB(db)
	skillStore := database.NewSkillStoreWithDB(db)

	userUUID := uuid.New().String()
	err := userStore.Create(ctx, &database.User{
		Username: "testuser",
		UUID:     userUUID,
	}, &database.Namespace{})
	require.NoError(t, err)

	user, err := userStore.FindByUsername(ctx, "testuser")
	require.NoError(t, err)

	completedRepo, err := repoStore.CreateRepo(ctx, database.Repository{
		Name:           "completed-skill",
		UserID:         user.ID,
		GitPath:        "skills_testuser/completed-skill",
		Path:           "testuser/completed-skill",
		RepositoryType: types.SkillRepo,
		SyncStatus:     types.SyncStatusCompleted,
		Private:        false,
	})
	require.NoError(t, err)

	emptyStatusRepo, err := repoStore.CreateRepo(ctx, database.Repository{
		Name:           "empty-status-skill",
		UserID:         user.ID,
		GitPath:        "skills_testuser/empty-status-skill",
		Path:           "testuser/empty-status-skill",
		RepositoryType: types.SkillRepo,
		Private:        false,
	})
	require.NoError(t, err)

	pendingRepo, err := repoStore.CreateRepo(ctx, database.Repository{
		Name:           "pending-skill",
		UserID:         user.ID,
		GitPath:        "skills_testuser/pending-skill",
		Path:           "testuser/pending-skill",
		RepositoryType: types.SkillRepo,
		SyncStatus:     types.SyncStatusPending,
		Private:        false,
	})
	require.NoError(t, err)

	_, err = skillStore.Create(ctx, database.Skill{RepositoryID: completedRepo.ID})
	require.NoError(t, err)
	_, err = skillStore.Create(ctx, database.Skill{RepositoryID: emptyStatusRepo.ID})
	require.NoError(t, err)
	_, err = skillStore.Create(ctx, database.Skill{RepositoryID: pendingRepo.ID})
	require.NoError(t, err)

	items, total, err := store.ListForAgent(ctx, userUUID, user.Username, database.AgentSkillFilter{}, 10, 1)
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, items, 2)

	paths := make([]string, 0, len(items))
	for _, item := range items {
		paths = append(paths, item.Path)
	}

	require.Contains(t, paths, "testuser/completed-skill")
	require.Contains(t, paths, "testuser/empty-status-skill")
	require.NotContains(t, paths, "testuser/pending-skill")
}

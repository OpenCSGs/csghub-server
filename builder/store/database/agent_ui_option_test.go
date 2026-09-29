package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAgentUIOptionStore_ListFiltersAndPaginates(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentUIOptionStoreWithDB(db)
	spaceStore := database.NewSpaceStoreWithDB(db)
	deployStore := database.NewDeployTaskStoreWithDB(db)

	tag := requireAgentUITag(t, ctx, db)

	type spaceFixture struct {
		id      int64
		svcName string
	}
	createSpace := func(name string, private, withTag bool, latestStatus int) spaceFixture {
		t.Helper()
		uniqueName := name + "-" + uuid.NewString()[:8]
		repo := &database.Repository{
			UserID: 1, Path: "ui-test/" + uniqueName, GitPath: "ui-test/" + uniqueName,
			Name: uniqueName, Nickname: uniqueName, DefaultBranch: "main", Private: private,
			RepositoryType: types.SpaceRepo,
		}
		insertErr := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
		require.NoError(t, insertErr)
		space, createErr := spaceStore.Create(ctx, database.Space{RepositoryID: repo.ID, Sdk: types.GRADIO.Name})
		require.NoError(t, createErr)
		space, createErr = spaceStore.ByRepoID(ctx, repo.ID)
		require.NoError(t, createErr)
		if withTag {
			_, insertErr = db.Core.NewInsert().Model(&database.RepositoryTag{RepositoryID: repo.ID, TagID: tag.ID, Count: 1}).Exec(ctx)
			require.NoError(t, insertErr)
		}
		deploy := &database.Deploy{DeployName: "deploy-" + uniqueName, SvcName: "svc-" + uniqueName, RepoID: repo.ID, UserID: 1, SpaceID: space.ID, Type: types.SpaceType, Status: common.Running}
		if latestStatus != common.Running {
			older := *deploy
			older.DeployName += "-old"
			older.SvcName += "-old"
			require.NoError(t, deployStore.CreateDeploy(ctx, &older))
			_, updateErr := db.Core.NewUpdate().Table("deploys").Set("created_at = ?", time.Now().Add(-time.Minute)).Where("deploy_name = ?", older.DeployName).Exec(ctx)
			require.NoError(t, updateErr)
			deploy.Status = common.Stopped
		}
		require.NoError(t, deployStore.CreateDeploy(ctx, deploy))
		if latestStatus != common.Running {
			_, updateErr := db.Core.NewUpdate().Table("deploys").Set("created_at = ?", time.Now()).Where("deploy_name = ?", deploy.DeployName).Exec(ctx)
			require.NoError(t, updateErr)
		}
		return spaceFixture{id: space.ID, svcName: deploy.SvcName}
	}

	first := createSpace("first", false, true, common.Running)
	second := createSpace("second", false, true, common.Running)
	createSpace("private", true, true, common.Running)
	createSpace("untagged", false, false, common.Running)
	createSpace("stopped", false, true, common.Stopped)

	options, total, err := store.List(ctx, database.NewRepositoryAccessScope(database.RepositoryAccessReadable, []int64{first.id}), 1, 1)
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, options, 1)
	require.Equal(t, second.id, options[0].SpaceID)
	require.Equal(t, second.svcName, options[0].SvcName)

	options, total, err = store.List(ctx, database.NewRepositoryAccessScope(database.RepositoryAccessReadable, []int64{first.id}), 1, 2)
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, options, 1)
	require.Equal(t, first.id, options[0].SpaceID)
}

func TestAgentUIOptionStore_HasAgentUITag(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewAgentUIOptionStoreWithDB(db)

	tag := requireAgentUITag(t, ctx, db)
	uniqueName := "test-" + uuid.NewString()[:8]
	repo := &database.Repository{UserID: 1, Path: "ui-tag/" + uniqueName, GitPath: "ui-tag/" + uniqueName, Name: uniqueName, Nickname: uniqueName, DefaultBranch: "main", RepositoryType: types.SpaceRepo}
	err := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
	require.NoError(t, err)
	space, err := database.NewSpaceStoreWithDB(db).Create(ctx, database.Space{RepositoryID: repo.ID, Sdk: types.GRADIO.Name})
	require.NoError(t, err)
	space, err = database.NewSpaceStoreWithDB(db).ByRepoID(ctx, repo.ID)
	require.NoError(t, err)
	_, err = db.Core.NewInsert().Model(&database.RepositoryTag{RepositoryID: repo.ID, TagID: tag.ID, Count: 1}).Exec(ctx)
	require.NoError(t, err)

	marked, err := store.HasAgentUITag(ctx, space.ID)
	require.NoError(t, err)
	require.True(t, marked)
	marked, err = store.HasAgentUITag(ctx, space.ID+1000)
	require.NoError(t, err)
	require.False(t, marked)
}

func requireAgentUITag(t *testing.T, ctx context.Context, db *database.DB) *database.Tag {
	t.Helper()
	tag := &database.Tag{Name: "agent-ui", Category: "task", Scope: types.SpaceTagScope, BuiltIn: true, Group: ""}
	err := db.Core.NewSelect().Model(tag).Where("name = ? AND category = ? AND scope = ?", tag.Name, tag.Category, tag.Scope).Scan(ctx)
	if err == nil {
		return tag
	}
	err = db.Core.NewInsert().Model(tag).Scan(ctx, tag)
	require.NoError(t, err)
	return tag
}

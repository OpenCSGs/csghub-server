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
		repoID  int64
		svcName string
	}
	createSpace := func(ns, name string, private, withTag bool, latestStatus int) spaceFixture {
		t.Helper()
		uniqueName := name + "-" + uuid.NewString()[:8]
		repo := &database.Repository{
			UserID: 1, Path: ns + "/" + uniqueName, GitPath: ns + "/" + uniqueName,
			Name: uniqueName, Nickname: uniqueName, DefaultBranch: "main", Private: private,
			RepositoryType: types.SpaceRepo,
		}
		insertErr := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
		require.NoError(t, insertErr)
		_, createErr := spaceStore.Create(ctx, database.Space{RepositoryID: repo.ID, Sdk: types.GRADIO.Name})
		require.NoError(t, createErr)
		space, createErr := spaceStore.ByRepoID(ctx, repo.ID)
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
		return spaceFixture{id: space.ID, repoID: repo.ID, svcName: deploy.SvcName}
	}
	// createSpaceNoDeploy creates a tagged, public space with no deploy rows.
	createSpaceNoDeploy := func(ns, name string) spaceFixture {
		t.Helper()
		uniqueName := name + "-" + uuid.NewString()[:8]
		repo := &database.Repository{
			UserID: 1, Path: ns + "/" + uniqueName, GitPath: ns + "/" + uniqueName,
			Name: uniqueName, Nickname: uniqueName, DefaultBranch: "main",
			RepositoryType: types.SpaceRepo,
		}
		require.NoError(t, db.Core.NewInsert().Model(repo).Scan(ctx, repo))
		_, err := spaceStore.Create(ctx, database.Space{RepositoryID: repo.ID, Sdk: types.GRADIO.Name})
		require.NoError(t, err)
		space, err := spaceStore.ByRepoID(ctx, repo.ID)
		require.NoError(t, err)
		_, err = db.Core.NewInsert().Model(&database.RepositoryTag{RepositoryID: repo.ID, TagID: tag.ID, Count: 1}).Exec(ctx)
		require.NoError(t, err)
		return spaceFixture{id: space.ID, repoID: repo.ID}
	}

	createSpace("ui-test", "first", false, true, common.Running)
	second := createSpace("ui-test", "second", false, true, common.Running)
	private := createSpace("ui-test", "private", true, true, common.Running)
	createSpace("ui-test", "untagged", false, false, common.Running)
	stopped := createSpace("ui-test", "stopped", false, true, common.Stopped)
	noDeploy := createSpaceNoDeploy("ui-test", "nodeploy")

	t.Run("public spaces are listed regardless of read scope or deploy status", func(t *testing.T) {
		// an empty granted-ID set simulates a read-scope cache that has not
		// caught up yet: public spaces must still appear immediately
		options, total, err := store.List(ctx, database.NewRepositoryAccessScope(database.RepositoryAccessReadable, nil), "", 10, 1)
		require.NoError(t, err)
		require.Equal(t, 4, total)
		require.Len(t, options, 4)
		byID := make(map[int64]types.AgentUIOption)
		for _, o := range options {
			byID[o.SpaceID] = o
		}
		// raw deploy status and svc name are returned for the component to aggregate
		require.Equal(t, common.Running, *byID[second.id].DeployStatus)
		require.Equal(t, second.svcName, byID[second.id].SvcName)
		require.Equal(t, common.Stopped, *byID[stopped.id].DeployStatus)
		require.Nil(t, byID[noDeploy.id].DeployStatus)
		require.Empty(t, byID[noDeploy.id].SvcName)
		// without a grant, the private space is not listed
		require.NotContains(t, byID, private.id)
	})

	t.Run("granted private space is listed alongside public ones", func(t *testing.T) {
		options, total, err := store.List(ctx, database.NewRepositoryAccessScope(database.RepositoryAccessReadable, []int64{private.repoID}), "", 10, 1)
		require.NoError(t, err)
		require.Equal(t, 5, total)
		byID := make(map[int64]types.AgentUIOption)
		for _, o := range options {
			byID[o.SpaceID] = o
		}
		require.Contains(t, byID, private.id)
		require.True(t, byID[private.id].Private)
	})

	t.Run("admin scope lists all tagged including private", func(t *testing.T) {
		_, total, err := store.List(ctx, database.NewRepositoryAccessScope(database.RepositoryAccessAdmin, nil), "", 10, 1)
		require.NoError(t, err)
		require.Equal(t, 5, total)
	})

	t.Run("ordering is space id DESC", func(t *testing.T) {
		options, total, err := store.List(ctx, database.NewRepositoryAccessScope(database.RepositoryAccessReadable, nil), "", 1, 1)
		require.NoError(t, err)
		require.Equal(t, 4, total)
		require.Len(t, options, 1)
		require.Equal(t, noDeploy.id, options[0].SpaceID)
	})

	t.Run("namespace filter returns only that namespace's spaces", func(t *testing.T) {
		other1 := createSpace("ui-other", "other1", false, true, common.Running)
		other2 := createSpaceNoDeploy("ui-other", "other2")
		scope := database.NewRepositoryAccessScope(database.RepositoryAccessReadable, nil)

		options, total, err := store.List(ctx, scope, "ui-other", 10, 1)
		require.NoError(t, err)
		require.Equal(t, 2, total)
		require.Len(t, options, 2)
		for _, o := range options {
			require.True(t, o.SpaceID == other1.id || o.SpaceID == other2.id)
			require.Contains(t, o.Path, "ui-other/")
		}

		_, total, err = store.List(ctx, scope, "ui-test", 10, 1)
		require.NoError(t, err)
		require.Equal(t, 4, total)

		// empty namespace returns all namespaces
		_, total, err = store.List(ctx, scope, "", 10, 1)
		require.NoError(t, err)
		require.Equal(t, 6, total)
	})
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
	_, err = database.NewSpaceStoreWithDB(db).Create(ctx, database.Space{RepositoryID: repo.ID, Sdk: types.GRADIO.Name})
	require.NoError(t, err)
	space, err := database.NewSpaceStoreWithDB(db).ByRepoID(ctx, repo.ID)
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

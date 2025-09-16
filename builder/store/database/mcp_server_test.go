package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestMCPServerStore_CURD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMCPServerStoreWithDB(db)

	repo, err := database.NewRepoStoreWithDB(db).CreateRepo(ctx, database.Repository{
		UserID:  int64(1),
		Path:    "foo/bar",
		Private: false,
	})
	require.Nil(t, err)

	server := database.MCPServer{
		RepositoryID:  repo.ID,
		ToolsNum:      1,
		Configuration: "config1",
		Schema:        "schema1",
	}

	resServer, err := store.Create(ctx, server)
	require.Nil(t, err)
	require.Equal(t, server.RepositoryID, resServer.RepositoryID)

	resServer, err = store.ByRepoID(ctx, repo.ID)
	require.Nil(t, err)
	require.Equal(t, server.RepositoryID, resServer.RepositoryID)

	var repoIDs []int64
	repoIDs = append(repoIDs, repo.ID)
	res1, err := store.ByRepoIDs(ctx, repoIDs)
	require.Nil(t, err)
	require.Equal(t, 1, len(res1))

	resServer, err = store.ByPath(ctx, "foo", "bar")
	require.Nil(t, err)
	require.Equal(t, server.RepositoryID, resServer.RepositoryID)

	res, err := store.Update(ctx, database.MCPServer{
		ID:            resServer.ID,
		ToolsNum:      2,
		Configuration: "config2",
		Schema:        "schema2",
	})
	require.Nil(t, err)
	require.Equal(t, 2, res.ToolsNum)
	require.Equal(t, "config2", res.Configuration)

	tool := database.MCPServerProperty{
		ID:          int64(1),
		MCPServerID: resServer.ID,
		Kind:        types.MCPPropTool,
		Name:        "tool1",
		Description: "description1",
		Schema:      "schema1",
	}

	resp, err := store.AddProperty(ctx, tool)
	require.Nil(t, err)
	require.Equal(t, tool.MCPServerID, resp.MCPServerID)
	require.Equal(t, tool.Kind, resp.Kind)
	require.Equal(t, tool.Name, resp.Name)

	req := &types.MCPPropertyFilter{
		Kind:    types.MCPPropTool,
		Per:     10,
		Page:    1,
		IsAdmin: true,
	}

	tools, total, err := store.ListProperties(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, tool.ID, tools[0].ID)
	require.Equal(t, tool.MCPServerID, tools[0].MCPServerID)
	require.Equal(t, tool.Kind, tools[0].Kind)
	require.Equal(t, tool.Name, tools[0].Name)

	err = store.DeleteProperty(ctx, tool)
	require.Nil(t, err)

	_, total, err = store.ListProperties(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 0, total)

	resp, err = store.AddProperty(ctx, tool)
	require.Nil(t, err)
	require.Equal(t, tool.ID, resp.ID)

	err = store.DeletePropertiesByServerID(ctx, resServer.ID)
	require.Nil(t, err)

	_, total, err = store.ListProperties(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 0, total)
}

func TestMCPServerStore_ByUsername(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	us := database.NewUserStoreWithDB(db)
	err := us.Create(ctx, &database.User{Username: "foo"}, &database.Namespace{})
	require.Nil(t, err)

	user, err := us.FindByUsername(ctx, "foo")
	require.Nil(t, err)

	repoStore := database.NewRepoStoreWithDB(db)
	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		UserID:  user.ID,
		Path:    "foo/bar",
		Private: false,
	})
	require.Nil(t, err)

	store := database.NewMCPServerStoreWithDB(db)

	server := database.MCPServer{
		RepositoryID:  repo.ID,
		ToolsNum:      1,
		Configuration: "config1",
		Schema:        "schema1",
	}

	resServer, err := store.Create(ctx, server)
	require.Nil(t, err)
	require.Equal(t, server.RepositoryID, resServer.RepositoryID)

	mcps, total, err := store.ByUsername(ctx, user.Username, 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, resServer.ID, mcps[0].ID)
}

func TestMCPServerStore_ByOrgPath(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	repoStore := database.NewRepoStoreWithDB(db)

	repo, err := repoStore.CreateRepo(ctx, database.Repository{
		Path:    "foo/bar",
		Private: false,
	})
	require.Nil(t, err)

	store := database.NewMCPServerStoreWithDB(db)

	server := database.MCPServer{
		RepositoryID:  repo.ID,
		ToolsNum:      1,
		Configuration: "config1",
		Schema:        "schema1",
	}

	resServer, err := store.Create(ctx, server)
	require.Nil(t, err)
	require.Equal(t, server.RepositoryID, resServer.RepositoryID)

	mcps, total, err := store.ByOrgPath(ctx, "foo", 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, resServer.ID, mcps[0].ID)
}

func TestMCPServerStore_UserLikes(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMCPServerStoreWithDB(db)

	repos := []*database.Repository{
		{Name: "repo1", Path: "p1", GitPath: "p1"},
		{Name: "repo2", Path: "p2", GitPath: "p2"},
		{Name: "repo3", Path: "p3", GitPath: "p3"},
		{Name: "repo4", Path: "p4", GitPath: "p4"},
	}

	for _, repo := range repos {
		err := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
		require.Nil(t, err)

		_, err = store.Create(ctx, database.MCPServer{
			RepositoryID: repo.ID,
		})
		require.Nil(t, err)
	}

	_, err := db.Core.NewInsert().Model(&database.UserLike{
		UserID: 123,
		RepoID: repos[0].ID,
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.UserLike{
		UserID: 123,
		RepoID: repos[2].ID,
	}).Exec(ctx)
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.UserLike{
		UserID:    123,
		RepoID:    repos[3].ID,
		DeletedAt: time.Now(),
	}).Exec(ctx)
	require.Nil(t, err)

	mcps, total, err := store.UserLikes(ctx, 123, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.Equal(t, 2, len(mcps))

	names := []string{}
	for _, mcp := range mcps {
		names = append(names, mcp.Repository.Name)
	}
	require.Equal(t, []string{"repo3", "repo1"}, names)

}

func TestMCPServerStore_CreateAndDeleteSpaceAndRepoForDeploy(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMCPServerStoreWithDB(db)

	repo := &database.Repository{
		Name: "repo1",
		Path: "p1",
	}

	space := &database.Space{
		Sdk: "mcp_server",
	}

	err := store.CreateSpaceAndRepoForDeploy(ctx, repo, space)
	require.Nil(t, err)

	err = store.DeleteSpaceAndRepoForDeploy(ctx, space.ID, repo.ID)
	require.Nil(t, err)
}

package database_test

import (
	"context"
	"testing"

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
		Kind:        string(types.MCPPropTool),
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

package database_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestModelStore_PathCaseInsensitive(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	repo := &database.Repository{Path: "AIWizards/JuliaDoc-MCP", Name: "JuliaDoc-MCP", RepositoryType: types.ModelRepo, Private: true}
	_, err := db.Core.NewInsert().Model(repo).Exec(ctx)
	require.NoError(t, err)
	resource := &database.Model{RepositoryID: repo.ID}
	_, err = db.Core.NewInsert().Model(resource).Exec(ctx)
	require.NoError(t, err)
	store := database.NewModelStoreWithDB(db)

	for _, input := range []struct{ namespace, name string }{
		{"AIWizards", "JuliaDoc-MCP"},
		{"aiwizards", "JuliaDoc-MCP"},
		{"AIWizards", "juliadoc-mcp"},
		{"aiwizards", "juliadoc-mcp"},
		{"AIWIZARDS", "JULIADOC-MCP"},
	} {
		t.Run(input.namespace+"/"+input.name, func(t *testing.T) {
			got, err := store.FindByPath(ctx, input.namespace, input.name)
			require.NoError(t, err)
			require.Equal(t, repo.ID, got.RepositoryID)
			require.Equal(t, repo.Path, got.Repository.Path)
			require.Equal(t, repo.Name, got.Repository.Name)
			require.True(t, got.Repository.Private)
		})
	}
	for _, input := range []struct{ namespace, name string }{
		{"other", "juliadoc-mcp"},
		{"aiwizards", "other"},
		{"aiwizards", "juliadoc%"},
		{"aiwizards", "juliadoc_mc%"},
	} {
		_, err := store.FindByPath(ctx, input.namespace, input.name)
		require.ErrorIs(t, err, errorx.ErrRepoNotFound, "unrelated paths and SQL wildcards must not match")
		require.ErrorIs(t, err, sql.ErrNoRows, "preserve the database cause for existing callers")
	}
}

func TestDatasetStore_PathCaseInsensitive(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	repo := &database.Repository{Path: "AIWizards/JuliaDoc-MCP", Name: "JuliaDoc-MCP", RepositoryType: types.DatasetRepo, Private: true}
	_, err := db.Core.NewInsert().Model(repo).Exec(ctx)
	require.NoError(t, err)
	resource := &database.Dataset{RepositoryID: repo.ID}
	_, err = db.Core.NewInsert().Model(resource).Exec(ctx)
	require.NoError(t, err)
	store := database.NewDatasetStoreWithDB(db)

	for _, input := range []struct{ namespace, name string }{
		{"AIWizards", "JuliaDoc-MCP"},
		{"aiwizards", "JuliaDoc-MCP"},
		{"AIWizards", "juliadoc-mcp"},
		{"aiwizards", "juliadoc-mcp"},
		{"AIWIZARDS", "JULIADOC-MCP"},
	} {
		t.Run(input.namespace+"/"+input.name, func(t *testing.T) {
			got, err := store.FindByPath(ctx, input.namespace, input.name)
			require.NoError(t, err)
			require.Equal(t, repo.ID, got.RepositoryID)
			require.Equal(t, repo.Path, got.Repository.Path)
			require.Equal(t, repo.Name, got.Repository.Name)
			require.True(t, got.Repository.Private)
		})
	}
	for _, input := range []struct{ namespace, name string }{
		{"other", "juliadoc-mcp"},
		{"aiwizards", "other"},
		{"aiwizards", "juliadoc%"},
		{"aiwizards", "juliadoc_mc%"},
	} {
		_, err := store.FindByPath(ctx, input.namespace, input.name)
		require.ErrorIs(t, err, errorx.ErrRepoNotFound, "unrelated paths and SQL wildcards must not match")
		require.ErrorIs(t, err, sql.ErrNoRows, "preserve the database cause for existing callers")
	}
}

func TestSpaceStore_PathCaseInsensitive(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	repo := &database.Repository{Path: "AIWizards/JuliaDoc-MCP", Name: "JuliaDoc-MCP", RepositoryType: types.SpaceRepo, Private: true}
	_, err := db.Core.NewInsert().Model(repo).Exec(ctx)
	require.NoError(t, err)
	resource := &database.Space{RepositoryID: repo.ID}
	_, err = db.Core.NewInsert().Model(resource).Exec(ctx)
	require.NoError(t, err)
	store := database.NewSpaceStoreWithDB(db)

	for _, input := range []struct{ namespace, name string }{
		{"AIWizards", "JuliaDoc-MCP"},
		{"aiwizards", "JuliaDoc-MCP"},
		{"AIWizards", "juliadoc-mcp"},
		{"aiwizards", "juliadoc-mcp"},
		{"AIWIZARDS", "JULIADOC-MCP"},
	} {
		t.Run(input.namespace+"/"+input.name, func(t *testing.T) {
			got, err := store.FindByPath(ctx, input.namespace, input.name)
			require.NoError(t, err)
			require.Equal(t, repo.ID, got.RepositoryID)
			require.Equal(t, repo.Path, got.Repository.Path)
			require.Equal(t, repo.Name, got.Repository.Name)
			require.True(t, got.Repository.Private)
		})
	}
	for _, input := range []struct{ namespace, name string }{
		{"other", "juliadoc-mcp"},
		{"aiwizards", "other"},
		{"aiwizards", "juliadoc%"},
		{"aiwizards", "juliadoc_mc%"},
	} {
		_, err := store.FindByPath(ctx, input.namespace, input.name)
		require.ErrorIs(t, err, errorx.ErrRepoNotFound, "unrelated paths and SQL wildcards must not match")
		require.ErrorIs(t, err, sql.ErrNoRows, "preserve the database cause for existing callers")
	}
}

func TestCodeStore_PathCaseInsensitive(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	repo := &database.Repository{Path: "AIWizards/JuliaDoc-MCP", Name: "JuliaDoc-MCP", RepositoryType: types.CodeRepo, Private: true}
	_, err := db.Core.NewInsert().Model(repo).Exec(ctx)
	require.NoError(t, err)
	resource := &database.Code{RepositoryID: repo.ID}
	_, err = db.Core.NewInsert().Model(resource).Exec(ctx)
	require.NoError(t, err)
	store := database.NewCodeStoreWithDB(db)

	for _, input := range []struct{ namespace, name string }{
		{"AIWizards", "JuliaDoc-MCP"},
		{"aiwizards", "JuliaDoc-MCP"},
		{"AIWizards", "juliadoc-mcp"},
		{"aiwizards", "juliadoc-mcp"},
		{"AIWIZARDS", "JULIADOC-MCP"},
	} {
		t.Run(input.namespace+"/"+input.name, func(t *testing.T) {
			got, err := store.FindByPath(ctx, input.namespace, input.name)
			require.NoError(t, err)
			require.Equal(t, repo.ID, got.RepositoryID)
			require.Equal(t, repo.Path, got.Repository.Path)
			require.Equal(t, repo.Name, got.Repository.Name)
			require.True(t, got.Repository.Private)
		})
	}
	for _, input := range []struct{ namespace, name string }{
		{"other", "juliadoc-mcp"},
		{"aiwizards", "other"},
		{"aiwizards", "juliadoc%"},
		{"aiwizards", "juliadoc_mc%"},
	} {
		_, err := store.FindByPath(ctx, input.namespace, input.name)
		require.ErrorIs(t, err, errorx.ErrRepoNotFound, "unrelated paths and SQL wildcards must not match")
		require.ErrorIs(t, err, sql.ErrNoRows, "preserve the database cause for existing callers")
	}
}

func TestSkillStore_PathCaseInsensitive(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	repo := &database.Repository{Path: "AIWizards/JuliaDoc-MCP", Name: "JuliaDoc-MCP", RepositoryType: types.SkillRepo, Private: true}
	_, err := db.Core.NewInsert().Model(repo).Exec(ctx)
	require.NoError(t, err)
	resource := &database.Skill{RepositoryID: repo.ID}
	_, err = db.Core.NewInsert().Model(resource).Exec(ctx)
	require.NoError(t, err)
	store := database.NewSkillStoreWithDB(db)

	for _, input := range []struct{ namespace, name string }{
		{"AIWizards", "JuliaDoc-MCP"},
		{"aiwizards", "JuliaDoc-MCP"},
		{"AIWizards", "juliadoc-mcp"},
		{"aiwizards", "juliadoc-mcp"},
		{"AIWIZARDS", "JULIADOC-MCP"},
	} {
		t.Run(input.namespace+"/"+input.name, func(t *testing.T) {
			got, err := store.FindByPath(ctx, input.namespace, input.name)
			require.NoError(t, err)
			require.Equal(t, repo.ID, got.RepositoryID)
			require.Equal(t, repo.Path, got.Repository.Path)
			require.Equal(t, repo.Name, got.Repository.Name)
			require.True(t, got.Repository.Private)
		})
	}
	for _, input := range []struct{ namespace, name string }{
		{"other", "juliadoc-mcp"},
		{"aiwizards", "other"},
		{"aiwizards", "juliadoc%"},
		{"aiwizards", "juliadoc_mc%"},
	} {
		_, err := store.FindByPath(ctx, input.namespace, input.name)
		require.ErrorIs(t, err, errorx.ErrRepoNotFound, "unrelated paths and SQL wildcards must not match")
		require.ErrorIs(t, err, sql.ErrNoRows, "preserve the database cause for existing callers")
	}
}

func TestPromptStore_PathCaseInsensitive(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	repo := &database.Repository{Path: "AIWizards/JuliaDoc-MCP", Name: "JuliaDoc-MCP", RepositoryType: types.PromptRepo, Private: true}
	_, err := db.Core.NewInsert().Model(repo).Exec(ctx)
	require.NoError(t, err)
	resource := &database.Prompt{RepositoryID: repo.ID}
	_, err = db.Core.NewInsert().Model(resource).Exec(ctx)
	require.NoError(t, err)
	store := database.NewPromptStoreWithDB(db)

	for _, input := range []struct{ namespace, name string }{
		{"AIWizards", "JuliaDoc-MCP"},
		{"aiwizards", "JuliaDoc-MCP"},
		{"AIWizards", "juliadoc-mcp"},
		{"aiwizards", "juliadoc-mcp"},
		{"AIWIZARDS", "JULIADOC-MCP"},
	} {
		t.Run(input.namespace+"/"+input.name, func(t *testing.T) {
			got, err := store.FindByPath(ctx, input.namespace, input.name)
			require.NoError(t, err)
			require.Equal(t, repo.ID, got.RepositoryID)
			require.Equal(t, repo.Path, got.Repository.Path)
			require.Equal(t, repo.Name, got.Repository.Name)
			require.True(t, got.Repository.Private)
		})
	}
	for _, input := range []struct{ namespace, name string }{
		{"other", "juliadoc-mcp"},
		{"aiwizards", "other"},
		{"aiwizards", "juliadoc%"},
		{"aiwizards", "juliadoc_mc%"},
	} {
		_, err := store.FindByPath(ctx, input.namespace, input.name)
		require.ErrorIs(t, err, errorx.ErrRepoNotFound, "unrelated paths and SQL wildcards must not match")
		require.ErrorIs(t, err, sql.ErrNoRows, "preserve the database cause for existing callers")
	}
}

func TestMCPServerStore_PathCaseInsensitive(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	repo := &database.Repository{Path: "AIWizards/JuliaDoc-MCP", Name: "JuliaDoc-MCP", RepositoryType: types.MCPServerRepo, Private: true}
	_, err := db.Core.NewInsert().Model(repo).Exec(ctx)
	require.NoError(t, err)
	resource := &database.MCPServer{RepositoryID: repo.ID}
	_, err = db.Core.NewInsert().Model(resource).Exec(ctx)
	require.NoError(t, err)
	store := database.NewMCPServerStoreWithDB(db)

	for _, input := range []struct{ namespace, name string }{
		{"AIWizards", "JuliaDoc-MCP"},
		{"aiwizards", "JuliaDoc-MCP"},
		{"AIWizards", "juliadoc-mcp"},
		{"aiwizards", "juliadoc-mcp"},
		{"AIWIZARDS", "JULIADOC-MCP"},
	} {
		t.Run(input.namespace+"/"+input.name, func(t *testing.T) {
			got, err := store.ByPath(ctx, input.namespace, input.name)
			require.NoError(t, err)
			require.Equal(t, repo.ID, got.RepositoryID)
			require.Equal(t, repo.Path, got.Repository.Path)
			require.Equal(t, repo.Name, got.Repository.Name)
			require.True(t, got.Repository.Private)
		})
	}
	for _, input := range []struct{ namespace, name string }{
		{"other", "juliadoc-mcp"},
		{"aiwizards", "other"},
		{"aiwizards", "juliadoc%"},
		{"aiwizards", "juliadoc_mc%"},
	} {
		_, err := store.ByPath(ctx, input.namespace, input.name)
		require.ErrorIs(t, err, errorx.ErrRepoNotFound, "unrelated paths and SQL wildcards must not match")
		require.ErrorIs(t, err, sql.ErrNoRows, "preserve the database cause for existing callers")
	}
}

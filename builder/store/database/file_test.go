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

func TestFileStore_FindByParentPath(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fs := database.NewFileStoreWithDB(db)

	// Create a repository
	rs := database.NewRepoStoreWithDB(db)
	repo, err := rs.CreateRepo(ctx, database.Repository{
		Name: "test-repo",
		Path: "test-path",
	})
	require.Nil(t, err)

	// Insert files with the same parent path
	files := []database.File{
		{Name: "file1", Path: "test-path/file1", ParentPath: "test-path", RepositoryID: repo.ID},
		{Name: "file2", Path: "test-path/file2", ParentPath: "test-path", RepositoryID: repo.ID},
	}
	err = fs.BatchCreate(ctx, files)
	require.Nil(t, err)

	// Query files by parent path
	result, err := fs.FindByParentPath(ctx, repo.ID, "test-path", nil)
	require.Nil(t, err)
	require.Equal(t, 2, len(result))

	names := []string{}
	for _, f := range result {
		names = append(names, f.Name)
	}
	require.ElementsMatch(t, []string{"file1", "file2"}, names)

	result, err = fs.FindByParentPath(ctx, repo.ID, "test-path", &types.OffsetPagination{
		Limit:  1,
		Offset: 1,
	})
	require.Nil(t, err)
	require.Equal(t, 1, len(result))

	names = []string{}
	for _, f := range result {
		names = append(names, f.Name)
	}
	require.ElementsMatch(t, []string{"file2"}, names)
}

func TestFileStore_BatchCreate(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fs := database.NewFileStoreWithDB(db)

	// Create a repository
	rs := database.NewRepoStoreWithDB(db)
	repo, err := rs.CreateRepo(ctx, database.Repository{
		Name: "test-repo",
		Path: "test-path",
	})
	require.Nil(t, err)

	// Insert multiple files
	files := []database.File{
		{Name: "file1", Path: "test-path/file1", ParentPath: "test-path", RepositoryID: repo.ID},
		{Name: "file2", Path: "test-path/file2", ParentPath: "test-path", RepositoryID: repo.ID},
	}
	err = fs.BatchCreate(ctx, files)
	require.Nil(t, err)

	// Validate files are inserted correctly
	result, err := fs.FindByParentPath(ctx, repo.ID, "test-path", nil)
	require.Nil(t, err)
	require.Equal(t, 2, len(result))

	names := []string{}
	for _, f := range result {
		names = append(names, f.Name)
	}
	require.ElementsMatch(t, []string{"file1", "file2"}, names)
}

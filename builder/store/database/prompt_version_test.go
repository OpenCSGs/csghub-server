package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
)

func TestPromptVersionStore(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewPromptVersionStoreWithDB(db)

	first, err := store.Create(ctx, database.PromptVersion{
		PromptID: 1, FilePath: "folder/a.jsonl", Version: "v1", Hash: "commit-1", Changelog: "initial",
	})
	require.NoError(t, err)
	require.NotZero(t, first.ID)

	_, err = store.Create(ctx, database.PromptVersion{
		PromptID: 1, FilePath: "folder/b.jsonl", Version: "v1", Hash: "commit-2",
	})
	require.NoError(t, err, "different files may use the same version name")

	versions, err := store.ByPromptIDAndFilePath(ctx, 1, "folder/a.jsonl")
	require.NoError(t, err)
	require.Len(t, versions, 1)

	version, err := store.ByPromptIDFilePathAndVersion(ctx, 1, "folder/a.jsonl", "v1")
	require.NoError(t, err)
	require.Equal(t, "commit-1", version.Hash)

	_, err = store.UpdateHash(ctx, first.ID, "commit-4")
	require.NoError(t, err)
	version, err = store.ByPromptIDFilePathAndVersion(ctx, 1, "folder/a.jsonl", "v1")
	require.NoError(t, err)
	require.Equal(t, "commit-4", version.Hash)

	// InitTestDB wraps each test in a transaction. Keep the expected constraint
	// violation last because PostgreSQL aborts that transaction after the error.
	_, err = store.Create(ctx, database.PromptVersion{
		PromptID: 1, FilePath: "folder/a.jsonl", Version: "v1", Hash: "commit-3",
	})
	require.True(t, errors.Is(err, errorx.ErrDatabaseDuplicateKey))
}

package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestMetadata_FindByRepoID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMetadataStoreWithDB(db)

	err := store.Upsert(ctx, &database.Metadata{
		ID:              1,
		RepositoryID:    1,
		ModelParams:     3.12,
		ModelType:       "qwen",
		MiniGPUMemoryGB: 6.4,
		TensorType:      "fp16",
	})
	require.Nil(t, err)
	repo := &database.Repository{
		ID:      1,
		Path:    "foo/bar",
		GitPath: "foo/bar2",
		Private: true,
	}
	err = db.Core.NewInsert().Model(repo).Scan(ctx, repo)
	require.Nil(t, err)
	meta, err := store.FindByRepoID(ctx, 1)
	require.Nil(t, err)
	require.Equal(t, repo.ID, meta.RepositoryID)
}

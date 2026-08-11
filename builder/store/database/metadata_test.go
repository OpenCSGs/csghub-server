package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
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

func TestMetadata_UpdatePDRecommendation(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMetadataStoreWithDB(db)

	// Create a repository and metadata row
	repo := &database.Repository{
		ID:      1,
		Path:    "foo/bar",
		GitPath: "foo/bar2",
		Private: true,
	}
	err := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
	require.Nil(t, err)

	err = store.Upsert(ctx, &database.Metadata{
		RepositoryID: 1,
		ModelParams:  235,
		TensorType:   "BF16",
	})
	require.Nil(t, err)

	// Initially PDRecommendation should be nil/empty
	meta, err := store.FindByRepoID(ctx, 1)
	require.Nil(t, err)
	require.True(t, meta.PDRecommendation.IsEmpty())

	// Update PD recommendation
	rec := &types.PDRecommendation{
		ModelName:    "foo/bar",
		TotalParamsB: 235,
		TotalExperts: 128,
		ActiveExperts: 8,
		Precision:    "bf16",
		Prefill: types.PDRoleConfig{
			TP:          8,
			EP:          8,
			DP:          1,
			TotalGPUs:   8,
			Pods:        1,
			TotalVRAMGB: 460,
		},
		Decode: types.PDRoleConfig{
			TP:          8,
			EP:          8,
			DP:          1,
			TotalGPUs:   8,
			Pods:        1,
			TotalVRAMGB: 460,
		},
	}
	err = store.UpdatePDRecommendation(ctx, 1, rec)
	require.Nil(t, err)

	// Verify it was written
	meta, err = store.FindByRepoID(ctx, 1)
	require.Nil(t, err)
	require.False(t, meta.PDRecommendation.IsEmpty())
	require.Equal(t, "foo/bar", meta.PDRecommendation.ModelName)
	require.Equal(t, 235.0, meta.PDRecommendation.TotalParamsB)
	require.Equal(t, 8, meta.PDRecommendation.Prefill.TP)
	require.Equal(t, 8, meta.PDRecommendation.Decode.TP)
	require.Equal(t, 460.0, meta.PDRecommendation.Prefill.TotalVRAMGB)

	// Overwrite with a new recommendation — should replace the old value
	rec2 := &types.PDRecommendation{
		ModelName:    "foo/bar",
		TotalParamsB: 235,
		TotalExperts: 128,
		ActiveExperts: 8,
		Precision:    "bf16",
		Prefill: types.PDRoleConfig{
			TP:          4,
			EP:          4,
			DP:          1,
			TotalGPUs:   4,
			Pods:        1,
			TotalVRAMGB: 460,
		},
		Decode: types.PDRoleConfig{
			TP:          4,
			EP:          4,
			DP:          1,
			TotalGPUs:   4,
			Pods:        1,
			TotalVRAMGB: 460,
		},
	}
	err = store.UpdatePDRecommendation(ctx, 1, rec2)
	require.Nil(t, err)

	// Verify the value was overwritten, not skipped
	meta, err = store.FindByRepoID(ctx, 1)
	require.Nil(t, err)
	require.Equal(t, 4, meta.PDRecommendation.Prefill.TP)
	require.Equal(t, 4, meta.PDRecommendation.Decode.TP)
}

func TestMetadata_UpdateModelArchType(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMetadataStoreWithDB(db)

	repo := &database.Repository{
		ID:      1,
		Path:    "foo/bar",
		GitPath: "foo/bar2",
		Private: true,
	}
	err := db.Core.NewInsert().Model(repo).Scan(ctx, repo)
	require.Nil(t, err)

	err = store.Upsert(ctx, &database.Metadata{
		RepositoryID: 1,
		ModelParams:  235,
	})
	require.Nil(t, err)

	// Update arch type to MoE
	err = store.UpdateModelArchType(ctx, 1, types.ModelArchTypeMoE)
	require.Nil(t, err)

	meta, err := store.FindByRepoID(ctx, 1)
	require.Nil(t, err)
	require.Equal(t, types.ModelArchTypeMoE, meta.ModelArchType)

	// Overwrite to dense
	err = store.UpdateModelArchType(ctx, 1, types.ModelArchTypeDense)
	require.Nil(t, err)

	meta, err = store.FindByRepoID(ctx, 1)
	require.Nil(t, err)
	require.Equal(t, types.ModelArchTypeDense, meta.ModelArchType)
}

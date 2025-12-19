package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestSpaceResourceStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSpaceResourceStoreWithDB(db)

	_, err := store.Create(ctx, database.SpaceResource{
		Name:      "r1",
		ClusterID: "c1",
	})
	require.Nil(t, err)
	sr := &database.SpaceResource{}
	err = db.Core.NewSelect().Model(sr).Where("name=?", "r1").Scan(ctx, sr)
	require.Nil(t, err)
	require.Equal(t, "c1", sr.ClusterID)

	sr, err = store.FindByID(ctx, sr.ID)
	require.Nil(t, err)
	require.Equal(t, "c1", sr.ClusterID)

	sr, err = store.FindByName(ctx, "r1")
	require.Nil(t, err)
	require.Equal(t, "c1", sr.ClusterID)

	srs, err := store.FindAll(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(srs))
	require.Equal(t, "c1", srs[0].ClusterID)

	srs, _, err = store.Index(ctx, "c1", 50, 1)
	require.Nil(t, err)
	require.Equal(t, 1, len(srs))
	require.Equal(t, "c1", srs[0].ClusterID)

	sr.Name = "r2"
	_, err = store.Update(ctx, *sr)
	require.Nil(t, err)
	sr, err = store.FindByID(ctx, sr.ID)
	require.Nil(t, err)
	require.Equal(t, "r2", sr.Name)

	err = store.Delete(ctx, *sr)
	require.Nil(t, err)
	_, err = store.FindByID(ctx, sr.ID)
	require.NotNil(t, err)

}

func TestSpaceResourceStore_FindAllResourceTypes(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSpaceResourceStoreWithDB(db)

	// Create test resources with different hardware types
	resources := []database.SpaceResource{
		{
			Name:      "r1",
			ClusterID: "c1",
			Resources: `{"gpu":{"type":"NVIDIA A100"},"cpu":{"type":"Intel Xeon"}}`,
		},
		{
			Name:      "r2",
			ClusterID: "c1",
			Resources: `{"npu":{"type":"Ascend 910"},"cpu":{"type":"Intel Xeon"}}`, // Duplicate CPU type
		},
		{
			Name:      "r3",
			ClusterID: "c1",
			Resources: `{"gcu":{"type":"Enflame G100"}}`,
		},
	}

	for _, r := range resources {
		_, err := store.Create(ctx, r)
		require.Nil(t, err)
	}

	// Test FindAllResourceTypes method
	types, err := store.FindAllResourceTypes(ctx, "c1")
	require.Nil(t, err)

	// Expected unique types: NVIDIA A100, Intel Xeon, Ascend 910, Enflame G100
	expectedTypes := map[string]bool{
		"NVIDIA A100":  true,
		"Intel Xeon":   true,
		"Ascend 910":   true,
		"Enflame G100": true,
	}

	require.Equal(t, len(expectedTypes), len(types))

	// Verify all expected types are present
	typeMap := make(map[string]bool)
	for _, t := range types {
		typeMap[t] = true
	}

	for expected := range expectedTypes {
		require.True(t, typeMap[expected], "Expected type %s not found", expected)
	}
}

func TestSpaceResourceStore_FindAllResourceTypes_Empty(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSpaceResourceStoreWithDB(db)

	// Test when there are no resources
	types, err := store.FindAllResourceTypes(ctx, "c1")
	require.Nil(t, err)
	require.Empty(t, types)
}

func TestSpaceResourceStore_FindAllResourceTypes_InvalidJSON(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSpaceResourceStoreWithDB(db)

	// Create a resource with invalid JSON
	_, err := store.Create(ctx, database.SpaceResource{
		Name:      "invalid",
		ClusterID: "c1",
		Resources: `{invalid json}`,
	})
	require.Nil(t, err)

	// The method should ignore invalid JSON and return empty list
	types, err := store.FindAllResourceTypes(ctx, "c1")
	require.Nil(t, err)
	require.Empty(t, types)
}

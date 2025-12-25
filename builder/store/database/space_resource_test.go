package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
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

	srs, _, err = store.Index(ctx, types.SpaceResourceFilter{ClusterID: "c1"}, 50, 1)
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

func TestSpaceResourceStore_FindByGPU(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSpaceResourceStoreWithDB(db)
	// Create test resources with different GPU types
	_, err := store.Create(ctx, database.SpaceResource{
		Name:      "r1",
		ClusterID: "c1",
		Resources: `{"gpu": {"type": "A10", "num": "1", "resource_name": "nvidia.com/gpu", "labels": {"aliyun.accelerator/nvidia_name": "NVIDIA-A10"}}, "cpu": {"type": "Intel", "num": "2"}, "memory": "20Gi","replicas":2}`,
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.SpaceResource{
		Name:      "r2",
		ClusterID: "c1",
		Resources: `{"gpu": {"type": "A100", "num": "1", "resource_name": "nvidia.com/gpu", "labels": {"aliyun.accelerator/nvidia_name": "NVIDIA-A100"}}, "cpu": {"type": "Intel", "num": "4"}, "memory": "40Gi","replicas":1}`,
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.SpaceResource{
		Name:      "r3",
		ClusterID: "c1",
		Resources: `{"cpu": {"type": "Intel", "num": "8"}, "memory": "60Gi","replicas":1}`, // No GPU
	})
	require.Nil(t, err)

	// Test FindByHardwareType with A10
	results, err := store.FindByHardwareType(ctx, "A10")
	require.Nil(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, "r1", results[0].Name)

	// Test FindByHardwareType with A100
	results, err = store.FindByHardwareType(ctx, "A100")
	require.Nil(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, "r2", results[0].Name)

	// Test FindByHardwareType with non-existent GPU type
	results, err = store.FindByHardwareType(ctx, "H100")
	require.Nil(t, err)
	require.Equal(t, 0, len(results))

	// Test FindByHardwareType with different hardware types
	results, err = store.FindByHardwareType(ctx, "A10")
	require.Nil(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, "r1", results[0].Name)

	// Add test resources with other hardware types
	_, err = store.Create(ctx, database.SpaceResource{
		Name:      "r4",
		ClusterID: "c1",
		Resources: `{"npu": {"type": "Ascend910", "num": "1", "resource_name": "ascend.com/npu", "labels": {"vendor": "Huawei"}}, "cpu": {"type": "Huawei", "num": "8"}, "memory": "64Gi","replicas":1}`,
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.SpaceResource{
		Name:      "r5",
		ClusterID: "c1",
		Resources: `{"gcu": {"type": "G100", "num": "2", "resource_name": "enflame.com/gcu", "labels": {"vendor": "Enflame"}}, "cpu": {"type": "Intel", "num": "16"}, "memory": "128Gi","replicas":1}`,
	})
	require.Nil(t, err)

	// Test FindByHardwareType with NPU
	results, err = store.FindByHardwareType(ctx, "Ascend910")
	require.Nil(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, "r4", results[0].Name)

	// Test FindByHardwareType with GCU
	results, err = store.FindByHardwareType(ctx, "G100")
	require.Nil(t, err)
	require.Equal(t, 1, len(results))
	require.Equal(t, "r5", results[0].Name)

	// Test FindByHardwareType with non-existent hardware type
	results, err = store.FindByHardwareType(ctx, "MLU270")
	require.Nil(t, err)
	require.Equal(t, 0, len(results))
}

func TestSpaceResourceStore_Filter(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewSpaceResourceStoreWithDB(db)

	_, err := store.Create(ctx, database.SpaceResource{
		Name:      "r1",
		ClusterID: "c1",
		Resources: `{"cpu": { "type": "Intel","num": "200m"}}`,
	})
	require.Nil(t, err)
	sr := &database.SpaceResource{}
	err = db.Core.NewSelect().Model(sr).Where("name=?", "r1").Scan(ctx, sr)
	require.Nil(t, err)
	require.Equal(t, "c1", sr.ClusterID)

	srs, total, err := store.Index(ctx, types.SpaceResourceFilter{
		ResourceType: types.ResourceTypeCPU,
	}, 50, 1)
	require.Nil(t, err)
	require.Equal(t, 1, len(srs))
	require.Equal(t, 1, total)
	require.Equal(t, "c1", srs[0].ClusterID)
}

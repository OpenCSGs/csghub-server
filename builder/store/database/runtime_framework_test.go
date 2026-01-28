package database_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestRuntimeFrameworksStore_AddAndUpdateAndList(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	rfStore := database.NewRuntimeFrameworksStoreWithDB(db)

	_, err := rfStore.Add(ctx, database.RuntimeFramework{
		ID:        1,
		FrameName: "vllm",
		Type:      1,
	})
	require.Nil(t, err)

	rf, err := rfStore.Update(ctx, database.RuntimeFramework{
		ID:        1,
		FrameName: "vllm",
		Type:      2,
	})
	require.Nil(t, err)
	require.Equal(t, 2, rf.Type)

	res, err := rfStore.List(ctx, 2)
	require.Nil(t, err)
	require.Equal(t, 1, len(res))
}

func TestRuntimeFrameworksStore_FindByIDAndDelete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	rfStore := database.NewRuntimeFrameworksStoreWithDB(db)

	_, err := rfStore.Add(ctx, database.RuntimeFramework{
		ID:        1,
		FrameName: "vllm",
		Type:      1,
	})
	require.Nil(t, err)

	res, err := rfStore.FindByID(ctx, 1)
	require.Nil(t, err)
	require.Equal(t, "vllm", res.FrameName)

	err = rfStore.Delete(ctx, database.RuntimeFramework{
		ID:        1,
		FrameName: "vllm",
		Type:      1,
	})
	require.Nil(t, err)

	_, err = rfStore.FindByID(ctx, 1)
	require.Equal(t, sql.ErrNoRows, err)
}

func TestRuntimeFrameworksStore_FindEnabledByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	rfStore := database.NewRuntimeFrameworksStoreWithDB(db)
	_, err := rfStore.Add(ctx, database.RuntimeFramework{
		ID:        1,
		FrameName: "vllm",
		Type:      1,
		Enabled:   1,
	})
	require.Nil(t, err)

	res, err := rfStore.FindEnabledByID(ctx, 1)
	require.Nil(t, err)
	require.Equal(t, "vllm", res.FrameName)
}

func TestRuntimeFrameworksStore_FindEnabledByName(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	rfStore := database.NewRuntimeFrameworksStoreWithDB(db)
	_, err := rfStore.Add(ctx, database.RuntimeFramework{
		ID:        1,
		FrameName: "vllm",
		Type:      1,
		Enabled:   1,
	})
	require.Nil(t, err)

	res, err := rfStore.FindEnabledByName(ctx, "vllm")
	require.Nil(t, err)
	require.Equal(t, "vllm", res.FrameName)
}

func TestRuntimeFrameworksStore_ListAll(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	rfStore := database.NewRuntimeFrameworksStoreWithDB(db)
	_, err := rfStore.Add(ctx, database.RuntimeFramework{
		ID:        1,
		FrameName: "vllm",
		Type:      1,
		Enabled:   1,
	})
	require.Nil(t, err)

	res, err := rfStore.ListAll(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(res))
	require.Equal(t, "vllm", res[0].FrameName)
}

func TestRuntimeFrameworksStore_ListByIDs(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	rfStore := database.NewRuntimeFrameworksStoreWithDB(db)
	_, err := rfStore.Add(ctx, database.RuntimeFramework{
		ID:        1,
		FrameName: "vllm",
		Type:      1,
		Enabled:   1,
	})
	require.Nil(t, err)

	res, err := rfStore.ListByIDs(ctx, []int64{1})
	require.Nil(t, err)
	require.Equal(t, 1, len(res))
	require.Equal(t, "vllm", res[0].FrameName)
}

func TestRuntimeFrameworksStore_FindByNameAndComputeType(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	rfStore := database.NewRuntimeFrameworksStoreWithDB(db)
	_, err := rfStore.Add(ctx, database.RuntimeFramework{
		ID:            1,
		FrameName:     "vllm",
		FrameImage:    "vllm:v1.0",
		Type:          1,
		Enabled:       1,
		DriverVersion: "12.1",
		ComputeType:   "gpu",
	})
	require.Nil(t, err)

	rf, err := rfStore.FindByFrameImageAndComputeType(ctx, "vllm:v1.0", "gpu")
	require.Nil(t, err)
	require.Equal(t, "vllm", rf.FrameName)
}

func TestRuntimeFrameworksStore_RemoveRuntimeFrameworkAndArch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	rfStore := database.NewRuntimeFrameworksStoreWithDB(db)
	raStore := database.NewRuntimeArchitecturesStoreWithDB(db)

	// Add a runtime framework
	rf, err := rfStore.Add(ctx, database.RuntimeFramework{
		ID:           1,
		FrameName:    "test-framework",
		Type:         1,
		Enabled:      1,
		FrameVersion: "1.0.0",
	})
	require.Nil(t, err)

	// Add runtime architectures associated with the framework
	err = raStore.Add(ctx, database.RuntimeArchitecture{
		RuntimeFrameworkID: rf.ID,
		ArchitectureName:   "x86_64",
		ModelName:          "test-model-1",
	})
	require.Nil(t, err)

	err = raStore.Add(ctx, database.RuntimeArchitecture{
		RuntimeFrameworkID: rf.ID,
		ArchitectureName:   "arm64",
		ModelName:          "test-model-2",
	})
	require.Nil(t, err)

	// Verify that the runtime framework and architectures exist
	foundRf, err := rfStore.FindByID(ctx, rf.ID)
	require.Nil(t, err)
	require.Equal(t, "test-framework", foundRf.FrameName)

	archs, err := raStore.ListByRuntimeFrameworkID(ctx, rf.ID)
	require.Nil(t, err)
	require.Equal(t, 2, len(archs))

	// Remove the runtime framework and its architectures
	err = rfStore.RemoveRuntimeFrameworkAndArch(ctx, 1)
	require.Nil(t, err)

	// Verify that the runtime framework is deleted
	_, err = rfStore.FindByID(ctx, rf.ID)
	require.Equal(t, sql.ErrNoRows, err)

	// Verify that the associated architectures are deleted
	archs, err = raStore.ListByRuntimeFrameworkID(ctx, rf.ID)
	require.Nil(t, err)
	require.Equal(t, 0, len(archs))
}

func TestRuntimeFrameworksStore_FindSpaceSupportedCUDAVersionsz(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	rfStore := database.NewRuntimeFrameworksStoreWithDB(db)

	// Add test data with different frame names and driver versions
	testCases := []database.RuntimeFramework{
		// case 1: driver version is empty
		{
			FrameName:    "space",
			FrameImage:   "space_runtime:1.10.1",
			Enabled:      1,
			ComputeType:  "gpu",
			FrameVersion: "1.10.1",
		},

		{
			FrameName:    "space",
			FrameImage:   "space_runtime:1.2.1",
			Enabled:      1,
			ComputeType:  "gpu",
			FrameVersion: "1.2.1",
		},

		// case 2: driver version is not empty
		{
			FrameName:     "space",
			FrameImage:    "space_runtime_gpu:1.10.1",
			Enabled:       1,
			ComputeType:   "gpu",
			FrameVersion:  "1.10.1",
			DriverVersion: "12.1",
		},

		{
			FrameName:     "space",
			FrameImage:    "space_runtime_gpu:1.2.1",
			Enabled:       1,
			ComputeType:   "gpu",
			FrameVersion:  "1.2.1",
			DriverVersion: "12.1",
		},
	}

	for _, tc := range testCases {
		_, err := rfStore.Add(ctx, tc)
		require.Nil(t, err)
	}
	t.Run("FindSpaceLatestVersion_EmptyDriverVersion", func(t *testing.T) {
		// Test 1: Find latest version for space "space" with driver version "12.1"
		result, err := rfStore.FindSpaceLatestVersion(ctx, "space", "")
		require.Nil(t, err)
		require.Equal(t, "1.10.1", result.FrameVersion) // Should find 2 records (IDs 1 and 4)

	})

	t.Run("FindSpaceLatestVersion", func(t *testing.T) {
		// Test 2: Find latest version for space "space" with empty driver version
		result, err := rfStore.FindSpaceLatestVersion(ctx, "space", "12.1")
		require.Nil(t, err)
		require.Equal(t, "1.10.1", result.FrameVersion)
	})
}

func TestRuntimeFrameworksStore_FindByFrameNameAndDriverVersion(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	rfStore := database.NewRuntimeFrameworksStoreWithDB(db)

	// Add test data
	testCases := []database.RuntimeFramework{
		// Test case 1: Multiple records with same name and version, different driver versions
		{
			FrameName:     "test-framework",
			FrameVersion:  "1.0.0",
			FrameImage:    "test-framework:1.0.0-cpu",
			ComputeType:   "cpu",
			DriverVersion: "",
			Enabled:       1,
			Type:          0,
			ContainerPort: 8080,
		},
		{
			FrameName:     "test-framework",
			FrameVersion:  "1.0.0",
			FrameImage:    "test-framework:1.0.0-gpu-11.8",
			ComputeType:   "gpu",
			DriverVersion: "11.8",
			Enabled:       1,
			Type:          0,
			ContainerPort: 8080,
		},
		{
			FrameName:     "test-framework",
			FrameVersion:  "1.0.0",
			FrameImage:    "test-framework:1.0.0-gpu-12.1",
			ComputeType:   "gpu",
			DriverVersion: "12.1",
			Enabled:       1,
			Type:          0,
			ContainerPort: 8080,
		},
		// Test case 2: Different versions of the same framework
		{
			FrameName:     "test-framework",
			FrameVersion:  "2.0.0",
			FrameImage:    "test-framework:2.0.0-gpu-12.1",
			ComputeType:   "gpu",
			DriverVersion: "12.1",
			Enabled:       1,
			Type:          0,
			ContainerPort: 8080,
		},
		// Test case 3: Different framework
		{
			FrameName:     "other-framework",
			FrameVersion:  "1.0.0",
			FrameImage:    "other-framework:1.0.0-gpu-12.1",
			ComputeType:   "gpu",
			DriverVersion: "12.1",
			Enabled:       1,
			Type:          0,
			ContainerPort: 8080,
		},
	}

	for _, tc := range testCases {
		_, err := rfStore.Add(ctx, tc)
		require.Nil(t, err)
	}

	// Test 1: Find existing record with specific driver version
	t.Run("FindExistingWithDriverVersion", func(t *testing.T) {
		result, err := rfStore.FindByFrameNameAndDriverVersion(ctx, "test-framework", "1.0.0", "12.1")
		require.Nil(t, err)
		require.NotNil(t, result)
		require.Equal(t, "test-framework", result.FrameName)
		require.Equal(t, "1.0.0", result.FrameVersion)
		require.Equal(t, "12.1", result.DriverVersion)
		require.Equal(t, "test-framework:1.0.0-gpu-12.1", result.FrameImage)
	})

	// Test 2: Find existing record with empty driver version
	t.Run("FindExistingWithEmptyDriverVersion", func(t *testing.T) {
		result, err := rfStore.FindByFrameNameAndDriverVersion(ctx, "test-framework", "1.0.0", "")
		require.Nil(t, err)
		require.NotNil(t, result)
		require.Equal(t, "test-framework", result.FrameName)
		require.Equal(t, "1.0.0", result.FrameVersion)
		require.Equal(t, "", result.DriverVersion)
		require.Equal(t, "test-framework:1.0.0-cpu", result.FrameImage)
	})

	// Test 3: Find non-existing record (wrong version)
	t.Run("FindNonExistingVersion", func(t *testing.T) {
		result, err := rfStore.FindByFrameNameAndDriverVersion(ctx, "test-framework", "3.0.0", "12.1")
		require.Nil(t, err)
		require.Nil(t, result)
	})

	// Test 4: Find non-existing record (wrong driver version)
	t.Run("FindNonExistingDriverVersion", func(t *testing.T) {
		result, err := rfStore.FindByFrameNameAndDriverVersion(ctx, "test-framework", "1.0.0", "13.0")
		require.Nil(t, err)
		require.Nil(t, result)
	})

	// Test 5: Find non-existing record (wrong framework name)
	t.Run("FindNonExistingFramework", func(t *testing.T) {
		result, err := rfStore.FindByFrameNameAndDriverVersion(ctx, "non-existing", "1.0.0", "12.1")
		require.Nil(t, err)
		require.Nil(t, result)
	})

	// Test 6: Find existing record with different version
	t.Run("FindExistingDifferentVersion", func(t *testing.T) {
		result, err := rfStore.FindByFrameNameAndDriverVersion(ctx, "test-framework", "2.0.0", "12.1")
		require.Nil(t, err)
		require.NotNil(t, result)
		require.Equal(t, "2.0.0", result.FrameVersion)
		require.Equal(t, "test-framework:2.0.0-gpu-12.1", result.FrameImage)
	})

	// Test 7: Find existing record with different framework name
	t.Run("FindExistingDifferentFramework", func(t *testing.T) {
		result, err := rfStore.FindByFrameNameAndDriverVersion(ctx, "other-framework", "1.0.0", "12.1")
		require.Nil(t, err)
		require.NotNil(t, result)
		require.Equal(t, "other-framework", result.FrameName)
		require.Equal(t, "other-framework:1.0.0-gpu-12.1", result.FrameImage)
	})
}

func TestRuntimeFrameworksStore_FindSpaceSupportedCUDAVersions(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	rfStore := database.NewRuntimeFrameworksStoreWithDB(db)

	// Add test data
	testCases := []database.RuntimeFramework{
		// CPU versions
		{
			FrameName:     "space",
			FrameVersion:  "1.0.0",
			FrameImage:    "space:1.0.0-cpu",
			ComputeType:   "cpu",
			DriverVersion: "",
			Enabled:       1,
			Type:          0,
			ContainerPort: 8080,
		},
		// GPU versions with different CUDA versions
		{
			FrameName:     "space",
			FrameVersion:  "1.0.0",
			FrameImage:    "space:1.0.0-gpu-11.8",
			ComputeType:   "gpu",
			DriverVersion: "11.8",
			Enabled:       1,
			Type:          0,
			ContainerPort: 8080,
		},
		{
			FrameName:     "space",
			FrameVersion:  "1.0.0",
			FrameImage:    "space:1.0.0-gpu-12.1",
			ComputeType:   "gpu",
			DriverVersion: "12.1",
			Enabled:       1,
			Type:          0,
			ContainerPort: 8080,
		},
		// Newer version
		{
			FrameName:     "space",
			FrameVersion:  "2.0.0",
			FrameImage:    "space:2.0.0-cpu",
			ComputeType:   "cpu",
			DriverVersion: "",
			Enabled:       1,
			Type:          0,
			ContainerPort: 8080,
		},
		{
			FrameName:     "space",
			FrameVersion:  "2.0.0",
			FrameImage:    "space:2.0.0-gpu-11.8",
			ComputeType:   "gpu",
			DriverVersion: "11.8",
			Enabled:       1,
			Type:          0,
			ContainerPort: 8080,
		},
		{
			FrameName:     "space",
			FrameVersion:  "2.0.0",
			FrameImage:    "space:2.0.0-gpu-12.1",
			ComputeType:   "gpu",
			DriverVersion: "12.1",
			Enabled:       1,
			Type:          0,
			ContainerPort: 8080,
		},
		{
			FrameName:     "space",
			FrameVersion:  "2.0.0",
			FrameImage:    "space:2.0.0-gpu-12.2",
			ComputeType:   "gpu",
			DriverVersion: "12.2",
			Enabled:       1,
			Type:          0,
			ContainerPort: 8080,
		},
	}

	for _, tc := range testCases {
		_, err := rfStore.Add(ctx, tc)
		require.Nil(t, err)
	}

	// Test 1: Find CUDA versions for GPU compute type
	t.Run("FindForGPUComputeType", func(t *testing.T) {
		result, err := rfStore.FindSpaceSupportedCUDAVersions(ctx, "gpu")
		require.Nil(t, err)
		require.NotNil(t, result)
		// Should return all GPU versions for the latest framework version (2.0.0)
		require.Equal(t, 3, len(result))
		// Verify all have the latest framework version
		for _, frame := range result {
			require.Equal(t, "space", frame.FrameName)
			require.Equal(t, "2.0.0", frame.FrameVersion)
			require.Equal(t, "gpu", frame.ComputeType)
		}
		// Verify all CUDA versions are present
		cudaVersions := make(map[string]bool)
		for _, frame := range result {
			cudaVersions[frame.DriverVersion] = true
		}
		require.Contains(t, cudaVersions, "11.8")
		require.Contains(t, cudaVersions, "12.1")
		require.Contains(t, cudaVersions, "12.2")
	})

	// Test 2: Find CUDA versions for CPU compute type
	t.Run("FindForCPUComputeType", func(t *testing.T) {
		result, err := rfStore.FindSpaceSupportedCUDAVersions(ctx, "cpu")
		require.Nil(t, err)
		require.NotNil(t, result)
		// Should return CPU version for the latest framework version (2.0.0)
		require.Equal(t, 1, len(result))
		require.Equal(t, "space", result[0].FrameName)
		require.Equal(t, "2.0.0", result[0].FrameVersion)
		require.Equal(t, "cpu", result[0].ComputeType)
		require.Equal(t, "", result[0].DriverVersion)
	})

	// Test 3: Find CUDA versions for non-existent compute type
	t.Run("FindForNonExistentComputeType", func(t *testing.T) {
		result, err := rfStore.FindSpaceSupportedCUDAVersions(ctx, "npu")
		require.Nil(t, err)
		require.NotNil(t, result)
		// Should return empty slice for non-existent compute type
		require.Equal(t, 0, len(result))
	})

	// Test 4: Find CUDA versions when no data exists
	t.Run("FindWhenNoDataExists", func(t *testing.T) {
		// Create a new clean DB for this test
		cleanDB := tests.InitTestDB()
		defer cleanDB.Close()
		cleanRfStore := database.NewRuntimeFrameworksStoreWithDB(cleanDB)

		result, err := cleanRfStore.FindSpaceSupportedCUDAVersions(ctx, "gpu")
		require.Nil(t, err)
		require.NotNil(t, result)
		// Should return empty slice when no data exists
		require.Equal(t, 0, len(result))
	})
}

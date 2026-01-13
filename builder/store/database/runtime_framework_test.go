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

func TestRuntimeFrameworksStore_FindByFrameNameAndDriverVersion(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	rfStore := database.NewRuntimeFrameworksStoreWithDB(db)

	// Add test data with different frame names and driver versions
	testCases := []database.RuntimeFramework{
		{
			ID:            1,
			FrameName:     "vllm",
			FrameImage:    "vllm:v1.0",
			Type:          1,
			Enabled:       1,
			DriverVersion: "12.1",
			ComputeType:   "gpu",
			FrameVersion:  "1.0.4",
		},
		{
			ID:            2,
			FrameName:     "vllm",
			FrameImage:    "vllm:v1.1",
			Type:          1,
			Enabled:       1,
			DriverVersion: "11.8",
			ComputeType:   "gpu",
			FrameVersion:  "1.0.4",
		},
		{
			ID:            3,
			FrameName:     "trtllm",
			FrameImage:    "trtllm:v1.0",
			Type:          1,
			Enabled:       1,
			DriverVersion: "12.1",
			ComputeType:   "gpu",
			FrameVersion:  "1.0.4",
		},
		{
			ID:            4,
			FrameName:     "vllm",
			FrameImage:    "vllm:v1.2",
			Type:          1,
			Enabled:       1,
			DriverVersion: "12.1",
			ComputeType:   "gpu",
			FrameVersion:  "1.0.4",
		},
	}

	for _, tc := range testCases {
		_, err := rfStore.Add(ctx, tc)
		require.Nil(t, err)
	}

	// Test 1: Find by frame name "vllm" and driver version "12.1"
	result, err := rfStore.FindByFrameNameAndDriverVersion(ctx, "vllm", "1.0.4", "12.1")
	require.Nil(t, err)
	require.Equal(t, 2, len(result)) // Should find 2 records (IDs 1 and 4)

	// Verify the found records
	frameNames := make(map[string]int)
	driverVersions := make(map[string]int)
	for _, rf := range result {
		frameNames[rf.FrameName]++
		driverVersions[rf.DriverVersion]++
		require.Equal(t, "vllm", rf.FrameName)
		require.Equal(t, "12.1", rf.DriverVersion)
	}
	require.Equal(t, 2, frameNames["vllm"])
	require.Equal(t, 2, driverVersions["12.1"])

	// Test 2: Find by frame name "vllm" and driver version "11.8"
	result, err = rfStore.FindByFrameNameAndDriverVersion(ctx, "vllm", "1.0.4", "11.8")
	require.Nil(t, err)
	require.Equal(t, 1, len(result)) // Should find 1 record (ID 2)
	require.Equal(t, "vllm", result[0].FrameName)
	require.Equal(t, "11.8", result[0].DriverVersion)

	// Test 3: Find by frame name "trtllm" and driver version "12.1"
	result, err = rfStore.FindByFrameNameAndDriverVersion(ctx, "trtllm", "1.0.4", "12.1")
	require.Nil(t, err)
	require.Equal(t, 1, len(result)) // Should find 1 record (ID 3)
	require.Equal(t, "trtllm", result[0].FrameName)
	require.Equal(t, "12.1", result[0].DriverVersion)

	// Test 4: Find by non-existent frame name and driver version combination
	result, err = rfStore.FindByFrameNameAndDriverVersion(ctx, "non-existent-frame", "1.0.4", "12.1")
	require.Nil(t, err)
	require.Equal(t, 0, len(result)) // Should find 0 records

	// Test 5: Find by non-existent driver version
	result, err = rfStore.FindByFrameNameAndDriverVersion(ctx, "vllm", "1.0.4", "non-existent-version")
	require.Nil(t, err)
	require.Equal(t, 0, len(result)) // Should find 0 records
}

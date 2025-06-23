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
		Type:          1,
		Enabled:       1,
		DriverVersion: "12.1",
		ComputeType:   "gpu",
	})
	require.Nil(t, err)

	rf, err := rfStore.FindByNameAndComputeType(ctx, "vllm", "12.1", "gpu")
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

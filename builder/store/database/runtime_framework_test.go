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

	err := rfStore.Add(ctx, database.RuntimeFramework{
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

	err := rfStore.Add(ctx, database.RuntimeFramework{
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
	err := rfStore.Add(ctx, database.RuntimeFramework{
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
	err := rfStore.Add(ctx, database.RuntimeFramework{
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
	err := rfStore.Add(ctx, database.RuntimeFramework{
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
	err := rfStore.Add(ctx, database.RuntimeFramework{
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

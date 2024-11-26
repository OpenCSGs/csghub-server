package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestRuntimeArchitecturesStore_AddAndListByRuntimeFrameworkID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	raStore := database.NewRuntimeArchitecturesStoreWithDB(db)

	err := raStore.Add(ctx, database.RuntimeArchitecture{
		RuntimeFrameworkID: 1,
		ArchitectureName:   "Qwen2ForCausalLM",
	})
	require.Nil(t, err)

	res, err := raStore.ListByRuntimeFrameworkID(ctx, 1)
	require.Nil(t, err)
	require.Equal(t, 1, len(res))
	require.Equal(t, "Qwen2ForCausalLM", res[0].ArchitectureName)
}

func TestRuntimeArchitecturesStore_DeleteByRuntimeIDAndArchName(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	raStore := database.NewRuntimeArchitecturesStoreWithDB(db)
	err := raStore.Add(ctx, database.RuntimeArchitecture{
		RuntimeFrameworkID: 1,
		ArchitectureName:   "Qwen2ForCausalLM",
	})
	require.Nil(t, err)

	err = raStore.DeleteByRuntimeIDAndArchName(ctx, 1, "Qwen2ForCausalLM")
	require.Nil(t, err)

	arch, err := raStore.FindByRuntimeIDAndArchName(ctx, 1, "Qwen2ForCausalLM")
	require.Equal(t, nil, err)
	require.Nil(t, nil, arch)
}

func TestRuntimeArchitecturesStore_FindByRuntimeIDAndArchName(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	raStore := database.NewRuntimeArchitecturesStoreWithDB(db)
	err := raStore.Add(ctx, database.RuntimeArchitecture{
		RuntimeFrameworkID: 1,
		ArchitectureName:   "Qwen2ForCausalLM",
	})
	require.Nil(t, err)

	res, err := raStore.FindByRuntimeIDAndArchName(ctx, 1, "Qwen2ForCausalLM")
	require.Nil(t, err)
	require.Equal(t, "Qwen2ForCausalLM", res.ArchitectureName)
	require.Equal(t, int64(1), res.RuntimeFrameworkID)
}

func TestRuntimeArchitecturesStore_ListByRArchName(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	raStore := database.NewRuntimeArchitecturesStoreWithDB(db)
	err := raStore.Add(ctx, database.RuntimeArchitecture{
		RuntimeFrameworkID: 1,
		ArchitectureName:   "Qwen2ForCausalLM",
	})
	require.Nil(t, err)

	res, err := raStore.ListByRArchName(ctx, "Qwen2ForCausalLM")
	require.Nil(t, err)
	require.Equal(t, 1, len(res))
	require.Equal(t, "Qwen2ForCausalLM", res[0].ArchitectureName)
}

func TestRuntimeArchitecturesStore_ListByRArchNameAndModel(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	raStore := database.NewRuntimeArchitecturesStoreWithDB(db)
	err := raStore.Add(ctx, database.RuntimeArchitecture{
		RuntimeFrameworkID: 1,
		ArchitectureName:   "Qwen2ForCausalLM",
	})
	require.Nil(t, err)

	res, err := raStore.ListByRArchNameAndModel(ctx, "Qwen2ForCausalLM", "model1")
	require.Nil(t, err)
	require.Equal(t, 1, len(res))
}

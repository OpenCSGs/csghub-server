package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestPromptPrefixStore_Get(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewPromptPrefixStoreWithDB(db, cfg)

	_, err = db.Core.NewInsert().Model(&database.PromptPrefix{
		EN:   "foo",
		Kind: string(types.PromptActionOptimize),
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.PromptPrefix{
		EN:   "bar",
		Kind: string(types.PromptActionOptimize),
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.PromptPrefix{
		EN:   "test",
		Kind: string(types.PromptActionSummarize),
	}).Exec(ctx)
	require.Nil(t, err)

	prefix, err := store.Get(ctx, types.PromptActionOptimize)
	require.Nil(t, err)
	require.Equal(t, "bar", prefix.EN)

}

func TestPromptPrefixStore_GetByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewPromptPrefixStoreWithDB(db, cfg)

	dbInput := database.PromptPrefix{
		EN:   "foo",
		Kind: string(types.PromptActionOptimize),
	}
	_, err = db.Core.NewInsert().Model(&dbInput).Exec(ctx)
	require.Nil(t, err)
	prefix, err := store.GetByID(ctx, dbInput.ID)
	require.Nil(t, err)
	require.Equal(t, "foo", prefix.EN)
}

func TestPromptPrefixStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	cfg, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewPromptPrefixStoreWithDB(db, cfg)
	dbInput := database.PromptPrefix{
		EN:   "foo",
		Kind: string(types.PromptActionOptimize),
	}
	res, err := store.Create(ctx, dbInput)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, "foo", res.EN)

	search := &types.SearchPromptPrefix{
		Kind: string(types.PromptActionOptimize),
	}
	prompts, total, err := store.Index(ctx, 1, 1, search)
	require.Nil(t, err)
	require.Equal(t, len(prompts), 1)
	require.Equal(t, prompts[0].Kind, string(types.PromptActionOptimize))
	require.Equal(t, total, 1)

	err = store.Delete(ctx, res.ID)
	require.Nil(t, err)
}

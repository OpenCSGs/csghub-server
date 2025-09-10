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

func TestLLMConfigStore_GetOptimization(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)
	_, err = db.Core.NewInsert().Model(&database.LLMConfig{
		Type:      1,
		Enabled:   true,
		ModelName: "c1",
	}).Exec(ctx)
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.LLMConfig{
		Type:      2,
		Enabled:   true,
		ModelName: "c2",
	}).Exec(ctx)
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.LLMConfig{
		Type:      1,
		Enabled:   false,
		ModelName: "c3",
	}).Exec(ctx)
	require.Nil(t, err)

	cfg, err := store.GetOptimization(ctx)
	require.Nil(t, err)
	require.Equal(t, "c1", cfg.ModelName)
}

func TestLLMConfigStore_GetModelForSummaryReadme(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)
	_, err = db.Core.NewInsert().Model(&database.LLMConfig{
		Type:      5,
		Enabled:   true,
		ModelName: "summary1",
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.LLMConfig{
		Type:      4,
		Enabled:   false,
		ModelName: "summary2",
	}).Exec(ctx)
	require.Nil(t, err)

	_, err = db.Core.NewInsert().Model(&database.LLMConfig{
		Type:      2,
		Enabled:   true,
		ModelName: "summary3",
	}).Exec(ctx)
	require.Nil(t, err)

	cfg, err := store.GetModelForSummaryReadme(ctx)
	require.Nil(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "summary1", cfg.ModelName)
}

func TestLLMConfigStore_GetByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	dbInput := database.LLMConfig{
		Type:      5,
		Enabled:   true,
		ModelName: "summary1",
	}
	_, err = db.Core.NewInsert().Model(&dbInput).Exec(ctx)
	require.Nil(t, err)

	cfg, err := store.GetByID(ctx, dbInput.ID)
	require.Nil(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "summary1", cfg.ModelName)
}

func TestLLMConfigStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)
	dbInput := database.LLMConfig{
		Type:      5,
		Enabled:   true,
		ModelName: "summary1",
	}
	res, err := store.Create(ctx, dbInput)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, "summary1", res.ModelName)

	searchType := 5
	search := &types.SearchLLMConfig{
		Type: &searchType,
	}
	cfgs, total, err := store.Index(ctx, 1, 1, search)
	require.Nil(t, err)
	require.Equal(t, len(cfgs), 1)
	require.Equal(t, cfgs[0].Type, 5)
	require.Equal(t, total, 1)

	err = store.Delete(ctx, res.ID)
	require.Nil(t, err)
}

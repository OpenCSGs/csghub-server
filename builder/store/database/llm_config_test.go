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
		Type:        1,
		Enabled:     true,
		ModelName:   "c1",
		OfficialName: "c1",
		Metadata:    map[string]any{"source": "test"},
	}).Exec(ctx)
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.LLMConfig{
		Type:        2,
		Enabled:     true,
		ModelName:   "c2",
		OfficialName: "c2",
	}).Exec(ctx)
	require.Nil(t, err)
	_, err = db.Core.NewInsert().Model(&database.LLMConfig{
		Type:        1,
		Enabled:     false,
		ModelName:   "c3",
		OfficialName: "c3",
	}).Exec(ctx)
	require.Nil(t, err)

	cfg, err := store.GetOptimization(ctx)
	require.Nil(t, err)
	require.Equal(t, "c1", cfg.ModelName)
	require.Equal(t, "c1", cfg.OfficialName)
	require.Equal(t, map[string]any{"source": "test"}, cfg.Metadata)
}

func TestLLMConfigStore_GetModelForSummaryReadme(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)
	_, err = db.Core.NewInsert().Model(&database.LLMConfig{
		Type:        5,
		Enabled:     true,
		ModelName:   "summary1",
		OfficialName: "summary1",
		Metadata:    map[string]any{"k": "v"},
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
	require.Equal(t, "summary1", cfg.OfficialName)
	require.Equal(t, map[string]any{"k": "v"}, cfg.Metadata)
}

func TestLLMConfigStore_GetByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	dbInput := database.LLMConfig{
		Type:        5,
		Enabled:     true,
		ModelName:   "summary1",
		OfficialName: "summary1",
		Metadata:    map[string]any{"k": "v"},
	}
	_, err = db.Core.NewInsert().Model(&dbInput).Exec(ctx)
	require.Nil(t, err)

	cfg, err := store.GetByID(ctx, dbInput.ID)
	require.Nil(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "summary1", cfg.ModelName)
	require.Equal(t, "summary1", cfg.OfficialName)
	require.Equal(t, map[string]any{"k": "v"}, cfg.Metadata)
}

func TestLLMConfigStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)
	dbInput := database.LLMConfig{
		Type:        5,
		Enabled:     true,
		ModelName:   "summary1",
		OfficialName: "summary1",
		Metadata:    map[string]any{"k": "v", "tasks": []interface{}{"text-generation", "text-to-image"}},
	}
	res, err := store.Create(ctx, dbInput)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, "summary1", res.ModelName)
	require.Equal(t, "summary1", res.OfficialName)
	require.Equal(t, map[string]any{"k": "v", "tasks": []interface{}{"text-generation", "text-to-image"}}, res.Metadata)

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

func TestLLMConfigStore_Search(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	// Create test data with hyphens and letter-number combinations
	testModels := []database.LLMConfig{
		{Type: 1, Enabled: true, ModelName: "deepseek-v3", OfficialName: "deepseek-v3"},
		{Type: 1, Enabled: true, ModelName: "openai/gpt-4", OfficialName: "gpt-4"},
		{Type: 1, Enabled: true, ModelName: "claude3-opus", OfficialName: "claude3-opus"},
		{Type: 1, Enabled: true, ModelName: "llama2-7b", OfficialName: "llama2-7b"},
	}

	for _, model := range testModels {
		_, err := store.Create(ctx, model)
		require.Nil(t, err)
	}

	// Test case 1: Search for "deepseek" should find "deepseek-v3"
	search1 := &types.SearchLLMConfig{
		Keyword: "deepseek",
	}
	cfgs1, total1, err := store.Index(ctx, 10, 1, search1)
	require.Nil(t, err)
	require.GreaterOrEqual(t, total1, 1)
	found := false
	for _, cfg := range cfgs1 {
		if cfg.ModelName == "deepseek-v3" {
			found = true
			break
		}
	}
	require.True(t, found, "Should find deepseek-v3 when searching for deepseek")

	// Test case 2: Search for "deepseek-v3" should find "deepseek-v3"
	search2 := &types.SearchLLMConfig{
		Keyword: "deepseek-v3",
	}
	cfgs2, total2, err := store.Index(ctx, 10, 1, search2)
	require.Nil(t, err)
	require.GreaterOrEqual(t, total2, 1)
	found = false
	for _, cfg := range cfgs2 {
		if cfg.ModelName == "deepseek-v3" {
			found = true
			break
		}
	}
	require.True(t, found, "Should find deepseek-v3 when searching for deepseek-v3")

	// Test case 3: Search for "gpt" should find "gpt-4"
	search3 := &types.SearchLLMConfig{
		Keyword: "gpt",
	}
	cfgs3, total3, err := store.Index(ctx, 10, 1, search3)
	require.Nil(t, err)
	require.GreaterOrEqual(t, total3, 1)
	found = false
	for _, cfg := range cfgs3 {
		if cfg.ModelName == "openai/gpt-4" {
			found = true
			break
		}
	}
	require.True(t, found, "Should find gpt-4 when searching for gpt")

	// Test case 4: Search for "gpt-4" should find "gpt-4"
	search4 := &types.SearchLLMConfig{
		Keyword: "openai/gpt-4",
	}
	cfgs4, total4, err := store.Index(ctx, 10, 1, search4)
	require.Nil(t, err)
	require.GreaterOrEqual(t, total4, 1)
	found = false
	for _, cfg := range cfgs4 {
		if cfg.ModelName == "openai/gpt-4" {
			found = true
			break
		}
	}
	require.True(t, found, "Should find gpt-4 when searching for gpt-4")
}

func TestLLMConfigStore_Index_EnabledFilter(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	searchType := 16
	base := database.LLMConfig{
		Type:        searchType,
		ApiEndpoint: "https://example.test/v1",
		AuthHeader:  "{}",
		Provider:    "test",
	}
	_, err = store.Create(ctx, database.LLMConfig{
		ModelName:   "idx-en-on",
		OfficialName: "idx-en-on",
		Enabled:     true,
		Type:        base.Type,
		ApiEndpoint: base.ApiEndpoint,
		AuthHeader:  base.AuthHeader,
		Provider:    base.Provider,
	})
	require.Nil(t, err)
	_, err = store.Create(ctx, database.LLMConfig{
		ModelName:   "idx-en-off",
		OfficialName: "idx-en-off",
		Enabled:     false,
		Type:        base.Type,
		ApiEndpoint: base.ApiEndpoint,
		AuthHeader:  base.AuthHeader,
		Provider:    base.Provider,
	})
	require.Nil(t, err)

	enabledTrue := true
	enabledFalse := false

	cfgsOn, totalOn, err := store.Index(ctx, 20, 1, &types.SearchLLMConfig{
		Type:    &searchType,
		Enabled: &enabledTrue,
	})
	require.Nil(t, err)
	require.Equal(t, 1, totalOn)
	require.Len(t, cfgsOn, 1)
	require.Equal(t, "idx-en-on", cfgsOn[0].ModelName)
	require.True(t, cfgsOn[0].Enabled)

	cfgsOff, totalOff, err := store.Index(ctx, 20, 1, &types.SearchLLMConfig{
		Type:    &searchType,
		Enabled: &enabledFalse,
	})
	require.Nil(t, err)
	require.Equal(t, 1, totalOff)
	require.Len(t, cfgsOff, 1)
	require.Equal(t, "idx-en-off", cfgsOff[0].ModelName)
	require.False(t, cfgsOff[0].Enabled)

	cfgsBoth, totalBoth, err := store.Index(ctx, 20, 1, &types.SearchLLMConfig{
		Type: &searchType,
	})
	require.Nil(t, err)
	require.Equal(t, 2, totalBoth)
	require.Len(t, cfgsBoth, 2)

	cfgsKeyword, totalKeyword, err := store.Index(ctx, 20, 1, &types.SearchLLMConfig{
		Keyword: "idx-en-",
		Enabled: &enabledTrue,
	})
	require.Nil(t, err)
	require.Equal(t, 1, totalKeyword)
	require.Len(t, cfgsKeyword, 1)
	require.Equal(t, "idx-en-on", cfgsKeyword[0].ModelName)
}

package database_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
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
		Metadata:  map[string]any{"source": "test"},
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
	require.Equal(t, map[string]any{"source": "test"}, cfg.Metadata)
}

// The internal llm client selects upstreams by health (builder/llm
// preferredUpstreams), so the queries backing GetOptimization and friends
// must load Upstreams.HealthState; otherwise an unhealthy upstream would
// still be picked first.
func TestLLMConfigStore_GetOptimization_LoadsUpstreamHealthState(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	cfgRow := &database.LLMConfig{Type: 1, Enabled: true, ModelName: "c-health"}
	_, err = db.Core.NewInsert().Model(cfgRow).Exec(ctx)
	require.Nil(t, err)

	upstreamA := &database.Upstream{LLMConfigID: cfgRow.ID, URL: "https://a.example.com", Enabled: true, ModelName: "ma"}
	_, err = db.Core.NewInsert().Model(upstreamA).Exec(ctx)
	require.Nil(t, err)
	upstreamB := &database.Upstream{LLMConfigID: cfgRow.ID, URL: "https://b.example.com", Enabled: true, ModelName: "mb"}
	_, err = db.Core.NewInsert().Model(upstreamB).Exec(ctx)
	require.Nil(t, err)

	// The gateway marked upstream A unhealthy.
	_, err = db.Core.NewInsert().Model(&database.AIGatewayUpstreamHealthState{
		UpstreamID:  upstreamA.ID,
		HealthState: "unhealthy",
		LastCheckAt: time.Now(),
	}).Exec(ctx)
	require.Nil(t, err)

	cfg, err := store.GetOptimization(ctx)
	require.Nil(t, err)
	require.Len(t, cfg.Upstreams, 2)
	for _, u := range cfg.Upstreams {
		switch u.URL {
		case "https://a.example.com":
			require.NotNil(t, u.HealthState, "health state must be loaded for selection")
			require.Equal(t, "unhealthy", u.HealthState.HealthState)
		case "https://b.example.com":
			require.Nil(t, u.HealthState)
		}
	}
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
		Metadata:  map[string]any{"k": "v"},
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
		Type:      5,
		Enabled:   true,
		ModelName: "summary1",
		Metadata:  map[string]any{"k": "v"},
	}
	_, err = db.Core.NewInsert().Model(&dbInput).Exec(ctx)
	require.Nil(t, err)

	cfg, err := store.GetByID(ctx, dbInput.ID)
	require.Nil(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "summary1", cfg.ModelName)
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
		Type:      5,
		Enabled:   true,
		ModelName: "summary1",
		RoutingPolicy: types.RoutingPolicy{
			Strategy:      "session_hash",
			SessionHeader: "X-Session-ID",
			HashReplicas:  128,
		},
		Metadata: map[string]any{"k": "v", "tasks": []interface{}{"text-generation", "text-to-image"}},
	}
	res, err := store.Create(ctx, dbInput)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, "summary1", res.ModelName)
	// OfficialName, Upstreams are now relational, tested separately
	require.Equal(t, map[string]any{"k": "v", "tasks": []interface{}{"text-generation", "text-to-image"}}, res.Metadata)

	searchType := 5
	search := &types.SearchLLMConfig{
		Types: []int{searchType},
	}
	cfgs, total, err := store.Index(ctx, 1, 1, search)
	require.Nil(t, err)
	require.Equal(t, len(cfgs), 1)
	require.Equal(t, cfgs[0].Type, 5)
	require.Equal(t, total, 1)

	err = store.Delete(ctx, res.ID)
	require.Nil(t, err)
}

func TestLLMConfigStore_Index_MultiType(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	// Create a config that supports both optimization (1) and summary readme (4)
	_, err = store.Create(ctx, database.LLMConfig{
		Type:      database.LLMTypeOptimization | database.LLMTypeSummaryReadme,
		Enabled:   true,
		ModelName: "multi-type-model",
	})
	require.Nil(t, err)

	// Create a config that supports only comparison (2)
	_, err = store.Create(ctx, database.LLMConfig{
		Type:      database.LLMTypeComparison,
		Enabled:   true,
		ModelName: "comparison-only-model",
	})
	require.Nil(t, err)

	// Searching by optimization should find the multi-type config
	cfgs, total, err := store.Index(ctx, 10, 1, &types.SearchLLMConfig{
		Types: []int{database.LLMTypeOptimization},
	})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Len(t, cfgs, 1)
	require.Equal(t, "multi-type-model", cfgs[0].ModelName)

	// Searching by summary readme should also find the multi-type config
	cfgs, total, err = store.Index(ctx, 10, 1, &types.SearchLLMConfig{
		Types: []int{database.LLMTypeSummaryReadme},
	})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Len(t, cfgs, 1)
	require.Equal(t, "multi-type-model", cfgs[0].ModelName)

	// Searching by both optimization and comparison should find both configs
	cfgs, total, err = store.Index(ctx, 10, 1, &types.SearchLLMConfig{
		Types: []int{database.LLMTypeOptimization, database.LLMTypeComparison},
	})
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.Len(t, cfgs, 2)
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
		{Type: 1, Enabled: true, ModelName: "deepseek-v3"},
		{Type: 1, Enabled: true, ModelName: "openai/gpt-4"},
		{Type: 1, Enabled: true, ModelName: "claude3-opus"},
		{Type: 1, Enabled: true, ModelName: "llama2-7b"},
		{Type: 1, Enabled: true, ModelName: "Qwen/Qwen3Guard-Gen-0.6B"},
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
		Type: searchType,
	}
	_, err = store.Create(ctx, database.LLMConfig{
		ModelName: "idx-en-on",
		Enabled:   true,
		Type:      base.Type,
	})
	require.Nil(t, err)
	_, err = store.Create(ctx, database.LLMConfig{
		ModelName: "idx-en-off",
		Enabled:   false,
		Type:      base.Type,
	})
	require.Nil(t, err)

	enabledTrue := true
	enabledFalse := false

	cfgsOn, totalOn, err := store.Index(ctx, 20, 1, &types.SearchLLMConfig{
		Types:   []int{searchType},
		Enabled: &enabledTrue,
	})
	require.Nil(t, err)
	require.Equal(t, 1, totalOn)
	require.Len(t, cfgsOn, 1)
	require.Equal(t, "idx-en-on", cfgsOn[0].ModelName)
	require.True(t, cfgsOn[0].Enabled)

	cfgsOff, totalOff, err := store.Index(ctx, 20, 1, &types.SearchLLMConfig{
		Types:   []int{searchType},
		Enabled: &enabledFalse,
	})
	require.Nil(t, err)
	require.Equal(t, 1, totalOff)
	require.Len(t, cfgsOff, 1)
	require.Equal(t, "idx-en-off", cfgsOff[0].ModelName)
	require.False(t, cfgsOff[0].Enabled)

	cfgsBoth, totalBoth, err := store.Index(ctx, 20, 1, &types.SearchLLMConfig{
		Types: []int{searchType},
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

func TestLLMConfigStore_IndexWithRepo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	// Create a namespace and repository first
	namespace := database.Namespace{
		Path: "test-ns-indexwithrepo",
	}
	_, err = db.Core.NewInsert().Model(&namespace).Exec(ctx)
	require.Nil(t, err)

	user := database.User{
		GitID:    12345,
		Username: "test-user-indexwithrepo",
		NickName: "Test User",
		Email:    "test-indexwithrepo@example.com",
	}
	_, err = db.Core.NewInsert().Model(&user).Exec(ctx)
	require.Nil(t, err)

	repo := database.Repository{
		Path:        "test-ns-indexwithrepo/test-repo",
		GitPath:     "models/test-ns-indexwithrepo/test-repo",
		Name:        "test-repo",
		Nickname:    "Test Repo",
		Description: "A test repository",
		UserID:      user.ID,
	}
	_, err = db.Core.NewInsert().Model(&repo).Exec(ctx)
	require.Nil(t, err)

	// Create LLMConfigs with and without repo
	llmType := 16
	enabled := true

	_, err = store.Create(ctx, database.LLMConfig{
		ModelName: "with-repo-model",
		Type:      llmType,
		Enabled:   true,
		RepoID:    repo.ID,
	})
	require.Nil(t, err)

	_, err = store.Create(ctx, database.LLMConfig{
		ModelName: "no-repo-model",
		Type:      llmType,
		Enabled:   true,
		RepoID:    0,
	})
	require.Nil(t, err)

	// Test IndexWithRepo
	search := &types.SearchLLMConfig{
		Types:   []int{llmType},
		Enabled: &enabled,
	}
	cfgs, total, err := store.IndexWithRepo(ctx, 10, 1, search)
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.Len(t, cfgs, 2)

	var withRepo, withoutRepo *database.LLMConfig
	for _, cfg := range cfgs {
		if cfg.ModelName == "with-repo-model" {
			withRepo = cfg
		} else if cfg.ModelName == "no-repo-model" {
			withoutRepo = cfg
		}
	}

	require.NotNil(t, withRepo)
	require.NotNil(t, withRepo.Repo)
	require.Equal(t, repo.ID, withRepo.Repo.ID)
	require.Equal(t, "test-repo", withRepo.Repo.Name)
	require.Equal(t, "test-ns-indexwithrepo/test-repo", withRepo.Repo.Path)
	require.Equal(t, "models/test-ns-indexwithrepo/test-repo", withRepo.Repo.GitPath)
	require.Equal(t, "Test Repo", withRepo.Repo.Nickname)
	require.Equal(t, "A test repository", withRepo.Repo.Description)

	require.NotNil(t, withoutRepo)
	require.Nil(t, withoutRepo.Repo)
}

func TestLLMConfigStore_IndexWithRepo_Empty(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	llmType := 16
	search := &types.SearchLLMConfig{
		Types: []int{llmType},
	}
	cfgs, total, err := store.IndexWithRepo(ctx, 10, 1, search)
	require.Nil(t, err)
	require.Equal(t, 0, total)
	require.Len(t, cfgs, 0)
}

func TestLLMConfigStore_Index_SortByModelSizeB(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	llmType := 16
	_, err = store.Create(ctx, database.LLMConfig{
		ModelName:  "size-small",
		Type:       llmType,
		Enabled:    true,
		ModelSizeB: 0.5,
	})
	require.Nil(t, err)
	_, err = store.Create(ctx, database.LLMConfig{
		ModelName:  "size-large",
		Type:       llmType,
		Enabled:    true,
		ModelSizeB: 70,
	})
	require.Nil(t, err)
	_, err = store.Create(ctx, database.LLMConfig{
		ModelName:  "size-medium",
		Type:       llmType,
		Enabled:    true,
		ModelSizeB: 7,
	})
	require.Nil(t, err)

	cfgsAsc, _, err := store.Index(ctx, 10, 1, &types.SearchLLMConfig{
		Types:     []int{llmType},
		SortBy:    "model_size_b",
		SortOrder: "ASC",
	})
	require.Nil(t, err)
	require.Len(t, cfgsAsc, 3)
	require.Equal(t, "size-small", cfgsAsc[0].ModelName)
	require.Equal(t, "size-medium", cfgsAsc[1].ModelName)
	require.Equal(t, "size-large", cfgsAsc[2].ModelName)

	cfgsDesc, _, err := store.Index(ctx, 10, 1, &types.SearchLLMConfig{
		Types:     []int{llmType},
		SortBy:    "model_size_b",
		SortOrder: "DESC",
	})
	require.Nil(t, err)
	require.Len(t, cfgsDesc, 3)
	require.Equal(t, "size-large", cfgsDesc[0].ModelName)
	require.Equal(t, "size-medium", cfgsDesc[1].ModelName)
	require.Equal(t, "size-small", cfgsDesc[2].ModelName)
}

func TestLLMConfigStore_Index_PagesFollowATotalOrder(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	// Configs sharing a sort key are the norm (every config the deploy sync
	// creates has model_size_b = 0). Paged reads must partition them
	// deterministically, or one config is listed twice while another is skipped.
	const (
		modelCount = 120
		pageSize   = 50
	)
	search := &types.SearchLLMConfig{
		Keyword:   "tied-paging-",
		Types:     []int{database.LLMTypeAigatewayExternal},
		SortBy:    "model_size_b",
		SortOrder: "DESC",
	}

	var createdIDs []int64
	for i := 0; i < modelCount; i++ {
		cfg, err := store.Create(ctx, database.LLMConfig{
			ModelName:  fmt.Sprintf("tied-paging-%03d", i),
			Type:       database.LLMTypeAigatewayExternal,
			Enabled:    true,
			ModelSizeB: 0,
		})
		require.Nil(t, err)
		createdIDs = append(createdIDs, cfg.ID)
	}
	// Updating rows moves them to the end of the heap, so the physical order no
	// longer matches the ids: an implicit tie order would now differ from the
	// tie-breaker the sort promises.
	_, err = db.Core.NewRaw(
		`UPDATE llm_configs SET provider = 'moved' WHERE model_name ~ 'tied-paging-[0-9]*[13579]$'`,
	).Exec(ctx)
	require.Nil(t, err)

	expected := slices.Clone(createdIDs)
	slices.Sort(expected)

	var listed []int64
	for page := 1; ; page++ {
		cfgs, _, err := store.Index(ctx, pageSize, page, search)
		require.Nil(t, err)
		for _, cfg := range cfgs {
			listed = append(listed, cfg.ID)
		}
		if len(cfgs) < pageSize {
			break
		}
	}
	require.Equal(t, expected, listed, "pages must follow the sort's total order: every tied config exactly once, ordered by id")
}

func TestLLMConfig_PopulateDerivedFields_NoUpstreams(t *testing.T) {
	cfg := &database.LLMConfig{
		ModelName:   "original-model",
		ApiEndpoint: "original-endpoint",
		AuthHeader:  "original-auth",
		Provider:    "original-provider",
	}
	cfg.PopulateDerivedFields()
	require.Equal(t, "original-model", cfg.ModelName)
	require.Equal(t, "original-endpoint", cfg.ApiEndpoint)
	require.Equal(t, "original-auth", cfg.AuthHeader)
	require.Equal(t, "original-provider", cfg.Provider)
}

func TestLLMConfig_PopulateDerivedFields_HealthyUpstream(t *testing.T) {
	cfg := &database.LLMConfig{
		ModelName:   "original-model",
		ApiEndpoint: "original-endpoint",
		AuthHeader:  "original-auth",
		Provider:    "original-provider",
		Upstreams: []database.Upstream{
			{
				URL:        "http://disabled-upstream",
				Enabled:    false,
				ModelName:  "disabled-model",
				AuthHeader: `{"Authorization":"disabled"}`,
				Provider:   "disabled-provider",
			},
			{
				URL:        "http://healthy-upstream",
				Enabled:    true,
				ModelName:  "healthy-model",
				AuthHeader: `{"Authorization":"healthy"}`,
				Provider:   "healthy-provider",
				HealthState: &database.AIGatewayUpstreamHealthState{
					HealthState: "healthy",
				},
			},
		},
	}
	cfg.PopulateDerivedFields()
	require.Equal(t, "http://healthy-upstream", cfg.ApiEndpoint)
	require.Equal(t, `{"Authorization":"healthy"}`, cfg.AuthHeader)
	require.Equal(t, "healthy-provider", cfg.Provider)
	require.Equal(t, "healthy-model", cfg.ModelName)
}

func TestLLMConfig_PopulateDerivedFields_UnhealthyUpstream(t *testing.T) {
	cfg := &database.LLMConfig{
		ModelName:   "original-model",
		ApiEndpoint: "original-endpoint",
		Upstreams: []database.Upstream{
			{
				URL:        "http://unhealthy-upstream",
				Enabled:    true,
				ModelName:  "unhealthy-model",
				AuthHeader: `{"Authorization":"unhealthy"}`,
				Provider:   "unhealthy-provider",
				HealthState: &database.AIGatewayUpstreamHealthState{
					HealthState: "unhealthy",
				},
			},
			{
				URL:        "http://fallback-upstream",
				Enabled:    true,
				ModelName:  "fallback-model",
				AuthHeader: `{"Authorization":"fallback"}`,
				Provider:   "fallback-provider",
			},
		},
	}
	cfg.PopulateDerivedFields()
	// Should skip unhealthy upstream and use the fallback (no health state = healthy)
	require.Equal(t, "http://fallback-upstream", cfg.ApiEndpoint)
	require.Equal(t, "fallback-model", cfg.ModelName)
	require.Equal(t, "fallback-provider", cfg.Provider)
}

func TestLLMConfig_PopulateDerivedFields_DegradedIsHealthy(t *testing.T) {
	cfg := &database.LLMConfig{
		ModelName: "original-model",
		Upstreams: []database.Upstream{
			{
				URL:        "http://degraded-upstream",
				Enabled:    true,
				ModelName:  "degraded-model",
				AuthHeader: `{"Authorization":"degraded"}`,
				Provider:   "degraded-provider",
				HealthState: &database.AIGatewayUpstreamHealthState{
					HealthState: "degraded",
				},
			},
		},
	}
	cfg.PopulateDerivedFields()
	// "degraded" is not "unhealthy", so it should be considered healthy
	require.Equal(t, "http://degraded-upstream", cfg.ApiEndpoint)
	require.Equal(t, "degraded-model", cfg.ModelName)
}

func TestLLMConfig_PopulateDerivedFields_OverridesModelName(t *testing.T) {
	cfg := &database.LLMConfig{
		ModelName: "config-model",
		Upstreams: []database.Upstream{
			{
				URL:        "http://upstream",
				Enabled:    true,
				ModelName:  "upstream-model",
				AuthHeader: "{}",
				Provider:   "upstream-provider",
			},
		},
	}
	cfg.PopulateDerivedFields()
	require.Equal(t, "upstream-model", cfg.ModelName)
}

func TestLLMConfig_PopulateDerivedFields_EmptyUpstreamModelName(t *testing.T) {
	cfg := &database.LLMConfig{
		ModelName: "config-model",
		Upstreams: []database.Upstream{
			{
				URL:        "http://upstream",
				Enabled:    true,
				ModelName:  "",
				AuthHeader: "{}",
				Provider:   "upstream-provider",
			},
		},
	}
	cfg.PopulateDerivedFields()
	// Should keep original ModelName when upstream ModelName is empty
	require.Equal(t, "config-model", cfg.ModelName)
}

func TestLLMConfig_Tasks(t *testing.T) {
	tests := []struct {
		name   string
		config *database.LLMConfig
		want   []string
	}{
		{name: "nil config", config: nil, want: nil},
		{name: "nil metadata", config: &database.LLMConfig{}, want: nil},
		{name: "no tasks key", config: &database.LLMConfig{Metadata: map[string]any{"other": "value"}}, want: nil},
		{
			name:   "jsonb round trip []any",
			config: &database.LLMConfig{Metadata: map[string]any{"tasks": []any{"text-generation", "auto-speech-recognition"}}},
			want:   []string{"text-generation", "auto-speech-recognition"},
		},
		{
			name:   "programmatic []string",
			config: &database.LLMConfig{Metadata: map[string]any{"tasks": []string{"text-to-image"}}},
			want:   []string{"text-to-image"},
		},
		{name: "non slice value", config: &database.LLMConfig{Metadata: map[string]any{"tasks": "text-generation"}}, want: nil},
		{name: "slice with non string entries", config: &database.LLMConfig{Metadata: map[string]any{"tasks": []any{"text-generation", 42}}}, want: []string{"text-generation"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, test.config.Tasks())
		})
	}
}

func TestResolveLLMTasks(t *testing.T) {
	ctx := context.TODO()

	t.Run("nil store", func(t *testing.T) {
		require.Nil(t, database.ResolveLLMTasks(ctx, nil, 7))
	})

	t.Run("store error", func(t *testing.T) {
		store := mockdatabase.NewMockLLMConfigStore(t)
		store.EXPECT().GetByID(ctx, int64(7)).Return(nil, errors.New("boom")).Once()
		require.Nil(t, database.ResolveLLMTasks(ctx, store, 7))
	})

	t.Run("config not found", func(t *testing.T) {
		store := mockdatabase.NewMockLLMConfigStore(t)
		store.EXPECT().GetByID(ctx, int64(7)).Return(nil, nil).Once()
		require.Nil(t, database.ResolveLLMTasks(ctx, store, 7))
	})

	t.Run("resolves tasks", func(t *testing.T) {
		store := mockdatabase.NewMockLLMConfigStore(t)
		store.EXPECT().GetByID(ctx, int64(7)).Return(&database.LLMConfig{
			Metadata: map[string]any{"tasks": []any{"auto-speech-recognition"}},
		}, nil).Once()
		require.Equal(t, []string{"auto-speech-recognition"}, database.ResolveLLMTasks(ctx, store, 7))
	})
}

func TestLLMConfigStore_Index_AvailableFirst(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	insertConfig := func(t *testing.T, name string, enabled bool) int64 {
		cfg := &database.LLMConfig{Type: 16, Enabled: enabled, ModelName: name}
		_, err := db.Core.NewInsert().Model(cfg).Exec(ctx)
		require.Nil(t, err)
		return cfg.ID
	}
	insertUpstream := func(t *testing.T, llmConfigID int64, enabled, healthCheckEnabled bool) int64 {
		up := &database.Upstream{
			LLMConfigID:        llmConfigID,
			URL:                "http://upstream.example.com/v1",
			Enabled:            enabled,
			HealthCheckEnabled: healthCheckEnabled,
		}
		_, err := db.Core.NewInsert().Model(up).Exec(ctx)
		require.Nil(t, err)
		return up.ID
	}
	setHealth := func(t *testing.T, upstreamID int64, state string) {
		_, err := db.Core.NewInsert().Model(&database.AIGatewayUpstreamHealthState{
			UpstreamID:  upstreamID,
			HealthState: state,
			LastCheckAt: time.Now(),
		}).Exec(ctx)
		require.Nil(t, err)
	}

	// inserted in "wrong" order on purpose: unhealthy first, then unavailable.
	unhealthyID := insertConfig(t, "model-unhealthy", true)
	unhealthyUp := insertUpstream(t, unhealthyID, true, true)
	setHealth(t, unhealthyUp, "unhealthy")

	disabledID := insertConfig(t, "model-disabled", false)
	insertUpstream(t, disabledID, true, true)

	availableID := insertConfig(t, "model-available", true)
	availableUp := insertUpstream(t, availableID, true, true)
	setHealth(t, availableUp, "healthy")

	// mirrors computeLLMAvailability: zero upstream rows count as available
	insertConfig(t, "model-no-upstream", true)

	// upstream rows exist but none is enabled -> unavailable
	noEnabledID := insertConfig(t, "model-no-enabled-upstream", true)
	insertUpstream(t, noEnabledID, false, true)

	configs, total, err := store.Index(ctx, 10, 1, &types.SearchLLMConfig{AvailableFirst: true})
	require.Nil(t, err)
	require.Equal(t, 5, total)
	require.Len(t, configs, 5)
	// available surfaces first (including the zero-upstream config, whose
	// is_available flag is also true); the unavailable ones (disabled config,
	// unhealthy upstream, only-disabled upstreams) follow in the default
	// updated_at/id order.
	require.ElementsMatch(
		t, []string{"model-available", "model-no-upstream"},
		[]string{configs[0].ModelName, configs[1].ModelName},
	)
	require.ElementsMatch(
		t, []string{"model-unhealthy", "model-disabled", "model-no-enabled-upstream"},
		[]string{configs[2].ModelName, configs[3].ModelName, configs[4].ModelName},
	)

	// without the flag the historical ordering applies (no availability key)
	configs, total, err = store.Index(ctx, 10, 1, &types.SearchLLMConfig{})
	require.Nil(t, err)
	require.Equal(t, 5, total)
	names := []string{configs[0].ModelName, configs[1].ModelName, configs[2].ModelName, configs[3].ModelName, configs[4].ModelName}
	require.Contains(t, names, "model-available")
	require.Contains(t, names, "model-no-upstream")
}

// gatewayListFixture inserts a small catalogue covering the dimensions the
// gateway listing filters on: an external model, a public serverless model,
// a private inference model, and a disabled model.
type gatewayListFixture struct {
	db         *database.DB
	store      database.LLMConfigStore
	external   *database.LLMConfig
	serverless *database.LLMConfig
	private    *database.LLMConfig
}

// gatewayUpstreamSourceID hands out unique upstream source IDs; the
// ai_gateway_upstreams table enforces (source, source_id) uniqueness and the
// csghub source defaults to 0 otherwise.
var gatewayUpstreamSourceID atomic.Int64

// insertGatewayConfig persists a config and its upstreams. Bun does not
// cascade has-many relations on insert, so the upstreams are written
// separately against the config.
func insertGatewayConfig(ctx context.Context, t *testing.T, db *database.DB, cfg *database.LLMConfig) {
	t.Helper()
	_, err := db.Core.NewInsert().Model(cfg).Exec(ctx)
	require.Nil(t, err)
	for i := range cfg.Upstreams {
		cfg.Upstreams[i].LLMConfigID = cfg.ID
		if cfg.Upstreams[i].SourceID == 0 {
			cfg.Upstreams[i].SourceID = gatewayUpstreamSourceID.Add(1)
		}
		_, err = db.Core.NewInsert().Model(&cfg.Upstreams[i]).Exec(ctx)
		require.Nil(t, err)
	}
}

func newGatewayListFixture(t *testing.T) gatewayListFixture {
	t.Helper()
	db := tests.InitTestDB()
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	serverlessUpstream := database.Upstream{
		SourceID: 101,
		Source:   types.UpstreamSourceCSGHubDeploy,
		URL:      "http://svc-upstream/v1",
		Enabled:  true,
		Metadata: &types.UpstreamMetadata{
			InternalModelInfo: &types.InternalModelInfo{
				CSGHubModelID: "ns/svc-model",
				LegacyModelID: "glm-svc",
				OwnerUUID:     "owner-uuid-1",
				SvcType:       types.ServerlessType,
				SecureLevel:   types.EndpointPublic,
				Task:          "text-generation",
			},
		},
	}
	privateUpstream := database.Upstream{
		SourceID: 102,
		Source:   types.UpstreamSourceCSGHubDeploy,
		URL:      "http://private-upstream/v1",
		Enabled:  true,
		Metadata: &types.UpstreamMetadata{
			InternalModelInfo: &types.InternalModelInfo{
				CSGHubModelID: "ns/private-model",
				LegacyModelID: "glm-private",
				OwnerUUID:     "owner-uuid-1",
				SvcType:       types.InferenceType,
				SecureLevel:   2,
				Task:          "text-generation",
			},
		},
	}

	external := &database.LLMConfig{
		ModelName: "ext-model",
		Type:      database.LLMTypeAigatewayExternal,
		Enabled:   true,
		Provider:  "openai",
		Metadata:  map[string]any{"tasks": []any{"text-generation"}},
		Upstreams: []database.Upstream{{Source: types.UpstreamSourceExternal, URL: "http://ext/v1", Enabled: true, Provider: "openai"}},
	}
	serverless := &database.LLMConfig{
		ModelName: "svc-model",
		Type:      database.LLMTypeAigatewayExternal,
		Enabled:   true,
		Upstreams: []database.Upstream{serverlessUpstream},
	}
	private := &database.LLMConfig{
		ModelName: "private-model",
		Type:      database.LLMTypeAigatewayExternal,
		Enabled:   true,
		Upstreams: []database.Upstream{privateUpstream},
	}
	disabled := &database.LLMConfig{
		ModelName: "disabled-model",
		Type:      database.LLMTypeAigatewayExternal,
		Enabled:   false,
		Upstreams: []database.Upstream{{Source: types.UpstreamSourceExternal, URL: "http://x/v1", Enabled: true}},
	}
	for _, cfg := range []*database.LLMConfig{external, serverless, private, disabled} {
		insertGatewayConfig(ctx, t, db, cfg)
	}
	return gatewayListFixture{db: db, store: store, external: external, serverless: serverless, private: private}
}

func modelNames(configs []*database.LLMConfig) []string {
	names := make([]string, 0, len(configs))
	for _, cfg := range configs {
		names = append(names, cfg.ModelName)
	}
	return names
}

func TestLLMConfigStore_ListGatewayModels_Visibility(t *testing.T) {
	f := newGatewayListFixture(t)
	ctx := context.TODO()

	// Anonymous callers see public and external models only.
	configs, total, err := f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{})
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.ElementsMatch(t, []string{"ext-model", "svc-model"}, modelNames(configs))

	// The deploy owner also sees the private model.
	configs, total, err = f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{CallerUUID: "owner-uuid-1"})
	require.Nil(t, err)
	require.Equal(t, 3, total)
	require.ElementsMatch(t, []string{"ext-model", "svc-model", "private-model"}, modelNames(configs))
}

func TestLLMConfigStore_ListGatewayModels_KeywordMatchesPublicModelID(t *testing.T) {
	f := newGatewayListFixture(t)
	ctx := context.TODO()

	// The keyword matches the public model ID: legacy_model_id for internal
	// models (not the internal model_name), model_name for external ones.
	configs, total, err := f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{Keyword: "glm-svc"})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"svc-model"}, modelNames(configs))

	configs, total, err = f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{Keyword: "EXT"})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"ext-model"}, modelNames(configs))

	// The internal model_name is not a public ID.
	configs, _, err = f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{Keyword: "svc-model"})
	require.Nil(t, err)
	require.Equal(t, 0, len(configs))
}

func TestLLMConfigStore_ListGatewayModels_LLMTypes(t *testing.T) {
	f := newGatewayListFixture(t)
	ctx := context.TODO()

	configs, total, err := f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{LLMTypes: []string{"external_llm"}})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"ext-model"}, modelNames(configs))

	configs, total, err = f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{LLMTypes: []string{"serverless"}})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"svc-model"}, modelNames(configs))

	configs, total, err = f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{LLMTypes: []string{"inference"}, CallerUUID: "owner-uuid-1"})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"private-model"}, modelNames(configs))

	configs, _, err = f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{LLMTypes: []string{"external_llm", "serverless"}})
	require.Nil(t, err)
	require.Len(t, configs, 2)
}

func TestLLMConfigStore_ListGatewayModels_Task(t *testing.T) {
	f := newGatewayListFixture(t)
	ctx := context.TODO()

	configs, total, err := f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{Task: "text-generation"})
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.ElementsMatch(t, []string{"ext-model", "svc-model"}, modelNames(configs))

	configs, total, err = f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{Task: "text-to-image"})
	require.Nil(t, err)
	require.Equal(t, 0, total)
	require.Empty(t, configs)
}

// TestLLMConfigStore_ListGatewayModels_InternalModelTaskLabels pins the
// catalogue view's tasks column to the TaskLabels semantics the API response
// uses: an internal model synced with the expanded Tasks list (multimodal
// image-text-to-text plus text-generation) must surface under both labels,
// while a legacy row carrying only the raw single task must not.
func TestLLMConfigStore_ListGatewayModels_InternalModelTaskLabels(t *testing.T) {
	f := newGatewayListFixture(t)
	ctx := context.TODO()

	multimodal := &database.LLMConfig{
		ModelName: "multimodal-model",
		Type:      database.LLMTypeAigatewayExternal,
		Enabled:   true,
		Upstreams: []database.Upstream{{
			Source:  types.UpstreamSourceCSGHubDeploy,
			URL:     "http://multimodal-upstream/v1",
			Enabled: true,
			Metadata: &types.UpstreamMetadata{
				InternalModelInfo: &types.InternalModelInfo{
					CSGHubModelID: "ns/multimodal-model",
					LegacyModelID: "glm-mm",
					OwnerUUID:     "owner-uuid-1",
					SvcType:       types.InferenceType,
					SecureLevel:   2,
					Task:          "image-text-to-text",
					Tasks:         []string{"image-text-to-text", "text-generation"},
				},
			},
		}},
	}
	legacy := &database.LLMConfig{
		ModelName: "legacy-multimodal-model",
		Type:      database.LLMTypeAigatewayExternal,
		Enabled:   true,
		Upstreams: []database.Upstream{{
			Source:  types.UpstreamSourceCSGHubDeploy,
			URL:     "http://legacy-upstream/v1",
			Enabled: true,
			Metadata: &types.UpstreamMetadata{
				InternalModelInfo: &types.InternalModelInfo{
					CSGHubModelID: "ns/legacy-multimodal-model",
					LegacyModelID: "glm-legacy-mm",
					OwnerUUID:     "owner-uuid-1",
					SvcType:       types.InferenceType,
					SecureLevel:   2,
					Task:          "image-text-to-text",
				},
			},
		}},
	}
	for _, cfg := range []*database.LLMConfig{multimodal, legacy} {
		insertGatewayConfig(ctx, t, f.db, cfg)
	}

	// The expanded label surfaces the multimodal model alongside the rows
	// whose raw task is text-generation; the legacy row (raw task
	// image-text-to-text, no Tasks list) must not appear.
	configs, total, err := f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{
		CallerUUID: "owner-uuid-1",
		Task:       "text-generation",
	})
	require.Nil(t, err)
	require.Equal(t, 4, total)
	require.ElementsMatch(
		t, []string{"ext-model", "svc-model", "private-model", "multimodal-model"},
		modelNames(configs),
	)

	// The raw label still matches both the expanded list and the legacy row.
	configs, total, err = f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{
		CallerUUID: "owner-uuid-1",
		Task:       "image-text-to-text",
	})
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.ElementsMatch(t, []string{"multimodal-model", "legacy-multimodal-model"}, modelNames(configs))
}

func TestLLMConfigStore_ListGatewayModels_HasAssociatedModel(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	repo := &database.Repository{UserID: 1, Path: "ns/linked-model", GitPath: "x", Name: "linked-model", Nickname: "linked"}
	_, err = db.Core.NewInsert().Model(repo).Exec(ctx)
	require.Nil(t, err)

	linked := &database.LLMConfig{
		ModelName: "linked-model", Type: database.LLMTypeAigatewayExternal, Enabled: true,
		RepoID:    repo.ID,
		Upstreams: []database.Upstream{{Source: types.UpstreamSourceExternal, URL: "http://x/v1", Enabled: true}},
	}
	unlinked := &database.LLMConfig{
		ModelName: "unlinked-model", Type: database.LLMTypeAigatewayExternal, Enabled: true,
		Upstreams: []database.Upstream{{Source: types.UpstreamSourceExternal, URL: "http://x/v1", Enabled: true}},
	}
	for _, cfg := range []*database.LLMConfig{linked, unlinked} {
		insertGatewayConfig(ctx, t, db, cfg)
	}

	associated := true
	configs, total, err := store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{HasAssociatedModel: &associated})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"linked-model"}, modelNames(configs))

	notAssociated := false
	configs, total, err = store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{HasAssociatedModel: &notAssociated})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"unlinked-model"}, modelNames(configs))
}

// A soft-deleted repository counts as unassociated, exactly like the
// Relation("Repo") loading the response metadata relies on: Bun appends
// deleted_at IS NULL for the belongs-to relation, so the model then reports
// no repo_path either.
func TestLLMConfigStore_ListGatewayModels_SoftDeletedRepo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	repo := &database.Repository{UserID: 1, Path: "ns/doomed-model", GitPath: "x", Name: "doomed-model", Nickname: "doomed"}
	_, err = db.Core.NewInsert().Model(repo).Exec(ctx)
	require.Nil(t, err)

	linked := &database.LLMConfig{
		ModelName: "doomed-model", Type: database.LLMTypeAigatewayExternal, Enabled: true,
		RepoID:    repo.ID,
		Upstreams: []database.Upstream{{Source: types.UpstreamSourceExternal, URL: "http://x/v1", Enabled: true}},
	}
	insertGatewayConfig(ctx, t, db, linked)

	associated := true
	_, total, err := store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{HasAssociatedModel: &associated})
	require.Nil(t, err)
	require.Equal(t, 1, total)

	_, err = db.Core.NewUpdate().Model((*database.Repository)(nil)).
		Set("deleted_at = ?", time.Now()).
		Where("id = ?", repo.ID).Exec(ctx)
	require.Nil(t, err)

	_, total, err = store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{HasAssociatedModel: &associated})
	require.Nil(t, err)
	require.Equal(t, 0, total, "a soft-deleted repository no longer counts as associated")

	notAssociated := false
	configs, total, err := store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{HasAssociatedModel: &notAssociated})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"doomed-model"}, modelNames(configs))
}

func TestLLMConfigStore_ListGatewayModels_Availability(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	healthy := &database.LLMConfig{
		ModelName: "healthy-model", Type: database.LLMTypeAigatewayExternal, Enabled: true,
		Upstreams: []database.Upstream{{Source: types.UpstreamSourceExternal, URL: "http://ok/v1", Enabled: true}},
	}
	unhealthyOnly := &database.LLMConfig{
		ModelName: "unhealthy-model", Type: database.LLMTypeAigatewayExternal, Enabled: true,
		Upstreams: []database.Upstream{{
			Source: types.UpstreamSourceExternal, URL: "http://bad/v1", Enabled: true,
			HealthCheckEnabled: true,
		}},
	}
	mixed := &database.LLMConfig{
		ModelName: "mixed-model", Type: database.LLMTypeAigatewayExternal, Enabled: true,
		Upstreams: []database.Upstream{
			{
				Source: types.UpstreamSourceExternal, URL: "http://bad/v1", Enabled: true,
				HealthCheckEnabled: true,
			},
			{Source: types.UpstreamSourceExternal, URL: "http://ok/v1", Enabled: true},
		},
	}
	circuitOpen := &database.LLMConfig{
		ModelName: "circuit-model", Type: database.LLMTypeAigatewayExternal, Enabled: true,
		Upstreams: []database.Upstream{{
			Source: types.UpstreamSourceExternal, URL: "http://open/v1", Enabled: true,
			CircuitBreakerEnabled: true,
		}},
	}
	noUpstream := &database.LLMConfig{
		ModelName: "no-upstream-model", Type: database.LLMTypeAigatewayExternal, Enabled: true,
	}
	for _, cfg := range []*database.LLMConfig{healthy, unhealthyOnly, mixed, circuitOpen, noUpstream} {
		insertGatewayConfig(ctx, t, db, cfg)
	}
	// Health/circuit states are has-one relations Bun does not cascade; the
	// gateway availability rule reads them from their own tables.
	for _, cfg := range []*database.LLMConfig{unhealthyOnly, mixed} {
		for _, u := range cfg.Upstreams {
			if u.URL != "http://bad/v1" {
				continue
			}
			_, err = db.Core.NewInsert().Model(&database.AIGatewayUpstreamHealthState{
				UpstreamID: u.ID, HealthState: "unhealthy", LastCheckAt: time.Now(),
			}).Exec(ctx)
			require.Nil(t, err)
		}
	}
	for _, u := range circuitOpen.Upstreams {
		_, err = db.Core.NewInsert().Model(&database.AIGatewayUpstreamCircuitState{
			UpstreamID: u.ID, CircuitState: "open",
		}).Exec(ctx)
		require.Nil(t, err)
	}

	configs, total, err := store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{})
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.ElementsMatch(t, []string{"healthy-model", "mixed-model"}, modelNames(configs))
}

func TestLLMConfigStore_ListGatewayModels_PinnedFirst(t *testing.T) {
	f := newGatewayListFixture(t)
	ctx := context.TODO()
	db := f.db

	now := time.Now()
	// Pin the external model first, then the internal one. The entity IDs
	// follow pinnedModelKey: external models carry a ":provider" suffix,
	// internal models use the public model ID (the legacy_model_id). The
	// newest pin must surface first, mirroring ListByUserAndType's
	// created_at DESC. The created_at values are pinned afterwards because
	// the embedded times struct is not exportable to this test package.
	pins := []struct {
		entityID  string
		createdAt time.Time
	}{
		{"ext-model:openai", now.Add(-2 * time.Hour)},
		{"glm-svc", now.Add(-1 * time.Hour)},
	}
	for _, pin := range pins {
		pref := &database.AgentUserPreference{
			UserUUID:   "user-uuid-1",
			EntityType: types.AgentUserPreferenceEntityTypeAgentModel,
			EntityID:   pin.entityID,
			Action:     types.AgentUserPreferenceActionPin,
		}
		_, err := db.Core.NewInsert().Model(pref).Exec(ctx)
		require.Nil(t, err)
		_, err = db.Core.NewUpdate().Model((*database.AgentUserPreference)(nil)).
			Set("created_at = ?, updated_at = ?", pin.createdAt, pin.createdAt).
			Where("id = ?", pref.ID).Exec(ctx)
		require.Nil(t, err)
	}

	configs, total, err := f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{PinnedUserUUID: "user-uuid-1"})
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.Equal(t, []string{"svc-model", "ext-model"}, modelNames(configs),
		"pinned models lead in newest-pin-first order, unpinned follow")
	// The pinned flag rides on the same statement as the ordering.
	byName := map[string]bool{}
	for _, cfg := range configs {
		byName[cfg.ModelName] = cfg.IsPinned
	}
	require.Equal(t, map[string]bool{"svc-model": true, "ext-model": true}, byName,
		"both pinned models carry the flag from the listing statement")

	// Another user's pins change nothing.
	configs, total, err = f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{PinnedUserUUID: "user-uuid-2"})
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.ElementsMatch(t, []string{"ext-model", "svc-model"}, modelNames(configs))
	for _, cfg := range configs {
		require.False(t, cfg.IsPinned, "another user's pins must not flag this listing")
	}
}

// TestLLMConfigStore_ListGatewayModels_PricedFilter exercises the SaaS
// price-configured filter against real account_prices rows through the
// v_aigateway_model_prices view.
func TestLLMConfigStore_ListGatewayModels_PricedFilter(t *testing.T) {
	f := newGatewayListFixture(t)
	ctx := context.TODO()
	db := f.db

	// Only the external model has a configured price (sku_unit > 0): the
	// inference model passes unconditionally, the unpriced serverless and
	// external drop out. A stale older price row must not leak into the
	// latest-per-tier view.
	now := time.Now()
	prices := []database.AccountPrice{
		{ResourceID: "thirdparty://ext-model", SkuType: 1, SkuKind: 4, SkuStatus: 1,
			SkuPrice: 100, SkuUnit: 1_000_000, SkuPriceCurrency: "USD", SkuDesc: "token"},
		{ResourceID: "thirdparty://ext-model", SkuType: 1, SkuKind: 4, SkuStatus: 1,
			SkuPrice: 300, SkuUnit: 1_000_000, SkuPriceCurrency: "USD", SkuDesc: "token"},
		// sku_unit = 0 rows never count as configured.
		{ResourceID: "csghub://serverless/ns/svc-model", SkuType: 1, SkuKind: 4, SkuStatus: 1,
			SkuPrice: 500, SkuUnit: 0, SkuPriceCurrency: "USD", SkuDesc: "token"},
	}
	for i := range prices {
		_, err := db.Core.NewInsert().Model(&prices[i]).Exec(ctx)
		require.Nil(t, err)
		// created_at is pinned by an update because the embedded times struct
		// is not exportable to this test package.
		createdAt := now.Add(-time.Duration(2-i) * time.Hour)
		_, err = db.Core.NewUpdate().TableExpr("account_prices").
			Set("created_at = ?, updated_at = ?", createdAt, createdAt).
			Where("id = ?", prices[i].ID).Exec(ctx)
		require.Nil(t, err)
	}

	configs, total, err := f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{
		RequireConfiguredPrice: true,
		CallerUUID:             "owner-uuid-1",
	})
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.ElementsMatch(t, []string{"ext-model", "private-model"}, modelNames(configs))

	// Without the filter the unpriced serverless model is listed again.
	configs, total, err = f.store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{
		CallerUUID: "owner-uuid-1",
	})
	require.Nil(t, err)
	require.Equal(t, 3, total)
	require.ElementsMatch(t, []string{"ext-model", "private-model", "svc-model"}, modelNames(configs))

	// The view exposes only the latest row per (resource, kind).
	priceStore := database.NewAccountPriceStoreWithDB(db)
	rows, err := priceStore.ListGatewayModelPrices(ctx, []string{"thirdparty://ext-model"})
	require.Nil(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(300), rows[0].SkuPrice, "the newest price row wins")
	require.Equal(t, types.SKUKind(4), rows[0].SkuKind)
}

func TestLLMConfigStore_ListGatewayModels_PaginationAndCount(t *testing.T) {
	f := newGatewayListFixture(t)
	ctx := context.TODO()

	configs, total, err := f.store.ListGatewayModels(ctx, 1, 0, &types.GatewayModelListQuery{})
	require.Nil(t, err)
	require.Equal(t, 2, total, "the count ignores the window")
	require.Len(t, configs, 1)

	configs, total, err = f.store.ListGatewayModels(ctx, 1, 1, &types.GatewayModelListQuery{})
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.Len(t, configs, 1)

	// A window past the end yields no rows but a valid count.
	configs, total, err = f.store.ListGatewayModels(ctx, 1, 5, &types.GatewayModelListQuery{})
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.Empty(t, configs)

	// limit 0 is a real LIMIT 0 (no rows), not "unlimited" — that is -1.
	configs, total, err = f.store.ListGatewayModels(ctx, 0, 0, &types.GatewayModelListQuery{})
	require.Nil(t, err)
	require.Equal(t, 2, total)
	require.Empty(t, configs)
}

func TestLLMConfigStore_ListGatewayModels_SkipsConfigWithoutInternalInfo(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	// A csghub-source upstream without internal model info produces no model,
	// matching llmConfigToModel's skip.
	broken := &database.LLMConfig{
		ModelName: "broken-internal", Type: database.LLMTypeAigatewayExternal, Enabled: true,
		Upstreams: []database.Upstream{{Source: types.UpstreamSourceCSGHubDeploy, URL: "http://broken/v1", Enabled: true}},
	}
	ok := &database.LLMConfig{
		ModelName: "ok-external", Type: database.LLMTypeAigatewayExternal, Enabled: true,
		Upstreams: []database.Upstream{{Source: types.UpstreamSourceExternal, URL: "http://ok/v1", Enabled: true}},
	}
	for _, cfg := range []*database.LLMConfig{broken, ok} {
		insertGatewayConfig(ctx, t, db, cfg)
	}

	configs, total, err := store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"ok-external"}, modelNames(configs))
}

// Non-string elements of the external tasks array are skipped exactly like
// the previous Go conversion did; they neither break the query nor leak into
// the task string.
func TestLLMConfigStore_ListGatewayModels_NonStringTaskElementsSkipped(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	cfg := &database.LLMConfig{
		ModelName: "mixed-tasks-model", Type: database.LLMTypeAigatewayExternal, Enabled: true,
		Metadata:  map[string]any{"tasks": []any{42, "text-generation", map[string]any{"bad": 1}}},
		Upstreams: []database.Upstream{{Source: types.UpstreamSourceExternal, URL: "http://x/v1", Enabled: true}},
	}
	insertGatewayConfig(ctx, t, db, cfg)

	matched, total, err := store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{Task: "text-generation"})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"mixed-tasks-model"}, modelNames(matched))

	unmatched, total, err := store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{Task: "42"})
	require.Nil(t, err)
	require.Equal(t, 0, total, "a non-string element must not become a matchable task")
	require.Empty(t, unmatched)
}

// An internal model associates through its csghub_model_id only: a valid
// linked repository must not flip the verdict for it (branch alignment with
// the response conversion).
func TestLLMConfigStore_ListGatewayModels_HasAssociated_InternalBranchOnly(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()
	config, err := config.LoadConfig()
	require.Nil(t, err)
	store := database.NewLLMConfigStoreWithDB(db, config)

	repo := &database.Repository{UserID: 1, Path: "ns/some-repo", GitPath: "x", Name: "some-repo", Nickname: "some"}
	_, err = db.Core.NewInsert().Model(repo).Exec(ctx)
	require.Nil(t, err)

	// Internal model whose csghub_model_id is empty but which carries a
	// valid repo_id — malformed data that must not count as associated.
	internal := &database.LLMConfig{
		ModelName: "internal-no-csghub-id", Type: database.LLMTypeAigatewayExternal, Enabled: true,
		RepoID: repo.ID,
		Upstreams: []database.Upstream{{
			Source: types.UpstreamSourceCSGHubDeploy, URL: "http://x/v1", Enabled: true,
			Metadata: &types.UpstreamMetadata{
				InternalModelInfo: &types.InternalModelInfo{
					LegacyModelID: "internal-no-csghub-id",
					OwnerUUID:     "owner-uuid-1",
					SvcType:       types.InferenceType,
					SecureLevel:   types.EndpointPublic,
				},
			},
		}},
	}
	insertGatewayConfig(ctx, t, db, internal)

	associated := true
	_, total, err := store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{HasAssociatedModel: &associated})
	require.Nil(t, err)
	require.Equal(t, 0, total, "an internal model without csghub_model_id is not associated")

	notAssociated := false
	configs, total, err := store.ListGatewayModels(ctx, -1, 0, &types.GatewayModelListQuery{HasAssociatedModel: &notAssociated})
	require.Nil(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, []string{"internal-no-csghub-id"}, modelNames(configs))
}

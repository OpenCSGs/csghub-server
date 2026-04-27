package component

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestLLMServiceComponent_CreateLLMConfig(t *testing.T) {
	ctx := context.TODO()
	stores := tests.NewMockStores(t)
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
	}
	req := &types.CreateLLMConfigReq{
		ModelName:   "new-model",
		ApiEndpoint: "http://new.endpoint",
		Upstreams: []types.UpstreamConfig{
			{URL: "http://new.endpoint", Enabled: true, Weight: 1},
		},
		AuthHeader: "Bearer token",
		Type:       16,
		Enabled:    true,
		Provider:   "test-provider",
		RoutingPolicy: types.RoutingPolicy{
			Strategy:      "session_hash",
			SessionHeader: "X-Session-ID",
			HashReplicas:  128,
		},
		Metadata: map[string]any{"tasks": []any{"text-generation"}},
	}
	dbLLMConfig := &database.LLMConfig{
		ID:          123,
		ModelName:   "new-model",
		ApiEndpoint: "http://new.endpoint",
		Upstreams: []types.UpstreamConfig{
			{URL: "http://new.endpoint", Enabled: true, Weight: 1},
		},
		AuthHeader: "Bearer token",
		Type:       16,
		Enabled:    true,
		Provider:   "test-provider",
		RoutingPolicy: types.RoutingPolicy{
			Strategy:      "session_hash",
			SessionHeader: "X-Session-ID",
			HashReplicas:  128,
		},
		Metadata: map[string]any{"tasks": []any{"text-generation"}},
	}
	stores.LLMConfigMock().EXPECT().Create(ctx, database.LLMConfig{
		ModelName:   "new-model",
		ApiEndpoint: "http://new.endpoint",
		Upstreams: []types.UpstreamConfig{
			{URL: "http://new.endpoint", Enabled: true, Weight: 1},
		},
		AuthHeader: "Bearer token",
		Type:       16,
		Enabled:    true,
		Provider:   "test-provider",
		RoutingPolicy: types.RoutingPolicy{
			Strategy:      "session_hash",
			SessionHeader: "X-Session-ID",
			HashReplicas:  128,
		},
		Metadata: map[string]any{"tasks": []any{"text-generation"}},
	}).Return(dbLLMConfig, nil)
	res, err := mc.CreateLLMConfig(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.ID, int64(123))
	require.Equal(t, res.ModelName, "new-model")
}

func TestLLMServiceComponent_CreatePromptPrefix(t *testing.T) {
	ctx := context.TODO()
	stores := tests.NewMockStores(t)
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
	}
	req := &types.CreatePromptPrefixReq{
		ZH:   "zh",
		EN:   "en",
		Kind: "kind",
	}
	dbPromptPrefix := &database.PromptPrefix{
		ID:   123,
		ZH:   "zh",
		EN:   "en",
		Kind: "kind",
	}
	stores.PromptPrefixMock().EXPECT().Create(ctx, database.PromptPrefix{
		ZH:   "zh",
		EN:   "en",
		Kind: "kind",
	}).Return(dbPromptPrefix, nil)
	res, err := mc.CreatePromptPrefix(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.Kind, "kind")
}
func TestLLMServiceComponent_IndexLLMConfig(t *testing.T) {
	ctx := context.TODO()
	stores := tests.NewMockStores(t)
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
	}
	per := 1
	page := 1
	search := &types.SearchLLMConfig{
		Keyword: "",
	}
	dbLLMConfig := &database.LLMConfig{
		ID:          123,
		ModelName:   "new-model",
		ApiEndpoint: "http://new.endpoint",
		AuthHeader:  "Bearer token",
		Type:        666,
		Enabled:     true,
	}
	stores.LLMConfigMock().EXPECT().Index(ctx, per, page, search).Return([]*database.LLMConfig{dbLLMConfig}, 1, nil)
	res, total, err := mc.IndexLLMConfig(ctx, per, page, search)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res, []*database.LLMConfig{dbLLMConfig})
	require.Equal(t, total, 1)
}

func TestLLMServiceComponent_IndexPromptPrefix(t *testing.T) {
	ctx := context.TODO()
	stores := tests.NewMockStores(t)
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
	}
	per := 1
	page := 1
	dbPromptPrefix := &database.PromptPrefix{
		ID: 123,
		ZH: "zh",
	}
	search := &types.SearchPromptPrefix{}
	stores.PromptPrefixMock().EXPECT().Index(ctx, per, page, search).Return([]*database.PromptPrefix{dbPromptPrefix}, 1, nil)
	res, total, err := mc.IndexPromptPrefix(ctx, per, page, search)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res, []*database.PromptPrefix{dbPromptPrefix})
	require.Equal(t, total, 1)
}

func TestLLMServiceComponent_UpdateLLMConfig(t *testing.T) {
	ctx := context.TODO()
	stores := tests.NewMockStores(t)
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
	}
	newName := "new-model"
	metadata := map[string]any{"tasks": []any{"text-to-image"}}
	endpoints := []types.UpstreamConfig{
		{URL: "http://new-model.endpoint", Enabled: true, Weight: 1},
	}
	routingPolicy := types.RoutingPolicy{
		Strategy:      "session_hash",
		SessionHeader: "X-Session-ID",
		HashReplicas:  128,
	}
	req := &types.UpdateLLMConfigReq{
		ID:            123,
		ModelName:     &newName,
		Upstreams:     &endpoints,
		RoutingPolicy: &routingPolicy,
		Metadata:      &metadata,
	}
	dbLLMConfig := &database.LLMConfig{
		ID:            123,
		ModelName:     newName,
		Upstreams:     endpoints,
		RoutingPolicy: routingPolicy,
		Metadata:      metadata,
	}
	stores.LLMConfigMock().EXPECT().GetByID(ctx, int64(123)).Return(dbLLMConfig, nil)
	stores.LLMConfigMock().EXPECT().Update(ctx, database.LLMConfig{
		ID:            123,
		ModelName:     newName,
		ApiEndpoint:   "http://new-model.endpoint",
		Upstreams:     endpoints,
		RoutingPolicy: routingPolicy,
		Metadata:      metadata,
	}).Return(dbLLMConfig, nil)
	res, err := mc.UpdateLLMConfig(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.ID, int64(123))
	require.Equal(t, res.ModelName, "new-model")
}

func TestLLMServiceComponent_validateLLMEndpointConfig(t *testing.T) {
	mc := &llmServiceComponentImpl{}
	testCases := []struct {
		name        string
		apiEndpoint string
		upstreams   []types.UpstreamConfig
		wantErr     bool
		errContains string
	}{
		{
			name:        "api_endpoint and upstreams are both empty",
			apiEndpoint: "",
			upstreams:   nil,
			wantErr:     true,
			errContains: "api_endpoint or upstreams must be provided",
		},
		{
			name:        "upstream url is empty",
			apiEndpoint: "",
			upstreams: []types.UpstreamConfig{
				{URL: " ", Enabled: true},
			},
			wantErr:     true,
			errContains: "upstream url cannot be empty",
		},
		{
			name:        "all upstreams disabled and api_endpoint empty",
			apiEndpoint: "",
			upstreams: []types.UpstreamConfig{
				{URL: "http://a", Enabled: false},
				{URL: "http://b", Enabled: false},
			},
			wantErr:     true,
			errContains: "at least one enabled upstream must be provided",
		},
		{
			name:        "api_endpoint provided without upstreams",
			apiEndpoint: "http://primary",
			upstreams:   nil,
			wantErr:     false,
		},
		{
			name:        "valid enabled upstream",
			apiEndpoint: "",
			upstreams: []types.UpstreamConfig{
				{URL: "http://a", Enabled: true},
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := mc.validateLLMEndpointConfig(tc.apiEndpoint, tc.upstreams)
			if tc.wantErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errContains)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestLLMServiceComponent_normalizePrimaryEndpoint(t *testing.T) {
	mc := &llmServiceComponentImpl{}
	testCases := []struct {
		name        string
		apiEndpoint string
		upstreams   []types.UpstreamConfig
		want        string
	}{
		{
			name:        "keep api_endpoint when provided",
			apiEndpoint: "http://primary",
			upstreams: []types.UpstreamConfig{
				{URL: "http://a", Enabled: true},
			},
			want: "http://primary",
		},
		{
			name:        "pick first enabled upstream",
			apiEndpoint: "",
			upstreams: []types.UpstreamConfig{
				{URL: "http://disabled", Enabled: false},
				{URL: "http://enabled", Enabled: true},
			},
			want: "http://enabled",
		},
		{
			name:        "return empty when no enabled upstream",
			apiEndpoint: "",
			upstreams: []types.UpstreamConfig{
				{URL: "http://disabled", Enabled: false},
			},
			want: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := mc.normalizePrimaryEndpoint(tc.apiEndpoint, tc.upstreams)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestLLMServiceComponent_UpdatePromptPrefix(t *testing.T) {
	ctx := context.TODO()
	stores := tests.NewMockStores(t)
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
	}
	newKind := "new-kind"
	req := &types.UpdatePromptPrefixReq{
		ID:   123,
		Kind: &newKind,
	}
	dbPromptPrefix := &database.PromptPrefix{
		ID:   123,
		Kind: newKind,
	}
	stores.PromptPrefixMock().EXPECT().GetByID(ctx, int64(123)).Return(dbPromptPrefix, nil)
	stores.PromptPrefixMock().EXPECT().Update(ctx, database.PromptPrefix{
		ID:   123,
		Kind: newKind,
	}).Return(dbPromptPrefix, nil)
	res, err := mc.UpdatePromptPrefix(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.ID, int64(123))
	require.Equal(t, res.Kind, "new-kind")
}

func TestLLMServiceComponent_DeleteLLMConfig(t *testing.T) {
	ctx := context.TODO()
	stores := tests.NewMockStores(t)
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
	}
	stores.LLMConfigMock().EXPECT().Delete(ctx, int64(123)).Return(nil)
	err := mc.DeleteLLMConfig(ctx, int64(123))
	require.Nil(t, err)
}

func TestLLMServiceComponent_DeletePromptPrefix(t *testing.T) {
	ctx := context.TODO()
	stores := tests.NewMockStores(t)
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
	}
	stores.PromptPrefixMock().EXPECT().Delete(ctx, int64(123)).Return(nil)
	err := mc.DeletePromptPrefix(ctx, int64(123))
	require.Nil(t, err)
}

func TestLLMServiceComponent_ListExternalLLMs(t *testing.T) {
	ctx := context.TODO()
	stores := tests.NewMockStores(t)
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
		repoStore:         stores.Repo,
	}

	// Mock repository
	dbRepo := &database.Repository{
		ID:          456,
		Path:        "test/model",
		Name:        "model",
		Nickname:    "Test Model",
		Description: "A test model",
	}

	// Mock LLMConfig with repo_id and preloaded Repo
	typeVal := database.LLMTypeAigatewayExternal
	enabled := true
	dbLLMConfig := &database.LLMConfig{
		ID:          123,
		ModelName:   "external-model",
		Type:        typeVal,
		Enabled:     true,
		Provider:    "test-provider",
		RepoID:      456,
		Repo:        dbRepo,
	}

	// Mock search params
	search := &types.SearchLLMConfig{
		Type:    &typeVal,
		Enabled: &enabled,
	}

	// Setup mock expectations for LLMConfig store
	stores.LLMConfigMock().EXPECT().IndexWithRepo(ctx, math.MaxInt, 1, search).Return([]*database.LLMConfig{dbLLMConfig}, 1, nil)

	// Mock tags
	dbTags := []database.Tag{
		{Name: "text-generation", Category: "task", Group: "nlp"},
		{Name: "transformer", Category: "framework", Group: "architecture"},
	}
	stores.RepoMock().EXPECT().Tags(ctx, int64(456)).Return(dbTags, nil)

	// Call the method
	res, err := mc.ListExternalLLMs(ctx)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Len(t, res, 1)
	require.Equal(t, int64(123), res[0].ID)
	require.Equal(t, "external-model", res[0].ModelName)
	require.NotNil(t, res[0].Repo)
	require.Equal(t, int64(456), res[0].Repo.ID)
	require.Len(t, res[0].Repo.Tags, 2)
	require.Equal(t, "text-generation", res[0].Repo.Tags[0].Name)
}

func TestLLMServiceComponent_ListExternalLLMs_NoRepo(t *testing.T) {
	ctx := context.TODO()
	stores := tests.NewMockStores(t)
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
		repoStore:         stores.Repo,
	}

	// Mock LLMConfig without repo
	typeVal := database.LLMTypeAigatewayExternal
	enabled := true
	dbLLMConfig := &database.LLMConfig{
		ID:          123,
		ModelName:   "external-model-no-repo",
		Type:        typeVal,
		Enabled:     true,
		Provider:    "test-provider",
		RepoID:      0,
		Repo:        nil,
	}

	search := &types.SearchLLMConfig{
		Type:    &typeVal,
		Enabled: &enabled,
	}

	stores.LLMConfigMock().EXPECT().IndexWithRepo(ctx, math.MaxInt, 1, search).Return([]*database.LLMConfig{dbLLMConfig}, 1, nil)

	res, err := mc.ListExternalLLMs(ctx)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Len(t, res, 1)
	require.Equal(t, int64(123), res[0].ID)
	require.Nil(t, res[0].Repo)
}

func TestLLMServiceComponent_ListExternalLLMs_Error(t *testing.T) {
	ctx := context.TODO()
	stores := tests.NewMockStores(t)
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
		repoStore:         stores.Repo,
	}

	typeVal := database.LLMTypeAigatewayExternal
	enabled := true
	search := &types.SearchLLMConfig{
		Type:    &typeVal,
		Enabled: &enabled,
	}

	stores.LLMConfigMock().EXPECT().IndexWithRepo(ctx, math.MaxInt, 1, search).Return(nil, 0, fmt.Errorf("db error"))

	res, err := mc.ListExternalLLMs(ctx)
	require.NotNil(t, err)
	require.Nil(t, res)
}

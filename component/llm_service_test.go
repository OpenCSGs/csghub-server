package component

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestLLMServiceComponent_CreateLLMConfig(t *testing.T) {
	ctx := context.TODO()
	stores := tests.NewMockStores(t)
	upstreamStore := mockdatabase.NewMockUpstreamStore(t)
	upstreamStore.EXPECT().Create(ctx, mock.Anything).Return(nil).Maybe()
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
		upstreamStore:     upstreamStore,
	}
	req := &types.CreateLLMConfigReq{
		ModelName: "new-model",
		Type:      16,
		Enabled:   true,
		Upstreams: []types.UpstreamConfig{
			{URL: "http://upstream.example.com/v1", Enabled: true, Weight: 1},
		},
		RoutingPolicy: types.RoutingPolicy{
			Strategy:      "session_hash",
			SessionHeader: "X-Session-ID",
			HashReplicas:  128,
		},
		Metadata:   map[string]any{"tasks": []any{"text-generation"}},
		ModelSizeB: 7.5,
	}
	dbLLMConfig := &database.LLMConfig{
		ID:        123,
		ModelName: "new-model",
		Type:      16,
		Enabled:   true,
		RoutingPolicy: types.RoutingPolicy{
			Strategy:      "session_hash",
			SessionHeader: "X-Session-ID",
			HashReplicas:  128,
		},
		Metadata:   map[string]any{"tasks": []any{"text-generation"}},
		ModelSizeB: 7.5,
	}
	stores.LLMConfigMock().EXPECT().Create(ctx, database.LLMConfig{
		ModelName: "new-model",
		Type:      16,
		Enabled:   true,
		RoutingPolicy: types.RoutingPolicy{
			Strategy:      "session_hash",
			SessionHeader: "X-Session-ID",
			HashReplicas:  128,
		},
		Metadata:   map[string]any{"tasks": []any{"text-generation"}},
		ModelSizeB: 7.5,
	}).Return(dbLLMConfig, nil)
	res, err := mc.CreateLLMConfig(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.ID, int64(123))
	require.Equal(t, res.ModelName, "new-model")
	require.Equal(t, 7.5, res.ModelSizeB)
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
		ID:        123,
		ModelName: "new-model",
		Type:      666,
		Enabled:   true,
	}
	stores.LLMConfigMock().EXPECT().Index(ctx, per, page, search).Return([]*database.LLMConfig{dbLLMConfig}, 1, nil)
	res, total, err := mc.IndexLLMConfig(ctx, per, page, search)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, []*types.LLMConfig{{ID: 123, ModelName: "new-model", OfficialName: "new-model", Type: 666, Enabled: true, IsAvailable: true, Upstreams: []types.UpstreamConfig{}}}, res)
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
	upstreamStore := mockdatabase.NewMockUpstreamStore(t)
	upstreamStore.EXPECT().ListByLLMConfigID(ctx, int64(123)).Return([]*database.Upstream{}, nil).Maybe()
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
		upstreamStore:     upstreamStore,
	}
	newName := "new-model"
	metadata := map[string]any{"tasks": []any{"text-to-image"}}
	routingPolicy := types.RoutingPolicy{
		Strategy:      "session_hash",
		SessionHeader: "X-Session-ID",
		HashReplicas:  128,
	}
	newSize := 13.0
	req := &types.UpdateLLMConfigReq{
		ID:            123,
		ModelName:     &newName,
		RoutingPolicy: &routingPolicy,
		Metadata:      &metadata,
		ModelSizeB:    &newSize,
	}
	dbLLMConfig := &database.LLMConfig{
		ID:            123,
		ModelName:     newName,
		RoutingPolicy: routingPolicy,
		Metadata:      metadata,
		ModelSizeB:    newSize,
	}
	stores.LLMConfigMock().EXPECT().GetByID(ctx, int64(123)).Return(dbLLMConfig, nil)
	stores.LLMConfigMock().EXPECT().Update(ctx, database.LLMConfig{
		ID:            123,
		ModelName:     newName,
		RoutingPolicy: routingPolicy,
		Metadata:      metadata,
		ModelSizeB:    newSize,
	}).Return(dbLLMConfig, nil)
	res, err := mc.UpdateLLMConfig(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, res.ID, int64(123))
	require.Equal(t, res.ModelName, "new-model")
	require.Equal(t, 13.0, res.ModelSizeB)
}

func TestLLMServiceComponent_CreateUpstream(t *testing.T) {
	ctx := context.TODO()
	stores := tests.NewMockStores(t)
	upstreamStore := mockdatabase.NewMockUpstreamStore(t)
	upstreamStore.EXPECT().Create(ctx, mock.Anything).Return(nil).Maybe()
	stores.LLMConfigMock().EXPECT().GetByID(ctx, int64(100)).Return(&database.LLMConfig{ID: 100}, nil)
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
		upstreamStore:     upstreamStore,
	}
	req := &types.CreateUpstreamReq{
		LLMConfigID: 100,
		URL:         "http://upstream.example.com/v1",
		Weight:      2,
		Enabled:     true,
		Provider:    "test-provider",
	}
	res, err := mc.CreateUpstream(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, "http://upstream.example.com/v1", res.URL)
	require.Equal(t, 2, res.Weight)
}

func TestLLMServiceComponent_UpdateUpstream(t *testing.T) {
	ctx := context.TODO()
	stores := tests.NewMockStores(t)
	upstreamStore := mockdatabase.NewMockUpstreamStore(t)
	upstreamStore.EXPECT().GetByID(ctx, int64(10)).Return(&database.Upstream{
		ID:          10,
		LLMConfigID: 100,
		URL:         "http://old-endpoint",
		Weight:      1,
		Enabled:     true,
	}, nil)
	upstreamStore.EXPECT().Update(ctx, mock.Anything).Return(nil).Maybe()
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
		upstreamStore:     upstreamStore,
	}
	newURL := "http://new-endpoint"
	newWeight := 3
	req := &types.UpdateUpstreamReq{
		ID:     10,
		URL:    &newURL,
		Weight: &newWeight,
	}
	res, err := mc.UpdateUpstream(ctx, req)
	require.Nil(t, err)
	require.NotNil(t, res)
	require.Equal(t, int64(10), res.ID)
	require.Equal(t, "http://new-endpoint", res.URL)
	require.Equal(t, 3, res.Weight)
}

func TestLLMServiceComponent_DeleteUpstream(t *testing.T) {
	ctx := context.TODO()
	upstreamStore := mockdatabase.NewMockUpstreamStore(t)
	upstreamStore.EXPECT().Delete(ctx, int64(10)).Return(nil)
	mc := &llmServiceComponentImpl{
		upstreamStore: upstreamStore,
	}
	err := mc.DeleteUpstream(ctx, 10)
	require.Nil(t, err)
}

func TestLLMServiceComponent_validateLLMEndpointConfig(t *testing.T) {
	mc := &llmServiceComponentImpl{}
	testCases := []struct {
		name        string
		upstreams   []types.UpstreamConfig
		wantErr     bool
		errContains string
	}{
		{
			name:        "api_endpoint and upstreams are both empty",
			upstreams:   nil,
			wantErr:     true,
			errContains: "upstreams must be provided",
		},
		{
			name: "upstream url is empty",
			upstreams: []types.UpstreamConfig{
				{URL: " ", Enabled: true},
			},
			wantErr:     true,
			errContains: "upstream url cannot be empty",
		},
		{
			name: "all upstreams disabled and api_endpoint empty",
			upstreams: []types.UpstreamConfig{
				{URL: "http://a", Enabled: false},
				{URL: "http://b", Enabled: false},
			},
			wantErr:     true,
			errContains: "at least one enabled upstream must be provided",
		},
		{
			name:        "api_endpoint provided without upstreams",
			upstreams:   nil,
			wantErr:     true,
			errContains: "upstreams must be provided",
		},
		{
			name: "valid enabled upstream",
			upstreams: []types.UpstreamConfig{
				{URL: "http://a", Enabled: true},
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := mc.validateLLMEndpointConfig(tc.upstreams)
			if tc.wantErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errContains)
				return
			}
			require.NoError(t, err)
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
	upstreamStore := mockdatabase.NewMockUpstreamStore(t)
	upstreamStore.EXPECT().DeleteByLLMConfigID(ctx, int64(123)).Return(nil)
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
		upstreamStore:     upstreamStore,
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
		ID:        123,
		ModelName: "external-model",
		Type:      typeVal,
		Enabled:   true,
		RepoID:    456,
		Repo:      dbRepo,
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
		ID:        123,
		ModelName: "external-model-no-repo",
		Type:      typeVal,
		Enabled:   true,
		RepoID:    0,
		Repo:      nil,
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

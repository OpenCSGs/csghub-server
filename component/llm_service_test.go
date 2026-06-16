package component

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	aigatewaytypes "opencsg.com/csghub-server/aigateway/types"
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
	callCount := 0
	upstreamStore.EXPECT().GetByID(ctx, int64(10)).RunAndReturn(func(ctx context.Context, id int64) (*database.Upstream, error) {
		callCount++
		if callCount == 1 {
			return &database.Upstream{
				ID:          10,
				LLMConfigID: 100,
				URL:         "http://old-endpoint",
				Weight:      1,
				Enabled:     true,
			}, nil
		}
		return &database.Upstream{
			ID:          10,
			LLMConfigID: 100,
			URL:         "http://new-endpoint",
			Weight:      3,
			Enabled:     true,
		}, nil
	}).Times(2)
	upstreamStore.EXPECT().Update(ctx, mock.Anything).Return(nil).Maybe()
	healthStateStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
	circuitStateStore := mockdatabase.NewMockAIGatewayUpstreamCircuitStateStore(t)
	mc := &llmServiceComponentImpl{
		llmConfigStore:    stores.LLMConfig,
		promptPrefixStore: stores.PromptPrefix,
		upstreamStore:     upstreamStore,
		healthStateStore:  healthStateStore,
		circuitStateStore: circuitStateStore,
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

func TestComputeUpstreamAvailability(t *testing.T) {
	testCases := []struct {
		name              string
		upstream          types.UpstreamConfig
		wantAvailable     bool
		wantReason        string
	}{
		{
			name: "disabled upstream is unavailable",
			upstream: types.UpstreamConfig{
				Enabled: false,
			},
			wantAvailable: false,
			wantReason:    aigatewaytypes.ReasonUpstreamDisabled,
		},
		{
			name: "enabled upstream without health check or circuit breaker is available",
			upstream: types.UpstreamConfig{
				Enabled:               true,
				HealthCheckEnabled:    false,
				CircuitBreakerEnabled: false,
			},
			wantAvailable: true,
			wantReason:    "",
		},
		{
			name: "enabled upstream with healthy state is available",
			upstream: types.UpstreamConfig{
				Enabled:            true,
				HealthCheckEnabled: true,
				HealthState:        string(aigatewaytypes.HealthStateHealthy),
			},
			wantAvailable: true,
			wantReason:    "",
		},
		{
			name: "enabled upstream with unhealthy state is unavailable",
			upstream: types.UpstreamConfig{
				Enabled:            true,
				HealthCheckEnabled: true,
				HealthState:        string(aigatewaytypes.HealthStateUnhealthy),
			},
			wantAvailable: false,
			wantReason:    aigatewaytypes.ReasonHealthStateUnhealthy,
		},
		{
			name: "enabled upstream with open circuit is unavailable",
			upstream: types.UpstreamConfig{
				Enabled:               true,
				CircuitBreakerEnabled: true,
				CircuitState:          string(aigatewaytypes.CircuitStateOpen),
			},
			wantAvailable: false,
			wantReason:    aigatewaytypes.ReasonCircuitBreakerOpen,
		},
		{
			name: "unknown health state makes upstream unavailable",
			upstream: types.UpstreamConfig{
				Enabled:            true,
				HealthCheckEnabled: true,
				HealthState:        string(aigatewaytypes.HealthStateUnknown),
			},
			wantAvailable: false,
			wantReason:    aigatewaytypes.ReasonHealthStateUnknown,
		},
		{
			name: "unknown circuit state makes upstream unavailable",
			upstream: types.UpstreamConfig{
				Enabled:               true,
				CircuitBreakerEnabled: true,
				CircuitState:          string(aigatewaytypes.CircuitStateUnknown),
			},
			wantAvailable: false,
			wantReason:    aigatewaytypes.ReasonCircuitStateUnknown,
		},
		{
			name: "circuit unknown takes priority over health unknown",
			upstream: types.UpstreamConfig{
				Enabled:               true,
				HealthCheckEnabled:    true,
				CircuitBreakerEnabled: true,
				HealthState:           string(aigatewaytypes.HealthStateUnknown),
				CircuitState:          string(aigatewaytypes.CircuitStateUnknown),
			},
			wantAvailable: false,
			wantReason:    aigatewaytypes.ReasonCircuitStateUnknown,
		},
		{
			name: "health unknown with healthy circuit state",
			upstream: types.UpstreamConfig{
				Enabled:               true,
				HealthCheckEnabled:    true,
				CircuitBreakerEnabled: true,
				HealthState:           string(aigatewaytypes.HealthStateUnknown),
				CircuitState:          string(aigatewaytypes.CircuitStateClosed),
			},
			wantAvailable: false,
			wantReason:    aigatewaytypes.ReasonHealthStateUnknown,
		},
		{
			name: "empty health state treated as unknown when health check enabled",
			upstream: types.UpstreamConfig{
				Enabled:            true,
				HealthCheckEnabled: true,
				HealthState:        "",
			},
			wantAvailable: true,
			wantReason:    "",
		},
		{
			name: "degraded health state still available",
			upstream: types.UpstreamConfig{
				Enabled:            true,
				HealthCheckEnabled: true,
				HealthState:        string(aigatewaytypes.HealthStateDegraded),
			},
			wantAvailable: true,
			wantReason:    "",
		},
		{
			name: "half-open circuit state still available",
			upstream: types.UpstreamConfig{
				Enabled:               true,
				CircuitBreakerEnabled: true,
				CircuitState:          string(aigatewaytypes.CircuitStateHalfOpen),
			},
			wantAvailable: true,
			wantReason:    "",
		},
		{
			name: "health check disabled ignores unknown health state",
			upstream: types.UpstreamConfig{
				Enabled:            true,
				HealthCheckEnabled: false,
				HealthState:        string(aigatewaytypes.HealthStateUnknown),
			},
			wantAvailable: true,
			wantReason:    "",
		},
		{
			name: "circuit breaker disabled ignores unknown circuit state",
			upstream: types.UpstreamConfig{
				Enabled:               true,
				CircuitBreakerEnabled: false,
				CircuitState:          string(aigatewaytypes.CircuitStateUnknown),
			},
			wantAvailable: true,
			wantReason:    "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			available, reason := computeUpstreamAvailability(tc.upstream)
			require.Equal(t, tc.wantAvailable, available)
			require.Equal(t, tc.wantReason, reason)
		})
	}
}

func TestComputeUpstreamAvailabilityStatus(t *testing.T) {
	testCases := []struct {
		name           string
		upstream       types.UpstreamConfig
		wantStatus     string
	}{
		{
			name: "disabled upstream",
			upstream: types.UpstreamConfig{
				Enabled: false,
			},
			wantStatus: string(aigatewaytypes.UpstreamStatusDisabled),
		},
		{
			name: "unknown health state returns unknown status",
			upstream: types.UpstreamConfig{
				Enabled:            true,
				HealthCheckEnabled: true,
				HealthState:        string(aigatewaytypes.HealthStateUnknown),
			},
			wantStatus: string(aigatewaytypes.UpstreamStatusUnknown),
		},
		{
			name: "unknown circuit state returns unknown status",
			upstream: types.UpstreamConfig{
				Enabled:               true,
				CircuitBreakerEnabled: true,
				CircuitState:          string(aigatewaytypes.CircuitStateUnknown),
			},
			wantStatus: string(aigatewaytypes.UpstreamStatusUnknown),
		},
		{
			name: "circuit unknown takes priority over health unknown",
			upstream: types.UpstreamConfig{
				Enabled:               true,
				HealthCheckEnabled:    true,
				CircuitBreakerEnabled: true,
				HealthState:           string(aigatewaytypes.HealthStateUnknown),
				CircuitState:          string(aigatewaytypes.CircuitStateUnknown),
			},
			wantStatus: string(aigatewaytypes.UpstreamStatusUnknown),
		},
		{
			name: "open circuit returns unavailable",
			upstream: types.UpstreamConfig{
				Enabled:               true,
				CircuitBreakerEnabled: true,
				CircuitState:          string(aigatewaytypes.CircuitStateOpen),
			},
			wantStatus: string(aigatewaytypes.UpstreamStatusUnavailable),
		},
		{
			name: "unhealthy health returns unavailable",
			upstream: types.UpstreamConfig{
				Enabled:            true,
				HealthCheckEnabled: true,
				HealthState:        string(aigatewaytypes.HealthStateUnhealthy),
			},
			wantStatus: string(aigatewaytypes.UpstreamStatusUnavailable),
		},
		{
			name: "degraded health returns degraded",
			upstream: types.UpstreamConfig{
				Enabled:            true,
				HealthCheckEnabled: true,
				HealthState:        string(aigatewaytypes.HealthStateDegraded),
			},
			wantStatus: string(aigatewaytypes.UpstreamStatusDegraded),
		},
		{
			name: "healthy upstream returns available",
			upstream: types.UpstreamConfig{
				Enabled:            true,
				HealthCheckEnabled: true,
				HealthState:        string(aigatewaytypes.HealthStateHealthy),
			},
			wantStatus: string(aigatewaytypes.UpstreamStatusAvailable),
		},
		{
			name: "no health check or circuit breaker returns available",
			upstream: types.UpstreamConfig{
				Enabled:               true,
				HealthCheckEnabled:    false,
				CircuitBreakerEnabled: false,
			},
			wantStatus: string(aigatewaytypes.UpstreamStatusAvailable),
		},
		{
			name: "health check disabled ignores unknown health state",
			upstream: types.UpstreamConfig{
				Enabled:            true,
				HealthCheckEnabled: false,
				HealthState:        string(aigatewaytypes.HealthStateUnknown),
			},
			wantStatus: string(aigatewaytypes.UpstreamStatusAvailable),
		},
		{
			name: "circuit breaker disabled ignores unknown circuit state",
			upstream: types.UpstreamConfig{
				Enabled:               true,
				CircuitBreakerEnabled: false,
				CircuitState:          string(aigatewaytypes.CircuitStateUnknown),
			},
			wantStatus: string(aigatewaytypes.UpstreamStatusAvailable),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			status := computeUpstreamAvailabilityStatus(tc.upstream)
			require.Equal(t, tc.wantStatus, status)
		})
	}
}

func TestComputeLLMAvailability(t *testing.T) {
	testCases := []struct {
		name          string
		upstreams     []types.UpstreamConfig
		wantAvailable bool
		wantReason    string
	}{
		{
			name:          "no upstreams is available",
			upstreams:     nil,
			wantAvailable: true,
			wantReason:    "",
		},
		{
			name: "one available upstream makes LLM available",
			upstreams: []types.UpstreamConfig{
				{Enabled: true, HealthCheckEnabled: true, HealthState: string(aigatewaytypes.HealthStateHealthy)},
			},
			wantAvailable: true,
			wantReason:    "",
		},
		{
			name: "all upstreams unknown makes LLM unavailable",
			upstreams: []types.UpstreamConfig{
				{Enabled: true, HealthCheckEnabled: true, HealthState: string(aigatewaytypes.HealthStateUnknown)},
			},
			wantAvailable: false,
			wantReason:    aigatewaytypes.ReasonAllUpstreamsUnavailable,
		},
		{
			name: "mixed available and unknown upstreams makes LLM available",
			upstreams: []types.UpstreamConfig{
				{Enabled: true, HealthCheckEnabled: true, HealthState: string(aigatewaytypes.HealthStateUnknown)},
				{Enabled: true, HealthCheckEnabled: true, HealthState: string(aigatewaytypes.HealthStateHealthy)},
			},
			wantAvailable: true,
			wantReason:    "",
		},
		{
			name: "all upstreams disabled makes LLM unavailable",
			upstreams: []types.UpstreamConfig{
				{Enabled: false},
			},
			wantAvailable: false,
			wantReason:    aigatewaytypes.ReasonAllUpstreamsUnavailable,
		},
		{
			name: "all upstreams unhealthy makes LLM unavailable",
			upstreams: []types.UpstreamConfig{
				{Enabled: true, HealthCheckEnabled: true, HealthState: string(aigatewaytypes.HealthStateUnhealthy)},
			},
			wantAvailable: false,
			wantReason:    aigatewaytypes.ReasonAllUpstreamsUnavailable,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			available, reason := computeLLMAvailability(tc.upstreams)
			require.Equal(t, tc.wantAvailable, available)
			require.Equal(t, tc.wantReason, reason)
		})
	}
}

func TestBuildUpstreamConfigs_UnknownStateWhenNoDBRecord(t *testing.T) {
	testCases := []struct {
		name                string
		dbUpstream          database.Upstream
		wantHealthState     string
		wantCircuitState    string
		wantIsAvailable     bool
		wantAvailabilitySts string
	}{
		{
			name: "health check enabled with no DB health record -> unknown health state and unavailable",
			dbUpstream: database.Upstream{
				ID:                 1,
				URL:                "http://upstream.example.com/v1",
				Enabled:            true,
				HealthCheckEnabled: true,
				HealthState:        nil,
				CircuitState:       nil,
			},
			wantHealthState:     string(aigatewaytypes.HealthStateUnknown),
			wantCircuitState:    "",
			wantIsAvailable:     false,
			wantAvailabilitySts: string(aigatewaytypes.UpstreamStatusUnknown),
		},
		{
			name: "circuit breaker enabled with no DB circuit record -> unknown circuit state and unavailable",
			dbUpstream: database.Upstream{
				ID:                    2,
				URL:                   "http://upstream.example.com/v1",
				Enabled:               true,
				CircuitBreakerEnabled: true,
				HealthState:           nil,
				CircuitState:          nil,
			},
			wantHealthState:     "",
			wantCircuitState:    string(aigatewaytypes.CircuitStateUnknown),
			wantIsAvailable:     false,
			wantAvailabilitySts: string(aigatewaytypes.UpstreamStatusUnknown),
		},
		{
			name: "both enabled with no DB records -> both unknown and unavailable",
			dbUpstream: database.Upstream{
				ID:                    3,
				URL:                   "http://upstream.example.com/v1",
				Enabled:               true,
				HealthCheckEnabled:    true,
				CircuitBreakerEnabled: true,
				HealthState:           nil,
				CircuitState:          nil,
			},
			wantHealthState:     string(aigatewaytypes.HealthStateUnknown),
			wantCircuitState:    string(aigatewaytypes.CircuitStateUnknown),
			wantIsAvailable:     false,
			wantAvailabilitySts: string(aigatewaytypes.UpstreamStatusUnknown),
		},
		{
			name: "neither enabled with no DB records -> available",
			dbUpstream: database.Upstream{
				ID:                    4,
				URL:                   "http://upstream.example.com/v1",
				Enabled:               true,
				HealthCheckEnabled:    false,
				CircuitBreakerEnabled: false,
				HealthState:           nil,
				CircuitState:          nil,
			},
			wantHealthState:     "",
			wantCircuitState:    "",
			wantIsAvailable:     true,
			wantAvailabilitySts: string(aigatewaytypes.UpstreamStatusAvailable),
		},
		{
			name: "health check enabled with existing healthy record -> available",
			dbUpstream: database.Upstream{
				ID:                 5,
				URL:                "http://upstream.example.com/v1",
				Enabled:            true,
				HealthCheckEnabled: true,
				HealthState:        &database.AIGatewayUpstreamHealthState{HealthState: string(aigatewaytypes.HealthStateHealthy)},
				CircuitState:       nil,
			},
			wantHealthState:     string(aigatewaytypes.HealthStateHealthy),
			wantCircuitState:    "",
			wantIsAvailable:     true,
			wantAvailabilitySts: string(aigatewaytypes.UpstreamStatusAvailable),
		},
		{
			name: "disabled upstream with no DB records -> disabled status",
			dbUpstream: database.Upstream{
				ID:                    6,
				URL:                   "http://upstream.example.com/v1",
				Enabled:               false,
				HealthCheckEnabled:    true,
				CircuitBreakerEnabled: true,
				HealthState:           nil,
				CircuitState:          nil,
			},
			wantHealthState:     string(aigatewaytypes.HealthStateUnknown),
			wantCircuitState:    string(aigatewaytypes.CircuitStateUnknown),
			wantIsAvailable:     false,
			wantAvailabilitySts: string(aigatewaytypes.UpstreamStatusDisabled),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := buildUpstreamConfigs([]database.Upstream{tc.dbUpstream})
			require.Len(t, result, 1)
			uc := result[0]
			require.Equal(t, tc.wantHealthState, uc.HealthState)
			require.Equal(t, tc.wantCircuitState, uc.CircuitState)
			require.Equal(t, tc.wantIsAvailable, uc.IsAvailable)
			require.Equal(t, tc.wantAvailabilitySts, uc.AvailabilityStatus)
		})
	}
}

func TestUpdateUpstream_ResetStaleStateOnReEnable(t *testing.T) {
	ctx := context.TODO()

	t.Run("disabled_to_enabled_resets_health_state_to_unknown", func(t *testing.T) {
		upstreamStore := mockdatabase.NewMockUpstreamStore(t)
		healthStateStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
		circuitStateStore := mockdatabase.NewMockAIGatewayUpstreamCircuitStateStore(t)

		// Upstream is disabled with stale healthy state in DB
		disabledUp := &database.Upstream{
			ID:                    10,
			URL:                   "http://upstream.example.com/v1",
			Enabled:               false,
			HealthCheckEnabled:    true,
			CircuitBreakerEnabled: true,
			HealthState:           &database.AIGatewayUpstreamHealthState{ID: 1, UpstreamID: 10, HealthState: string(aigatewaytypes.HealthStateHealthy)},
			CircuitState:          &database.AIGatewayUpstreamCircuitState{ID: 1, UpstreamID: 10, CircuitState: string(aigatewaytypes.CircuitStateClosed)},
		}
		enabledUp := &database.Upstream{
			ID:                    10,
			URL:                   "http://upstream.example.com/v1",
			Enabled:               true,
			HealthCheckEnabled:    true,
			CircuitBreakerEnabled: true,
			HealthState:           &database.AIGatewayUpstreamHealthState{ID: 1, UpstreamID: 10, HealthState: string(aigatewaytypes.HealthStateUnknown)},
			CircuitState:          &database.AIGatewayUpstreamCircuitState{ID: 1, UpstreamID: 10, CircuitState: string(aigatewaytypes.CircuitStateUnknown)},
		}
		getByIDCallCount := 0
		upstreamStore.EXPECT().GetByID(ctx, int64(10)).RunAndReturn(func(ctx context.Context, id int64) (*database.Upstream, error) {
			getByIDCallCount++
			if getByIDCallCount == 1 {
				return disabledUp, nil
			}
			return enabledUp, nil
		}).Times(2)
		upstreamStore.EXPECT().Update(ctx, mock.Anything).Return(nil)

		// Health state should be fetched and updated to unknown
		healthStateStore.EXPECT().GetByUpstreamID(ctx, int64(10)).Return(&database.AIGatewayUpstreamHealthState{
			ID:          1,
			UpstreamID:  10,
			HealthState: string(aigatewaytypes.HealthStateHealthy),
		}, nil)
		healthStateStore.EXPECT().Update(ctx, mock.MatchedBy(func(s *database.AIGatewayUpstreamHealthState) bool {
			return s.UpstreamID == 10 && s.HealthState == string(aigatewaytypes.HealthStateUnknown)
		})).Return(nil)

		// Circuit state should be fetched and updated to unknown
		circuitStateStore.EXPECT().GetByUpstreamID(ctx, int64(10)).Return(&database.AIGatewayUpstreamCircuitState{
			ID:           1,
			UpstreamID:   10,
			CircuitState: string(aigatewaytypes.CircuitStateClosed),
		}, nil)
		circuitStateStore.EXPECT().Update(ctx, mock.MatchedBy(func(s *database.AIGatewayUpstreamCircuitState) bool {
			return s.UpstreamID == 10 && s.CircuitState == string(aigatewaytypes.CircuitStateUnknown)
		})).Return(nil)

		mc := &llmServiceComponentImpl{
			upstreamStore:     upstreamStore,
			healthStateStore:  healthStateStore,
			circuitStateStore: circuitStateStore,
		}
		enabled := true
		res, err := mc.UpdateUpstream(ctx, &types.UpdateUpstreamReq{ID: 10, Enabled: &enabled})
		require.Nil(t, err)
		require.NotNil(t, res)
		require.Equal(t, string(aigatewaytypes.UpstreamStatusUnknown), res.AvailabilityStatus)
		require.False(t, res.IsAvailable)
	})

	t.Run("health_check_re_enabled_resets_health_state_to_unknown", func(t *testing.T) {
		upstreamStore := mockdatabase.NewMockUpstreamStore(t)
		healthStateStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
		circuitStateStore := mockdatabase.NewMockAIGatewayUpstreamCircuitStateStore(t)

		// Upstream is enabled, health check was off, stale healthy record in DB
		callCount := 0
		upstreamStore.EXPECT().GetByID(ctx, int64(10)).RunAndReturn(func(ctx context.Context, id int64) (*database.Upstream, error) {
			callCount++
			if callCount == 1 {
				return &database.Upstream{
					ID:                    10,
					URL:                   "http://upstream.example.com/v1",
					Enabled:               true,
					HealthCheckEnabled:    false,
					CircuitBreakerEnabled: false,
					HealthState:           &database.AIGatewayUpstreamHealthState{ID: 1, UpstreamID: 10, HealthState: string(aigatewaytypes.HealthStateHealthy)},
				}, nil
			}
			return &database.Upstream{
				ID:                    10,
				URL:                   "http://upstream.example.com/v1",
				Enabled:               true,
				HealthCheckEnabled:    true,
				CircuitBreakerEnabled: false,
				HealthState:           &database.AIGatewayUpstreamHealthState{ID: 1, UpstreamID: 10, HealthState: string(aigatewaytypes.HealthStateUnknown)},
			}, nil
		}).Times(2)
		upstreamStore.EXPECT().Update(ctx, mock.Anything).Return(nil)

		// Health state should be reset to unknown when health check is re-enabled
		healthStateStore.EXPECT().GetByUpstreamID(ctx, int64(10)).Return(&database.AIGatewayUpstreamHealthState{
			ID:          1,
			UpstreamID:  10,
			HealthState: string(aigatewaytypes.HealthStateHealthy),
		}, nil)
		healthStateStore.EXPECT().Update(ctx, mock.MatchedBy(func(s *database.AIGatewayUpstreamHealthState) bool {
			return s.UpstreamID == 10 && s.HealthState == string(aigatewaytypes.HealthStateUnknown)
		})).Return(nil)

		mc := &llmServiceComponentImpl{
			upstreamStore:     upstreamStore,
			healthStateStore:  healthStateStore,
			circuitStateStore: circuitStateStore,
		}
		healthCheckOn := true
		res, err := mc.UpdateUpstream(ctx, &types.UpdateUpstreamReq{ID: 10, HealthCheckEnabled: &healthCheckOn})
		require.Nil(t, err)
		require.NotNil(t, res)
		require.Equal(t, string(aigatewaytypes.UpstreamStatusUnknown), res.AvailabilityStatus)
		require.False(t, res.IsAvailable)
	})

	t.Run("circuit_breaker_re_enabled_resets_circuit_state_to_unknown", func(t *testing.T) {
		upstreamStore := mockdatabase.NewMockUpstreamStore(t)
		healthStateStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
		circuitStateStore := mockdatabase.NewMockAIGatewayUpstreamCircuitStateStore(t)

		// Upstream is enabled, circuit breaker was off, stale closed record in DB
		callCount := 0
		upstreamStore.EXPECT().GetByID(ctx, int64(10)).RunAndReturn(func(ctx context.Context, id int64) (*database.Upstream, error) {
			callCount++
			if callCount == 1 {
				return &database.Upstream{
					ID:                    10,
					URL:                   "http://upstream.example.com/v1",
					Enabled:               true,
					HealthCheckEnabled:    false,
					CircuitBreakerEnabled: false,
					CircuitState:          &database.AIGatewayUpstreamCircuitState{ID: 1, UpstreamID: 10, CircuitState: string(aigatewaytypes.CircuitStateClosed)},
				}, nil
			}
			return &database.Upstream{
				ID:                    10,
				URL:                   "http://upstream.example.com/v1",
				Enabled:               true,
				HealthCheckEnabled:    false,
				CircuitBreakerEnabled: true,
				CircuitState:          &database.AIGatewayUpstreamCircuitState{ID: 1, UpstreamID: 10, CircuitState: string(aigatewaytypes.CircuitStateUnknown)},
			}, nil
		}).Times(2)
		upstreamStore.EXPECT().Update(ctx, mock.Anything).Return(nil)

		// Circuit state should be reset to unknown when circuit breaker is re-enabled
		circuitStateStore.EXPECT().GetByUpstreamID(ctx, int64(10)).Return(&database.AIGatewayUpstreamCircuitState{
			ID:           1,
			UpstreamID:   10,
			CircuitState: string(aigatewaytypes.CircuitStateClosed),
		}, nil)
		circuitStateStore.EXPECT().Update(ctx, mock.MatchedBy(func(s *database.AIGatewayUpstreamCircuitState) bool {
			return s.UpstreamID == 10 && s.CircuitState == string(aigatewaytypes.CircuitStateUnknown)
		})).Return(nil)

		mc := &llmServiceComponentImpl{
			upstreamStore:     upstreamStore,
			healthStateStore:  healthStateStore,
			circuitStateStore: circuitStateStore,
		}
		cbOn := true
		res, err := mc.UpdateUpstream(ctx, &types.UpdateUpstreamReq{ID: 10, CircuitBreakerEnabled: &cbOn})
		require.Nil(t, err)
		require.NotNil(t, res)
		require.Equal(t, string(aigatewaytypes.UpstreamStatusUnknown), res.AvailabilityStatus)
		require.False(t, res.IsAvailable)
	})

	t.Run("no_reset_when_already_enabled_and_checks_unchanged", func(t *testing.T) {
		upstreamStore := mockdatabase.NewMockUpstreamStore(t)
		healthStateStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
		circuitStateStore := mockdatabase.NewMockAIGatewayUpstreamCircuitStateStore(t)

		// Already enabled with both checks enabled, just updating URL
		callCount := 0
		upstreamStore.EXPECT().GetByID(ctx, int64(10)).RunAndReturn(func(ctx context.Context, id int64) (*database.Upstream, error) {
			callCount++
			if callCount == 1 {
				return &database.Upstream{
					ID:                    10,
					URL:                   "http://old-endpoint",
					Enabled:               true,
					HealthCheckEnabled:    true,
					CircuitBreakerEnabled: true,
					HealthState:           &database.AIGatewayUpstreamHealthState{ID: 1, UpstreamID: 10, HealthState: string(aigatewaytypes.HealthStateHealthy)},
					CircuitState:          &database.AIGatewayUpstreamCircuitState{ID: 1, UpstreamID: 10, CircuitState: string(aigatewaytypes.CircuitStateClosed)},
				}, nil
			}
			return &database.Upstream{
				ID:                    10,
				URL:                   "http://new-endpoint",
				Enabled:               true,
				HealthCheckEnabled:    true,
				CircuitBreakerEnabled: true,
				HealthState:           &database.AIGatewayUpstreamHealthState{ID: 1, UpstreamID: 10, HealthState: string(aigatewaytypes.HealthStateHealthy)},
				CircuitState:          &database.AIGatewayUpstreamCircuitState{ID: 1, UpstreamID: 10, CircuitState: string(aigatewaytypes.CircuitStateClosed)},
			}, nil
		}).Times(2)
		upstreamStore.EXPECT().Update(ctx, mock.Anything).Return(nil)

		mc := &llmServiceComponentImpl{
			upstreamStore:     upstreamStore,
			healthStateStore:  healthStateStore,
			circuitStateStore: circuitStateStore,
		}
		newURL := "http://new-endpoint"
		res, err := mc.UpdateUpstream(ctx, &types.UpdateUpstreamReq{ID: 10, URL: &newURL})
		require.Nil(t, err)
		require.NotNil(t, res)
		require.Equal(t, string(aigatewaytypes.UpstreamStatusAvailable), res.AvailabilityStatus)
		require.True(t, res.IsAvailable)
	})

	t.Run("no_reset_when_no_db_record_exists", func(t *testing.T) {
		upstreamStore := mockdatabase.NewMockUpstreamStore(t)
		healthStateStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
		circuitStateStore := mockdatabase.NewMockAIGatewayUpstreamCircuitStateStore(t)

		// Upstream disabled, no health/circuit records in DB
		callCount := 0
		upstreamStore.EXPECT().GetByID(ctx, int64(10)).RunAndReturn(func(ctx context.Context, id int64) (*database.Upstream, error) {
			callCount++
			if callCount == 1 {
				return &database.Upstream{
					ID:                    10,
					URL:                   "http://upstream.example.com/v1",
					Enabled:               false,
					HealthCheckEnabled:    true,
					CircuitBreakerEnabled: true,
					HealthState:           nil,
					CircuitState:          nil,
				}, nil
			}
			return &database.Upstream{
				ID:                    10,
				URL:                   "http://upstream.example.com/v1",
				Enabled:               true,
				HealthCheckEnabled:    true,
				CircuitBreakerEnabled: true,
				HealthState:           nil,
				CircuitState:          nil,
			}, nil
		}).Times(2)
		upstreamStore.EXPECT().Update(ctx, mock.Anything).Return(nil)

		// GetByUpstreamID returns error (no record) - should not call Update
		healthStateStore.EXPECT().GetByUpstreamID(ctx, int64(10)).Return(nil, fmt.Errorf("not found"))
		circuitStateStore.EXPECT().GetByUpstreamID(ctx, int64(10)).Return(nil, fmt.Errorf("not found"))

		mc := &llmServiceComponentImpl{
			upstreamStore:     upstreamStore,
			healthStateStore:  healthStateStore,
			circuitStateStore: circuitStateStore,
		}
		enabled := true
		res, err := mc.UpdateUpstream(ctx, &types.UpdateUpstreamReq{ID: 10, Enabled: &enabled})
		require.Nil(t, err)
		require.NotNil(t, res)
		// buildUpstreamConfigs handles nil → unknown
		require.Equal(t, string(aigatewaytypes.UpstreamStatusUnknown), res.AvailabilityStatus)
		require.False(t, res.IsAvailable)
	})

	t.Run("disabling_upstream_does_not_reset_state", func(t *testing.T) {
		upstreamStore := mockdatabase.NewMockUpstreamStore(t)
		healthStateStore := mockdatabase.NewMockAIGatewayUpstreamHealthStateStore(t)
		circuitStateStore := mockdatabase.NewMockAIGatewayUpstreamCircuitStateStore(t)

		// Upstream enabled, disabling it
		callCount := 0
		upstreamStore.EXPECT().GetByID(ctx, int64(10)).RunAndReturn(func(ctx context.Context, id int64) (*database.Upstream, error) {
			callCount++
			if callCount == 1 {
				return &database.Upstream{
					ID:                    10,
					URL:                   "http://upstream.example.com/v1",
					Enabled:               true,
					HealthCheckEnabled:    true,
					CircuitBreakerEnabled: true,
					HealthState:           &database.AIGatewayUpstreamHealthState{ID: 1, UpstreamID: 10, HealthState: string(aigatewaytypes.HealthStateHealthy)},
					CircuitState:          &database.AIGatewayUpstreamCircuitState{ID: 1, UpstreamID: 10, CircuitState: string(aigatewaytypes.CircuitStateClosed)},
				}, nil
			}
			return &database.Upstream{
				ID:                    10,
				URL:                   "http://upstream.example.com/v1",
				Enabled:               false,
				HealthCheckEnabled:    true,
				CircuitBreakerEnabled: true,
				HealthState:           &database.AIGatewayUpstreamHealthState{ID: 1, UpstreamID: 10, HealthState: string(aigatewaytypes.HealthStateHealthy)},
				CircuitState:          &database.AIGatewayUpstreamCircuitState{ID: 1, UpstreamID: 10, CircuitState: string(aigatewaytypes.CircuitStateClosed)},
			}, nil
		}).Times(2)
		upstreamStore.EXPECT().Update(ctx, mock.Anything).Return(nil)

		mc := &llmServiceComponentImpl{
			upstreamStore:     upstreamStore,
			healthStateStore:  healthStateStore,
			circuitStateStore: circuitStateStore,
		}
		disabled := false
		res, err := mc.UpdateUpstream(ctx, &types.UpdateUpstreamReq{ID: 10, Enabled: &disabled})
		require.Nil(t, err)
		require.NotNil(t, res)
		require.Equal(t, string(aigatewaytypes.UpstreamStatusDisabled), res.AvailabilityStatus)
		require.False(t, res.IsAvailable)
	})
}

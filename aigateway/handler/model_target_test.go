package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"

	"opencsg.com/csghub-server/aigateway/component/router"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestResolveModelTarget_ExternalModel(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{
			ID: "backend-model",
		},
		Upstreams: []commontypes.UpstreamConfig{{URL: "https://api.example.com/v1/chat/completions", Enabled: true, ModelName: "backend-model"}},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()

	resolved, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "model1", http.Header{})

	require.NoError(t, err)
	require.Equal(t, model, resolved.Model)
	require.Equal(t, "https://api.example.com/v1/chat/completions", resolved.Target)
	require.Empty(t, resolved.Host)
	require.Equal(t, "backend-model", resolved.ModelName)
}

func TestResolveModelTarget_ModelNotFound(t *testing.T) {
	tester, _, _ := setupTest(t)
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "missing-model").Return(nil, nil).Once()

	_, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "missing-model", http.Header{})

	require.Error(t, err)
	targetErr, ok := err.(*modelTargetError)
	require.True(t, ok)
	require.Equal(t, "model_not_found", targetErr.APIError.Code)
}

func TestResolveModelTargetWithOptionsRequiredUpstreamID(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{ID: "public-model"},
		Upstreams: []commontypes.UpstreamConfig{
			{ID: 1, URL: "https://adapter.example.com/v1/chat/completions", Enabled: true, Provider: "anthropic", ModelName: "claude"},
			{ID: 2, URL: "https://native.example.com/v1/chat/completions", Enabled: true, Provider: "openai", ModelName: "gpt-4o"},
		},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "public-model").Return(model, nil).Once()

	resolved, err := tester.handler.resolveModelTargetWithOptions(context.Background(), "testuser", "public-model", http.Header{}, modelTargetResolveOptions{
		RequiredUpstreamID: 2,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), resolved.Upstream.ID)
	require.Equal(t, "https://native.example.com/v1/chat/completions", resolved.Target)
	require.Equal(t, "gpt-4o", resolved.ModelName)

	decision, err := responsespkg.ResolveRouting(responsespkg.RoutingTarget{
		ModelID: resolved.Model.ID,
		Target:  resolved.Target,
	})
	require.NoError(t, err)
	require.Equal(t, responsespkg.ResponsesModeChatAdapter, decision.Mode)
	require.Empty(t, decision.NativeURL)
}

func TestResolveModelTargetWithOptionsRequiredUpstreamIDUnavailable(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{ID: "public-model"},
		Upstreams: []commontypes.UpstreamConfig{
			{ID: 1, URL: "https://adapter.example.com/v1/chat/completions", Enabled: true, Provider: "anthropic", ModelName: "claude"},
		},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "public-model").Return(model, nil).Once()

	_, err := tester.handler.resolveModelTargetWithOptions(context.Background(), "testuser", "public-model", http.Header{}, modelTargetResolveOptions{
		RequiredUpstreamID: 2,
	})
	require.Error(t, err)
	targetErr, ok := err.(*modelTargetError)
	require.True(t, ok)
	require.Equal(t, "required_upstream_unavailable", targetErr.APIError.Code)
}

func TestResolveModelTarget_ExternalModelSessionHash(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{
			ID: "backend-model",
		},
		Upstreams: []commontypes.UpstreamConfig{
			{URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true, ModelName: "backend-model1"},
			{URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true, ModelName: "backend-model2"},
		},
		RoutingPolicy: commontypes.RoutingPolicy{
			Strategy:     router.RoutingStrategySessionHash,
			HashReplicas: 64,
		},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Twice()

	headers := http.Header{}
	headers.Set("X-Session-ID", "session-1")

	first, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "model1", headers)
	require.NoError(t, err)

	second, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "model1", headers)
	require.NoError(t, err)

	require.Equal(t, first.Target, second.Target)
	require.Equal(t, first.Target, first.Model.Endpoint)
}

func TestResolveModelTarget_ExternalModelSessionHash_FallbackToUsername(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{
			ID: "backend-model",
		},
		Upstreams: []commontypes.UpstreamConfig{
			{URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true, ModelName: "backend-model1"},
			{URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true, ModelName: "backend-model2"},
		},
		RoutingPolicy: commontypes.RoutingPolicy{
			Strategy:     router.RoutingStrategySessionHash,
			HashReplicas: 64,
		},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Twice()

	first, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "model1", http.Header{})
	require.NoError(t, err)

	second, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "model1", http.Header{})
	require.NoError(t, err)

	require.Equal(t, first.Target, second.Target)
	require.NotEmpty(t, first.Target)
}

func TestResolveModelTarget_ExternalModelFallsBackToFirstEnabledEndpoint(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{
			ID: "backend-model",
		},
		Upstreams: []commontypes.UpstreamConfig{
			{URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true, ModelName: "backend-model1"},
			{URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true, ModelName: "backend-model2"},
		},
		RoutingPolicy: commontypes.RoutingPolicy{
			Strategy: router.RoutingStrategySingle,
		},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()

	resolved, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "model1", http.Header{})
	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/node-a/v1/chat/completions", resolved.Target)
}

func TestResolveModelTarget_EndpointOverridesAuthAndProvider(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{
			ID: "backend-model",
		},
		ExternalModelInfo: types.ExternalModelInfo{
			AuthHead: "Bearer model-level-token",
		},
		Upstreams: []commontypes.UpstreamConfig{
			{
				URL:        "https://api.example.com/node-a/v1/chat/completions",
				Enabled:    true,
				Provider:   "endpoint-provider",
				AuthHeader: "Bearer endpoint-level-token",
				ModelName:  "backend-model1",
			},
			{
				URL:       "https://api.example.com/node-b/v1/chat/completions",
				Enabled:   true,
				ModelName: "backend-model2",
			},
		},
		RoutingPolicy: commontypes.RoutingPolicy{
			Strategy: router.RoutingStrategySingle,
		},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()

	resolved, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "model1", http.Header{})
	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/node-a/v1/chat/completions", resolved.Target)
	require.Equal(t, "Bearer endpoint-level-token", resolved.Model.AuthHead)
	require.Equal(t, "endpoint-provider", resolved.Model.Provider)
	require.Equal(t, "backend-model1", resolved.ModelName)
}

func TestResolveModelTarget_EndpointOverridesModelName(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{
			ID: "backend-model",
		},
		Upstreams: []commontypes.UpstreamConfig{
			{
				URL:       "https://api.example.com/node-a/v1/chat/completions",
				Enabled:   true,
				ModelName: "provider-specific-model",
			},
		},
		RoutingPolicy: commontypes.RoutingPolicy{
			Strategy: router.RoutingStrategySingle,
		},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()

	resolved, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "model1", http.Header{})
	require.NoError(t, err)
	require.Equal(t, "provider-specific-model", resolved.ModelName)
	require.Len(t, resolved.AttemptTargets, 0)
	require.Equal(t, "provider-specific-model", resolved.Upstream.ModelName)
}

func TestResolveModelTarget_GetModelByIDError(t *testing.T) {
	tester, _, _ := setupTest(t)
	expectedErr := errors.New("storage unavailable")
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(nil, expectedErr).Once()

	_, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "model1", http.Header{})

	require.Error(t, err)
	targetErr, ok := err.(*modelTargetError)
	require.True(t, ok)
	require.Equal(t, http.StatusInternalServerError, targetErr.Status)
	require.Equal(t, "internal_error", targetErr.APIError.Code)
	require.ErrorIs(t, targetErr.Cause, expectedErr)
}

func TestResolveModelTarget_CSGHubModel(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{
			ID: "raw-model-id",
		},
		InternalModelInfo: types.InternalModelInfo{
			CSGHubModelID: "namespace/model",
			ClusterID:     "cluster-1",
			SvcName:       "svc-model",
		},
		Endpoint: "https://model.internal/v1/chat/completions",
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
	tester.mocks.mockClsComp.EXPECT().GetClusterByID(mock.Anything, "cluster-1").Return(&database.ClusterInfo{
		ClusterID: "cluster-1",
	}, nil).Once()

	resolved, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "model1", http.Header{})

	require.NoError(t, err)
	require.Equal(t, "https://model.internal/v1/chat/completions", resolved.Target)
	require.Empty(t, resolved.Host)
	require.Equal(t, "namespace/model", resolved.ModelName)
	require.Empty(t, resolved.AttemptTargets)
}

func TestResolveModelTarget_CSGHubModelClusterNotFound(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{
			ID: "raw-model-id",
		},
		InternalModelInfo: types.InternalModelInfo{
			CSGHubModelID: "namespace/model",
			ClusterID:     "cluster-1",
			SvcName:       "svc-model",
		},
		Endpoint: "https://model.internal/v1/chat/completions",
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()
	tester.mocks.mockClsComp.EXPECT().GetClusterByID(mock.Anything, "cluster-1").Return(nil, errors.New("cluster missing")).Once()

	_, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "model1", http.Header{})

	require.Error(t, err)
	targetErr, ok := err.(*modelTargetError)
	require.True(t, ok)
	require.Equal(t, "cluster_not_found", targetErr.APIError.Code)
	require.Equal(t, http.StatusBadRequest, targetErr.Status)
}

func TestResolveCSGHubModelTarget_StableAcrossCalls(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{
			ID: "raw-model-id",
		},
		InternalModelInfo: types.InternalModelInfo{
			CSGHubModelID: "namespace/model",
			ClusterID:     "cluster-1",
		},
	}
	targetReq := commontypes.EndpointReq{
		ClusterID: "cluster-1",
		Target:    "https://origin.internal/v1/chat/completions",
		Endpoint:  "https://origin.internal/v1/chat/completions",
	}
	tester.mocks.mockClsComp.EXPECT().GetClusterByID(mock.Anything, "cluster-1").Return(&database.ClusterInfo{
		ClusterID:   "cluster-1",
		AppEndpoint: "",
	}, nil).Times(5)

	var firstTarget string
	var firstHost string
	var firstModelName string
	for i := range 5 {
		target, host, modelName, err := tester.handler.resolveCSGHubModelTarget(context.Background(), model, targetReq)
		require.NoError(t, err)
		if i == 0 {
			firstTarget = target
			firstHost = host
			firstModelName = modelName
			continue
		}
		require.Equal(t, firstTarget, target)
		require.Equal(t, firstHost, host)
		require.Equal(t, firstModelName, modelName)
	}
	require.Equal(t, "namespace/model", firstModelName)
}

func TestResolveEndpointModelTarget_SameSessionKeyStableAcrossCalls(t *testing.T) {
	tester, _, _ := setupTest(t)
	headers := http.Header{}
	headers.Set("X-Session-ID", "session-stable-key")

	var firstTarget string
	var firstEndpointURL string
	for i := range 5 {
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "backend-model",
			},
			Upstreams: []commontypes.UpstreamConfig{
				{URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true, ModelName: "backend-model-a"},
				{URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true, ModelName: "backend-model-b"},
			},
			RoutingPolicy: commontypes.RoutingPolicy{
				Strategy:     router.RoutingStrategySessionHash,
				HashReplicas: 64,
			},
		}
		targetReq := commontypes.EndpointReq{}
		result, err := tester.handler.resolveEndpointModelTarget(context.Background(), endpointTargetResolveInput{
			Model:     model,
			ModelID:   "model1",
			Username:  "testuser",
			Headers:   headers,
			TargetReq: &targetReq,
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotEmpty(t, result.Target)
		require.NotEmpty(t, result.Upstream.URL)

		if i == 0 {
			firstTarget = result.Target
			firstEndpointURL = result.Upstream.URL
			continue
		}
		require.Equal(t, firstTarget, result.Target)
		require.Equal(t, firstEndpointURL, result.Upstream.URL)
	}
}

func TestResolveModelTarget_ModelNotRunningWhenNoEndpoint(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{
			ID: "backend-model",
		},
		Endpoint:  "",
		Upstreams: []commontypes.UpstreamConfig{},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "model1").Return(model, nil).Once()

	_, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "model1", http.Header{})

	require.Error(t, err)
	targetErr, ok := err.(*modelTargetError)
	require.True(t, ok)
	require.Equal(t, "model_not_running", targetErr.APIError.Code)
	require.Equal(t, http.StatusBadRequest, targetErr.Status)
}

func TestResolveModelTarget_PricedExternalLLM(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{
			ID: "external-model",
			Metadata: map[string]any{
				types.MetaKeyLLMType:           commontypes.ProviderTypeExternalLLM,
				types.MetaKeyPricingConfigured: true,
			},
		},
		ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"},
		Upstreams:         []commontypes.UpstreamConfig{{URL: "https://api.example.com/v1/chat/completions", Enabled: true, ModelName: "provider-model"}},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "external-model").Return(model, nil).Once()

	resolved, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "external-model", http.Header{})

	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/v1/chat/completions", resolved.Target)
	require.Equal(t, "provider-model", resolved.ModelName)
}

func TestResolveModelTarget_InferenceWithoutPricingFlag(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{
			ID: "inference-model",
			Metadata: map[string]any{
				types.MetaKeyLLMType: commontypes.ProviderTypeInference,
			},
		},
		InternalModelInfo: types.InternalModelInfo{
			CSGHubModelID: "namespace/model",
			ClusterID:     "cluster-1",
			SvcName:       "svc-model",
		},
		Endpoint: "https://model.internal/v1/chat/completions",
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, "testuser", "inference-model").Return(model, nil).Once()
	tester.mocks.mockClsComp.EXPECT().GetClusterByID(mock.Anything, "cluster-1").Return(&database.ClusterInfo{
		ClusterID: "cluster-1",
	}, nil).Once()

	resolved, err := tester.handler.resolveModelTarget(context.Background(), "testuser", "inference-model", http.Header{})

	require.NoError(t, err)
	require.Equal(t, "https://model.internal/v1/chat/completions", resolved.Target)
	require.Equal(t, "namespace/model", resolved.ModelName)
}

func TestExtractSessionKeyForModel(t *testing.T) {
	model := &types.Model{
		RoutingPolicy: commontypes.RoutingPolicy{
			SessionHeader: "X-Custom-Session",
		},
	}

	t.Run("uses custom header first", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("X-Custom-Session", "custom-session")
		headers.Set("X-Session-ID", "default-session")

		sessionKey := extractSessionKeyForModel(model, headers, "fallback-user")
		require.Equal(t, "custom-session", sessionKey)
	})

	t.Run("falls back to username when header missing", func(t *testing.T) {
		sessionKey := extractSessionKeyForModel(model, http.Header{}, "fallback-user")
		require.Equal(t, "fallback-user", sessionKey)
	})

	t.Run("truncates long header value", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("X-Custom-Session", strings.Repeat("a", maxSessionKeyLength+10))

		sessionKey := extractSessionKeyForModel(model, headers, "")
		require.Len(t, sessionKey, maxSessionKeyLength)
	})
}

func TestApplyEndpointOverrides(t *testing.T) {
	model := &types.Model{
		ExternalModelInfo: types.ExternalModelInfo{
			AuthHead: "model-token",
		},
	}

	applyEndpointOverrides(model, commontypes.UpstreamConfig{
		Provider:   "endpoint-provider",
		AuthHeader: "endpoint-token",
	})
	require.Equal(t, "endpoint-provider", model.Provider)
	require.Equal(t, "endpoint-token", model.AuthHead)

	applyEndpointOverrides(model, commontypes.UpstreamConfig{})
	require.Equal(t, "", model.Provider)
	require.Equal(t, "endpoint-token", model.AuthHead)
}

func TestHandleModelTargetError(t *testing.T) {
	t.Run("wraps unknown error to internal error response", func(t *testing.T) {
		_, c, w := setupTest(t)

		handleModelTargetError(c, context.Background(), "model1", "resolve failed", errors.New("unexpected"))

		require.Equal(t, http.StatusInternalServerError, w.Code)
		var resp struct {
			Error types.Error `json:"error"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, "internal_error", resp.Error.Code)
	})

	t.Run("uses modelTargetError status and payload", func(t *testing.T) {
		_, c, w := setupTest(t)
		targetErr := newInvalidRequestModelTargetError(
			"model_not_running",
			"model 'model1' not running",
			modelTargetErrorOptions{},
		)

		handleModelTargetError(c, context.Background(), "model1", "resolve failed", targetErr)

		require.Equal(t, http.StatusBadRequest, w.Code)
		var resp struct {
			Error types.Error `json:"error"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Equal(t, "model_not_running", resp.Error.Code)
	})
}

func TestFilterAvailableUpstreams_EmptyUpstreams(t *testing.T) {
	h := &OpenAIHandlerImpl{}
	result, err := h.filterAvailableUpstreams(context.Background(), &types.Model{}, nil)
	require.NoError(t, err)
	require.Nil(t, result)
}

func TestFilterAvailableUpstreams_NilModel(t *testing.T) {
	h := &OpenAIHandlerImpl{}
	upstreams := []commontypes.UpstreamConfig{
		{URL: "https://api.example.com/v1", Enabled: true},
	}
	result, err := h.filterAvailableUpstreams(context.Background(), nil, upstreams)
	require.NoError(t, err)
	require.Equal(t, upstreams, result)
}

func TestFilterAvailableUpstreams_FiltersUnhealthy(t *testing.T) {
	h := &OpenAIHandlerImpl{}
	upstreams := []commontypes.UpstreamConfig{
		{URL: "https://api.example.com/healthy", Enabled: true, HealthCheckEnabled: true, HealthState: "healthy"},
		{URL: "https://api.example.com/unhealthy", Enabled: true, HealthCheckEnabled: true, HealthState: "unhealthy"},
		{URL: "https://api.example.com/no_check", Enabled: true, HealthCheckEnabled: false, HealthState: "unhealthy"},
	}

	result, err := h.filterAvailableUpstreams(context.Background(), &types.Model{}, upstreams)
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, "https://api.example.com/healthy", result[0].URL)
	require.Equal(t, "https://api.example.com/no_check", result[1].URL) // not checked, passes
}

func TestFilterAvailableUpstreams_FiltersCircuitOpen(t *testing.T) {
	h := &OpenAIHandlerImpl{}
	upstreams := []commontypes.UpstreamConfig{
		{URL: "https://api.example.com/closed", Enabled: true, CircuitBreakerEnabled: true, CircuitState: "closed"},
		{URL: "https://api.example.com/open", Enabled: true, CircuitBreakerEnabled: true, CircuitState: "open"},
		{URL: "https://api.example.com/cb_disabled", Enabled: true, CircuitBreakerEnabled: false, CircuitState: "open"},
	}

	result, err := h.filterAvailableUpstreams(context.Background(), &types.Model{}, upstreams)
	require.NoError(t, err)
	require.Len(t, result, 3)
	require.Equal(t, "https://api.example.com/closed", result[0].URL)
	require.Equal(t, "https://api.example.com/open", result[1].URL)        // no runtime circuit state, allow proxy
	require.Equal(t, "https://api.example.com/cb_disabled", result[2].URL) // CB disabled, passes
}

func TestFilterAvailableUpstreams_AllUnavailableReturnsError(t *testing.T) {
	h := &OpenAIHandlerImpl{}
	upstreams := []commontypes.UpstreamConfig{
		{URL: "https://api.example.com/dead1", Enabled: true, HealthCheckEnabled: true, HealthState: "unhealthy"},
		{URL: "https://api.example.com/dead2", Enabled: true, CircuitBreakerEnabled: true, CircuitState: "open"},
	}

	result, err := h.filterAvailableUpstreams(context.Background(),
		&types.Model{BaseModel: types.BaseModel{ID: "test-model"}}, upstreams)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "https://api.example.com/dead2", result[0].URL) // DB open but no runtime confirmation, allow proxy
}

func TestFilterAvailableUpstreams_DegradedStillPasses(t *testing.T) {
	h := &OpenAIHandlerImpl{}
	upstreams := []commontypes.UpstreamConfig{
		{URL: "https://api.example.com/degraded", Enabled: true, HealthCheckEnabled: true, HealthState: "degraded"},
	}

	result, err := h.filterAvailableUpstreams(context.Background(), &types.Model{}, upstreams)
	require.NoError(t, err)
	require.Len(t, result, 1) // degraded still passes
}

func TestFilterAvailableUpstreams_AllEnabledDefaults(t *testing.T) {
	// No HealthState/CircuitState set and no checks enabled — defaults to available.
	h := &OpenAIHandlerImpl{}
	upstreams := []commontypes.UpstreamConfig{
		{URL: "https://api.example.com/v1", Enabled: true},
		{URL: "https://api.example.com/v2", Enabled: true},
	}

	result, err := h.filterAvailableUpstreams(context.Background(), &types.Model{}, upstreams)
	require.NoError(t, err)
	require.Len(t, result, 2)
}

type stubGetCircuitState struct {
	state *types.ProviderCircuitStatus
	err   error
}

func (s *stubGetCircuitState) Start(_ context.Context) error { return nil }
func (s *stubGetCircuitState) Stop() error                   { return nil }
func (s *stubGetCircuitState) RecordRequestResult(_ context.Context, _ int64, _ string, _ bool, _ error) error {
	return nil
}
func (s *stubGetCircuitState) GetCircuitState(_ context.Context, _ int64) (*types.ProviderCircuitStatus, error) {
	return s.state, s.err
}

func TestFilterAvailableUpstreams_DBCircuitOpen_RTCacheClosed_AllowsUpstream(t *testing.T) {
	h := &OpenAIHandlerImpl{
		availabilityManager: &stubGetCircuitState{
			state: &types.ProviderCircuitStatus{CircuitState: types.CircuitStateClosed},
		},
	}
	upstreams := []commontypes.UpstreamConfig{
		{ID: 1, URL: "https://api.example.com/cb_open", Enabled: true, CircuitBreakerEnabled: true, CircuitState: "open"},
	}

	result, err := h.filterAvailableUpstreams(context.Background(), &types.Model{}, upstreams)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "https://api.example.com/cb_open", result[0].URL)
}

func TestFilterAvailableUpstreams_DBCircuitOpen_RTCacheOpen_SkipsUpstream(t *testing.T) {
	h := &OpenAIHandlerImpl{
		availabilityManager: &stubGetCircuitState{
			state: &types.ProviderCircuitStatus{CircuitState: types.CircuitStateOpen},
		},
	}
	upstreams := []commontypes.UpstreamConfig{
		{ID: 1, URL: "https://api.example.com/cb_open", Enabled: true, CircuitBreakerEnabled: true, CircuitState: "open"},
		{URL: "https://api.example.com/healthy", Enabled: true, HealthCheckEnabled: true, HealthState: "healthy"},
	}

	result, err := h.filterAvailableUpstreams(context.Background(), &types.Model{}, upstreams)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "https://api.example.com/healthy", result[0].URL)
}

func TestFilterAvailableUpstreams_DBCircuitOpen_RTCacheError_AllowsUpstream(t *testing.T) {
	h := &OpenAIHandlerImpl{
		availabilityManager: &stubGetCircuitState{
			err: errors.New("redis unreachable"),
		},
	}
	upstreams := []commontypes.UpstreamConfig{
		{ID: 1, URL: "https://api.example.com/cb_open", Enabled: true, CircuitBreakerEnabled: true, CircuitState: "open"},
		{URL: "https://api.example.com/healthy", Enabled: true, HealthCheckEnabled: true, HealthState: "healthy"},
	}

	result, err := h.filterAvailableUpstreams(context.Background(), &types.Model{}, upstreams)
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, "https://api.example.com/cb_open", result[0].URL)
	require.Equal(t, "https://api.example.com/healthy", result[1].URL)
}

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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
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
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1"}).Return(model, nil).Once()

	resolved, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1", Headers: http.Header{}})

	require.NoError(t, err)
	require.Equal(t, model, resolved.Model)
	require.Equal(t, "https://api.example.com/v1/chat/completions", resolved.Target)
	require.Empty(t, resolved.Host)
	require.Equal(t, "backend-model", resolved.ModelName)
}

func TestResolveModelTarget_ModelNotFound(t *testing.T) {
	tester, _, _ := setupTest(t)
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "missing-model"}).Return(nil, nil).Once()

	_, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "missing-model", Headers: http.Header{}})

	require.Error(t, err)
	targetErr, ok := err.(*modelTargetError)
	require.True(t, ok)
	require.Equal(t, "model_not_found", targetErr.APIError.Code)
}

// The virtual automatic-routing model has no upstream of its own.  The
// Planner replaces it before resolution, so reaching the resolver means an
// endpoint that cannot route automatically was asked to.
func TestResolveModelTarget_VirtualAutoModelIsRejected(t *testing.T) {
	tester, _, _ := setupTest(t)
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "auto"}).
		Return(&types.Model{BaseModel: types.BaseModel{ID: "auto"}, AutoRoute: true}, nil).Once()

	_, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "auto", Headers: http.Header{}})

	require.Error(t, err)
	targetErr, ok := err.(*modelTargetError)
	require.True(t, ok)
	require.Equal(t, "auto_routing_unsupported", targetErr.APIError.Code)
	require.Contains(t, targetErr.APIError.Message, "automatic routing")
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
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "public-model"}).Return(model, nil).Once()

	resolved, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{
		Ctx:                context.Background(),
		Caller:             &commontypes.GatewayWhitelistCaller{},
		ModelID:            "public-model",
		Headers:            http.Header{},
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
	require.Equal(t, "https://native.example.com/v1/chat/completions", decision.BackendURL)
}

func TestResolveModelTargetWithOptionsRequiredUpstreamIDUnavailable(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{ID: "public-model"},
		Upstreams: []commontypes.UpstreamConfig{
			{ID: 1, URL: "https://adapter.example.com/v1/chat/completions", Enabled: true, Provider: "anthropic", ModelName: "claude"},
		},
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "public-model"}).Return(model, nil).Once()

	_, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{
		Ctx:                context.Background(),
		Caller:             &commontypes.GatewayWhitelistCaller{},
		ModelID:            "public-model",
		Headers:            http.Header{},
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
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1"}).Return(model, nil).Twice()

	headers := http.Header{}
	headers.Set("X-Session-ID", "session-1")

	first, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1", Headers: headers})
	require.NoError(t, err)

	second, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1", Headers: headers})
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
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1"}).Return(model, nil).Twice()

	first, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1", Headers: http.Header{}})
	require.NoError(t, err)

	second, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1", Headers: http.Header{}})
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
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1"}).Return(model, nil).Once()

	resolved, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1", Headers: http.Header{}})
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
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1"}).Return(model, nil).Once()

	resolved, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1", Headers: http.Header{}})
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
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1"}).Return(model, nil).Once()

	resolved, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1", Headers: http.Header{}})
	require.NoError(t, err)
	require.Equal(t, "provider-specific-model", resolved.ModelName)
	require.Len(t, resolved.AttemptTargets, 0)
	require.Equal(t, "provider-specific-model", resolved.Upstream.ModelName)
}

func TestResolveModelTarget_GetModelByIDError(t *testing.T) {
	tester, _, _ := setupTest(t)
	expectedErr := errors.New("storage unavailable")
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1"}).Return(nil, expectedErr).Once()

	_, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1", Headers: http.Header{}})

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
			Host:          "model.internal",
		},
		Endpoint: "https://model.internal/v1/chat/completions",
	}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1"}).Return(model, nil).Once()

	resolved, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1", Headers: http.Header{}})

	require.NoError(t, err)
	require.Equal(t, "https://model.internal/v1/chat/completions", resolved.Target)
	require.Equal(t, "model.internal", resolved.Host)
	require.Equal(t, "namespace/model", resolved.ModelName)
	require.Empty(t, resolved.AttemptTargets)
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
			Host:          "origin.internal",
		},
		Endpoint: "https://origin.internal/v1/chat/completions",
	}
	targetReq := commontypes.EndpointReq{
		ClusterID: "cluster-1",
		Target:    "https://origin.internal/v1/chat/completions",
		Endpoint:  "https://origin.internal/v1/chat/completions",
	}

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

func TestResolveCSGHubModelTarget_ReturnsMetadataDirectly(t *testing.T) {
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{
			ID: "ns/model:svc1",
		},
		InternalModelInfo: types.InternalModelInfo{
			CSGHubModelID: "ns/model",
			ClusterID:     "cluster-1",
			Host:          "fty8n9msz6kg.default.svc.cluster.local",
		},
		Endpoint: "http://127.0.0.1:9099/v1",
	}
	targetReq := commontypes.EndpointReq{
		ClusterID: "cluster-1",
		Endpoint:  "http://fty8n9msz6kg.default.svc.cluster.local:8080/v1",
	}

	// No cluster query should happen — all data comes from the model metadata.
	target, host, modelName, err := tester.handler.resolveCSGHubModelTarget(context.Background(), model, targetReq)
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:9099/v1", target)
	require.Equal(t, "fty8n9msz6kg.default.svc.cluster.local", host)
	require.Equal(t, "ns/model", modelName)
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
			NsUUID:    "testuser",
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
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1"}).Return(model, nil).Once()

	_, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "model1", Headers: http.Header{}})

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
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "external-model"}).Return(model, nil).Once()

	resolved, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "external-model", Headers: http.Header{}})

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
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "inference-model"}).Return(model, nil).Once()

	resolved, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: &commontypes.GatewayWhitelistCaller{}, ModelID: "inference-model", Headers: http.Header{}})

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
		ID:         42,
		Provider:   "endpoint-provider",
		AuthHeader: "endpoint-token",
	})
	require.Equal(t, "endpoint-provider", model.Provider)
	require.Equal(t, "endpoint-token", model.AuthHead)
	require.Equal(t, int64(42), model.UpstreamID)

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

func TestPreferNativeUpstreams(t *testing.T) {
	chatOnly := commontypes.UpstreamConfig{ID: 1, URL: "https://a.example.com/v1", SupportedEndpoints: map[commontypes.ApiFormat]string{"chat": ""}}
	multi := commontypes.UpstreamConfig{ID: 2, URL: "https://b.example.com/v1", SupportedEndpoints: map[commontypes.ApiFormat]string{"chat": "", "messages": ""}}
	unknown := commontypes.UpstreamConfig{ID: 3, URL: "https://c.example.com/custom"}
	fullURLMsgs := commontypes.UpstreamConfig{ID: 4, URL: "https://d.example.com/v1/messages", IsFullURL: true, SupportedEndpoints: map[commontypes.ApiFormat]string{"messages": ""}}

	t.Run("no client protocol returns list unchanged", func(t *testing.T) {
		list := []commontypes.UpstreamConfig{chatOnly, multi}
		assert.Equal(t, list, preferNativeUpstreams(list, ""))
	})

	t.Run("single candidate returns list unchanged", func(t *testing.T) {
		list := []commontypes.UpstreamConfig{chatOnly}
		assert.Equal(t, list, preferNativeUpstreams(list, "messages"))
	})

	t.Run("declared non-supporting upstreams are filtered out", func(t *testing.T) {
		got := preferNativeUpstreams([]commontypes.UpstreamConfig{chatOnly, multi, fullURLMsgs}, "messages")
		require.Len(t, got, 2)
		assert.Equal(t, int64(2), got[0].ID)
		assert.Equal(t, int64(4), got[1].ID)
	})

	t.Run("unknown-format upstreams count as native capable", func(t *testing.T) {
		got := preferNativeUpstreams([]commontypes.UpstreamConfig{chatOnly, unknown}, "responses")
		// chatOnly declares chat only; unknown has no declaration so it keeps
		// the legacy passthrough behavior for any protocol.
		require.Len(t, got, 1)
		assert.Equal(t, int64(3), got[0].ID)
	})

	t.Run("all native returns list unchanged", func(t *testing.T) {
		list := []commontypes.UpstreamConfig{chatOnly, multi}
		assert.Equal(t, list, preferNativeUpstreams(list, "chat"))
	})
}

func TestResolveModelTarget_UpstreamPriorityRouting(t *testing.T) {
	newModel := func() *types.Model {
		return &types.Model{
			BaseModel: types.BaseModel{ID: "tiered-model"},
			Upstreams: []commontypes.UpstreamConfig{
				{ID: 1, URL: "https://low.example.com/v1/chat/completions", Enabled: true, ModelName: "backend-model"},
				{ID: 2, URL: "https://high.example.com/v1/chat/completions", Enabled: true, ModelName: "backend-model", Priority: 100},
			},
		}
	}

	t.Run("low priority caller never reaches the high priority upstream", func(t *testing.T) {
		tester, _, _ := setupTest(t)
		model := newModel()
		caller := &commontypes.GatewayWhitelistCaller{UserPriority: callerPriorityPtr(0)}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: caller, ModelID: "tiered-model"}).Return(model, nil).Once()

		resolved, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: caller, ModelID: "tiered-model", Headers: http.Header{}})

		require.NoError(t, err)
		require.Equal(t, "https://low.example.com/v1/chat/completions", resolved.Target)
		// The stored candidate set keeps only eligible upstreams, so fallback
		// attempts cannot escalate into the high-priority channel either.
		for _, candidate := range resolved.Model.Upstreams {
			require.NotEqual(t, int64(2), candidate.ID)
		}
	})

	t.Run("low priority caller with only high priority upstreams is denied", func(t *testing.T) {
		tester, _, _ := setupTest(t)
		model := &types.Model{
			BaseModel: types.BaseModel{ID: "tiered-model"},
			Upstreams: []commontypes.UpstreamConfig{
				{ID: 2, URL: "https://high.example.com/v1/chat/completions", Enabled: true, ModelName: "backend-model", Priority: 100},
			},
		}
		caller := &commontypes.GatewayWhitelistCaller{UserPriority: callerPriorityPtr(0)}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: caller, ModelID: "tiered-model"}).Return(model, nil).Once()

		_, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: caller, ModelID: "tiered-model", Headers: http.Header{}})

		require.Error(t, err)
		targetErr, ok := err.(*modelTargetError)
		require.True(t, ok)
		require.Equal(t, types.ErrCodeUpstreamPriorityDenied, targetErr.APIError.Code)
		require.Equal(t, http.StatusForbidden, targetErr.Status)
	})

	t.Run("unrestricted caller falls back to the low channel when high upstream is unhealthy", func(t *testing.T) {
		tester, _, _ := setupTest(t)
		model := &types.Model{
			BaseModel: types.BaseModel{ID: "tiered-model"},
			Upstreams: []commontypes.UpstreamConfig{
				{ID: 2, URL: "https://high.example.com/v1/chat/completions", Enabled: true, ModelName: "backend-model", Priority: 100, HealthCheckEnabled: true, HealthState: "unhealthy"},
				{ID: 1, URL: "https://low.example.com/v1/chat/completions", Enabled: true, ModelName: "backend-model"},
			},
		}
		caller := &commontypes.GatewayWhitelistCaller{}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: caller, ModelID: "tiered-model"}).Return(model, nil).Once()

		resolved, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: caller, ModelID: "tiered-model", Headers: http.Header{}})

		require.NoError(t, err)
		require.Equal(t, "https://low.example.com/v1/chat/completions", resolved.Target)
	})

	t.Run("intermediate numeric tag admits priority tiers up to its level", func(t *testing.T) {
		tester, _, _ := setupTest(t)
		model := &types.Model{
			BaseModel: types.BaseModel{ID: "tiered-model"},
			Upstreams: []commontypes.UpstreamConfig{
				{ID: 1, URL: "https://low.example.com/v1/chat/completions", Enabled: true, ModelName: "backend-model"},
				{ID: 2, URL: "https://mid.example.com/v1/chat/completions", Enabled: true, ModelName: "backend-model", Priority: 50},
				{ID: 3, URL: "https://high.example.com/v1/chat/completions", Enabled: true, ModelName: "backend-model", Priority: 100},
			},
		}
		caller := &commontypes.GatewayWhitelistCaller{UserPriority: callerPriorityPtr(50)}
		tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: caller, ModelID: "tiered-model"}).Return(model, nil).Once()

		resolved, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{Ctx: context.Background(), Caller: caller, ModelID: "tiered-model", Headers: http.Header{}})

		require.NoError(t, err)
		require.Equal(t, int64(2), resolved.Upstream.ID)
		// AttemptTargets carries the fallback candidates after the primary:
		// the mid tier is picked first, the low tier is the only fallback,
		// the priority-100 upstream is excluded for this caller.
		require.Len(t, resolved.AttemptTargets, 1)
		require.Equal(t, int64(1), resolved.AttemptTargets[0].ID)
	})
}

// callerPriorityPtr returns a pointer to the given channel priority for
// constructing restricted callers.
func callerPriorityPtr(priority int) *int {
	return &priority
}

func TestResolveModelTarget_PriorityDominatesNativePreference(t *testing.T) {
	// A high-priority upstream that only declares chat (so serving the
	// client's messages protocol needs an adapter) must still win the primary
	// pick over a low-priority upstream that serves messages natively: the
	// channel tier is an access/preference contract, the native pass-through
	// is only a tie-breaker inside a tier.
	tester, _, _ := setupTest(t)
	model := &types.Model{
		BaseModel: types.BaseModel{ID: "mixed-protocol-model"},
		Upstreams: []commontypes.UpstreamConfig{
			{
				ID: 1, URL: "https://native-low.example.com/v1", Enabled: true, ModelName: "backend",
				Priority:           0,
				SupportedEndpoints: map[commontypes.ApiFormat]string{commontypes.ApiFormatChat: "", commontypes.ApiFormatMessages: ""},
			},
			{
				ID: 2, URL: "https://adapter-high.example.com/v1", Enabled: true, ModelName: "backend",
				Priority:           100,
				SupportedEndpoints: map[commontypes.ApiFormat]string{commontypes.ApiFormatChat: ""},
			},
		},
	}
	caller := &commontypes.GatewayWhitelistCaller{}
	tester.mocks.openAIComp.EXPECT().GetModelByID(mock.Anything, &types.GetModelByIDReq{Caller: caller, ModelID: "mixed-protocol-model"}).Return(model, nil).Once()

	resolved, err := tester.handler.resolveModelTargetWithOptions(resolveModelTargetWithOptionsReq{
		Ctx: context.Background(), Caller: caller, ModelID: "mixed-protocol-model",
		Headers: http.Header{}, ClientProtocol: commontypes.ApiFormatMessages,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), resolved.Upstream.ID, "the high-priority tier must win the primary pick")
	// Fallback candidates stay ordered by tier: the low native upstream is
	// the only fallback after the high-tier primary.
	require.Len(t, resolved.AttemptTargets, 1)
	require.Equal(t, int64(1), resolved.AttemptTargets[0].ID)
}

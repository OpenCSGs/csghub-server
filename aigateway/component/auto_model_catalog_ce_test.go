//go:build !ee && !saas

package component

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/database"
	commontypes "opencsg.com/csghub-server/common/types"
)

// stubLLMConfigStore returns one enabled external model with a healthy
// upstream, which is the minimum a model needs to reach the model list.
func stubLLMConfigStore(t *testing.T) *mockdb.MockLLMConfigStore {
	t.Helper()
	store := mockdb.NewMockLLMConfigStore(t)
	store.EXPECT().IndexWithRepo(mock.Anything, 50, 1, mock.Anything).
		Return([]*database.LLMConfig{
			{
				ID:        1,
				ModelName: "glm-5.1",
				Type:      database.LLMTypeAigatewayExternal,
				Enabled:   true,
				Provider:  "zhipu",
				Metadata:  map[string]any{types.MetaKeyTasks: []any{"text-generation"}},
				Upstreams: []database.Upstream{
					{
						ID:       1,
						Source:   commontypes.UpstreamSourceExternal,
						URL:      "http://zhipu/v1",
						Enabled:  true,
						Provider: "zhipu",
					},
				},
			},
		}, 1, nil).Once()
	return store
}

func TestListModels_PublishesVirtualModelWhenRouterIsUp(t *testing.T) {
	router := mockcomponent.NewMockAutoModelRouter(t)
	router.EXPECT().Available(mock.Anything).Return(true).Once()
	router.EXPECT().ModelID().Return("auto").Once()

	comp := &openaiComponentImpl{extllmStore: stubLLMConfigStore(t), autoRouter: router}

	list, err := comp.ListModels(context.Background(), "", types.ListModelsReq{})
	require.NoError(t, err)
	require.Len(t, list.Data, 2)
	assert.Equal(t, "auto", list.Data[0].ID, "the virtual model leads the list")
	assert.True(t, list.Data[0].Availability.IsAvailable)
	assert.Equal(t, "glm-5.1", list.Data[1].ID)
	assert.Equal(t, 2, list.TotalCount)
}

func TestListModels_HidesVirtualModelWhenRouterIsDown(t *testing.T) {
	router := mockcomponent.NewMockAutoModelRouter(t)
	router.EXPECT().Available(mock.Anything).Return(false).Once()

	comp := &openaiComponentImpl{extllmStore: stubLLMConfigStore(t), autoRouter: router}

	list, err := comp.ListModels(context.Background(), "", types.ListModelsReq{})
	require.NoError(t, err)
	require.Len(t, list.Data, 1)
	assert.Equal(t, "glm-5.1", list.Data[0].ID)
}

func TestListModels_OmitsVirtualModelWhenRoutingIsNotConfigured(t *testing.T) {
	comp := &openaiComponentImpl{extllmStore: stubLLMConfigStore(t)}

	list, err := comp.ListModels(context.Background(), "", types.ListModelsReq{})
	require.NoError(t, err)
	require.Len(t, list.Data, 1)
	assert.Equal(t, "glm-5.1", list.Data[0].ID)
}

func TestAutoModelID(t *testing.T) {
	comp := &openaiComponentImpl{}
	assert.Empty(t, comp.AutoModelID(), "an unconfigured gateway advertises no virtual model")

	router := mockcomponent.NewMockAutoModelRouter(t)
	router.EXPECT().ModelID().Return("auto")
	comp.autoRouter = router
	assert.Equal(t, "auto", comp.AutoModelID())
}

func TestGetModelByID_VirtualModel(t *testing.T) {
	// The catalogue is consulted first, so a real model can never be
	// shadowed; the virtual entry answers only when nothing real owns the
	// ID.
	noRealModel := func(t *testing.T, modelID string) *mockdb.MockLLMConfigStore {
		t.Helper()
		store := mockdb.NewMockLLMConfigStore(t)
		store.EXPECT().GetByModelName(mock.Anything, modelID).Return(nil, nil).Once()
		return store
	}

	t.Run("resolves when no real model owns the ID", func(t *testing.T) {
		router := mockcomponent.NewMockAutoModelRouter(t)
		router.EXPECT().ModelID().Return("auto")
		router.EXPECT().Available(mock.Anything).Return(true).Once()

		comp := &openaiComponentImpl{extllmStore: noRealModel(t, "auto"), autoRouter: router}

		model, err := comp.GetModelByID(context.Background(), "", "auto")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, "auto", model.ID)
		assert.True(t, model.AutoRoute)
	})

	t.Run("is not found while the router is down", func(t *testing.T) {
		router := mockcomponent.NewMockAutoModelRouter(t)
		router.EXPECT().ModelID().Return("auto")
		router.EXPECT().Available(mock.Anything).Return(false).Once()

		comp := &openaiComponentImpl{extllmStore: noRealModel(t, "auto"), autoRouter: router}

		model, err := comp.GetModelByID(context.Background(), "", "auto")
		require.NoError(t, err)
		require.Nil(t, model)
	})

	t.Run("a real model of the same ID wins", func(t *testing.T) {
		router := mockcomponent.NewMockAutoModelRouter(t)

		store := mockdb.NewMockLLMConfigStore(t)
		store.EXPECT().GetByModelName(mock.Anything, "auto").
			Return(&database.LLMConfig{
				ID: 1, ModelName: "auto", Type: database.LLMTypeAigatewayExternal,
				Enabled: true, Provider: "zhipu",
				Upstreams: []database.Upstream{{
					ID: 1, Source: commontypes.UpstreamSourceExternal,
					URL: "http://zhipu/v1", Enabled: true, Provider: "zhipu",
				}},
			}, nil).Once()

		comp := &openaiComponentImpl{extllmStore: store, autoRouter: router}

		model, err := comp.GetModelByID(context.Background(), "", "auto")
		require.NoError(t, err)
		require.NotNil(t, model)
		assert.Equal(t, "auto", model.ID)
		assert.False(t, model.AutoRoute, "the real model is returned, not the virtual entry")
	})
}

// A caller that named a concrete model must not pay for automatic
// routing in any way.  The mock fails the test on any call that was not
// set up, so an unexpected Available or Select would be caught here.
func TestConcreteModelRequestNeverConsultsTheRouter(t *testing.T) {
	// No expectation is set at all: the strict mock fails the test on any
	// call, so this proves the router is not consulted in any way.
	router := mockcomponent.NewMockAutoModelRouter(t)

	store := mockdb.NewMockLLMConfigStore(t)
	store.EXPECT().GetByModelName(mock.Anything, "glm-5.1").
		Return(&database.LLMConfig{
			ID: 1, ModelName: "glm-5.1", Type: database.LLMTypeAigatewayExternal,
			Enabled: true, Provider: "zhipu",
			Upstreams: []database.Upstream{{
				ID: 1, Source: commontypes.UpstreamSourceExternal,
				URL: "http://zhipu/v1", Enabled: true, Provider: "zhipu",
			}},
		}, nil).Once()

	comp := &openaiComponentImpl{extllmStore: store, autoRouter: router}

	model, err := comp.GetModelByID(context.Background(), "", "glm-5.1")
	require.NoError(t, err)
	require.NotNil(t, model)
	assert.Equal(t, "glm-5.1", model.ID)
	assert.False(t, model.AutoRoute)
}

// A store holding one enabled model with a healthy upstream, plus an
// unhealthy variant, so listing and routing can be compared on the same
// catalogue.
func shadowingStore(t *testing.T, modelName string, healthy bool) *mockdb.MockLLMConfigStore {
	t.Helper()
	upstream := database.Upstream{
		ID: 7, Source: commontypes.UpstreamSourceExternal,
		URL: "http://zhipu/v1", Enabled: true, Provider: "zhipu",
	}
	if !healthy {
		upstream.HealthCheckEnabled = true
		upstream.HealthState = &database.AIGatewayUpstreamHealthState{HealthState: "unhealthy"}
	}
	return storeWithUpstream(t, modelName, upstream)
}

func storeWithUpstream(t *testing.T, modelName string, upstream database.Upstream) *mockdb.MockLLMConfigStore {
	t.Helper()
	store := mockdb.NewMockLLMConfigStore(t)
	store.EXPECT().IndexWithRepo(mock.Anything, 50, 1, mock.Anything).
		Return([]*database.LLMConfig{{
			ID: 1, ModelName: modelName, Type: database.LLMTypeAigatewayExternal,
			Enabled: true, Provider: "zhipu",
			Metadata:  map[string]any{types.MetaKeyTasks: []any{"text-generation"}},
			Upstreams: []database.Upstream{upstream},
		}}, 1, nil).Once()
	return store
}

// Whether a real model owns the virtual ID must not depend on its health,
// or the listing and the request path would resolve the same ID to
// different resources.
func TestVirtualModelOwnershipIgnoresHealth(t *testing.T) {
	for _, healthy := range []bool{true, false} {
		t.Run(fmt.Sprintf("real model healthy=%v", healthy), func(t *testing.T) {
			listRouter := mockcomponent.NewMockAutoModelRouter(t)
			listRouter.EXPECT().Available(mock.Anything).Return(true).Once()
			listRouter.EXPECT().ModelID().Return("auto")
			listComp := &openaiComponentImpl{extllmStore: shadowingStore(t, "auto", healthy), autoRouter: listRouter}

			list, err := listComp.ListModels(context.Background(), "", types.ListModelsReq{})
			require.NoError(t, err)
			for _, m := range list.Data {
				assert.False(t, m.AutoRoute, "the virtual model must not be published while a real model owns its ID")
			}

			// The request path must reach the same conclusion.
			routeRouter := mockcomponent.NewMockAutoModelRouter(t)
			routeRouter.EXPECT().ModelID().Return("auto")
			routeComp := &openaiComponentImpl{extllmStore: shadowingStore(t, "auto", healthy), autoRouter: routeRouter}

			decision, err := routeComp.ResolveAutoModel(context.Background(), types.AutoRouteRequest{
				Input: types.AutoRouteInput{Messages: []types.AutoRouteMessage{{Role: "user", Content: "hi"}}},
			})
			require.NoError(t, err)
			require.NotNil(t, decision)
			assert.True(t, decision.Shadowed)
			assert.Equal(t, "auto", decision.ModelID)
		})
	}
}

// Only an exactly-spelled model owns the ID, which is how the catalogue
// itself compares model names.
func TestVirtualModelOwnershipIsExact(t *testing.T) {
	router := mockcomponent.NewMockAutoModelRouter(t)
	router.EXPECT().ModelID().Return("auto")
	router.EXPECT().Select(mock.Anything, mock.Anything, mock.Anything).
		Return(&types.AutoRouteDecision{ModelID: "Auto"}, nil).Once()

	comp := &openaiComponentImpl{extllmStore: shadowingStore(t, "Auto", true), autoRouter: router}

	decision, err := comp.ResolveAutoModel(context.Background(), types.AutoRouteRequest{
		Input: types.AutoRouteInput{Messages: []types.AutoRouteMessage{{Role: "user", Content: "hi"}}},
	})
	require.NoError(t, err)
	assert.False(t, decision.Shadowed, "a differently-spelled model does not own the virtual ID")
}

// A pinned conversation reuses the model owning its upstream, without
// ranking a fresh one that would not have that upstream.
func TestResolveAutoModel_ReusesThePinnedModel(t *testing.T) {
	router := mockcomponent.NewMockAutoModelRouter(t)
	router.EXPECT().ModelID().Return("auto")
	// No Select expectation: ranking must not happen for a pinned turn.

	comp := &openaiComponentImpl{extllmStore: shadowingStore(t, "glm-5.1", true), autoRouter: router}

	decision, err := comp.ResolveAutoModel(context.Background(), types.AutoRouteRequest{
		Input:              types.AutoRouteInput{Messages: []types.AutoRouteMessage{{Role: "user", Content: "hi"}}},
		RequiredUpstreamID: 7,
	})
	require.NoError(t, err)
	assert.Equal(t, "glm-5.1", decision.ModelID)
	assert.True(t, decision.Pinned)
}

// Losing the pinned upstream is temporary, so it is reported as such
// rather than as a model that does not exist.
// A pinned upstream that can no longer serve the conversation is reported
// with the same code the ordinary resolution path raises, so each
// protocol renders the answer it already renders for that condition.
func TestResolveAutoModel_PinnedUpstreamUnusable(t *testing.T) {
	cases := []struct {
		name     string
		upstream database.Upstream
		pinned   int64
		reason   string
	}{
		{
			name: "disabled",
			upstream: database.Upstream{
				ID: 7, Source: commontypes.UpstreamSourceExternal,
				URL: "http://zhipu/v1", Enabled: false, Provider: "zhipu",
			},
			pinned: 7,
		},
		{
			name: "unhealthy",
			upstream: database.Upstream{
				ID: 7, Source: commontypes.UpstreamSourceExternal,
				URL: "http://zhipu/v1", Enabled: true, Provider: "zhipu",
				HealthCheckEnabled: true,
				HealthState:        &database.AIGatewayUpstreamHealthState{HealthState: "unhealthy"},
			},
			pinned: 7,
		},
		{
			name: "circuit open",
			upstream: database.Upstream{
				ID: 7, Source: commontypes.UpstreamSourceExternal,
				URL: "http://zhipu/v1", Enabled: true, Provider: "zhipu",
				CircuitBreakerEnabled: true,
				CircuitState:          &database.AIGatewayUpstreamCircuitState{CircuitState: "open"},
			},
			pinned: 7,
		},
		{
			name: "upstream row gone",
			upstream: database.Upstream{
				ID: 7, Source: commontypes.UpstreamSourceExternal,
				URL: "http://zhipu/v1", Enabled: true, Provider: "zhipu",
			},
			pinned: 999,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := mockcomponent.NewMockAutoModelRouter(t)
			router.EXPECT().ModelID().Return("auto")
			// No Select expectation: a pinned turn must not be ranked.

			comp := &openaiComponentImpl{
				extllmStore: storeWithUpstream(t, "glm-5.1", tc.upstream),
				autoRouter:  router,
			}

			_, err := comp.ResolveAutoModel(context.Background(), types.AutoRouteRequest{
				Input:              types.AutoRouteInput{Messages: []types.AutoRouteMessage{{Role: "user", Content: "hi"}}},
				RequiredUpstreamID: tc.pinned,
			})
			require.Error(t, err)
			var coded *autoRouteError
			require.ErrorAs(t, err, &coded)
			assert.Equal(t, autoRouteCodeRequiredUpstream, coded.ModelErrorCode(),
				"the same code the ordinary pinned-resolution path raises")
		})
	}
}

func TestResolveAutoModel(t *testing.T) {
	t.Run("hands the caller's available models to the router", func(t *testing.T) {
		router := mockcomponent.NewMockAutoModelRouter(t)
		router.EXPECT().ModelID().Return("auto")
		router.EXPECT().
			Select(mock.Anything, mock.Anything, mock.MatchedBy(func(candidates []types.Model) bool {
				return len(candidates) == 1 && candidates[0].ID == "glm-5.1"
			})).
			Return(&types.AutoRouteDecision{ModelID: "glm-5.1", Rank: 1}, nil).
			Once()

		comp := &openaiComponentImpl{extllmStore: stubLLMConfigStore(t), autoRouter: router}

		decision, err := comp.ResolveAutoModel(context.Background(), types.AutoRouteRequest{Input: types.AutoRouteInput{
			Messages: []types.AutoRouteMessage{{Role: "user", Content: "hello"}},
		}})
		require.NoError(t, err)
		assert.Equal(t, "glm-5.1", decision.ModelID)
	})

	t.Run("fails when routing is not configured", func(t *testing.T) {
		comp := &openaiComponentImpl{}
		_, err := comp.ResolveAutoModel(context.Background(), types.AutoRouteRequest{Input: types.AutoRouteInput{}})
		require.ErrorContains(t, err, "not configured")
	})

	t.Run("propagates a router failure", func(t *testing.T) {
		router := mockcomponent.NewMockAutoModelRouter(t)
		router.EXPECT().ModelID().Return("auto")
		router.EXPECT().Select(mock.Anything, mock.Anything, mock.Anything).
			Return(nil, errors.New("ranking unavailable")).Once()

		comp := &openaiComponentImpl{extllmStore: stubLLMConfigStore(t), autoRouter: router}

		_, err := comp.ResolveAutoModel(context.Background(), types.AutoRouteRequest{Input: types.AutoRouteInput{}})
		require.ErrorContains(t, err, "ranking unavailable")
	})
}

// The billing record is the only durable trace of a routing decision, so
// what it carries is asserted directly rather than inferred.
func TestBuildUsageExtraData_RecordsTheRoutingDecision(t *testing.T) {
	usage := &token.Usage{PromptTokens: 10, CompletionTokens: 5}
	meteringInfo := usageMeteringInfo{}

	t.Run("a routed request is separable and reviewable", func(t *testing.T) {
		extra, err := buildUsageExtraData(&types.Model{}, "deepseek-v4-flash", usage, "", meteringInfo,
			autoRouteExtra("auto", &types.AutoRouteDecision{
				ModelID:       "deepseek-v4-flash",
				BenchmarkID:   "deepseek-v4-flash",
				Rank:          1,
				IndexVersion:  "live-586d25da",
				PolicyVersion: "v2/1.0",
			}))
		require.NoError(t, err)

		var decoded map[string]any
		require.NoError(t, json.Unmarshal([]byte(extra), &decoded))
		route, ok := decoded["auto_route"].(map[string]any)
		require.True(t, ok, "the routing decision has to reach the billing record")
		assert.Equal(t, "auto", route["requested_model"])
		assert.Equal(t, "deepseek-v4-flash", route["candidate"])
		assert.Equal(t, float64(1), route["rank"])
		assert.Equal(t, "live-586d25da", route["index_version"])
		assert.Equal(t, "deepseek-v4-flash", decoded["model_name"], "the model billed is still the concrete one")
	})

	t.Run("a pinned turn is marked as such", func(t *testing.T) {
		extra, err := buildUsageExtraData(&types.Model{}, "glm-5.1", usage, "", meteringInfo,
			autoRouteExtra("auto", &types.AutoRouteDecision{ModelID: "glm-5.1", Pinned: true}))
		require.NoError(t, err)

		var decoded map[string]any
		require.NoError(t, json.Unmarshal([]byte(extra), &decoded))
		route := decoded["auto_route"].(map[string]any)
		assert.Equal(t, true, route["pinned"])
	})

	t.Run("an ordinary request carries nothing", func(t *testing.T) {
		extra, err := buildUsageExtraData(&types.Model{}, "glm-5.1", usage, "", meteringInfo,
			autoRouteExtra("auto", nil))
		require.NoError(t, err)
		assert.NotContains(t, extra, "auto_route")
	})

	t.Run("a shadowed request is an ordinary one", func(t *testing.T) {
		// A real model owned the virtual ID, so no routing took place.
		extra, err := buildUsageExtraData(&types.Model{}, "auto", usage, "", meteringInfo,
			autoRouteExtra("auto", &types.AutoRouteDecision{ModelID: "auto", Shadowed: true}))
		require.NoError(t, err)
		assert.NotContains(t, extra, "auto_route")
	})
}

// The decision travels from the Planner to billing on the request
// context, so that carrier is asserted rather than assumed.
func TestAutoRouteDecisionSurvivesContextPropagation(t *testing.T) {
	decision := &types.AutoRouteDecision{ModelID: "glm-5.1", Rank: 2}
	ctx := types.WithAutoRouteDecision(context.Background(), decision)

	// Billing runs asynchronously on a cancellation-free copy of the
	// request context, which is how it reaches the recording path.
	async, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()

	got := types.AutoRouteDecisionFromContext(async)
	require.NotNil(t, got)
	assert.Equal(t, "glm-5.1", got.ModelID)
	assert.Equal(t, 2, got.Rank)

	assert.Nil(t, types.AutoRouteDecisionFromContext(context.Background()))
	assert.Equal(t, context.Background(), types.WithAutoRouteDecision(context.Background(), nil))
}

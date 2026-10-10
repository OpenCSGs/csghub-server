package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

// autoRoutePlan is the plan a request that asked for the virtual model
// carries once the plan phase has run: the balance check was made and
// passed, which is what lets an alternate model be used without making it
// again.
func autoRoutePlan(alternates ...types.AutoRouteCandidate) *types.RequestPlan {
	return &types.RequestPlan{
		BalanceOK: true,
		AutoRoute: &types.AutoRouteDecision{
			ModelID:    "first-model",
			Alternates: alternates,
		},
	}
}

func TestShouldRetryChatAttemptWithAnotherModel(t *testing.T) {
	// A 400 says the request does not suit this model, which another model
	// may still accept; the upstream-level predicate deliberately refuses it.
	assert.True(t, shouldRetryChatAttemptWithAnotherModel(http.StatusBadRequest, false))
	assert.False(t, shouldRetryChatAttempt(http.StatusBadRequest, false))

	assert.True(t, shouldRetryChatAttemptWithAnotherModel(http.StatusServiceUnavailable, false))
	assert.False(t, shouldRetryChatAttemptWithAnotherModel(http.StatusOK, false))
	assert.False(t, shouldRetryChatAttemptWithAnotherModel(499, false),
		"a client-closed connection has nobody left to serve")
	assert.False(t, shouldRetryChatAttemptWithAnotherModel(http.StatusBadRequest, true),
		"a response already streaming cannot be replaced")
}

func TestChatRetryResponseWriterBuffersClientErrorWhileAModelRemains(t *testing.T) {
	downstream := newTestCommonResponseWriter()
	w := newChatRetryResponseWriterWithClientErrors(downstream, true)

	w.WriteHeader(http.StatusBadRequest)
	_, err := w.Write([]byte(`{"error":"max_tokens"}`))
	require.NoError(t, err)

	assert.Equal(t, 0, downstream.statusCode, "the failure must not reach the client yet")
	assert.Empty(t, downstream.body.String())

	require.NoError(t, w.ReplayBufferedResponse())
	assert.Equal(t, http.StatusBadRequest, downstream.statusCode)
	assert.Equal(t, `{"error":"max_tokens"}`, downstream.body.String())
}

func TestChatRetryResponseWriterCommitsClientErrorWithoutAlternates(t *testing.T) {
	downstream := newTestCommonResponseWriter()
	w := newChatRetryResponseWriterWithClientErrors(downstream, false)

	w.WriteHeader(http.StatusBadRequest)
	_, err := w.Write([]byte(`{"error":"max_tokens"}`))
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, downstream.statusCode,
		"without another model the failure is the answer and goes straight out")
	assert.Equal(t, `{"error":"max_tokens"}`, downstream.body.String())
}

func TestTakeAutoRouteAlternateDrainsInRankOrder(t *testing.T) {
	p := autoRoutePlan(
		types.AutoRouteCandidate{ModelID: "second-model", Rank: 2},
		types.AutoRouteCandidate{ModelID: "third-model", Rank: 3},
	)

	assert.Equal(t, 2, p.AutoRouteAlternatesRemaining())
	assert.Equal(t, "second-model", p.TakeAutoRouteAlternate().ModelID)
	assert.Equal(t, "third-model", p.TakeAutoRouteAlternate().ModelID)
	assert.Nil(t, p.TakeAutoRouteAlternate())
	assert.Equal(t, 0, p.AutoRouteAlternatesRemaining())
}

func TestAutoRouteAlternatesRemainingIsZeroForADirectRequest(t *testing.T) {
	assert.Equal(t, 0, (*types.RequestPlan)(nil).AutoRouteAlternatesRemaining())
	assert.Equal(t, 0, (&types.RequestPlan{}).AutoRouteAlternatesRemaining())
	assert.Nil(t, (&types.RequestPlan{}).TakeAutoRouteAlternate())
}

// chatModelFallbackTarget builds a resolved target pointing at one upstream
// with no same-model fallbacks, so only model-level fallback can rescue it.
func chatModelFallbackTarget(modelID, url string) *resolvedModelTarget {
	return &resolvedModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: modelID},
			Endpoint:  url,
			Upstreams: []commontypes.UpstreamConfig{{URL: url, Enabled: true, ModelName: modelID}},
		},
		Upstream:       commontypes.UpstreamConfig{URL: url, Enabled: true, ModelName: modelID},
		ModelName:      modelID,
		Target:         url,
		AttemptTargets: nil,
	}
}

func TestFinishOrTryAnotherModel_ServesTheNextRankedModelAfterA400(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil

	secondUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"choices":[{"message":{"content":"served by the second model"}}]}`))
		require.NoError(t, err)
	}))
	defer secondUpstream.Close()
	secondURL := secondUpstream.URL + "/v1/chat/completions"

	tester.mocks.openAIComp.EXPECT().
		GetModelByID(mock.Anything, "testuuid", "second-model").
		Return(chatModelFallbackTarget("second-model", secondURL).Model, nil)
	// The alternate model brings its own upstream, so the attempt acquires a
	// lease for it; nil means the upstream has no capacity policy.
	tester.mocks.openAIComp.EXPECT().
		AcquireCapacityAdmission(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Maybe()

	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		bytes.NewReader([]byte(`{"model":"auto","messages":[]}`)))

	downstream := newTestCommonResponseWriter()
	// The first model answered 400, buffered because an alternate remains.
	failed := newChatRetryResponseWriterWithClientErrors(downstream, true)
	failed.WriteHeader(http.StatusBadRequest)
	_, err := failed.Write([]byte(`{"error":{"message":"Range of max_tokens should be [1, 65536]"}}`))
	require.NoError(t, err)

	p := autoRoutePlan(types.AutoRouteCandidate{ModelID: "second-model", Rank: 2})
	modelTarget := chatModelFallbackTarget("first-model", "https://first.example.com/v1/chat/completions")
	chatCtx := &chatContext{responseWriter: downstream}
	chatReq := &types.ChatCompletionRequest{Model: "first-model"}

	final := tester.handler.finishOrTryAnotherModel(c, chatCtx, modelTarget, chatReq, failed, "testuser", p, types.ProtocolChat)

	require.NotNil(t, final)
	assert.Equal(t, http.StatusOK, final.StatusCode())
	assert.Equal(t, http.StatusOK, downstream.statusCode,
		"the client must receive the second model's answer, not the first model's 400")
	assert.Contains(t, downstream.body.String(), "served by the second model")
	assert.NotContains(t, downstream.body.String(), "max_tokens",
		"the replaced failure must never reach the client")
	assert.Equal(t, "second-model", modelTarget.Model.ID,
		"billing reads the model target afterwards, so it must name the model that served the turn")
	assert.Equal(t, 0, p.AutoRouteAlternatesRemaining())
}

func TestFinishOrTryAnotherModel_ReplaysTheFailureWhenNoModelRemains(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil

	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		bytes.NewReader([]byte(`{"model":"auto","messages":[]}`)))

	downstream := newTestCommonResponseWriter()
	failed := newChatRetryResponseWriterWithClientErrors(downstream, true)
	failed.WriteHeader(http.StatusBadRequest)
	_, err := failed.Write([]byte(`{"error":"no model left"}`))
	require.NoError(t, err)

	p := autoRoutePlan()
	modelTarget := chatModelFallbackTarget("first-model", "https://first.example.com/v1/chat/completions")

	final := tester.handler.finishOrTryAnotherModel(c, &chatContext{responseWriter: downstream},
		modelTarget, &types.ChatCompletionRequest{Model: "first-model"}, failed, "testuser", p, types.ProtocolChat)

	require.NotNil(t, final)
	assert.Equal(t, http.StatusBadRequest, downstream.statusCode)
	assert.Equal(t, `{"error":"no model left"}`, downstream.body.String())
}

func TestFinishOrTryAnotherModel_LeavesASuccessfulResponseAlone(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil

	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		bytes.NewReader([]byte(`{"model":"auto","messages":[]}`)))

	downstream := newTestCommonResponseWriter()
	ok := newChatRetryResponseWriterWithClientErrors(downstream, true)
	ok.WriteHeader(http.StatusOK)
	_, err := ok.Write([]byte(`{"choices":[]}`))
	require.NoError(t, err)

	p := autoRoutePlan(types.AutoRouteCandidate{ModelID: "second-model", Rank: 2})
	modelTarget := chatModelFallbackTarget("first-model", "https://first.example.com/v1/chat/completions")

	final := tester.handler.finishOrTryAnotherModel(c, &chatContext{responseWriter: downstream},
		modelTarget, &types.ChatCompletionRequest{Model: "first-model"}, ok, "testuser", p, types.ProtocolChat)

	require.NotNil(t, final)
	assert.Equal(t, http.StatusOK, downstream.statusCode)
	assert.Equal(t, "first-model", modelTarget.Model.ID, "a successful model must not be switched away from")
	assert.Equal(t, 1, p.AutoRouteAlternatesRemaining(), "an unused alternate must not be consumed")
}

// chatEchoUpstream records the body one attempt forwarded, so a test can
// assert on what the upstream would have received.
func chatEchoUpstream(t *testing.T, received *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		*received = string(body)
		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(`{"choices":[]}`))
		require.NoError(t, err)
	}))
}

func forwardedMaxTokens(t *testing.T, body string) (int, bool) {
	t.Helper()
	var payload struct {
		MaxTokens *int `json:"max_tokens"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &payload))
	if payload.MaxTokens == nil {
		return 0, false
	}
	return *payload.MaxTokens, true
}

func TestExecuteChatProxyAttempt_CapsMaxTokensForAnAutomaticallyRoutedRequest(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil
	tester.mocks.openAIComp.EXPECT().
		AcquireCapacityAdmission(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Maybe()

	var received string
	upstream := chatEchoUpstream(t, &received)
	defer upstream.Close()

	raw := `{"model":"auto","messages":[{"role":"user","content":"hi"}],"max_tokens":131072}`
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(raw)))
	chatReq := &types.ChatCompletionRequest{Model: "qwen3.6-plus", ClientModel: "auto", MaxTokens: 131072}
	chatReq.RawBody = json.RawMessage(raw)

	modelTarget := chatModelFallbackTarget("qwen3.6-plus", upstream.URL+"/v1/chat/completions")
	p := autoRoutePlan()

	_, err := tester.handler.executeChatProxyAttempt(c, newTestCommonResponseWriter(), modelTarget, chatReq, p)

	require.NoError(t, err)
	maxTokens, ok := forwardedMaxTokens(t, received)
	require.True(t, ok, "max_tokens must still be present")
	assert.Equal(t, 65536, maxTokens, "a caller that did not choose its model cannot size this field")
}

func TestExecuteChatProxyAttempt_LeavesMaxTokensAloneForADirectlyNamedModel(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil
	tester.mocks.openAIComp.EXPECT().
		AcquireCapacityAdmission(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Maybe()

	var received string
	upstream := chatEchoUpstream(t, &received)
	defer upstream.Close()

	raw := `{"model":"qwen3.6-plus","messages":[{"role":"user","content":"hi"}],"max_tokens":131072}`
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(raw)))
	chatReq := &types.ChatCompletionRequest{Model: "qwen3.6-plus", ClientModel: "qwen3.6-plus", MaxTokens: 131072}
	chatReq.RawBody = json.RawMessage(raw)

	modelTarget := chatModelFallbackTarget("qwen3.6-plus", upstream.URL+"/v1/chat/completions")

	// No AutoRoute on the plan: the caller named this model itself.
	_, err := tester.handler.executeChatProxyAttempt(c, newTestCommonResponseWriter(), modelTarget, chatReq, &types.RequestPlan{})

	require.NoError(t, err)
	maxTokens, ok := forwardedMaxTokens(t, received)
	require.True(t, ok)
	assert.Equal(t, 131072, maxTokens, "a caller that chose its own model keeps the field it sent")
}

func TestChatAlternateSatisfiesPlanGates(t *testing.T) {
	primary := chatModelFallbackTarget("first-model", "https://first.example.com/v1/chat/completions")
	primary.Upstream.Provider = "deepseek-ai"

	alternateWith := func(mutate func(*resolvedModelTarget)) *resolvedModelTarget {
		alternate := chatModelFallbackTarget("second-model", "https://second.example.com/v1/chat/completions")
		alternate.Upstream.Provider = "deepseek-ai"
		mutate(alternate)
		return alternate
	}

	t.Run("accepts a model covered by the decisions already made", func(t *testing.T) {
		err := chatAlternateSatisfiesPlanGates(alternateWith(func(*resolvedModelTarget) {}), primary, autoRoutePlan())
		assert.NoError(t, err)
	})

	t.Run("refuses a model needing a balance check the request never made", func(t *testing.T) {
		// The primary skipped the balance check, so the request holds no
		// balance decision to extend to a model that needs one.
		p := autoRoutePlan()
		p.BalanceOK = false

		err := chatAlternateSatisfiesPlanGates(alternateWith(func(*resolvedModelTarget) {}), primary, p)

		assert.ErrorContains(t, err, "balance check")
	})

	t.Run("accepts a balance-exempt model without a balance decision", func(t *testing.T) {
		p := autoRoutePlan()
		p.BalanceOK = false
		alternate := alternateWith(func(a *resolvedModelTarget) {
			a.Model.Metadata = map[string]any{types.MetaTaskKey: []any{types.MetaTaskValGuard}}
		})

		assert.NoError(t, chatAlternateSatisfiesPlanGates(alternate, primary, p))
	})

	t.Run("refuses a checked model when the request enabled no checking", func(t *testing.T) {
		// The primary was not enrolled, so stream-time output moderation was
		// never switched on; serving an enrolled model under it would run
		// that model with no output check at all.
		alternate := alternateWith(func(a *resolvedModelTarget) { a.Model.NeedSensitiveCheck = true })

		err := chatAlternateSatisfiesPlanGates(alternate, primary, autoRoutePlan())

		assert.ErrorContains(t, err, "did not enable")
	})

	t.Run("refuses a checked model behind a different provider", func(t *testing.T) {
		// The content-safety whitelist is built from the serving provider,
		// so the verdict reached for the primary says nothing about this one.
		checkedPrimary := chatModelFallbackTarget("first-model", "https://first.example.com/v1/chat/completions")
		checkedPrimary.Upstream.Provider = "deepseek-ai"
		checkedPrimary.Model.NeedSensitiveCheck = true
		alternate := alternateWith(func(a *resolvedModelTarget) {
			a.Model.NeedSensitiveCheck = true
			a.Upstream.Provider = "aliyuncs"
		})

		err := chatAlternateSatisfiesPlanGates(alternate, checkedPrimary, autoRoutePlan())

		assert.ErrorContains(t, err, "content-safety check")
	})

	t.Run("accepts a checked model behind the same provider when the primary was checked too", func(t *testing.T) {
		checkedPrimary := chatModelFallbackTarget("first-model", "https://first.example.com/v1/chat/completions")
		checkedPrimary.Upstream.Provider = "deepseek-ai"
		checkedPrimary.Model.NeedSensitiveCheck = true
		alternate := alternateWith(func(a *resolvedModelTarget) { a.Model.NeedSensitiveCheck = true })

		assert.NoError(t, chatAlternateSatisfiesPlanGates(alternate, checkedPrimary, autoRoutePlan()))
	})
}

func TestChatAlternateBackendURLRecomputesProtocolRouting(t *testing.T) {
	// The plan's BackendURL belongs to the model it resolved, so the
	// alternate's own upstream has to be routed for this protocol afresh.
	alternate := chatModelFallbackTarget("second-model", "https://second.example.com/v1/chat/completions")

	backendURL, err := chatAlternateBackendURL(alternate, types.ProtocolChat)

	require.NoError(t, err)
	assert.Equal(t, "https://second.example.com/v1/chat/completions", backendURL)
}

func TestChatAlternateBackendURLDefaultsToChat(t *testing.T) {
	alternate := chatModelFallbackTarget("second-model", "https://second.example.com/v1/chat/completions")

	backendURL, err := chatAlternateBackendURL(alternate, "")

	require.NoError(t, err)
	assert.Equal(t, "https://second.example.com/v1/chat/completions", backendURL)
}

func TestRecordAutoRouteFallbackNamesTheServingModel(t *testing.T) {
	decision := &types.AutoRouteDecision{ModelID: "first-model", BenchmarkID: "first-bench", Rank: 1}
	ctx := types.WithAutoRouteDecision(context.Background(), decision)

	recordAutoRouteFallback(ctx, &types.AutoRouteCandidate{
		ModelID: "second-model", BenchmarkID: "second-bench", Rank: 2,
	})

	// The billing record reads this decision, so it must agree with the
	// model the spend is charged to.
	assert.Equal(t, "second-model", decision.ModelID)
	assert.Equal(t, "second-bench", decision.BenchmarkID)
	assert.Equal(t, 2, decision.Rank)
	assert.Equal(t, 1, decision.Fallbacks)
}

func TestRecordAutoRouteFallbackIsANoOpForADirectRequest(t *testing.T) {
	assert.NotPanics(t, func() {
		recordAutoRouteFallback(context.Background(), &types.AutoRouteCandidate{ModelID: "second-model"})
	})
}

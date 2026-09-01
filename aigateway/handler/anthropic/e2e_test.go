package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	"opencsg.com/csghub-server/aigateway/handler/protocol"
	"opencsg.com/csghub-server/aigateway/types"
	commonType "opencsg.com/csghub-server/common/types"
)

// --- Test doubles ---

// fakePlanner implements plan.Planner.  It mimics the real Planner by running
// protocol.ResolveRouting on the provided ModelTarget, but skips balance,
// usage-limit, and sensitive checks unless explicitly configured.
type fakePlanner struct {
	target     *types.ModelTarget
	planErr    error
	errorCode  types.PlanErrorCategory
	sensitive  *types.SafetyDecision
	balanceErr error
	usageErr   error
}

func (f *fakePlanner) Plan(ctx context.Context, meta *types.RequestMetadata) (*types.RequestPlan, error) {
	if f.target == nil && f.planErr != nil {
		p := &types.RequestPlan{ErrorCode: f.errorCode}
		return p, f.planErr
	}

	pl := &types.RequestPlan{ModelTarget: f.target}

	// Run routing just like the real planner.
	decision, err := protocol.ResolveRouting(types.Protocol(meta.Protocol), protocol.RoutingTarget{
		ModelID:          f.target.Model.ID,
		Target:           f.target.Target,
		CSGHubHosted:     isTestCSGHubHosted(f.target),
		RuntimeFramework: f.target.Model.RuntimeFramework,
		ImageID:          f.target.Model.ImageID,
		UpstreamMetadata: f.target.Upstream.Metadata,
	})
	if err != nil {
		pl.ErrorCode = types.PlanErrUnknown
		return pl, err
	}
	pl.RouteMode = string(decision.Mode)
	pl.AdapterKind = string(decision.AdapterKind)
	pl.UpstreamProtocol = string(decision.UpstreamProtocol)
	if decision.Mode == protocol.ModeAdapter {
		pl.UpstreamCap = types.CapabilityFor(decision.UpstreamProtocol)
	}
	if decision.Mode == protocol.ModeDisabled {
		pl.ErrorCode = types.PlanErrDisabled
		pl.BackendURL = f.target.Target
		return pl, fmt.Errorf("protocol not available for this model")
	}

	// Balance check.
	if f.balanceErr != nil {
		pl.ErrorCode = types.PlanErrInsufficientBalance
		return pl, f.balanceErr
	}
	pl.BalanceOK = true

	// Backend URL.
	pl.BackendURL = decision.BackendURL
	if pl.BackendURL == "" {
		pl.BackendURL = f.target.Target
	}

	// Usage-limit check.
	if f.usageErr != nil {
		pl.ErrorCode = types.PlanErrUsageLimitExceeded
		return pl, f.usageErr
	}
	pl.UsageLimitOK = true

	// Sensitive check.
	if f.sensitive != nil && f.sensitive.IsSensitive {
		pl.Safety = f.sensitive
		pl.ErrorCode = types.PlanErrSensitive
		return pl, fmt.Errorf("content blocked due to safety policy")
	}

	return pl, nil
}

func isTestCSGHubHosted(mt *types.ModelTarget) bool {
	return mt != nil && mt.Model != nil && mt.Model.SvcName != ""
}

type fakeProxyExecutor struct{}

func (f *fakeProxyExecutor) ServeProxy(c *gin.Context, backendURL, host string, responseWriter types.HTTPResponseWriter) {
	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, backendURL, c.Request.Body)
	if err != nil {
		responseWriter.WriteHeader(http.StatusBadGateway)
		return
	}
	req.Header = c.Request.Header.Clone()
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		responseWriter.WriteHeader(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for k, vs := range resp.Header {
		for _, v := range vs {
			responseWriter.Header().Add(k, v)
		}
	}
	responseWriter.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(responseWriter, resp.Body)
	responseWriter.Flush()
}

type fakeUsageRecorder struct {
	recorded               bool
	inputTokens            int64
	outputTokens           int64
	cachedPromptTokens     int64
	cacheCreationTokens    int64
	targetModelName        string
}

func (f *fakeUsageRecorder) RecordUsage(ctx context.Context, nsUUID string, model *types.Model, targetModelName string, inputTokens, outputTokens, cachedPromptTokens, cacheCreationPromptTokens int64, apikey string) error {
	f.recorded = true
	f.inputTokens = inputTokens
	f.outputTokens = outputTokens
	f.cachedPromptTokens = cachedPromptTokens
	f.cacheCreationTokens = cacheCreationPromptTokens
	f.targetModelName = targetModelName
	return nil
}

type fakeUsageLimiter struct {
	committed            bool
	inputTokens          int64
	outputTokens         int64
	cachedPromptTokens   int64
	cacheCreationTokens  int64
}

func (f *fakeUsageLimiter) CommitUsageLimitFromUsage(ctx context.Context, nsUUID string, model *types.Model, inputTokens, outputTokens, cachedPromptTokens, cacheCreationPromptTokens int64) error {
	f.committed = true
	f.inputTokens = inputTokens
	f.outputTokens = outputTokens
	f.cachedPromptTokens = cachedPromptTokens
	f.cacheCreationTokens = cacheCreationPromptTokens
	return nil
}

type fakeMetricsRecorder struct {
	modelSet            bool
	modelID             string
	usageRecorded       bool
	inputTokens         int64
	outputTokens        int64
	cachedPromptTokens  int64
}

func (f *fakeMetricsRecorder) SetModelTarget(c *gin.Context, modelID string, target *types.ModelTarget, isStream bool) {
	f.modelSet = true
	f.modelID = modelID
}

func (f *fakeMetricsRecorder) RecordTokenUsage(c *gin.Context, inputTokens, outputTokens, cachedPromptTokens int64) {
	f.usageRecorded = true
	f.inputTokens = inputTokens
	f.outputTokens = outputTokens
	f.cachedPromptTokens = cachedPromptTokens
}

// --- Test helpers ---

func makeTestHandlerWithPlanner(planner *fakePlanner) *Handler {
	h, _, _, _ := makeTestHandlerWithPlannerAndFakes(planner)
	return h
}

func makeTestHandlerWithPlannerAndFakes(planner *fakePlanner) (*Handler, *fakeUsageRecorder, *fakeUsageLimiter, *fakeMetricsRecorder) {
	recorder := &fakeUsageRecorder{}
	limiter := &fakeUsageLimiter{}
	metrics := &fakeMetricsRecorder{}
	h := New(Deps{
		ProxyExecutor:   &fakeProxyExecutor{},
		UsageRecorder:   recorder,
		UsageLimiter:    limiter,
		MetricsRecorder: metrics,
	})
	return h, recorder, limiter, metrics
}

func makeMessagesTarget(upstreamURL, protocol string) *types.ModelTarget {
	meta := map[string]any{}
	if protocol != "" {
		meta["protocol"] = protocol
	}
	return &types.ModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
		},
		Upstream: commonType.UpstreamConfig{
			URL:      upstreamURL,
			Provider: "test",
			Metadata: meta,
		},
		Target:    upstreamURL,
		ModelName: "test-model",
	}
}

func validMessagesBody(model string) string {
	return fmt.Sprintf(`{
		"model": "%s",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": "Hello"}]
	}`, model)
}

// dispatchWithTarget runs the handler through the production Orchestrator with
// a fake Planner configured for the given target.
func dispatchWithTarget(t *testing.T, handler *Handler, body string, planner *fakePlanner) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	orch := plan.NewOrchestrator(planner)
	orch.Dispatch(c, handler, handler)
	return w
}

// --- Native passthrough tests ---

func TestE2E_Native_NonStream_200(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/messages", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{
			"id": "msg_123",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "Hello from native!"}],
			"model": "test-model",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`)
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/messages", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	require.Equal(t, 200, w.Code)
	var resp types.AnthropicMessagesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "message", resp.Type)
	assert.Equal(t, "assistant", resp.Role)
	require.Len(t, resp.Content, 1)
	assert.Equal(t, "Hello from native!", resp.Content[0].Text)
	assert.Equal(t, "end_turn", *resp.StopReason)
	assert.Equal(t, 10, resp.Usage.InputTokens)
	assert.Equal(t, 5, resp.Usage.OutputTokens)
}

func TestE2E_Native_Stream_200(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher := w.(http.Flusher)
		fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"test\",\"content\":[],\"usage\":{\"input_tokens\":5,\"output_tokens\":0}}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi!\"}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		flusher.Flush()
	}))
	defer upstream.Close()

	body := strings.Replace(validMessagesBody("test-model"), `"max_tokens": 1024`, `"max_tokens": 1024, "stream": true`, 1)
	target := makeMessagesTarget(upstream.URL+"/v1/messages", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, body, planner)

	require.Equal(t, 200, w.Code)
	bodyStr := w.Body.String()
	assert.Contains(t, bodyStr, "message_start")
	assert.Contains(t, bodyStr, "content_block_delta")
	assert.Contains(t, bodyStr, "Hi!")
	assert.Contains(t, bodyStr, "message_stop")
}

// --- Messages → Chat adapter tests ---

func TestE2E_ToChat_NonStream_200(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		var req map[string]any
		require.NoError(t, json.Unmarshal(readBody(r.Body), &req))
		assert.Equal(t, "test-model", req["model"])
		assert.Equal(t, float64(1024), req["max_tokens"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{
			"id": "chatcmpl-1",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "Hello via chat adapter!"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 8, "total_tokens": 18}
		}`)
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/chat/completions", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	require.Equal(t, 200, w.Code)
	var resp types.AnthropicMessagesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "message", resp.Type)
	require.Len(t, resp.Content, 1)
	assert.Equal(t, "text", resp.Content[0].Type)
	assert.Equal(t, "Hello via chat adapter!", resp.Content[0].Text)
	assert.Equal(t, "end_turn", *resp.StopReason)
	assert.Equal(t, 10, resp.Usage.InputTokens)
	assert.Equal(t, 8, resp.Usage.OutputTokens)
}

func TestE2E_ToChat_Stream_200(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher := w.(http.Flusher)
		chunks := []string{
			`data: {"id":"1","object":"chat.completion.chunk","model":"test","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"},"finish_reason":null}]}`,
			`data: {"id":"1","object":"chat.completion.chunk","model":"test","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
			`data: {"id":"1","object":"chat.completion.chunk","model":"test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
			`data: [DONE]`,
		}
		for _, chunk := range chunks {
			fmt.Fprintf(w, "%s\n\n", chunk)
			flusher.Flush()
		}
	}))
	defer upstream.Close()

	body := strings.Replace(validMessagesBody("test-model"), `"max_tokens": 1024`, `"max_tokens": 1024, "stream": true`, 1)
	target := makeMessagesTarget(upstream.URL+"/v1/chat/completions", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, body, planner)

	require.Equal(t, 200, w.Code)
	bodyStr := w.Body.String()
	assert.Contains(t, bodyStr, "message_start")
	assert.Contains(t, bodyStr, "content_block_start")
	assert.Contains(t, bodyStr, "text_delta")
	assert.Contains(t, bodyStr, "Hello")
	assert.Contains(t, bodyStr, "world")
	assert.Contains(t, bodyStr, "content_block_stop")
	assert.Contains(t, bodyStr, "message_delta")
	assert.Contains(t, bodyStr, "message_stop")
}

func TestE2E_ToChat_ToolUse_200(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{
			"id": "chatcmpl-2",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "",
					"tool_calls": [{
						"id": "call_1",
						"type": "function",
						"function": {"name": "get_weather", "arguments": "{\"city\":\"SF\"}"}
					}]
				},
				"finish_reason": "tool_calls"
			}],
			"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`)
	}))
	defer upstream.Close()

	body := `{
		"model": "test-model",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": "What's the weather in SF?"}],
		"tools": [{"name": "get_weather", "description": "Get weather", "input_schema": {"type": "object", "properties": {"city": {"type": "string"}}}}]
	}`
	target := makeMessagesTarget(upstream.URL+"/v1/chat/completions", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, body, planner)

	require.Equal(t, 200, w.Code)
	var resp types.AnthropicMessagesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Content, 1)
	assert.Equal(t, "tool_use", resp.Content[0].Type)
	assert.Equal(t, "call_1", resp.Content[0].ID)
	assert.Equal(t, "get_weather", resp.Content[0].Name)
	assert.Equal(t, "tool_use", *resp.StopReason)
}

// --- Messages → Responses adapter tests ---

func TestE2E_ToResponses_NonStream_200(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/responses", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{
			"id": "resp_1",
			"object": "response",
			"created_at": 1234567890,
			"status": "completed",
			"model": "test-model",
			"output": [{
				"type": "message",
				"role": "assistant",
				"content": [{"type": "output_text", "text": "Hello via responses adapter!"}]
			}],
			"usage": {"input_tokens": 10, "output_tokens": 7, "total_tokens": 17}
		}`)
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/responses", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	require.Equal(t, 200, w.Code)
	var resp types.AnthropicMessagesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "message", resp.Type)
	require.Len(t, resp.Content, 1)
	assert.Equal(t, "Hello via responses adapter!", resp.Content[0].Text)
	assert.Equal(t, "end_turn", *resp.StopReason)
	assert.Equal(t, 10, resp.Usage.InputTokens)
	assert.Equal(t, 7, resp.Usage.OutputTokens)
}

func TestE2E_ToResponses_Stream_200(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher := w.(http.Flusher)
		events := []string{
			`event: response.created\ndata: {"type":"response.created","response":{"id":"resp_1","status":"in_progress","model":"test"}}`,
			`event: response.output_text.delta\ndata: {"type":"response.output_text.delta","delta":"Hello"}`,
			`event: response.output_text.delta\ndata: {"type":"response.output_text.delta","delta":"!"}`,
			`event: response.completed\ndata: {"type":"response.completed","response":{"id":"resp_1","status":"completed","model":"test","usage":{"input_tokens":5,"output_tokens":2}}}`,
		}
		for _, e := range events {
			e = strings.ReplaceAll(e, `\n`, "\n")
			fmt.Fprint(w, e+"\n\n")
			flusher.Flush()
		}
	}))
	defer upstream.Close()

	body := strings.Replace(validMessagesBody("test-model"), `"max_tokens": 1024`, `"max_tokens": 1024, "stream": true`, 1)
	target := makeMessagesTarget(upstream.URL+"/v1/responses", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, body, planner)

	require.Equal(t, 200, w.Code)
	bodyStr := w.Body.String()
	assert.Contains(t, bodyStr, "message_start")
	assert.Contains(t, bodyStr, "text_delta")
	assert.Contains(t, bodyStr, "Hello")
	assert.Contains(t, bodyStr, "message_stop")
}

// --- Error status code tests ---

func TestE2E_Native_404(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		fmt.Fprint(w, `{"type":"error","error":{"type":"not_found_error","message":"model not found"}}`)
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/messages", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	assert.Equal(t, 404, w.Code)
}

func TestE2E_Native_429(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(429)
		fmt.Fprint(w, `{"type":"error","error":{"type":"rate_limit_error","message":"rate limited"}}`)
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/messages", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	assert.Equal(t, 429, w.Code)
}

func TestE2E_ToChat_502(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(502)
		fmt.Fprint(w, `{"error":{"message":"bad gateway"}}`)
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/chat/completions", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	// 502 is mapped to 529 overloaded_error in Anthropic format.
	assert.Equal(t, 529, w.Code)
	var errResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "error", errResp["type"])
	errObj, ok := errResp["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "overloaded_error", errObj["type"])
	assert.Contains(t, errObj["message"], "bad gateway")
}

func TestE2E_ToChat_503(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		fmt.Fprint(w, `{"error":{"message":"service unavailable"}}`)
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/chat/completions", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	assert.Equal(t, 529, w.Code)
	var errResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "error", errResp["type"])
}

func TestE2E_ToResponses_504(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(504)
		fmt.Fprint(w, `{"error":{"message":"gateway timeout"}}`)
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/responses", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	assert.Equal(t, 529, w.Code)
	var errResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "error", errResp["type"])
}

func TestE2E_ToChat_500(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, `{"error":{"message":"internal server error"}}`)
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/chat/completions", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	assert.Equal(t, 529, w.Code)
	var errResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "error", errResp["type"])
}

// --- Validation and routing error tests ---

func TestE2E_InvalidRequest_NoModel(t *testing.T) {
	target := makeMessagesTarget("http://upstream/v1/messages", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, `{"max_tokens": 1024, "messages": [{"role": "user", "content": "hi"}]}`, planner)

	assert.Equal(t, 400, w.Code)
	var errResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "error", errResp["type"])
}

func TestE2E_InvalidRequest_NoMaxTokens(t *testing.T) {
	target := makeMessagesTarget("http://upstream/v1/messages", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, `{"model": "test", "messages": [{"role": "user", "content": "hi"}]}`, planner)

	assert.Equal(t, 400, w.Code)
}

func TestE2E_Disabled_NoAdapter(t *testing.T) {
	// Unknown protocol falls back to chat, so adapter mode is selected.
	// The upstream is unreachable, so we get a 502 from the proxy.
	target := &types.ModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
		},
		Upstream: commonType.UpstreamConfig{
			URL:      "http://127.0.0.1:1/v1/unknown",
			Provider: "test",
			Metadata: map[string]any{"protocol": "unknown"},
		},
		Target:    "http://127.0.0.1:1/v1/unknown",
		ModelName: "test-model",
	}
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	assert.True(t, w.Code >= 400)
}

func TestE2E_UnsupportedCapability_CacheControl(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called")
	}))
	defer upstream.Close()

	body := `{
		"model": "test-model",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": [{"type": "text", "text": "hello", "cache_control": {"type": "ephemeral"}}]}]
	}`
	target := makeMessagesTarget(upstream.URL+"/v1/chat/completions", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, body, planner)

	assert.Equal(t, 400, w.Code)
	var errResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	errObj, ok := errResp["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "unsupported_capability", errObj["type"])
	assert.Contains(t, errObj["message"], "prompt_caching")
}

func TestE2E_UnsupportedCapability_Thinking(t *testing.T) {
	// Chat protocol now supports thinking via reasoning_effort translation.
	// The adapter should accept the request and proxy it to the upstream.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify reasoning_effort was sent to the upstream.
		var req map[string]any
		require.NoError(t, json.Unmarshal(readBody(r.Body), &req))
		assert.Equal(t, "low", req["reasoning_effort"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{
			"id": "chatcmpl-1",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "I thought about this."}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 5, "completion_tokens": 4, "total_tokens": 9}
		}`)
	}))
	defer upstream.Close()

	body := `{
		"model": "test-model",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": "think carefully"}],
		"thinking": {"type": "enabled", "budget_tokens": 5000}
	}`
	target := makeMessagesTarget(upstream.URL+"/v1/chat/completions", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, body, planner)

	require.Equal(t, 200, w.Code)
	var resp types.AnthropicMessagesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "I thought about this.", resp.Content[0].Text)
}

// --- Model target resolution error tests ---

func TestE2E_ModelNotFound(t *testing.T) {
	planner := &fakePlanner{
		planErr:   fmt.Errorf("model 'unknown' not found"),
		errorCode: types.PlanErrModelNotFound,
	}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("unknown"), planner)

	assert.Equal(t, 404, w.Code)
}

// --- Balance check error test ---

func TestE2E_InsufficientBalance(t *testing.T) {
	planner := &fakePlanner{
		target:     makeMessagesTarget("http://upstream/v1/messages", ""),
		balanceErr: fmt.Errorf("insufficient balance"),
	}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	assert.Equal(t, 402, w.Code)
}

// --- Sensitive content test ---

func TestE2E_SensitiveContent(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called for sensitive content")
	}))
	defer upstream.Close()

	planner := &fakePlanner{
		target:    makeMessagesTarget(upstream.URL+"/v1/messages", ""),
		sensitive: &types.SafetyDecision{IsSensitive: true, Message: "content blocked"},
	}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	assert.Equal(t, 200, w.Code)
	var resp types.AnthropicMessagesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Content, 1)
	assert.Contains(t, resp.Content[0].Text, "content blocked")
}

// --- CSGHub hosted test ---

func TestE2E_CSGHubHosted_ToChat(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{
			"id": "chatcmpl-1",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "Hello from CSGHub!"},
				"finish_reason": "stop"
			}],
			"usage": {"prompt_tokens": 5, "completion_tokens": 4, "total_tokens": 9}
		}`)
	}))
	defer upstream.Close()

	target := &types.ModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
			InternalModelInfo: types.InternalModelInfo{
				SvcName: "test-svc",
			},
		},
		Upstream: commonType.UpstreamConfig{
			URL:      upstream.URL,
			Provider: "test",
		},
		Target:    upstream.URL,
		ModelName: "test-model",
	}
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	require.Equal(t, 200, w.Code)
	var resp types.AnthropicMessagesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Hello from CSGHub!", resp.Content[0].Text)
}

// --- Helper: read request body ---

func readBody(r io.Reader) []byte {
	data, _ := io.ReadAll(r)
	return data
}

// --- New tests for review fixes ---

// TestE2E_UsageLimitExceeded verifies that a usage limit error is returned
// as a 429 rate_limit_error in Anthropic format.
func TestE2E_UsageLimitExceeded(t *testing.T) {
	planner := &fakePlanner{
		target:   makeMessagesTarget("http://upstream/v1/messages", ""),
		usageErr: fmt.Errorf("quota exceeded"),
	}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	assert.Equal(t, 429, w.Code)
	var errResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "error", errResp["type"])
	errObj, ok := errResp["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "rate_limit_error", errObj["type"])
}

// TestE2E_StopSequences_MappedInResponsesAdapter verifies that stop_sequences
// are mapped to the Responses API "stop" field (via ExtraFields) rather than
// being silently dropped or rejected.
func TestE2E_StopSequences_MappedInResponsesAdapter(t *testing.T) {
	var capturedBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id": "resp_1",
			"object": "response",
			"created_at": 1234567890,
			"status": "completed",
			"model": "test-upstream",
			"output": [{"type": "message", "role": "assistant", "content": [{"type": "output_text", "text": "Hello"}]}],
			"usage": {"input_tokens": 5, "output_tokens": 3}
		}`)
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/responses", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	body := `{
		"model": "test-model",
		"max_tokens": 1024,
		"stop_sequences": ["END"],
		"messages": [{"role": "user", "content": "Hello"}]
	}`
	w := dispatchWithTarget(t, handler, body, planner)

	assert.Equal(t, 200, w.Code)

	var upstreamReq map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(capturedBody, &upstreamReq))
	stopRaw, ok := upstreamReq["stop"]
	require.True(t, ok, "stop field should be present in upstream request")
	var stopArr []string
	require.NoError(t, json.Unmarshal(stopRaw, &stopArr))
	assert.Contains(t, stopArr, "END")
}

// TestE2E_ToChat_400_BadRequest verifies that a 400 from upstream is converted
// to Anthropic error format.
func TestE2E_ToChat_400_BadRequest(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		fmt.Fprint(w, `{"error":{"message":"invalid model"}}`)
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/chat/completions", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	assert.Equal(t, 400, w.Code)
	var errResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "error", errResp["type"])
	errObj, ok := errResp["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "invalid_request_error", errObj["type"])
	assert.Contains(t, errObj["message"], "invalid model")
}

// TestE2E_StreamError_ToChat verifies that when a streaming upstream returns
// an error status before any SSE data is sent, the client receives a proper
// HTTP error status with a JSON body (not a 200 with an SSE error event).
func TestE2E_StreamError_ToChat(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, `{"error":{"message":"internal error"}}`)
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/chat/completions", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	body := `{
		"model": "test-model",
		"max_tokens": 1024,
		"stream": true,
		"messages": [{"role": "user", "content": "Hello"}]
	}`
	w := dispatchWithTarget(t, handler, body, planner)

	// Pre-stream error: should return HTTP 529 with JSON body, not SSE.
	assert.Equal(t, 529, w.Code)
	bodyStr := w.Body.String()
	assert.NotContains(t, bodyStr, "event: error")
	assert.NotContains(t, bodyStr, "message_stop")
	var errResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Equal(t, "error", errResp["type"])
	errObj, ok := errResp["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "overloaded_error", errObj["type"])
	assert.Contains(t, errObj["message"], "internal error")
}

// TestE2E_ToResponses_ToolUse_Stream verifies that function_call arguments
// are correctly forwarded in streaming mode (item_id vs call_id fix).
func TestE2E_ToResponses_ToolUse_Stream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher, _ := w.(http.Flusher)
		fmt.Fprint(w, "event: response.created\ndata: {\"response\":{\"id\":\"resp_1\",\"status\":\"in_progress\"}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: response.output_item.added\ndata: {\"item\":{\"id\":\"item_abc\",\"type\":\"function_call\",\"call_id\":\"call_xyz\",\"name\":\"get_weather\"}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: response.function_call_arguments.delta\ndata: {\"item_id\":\"item_abc\",\"delta\":\"{\\\"city\\\":\\\"SF\\\"}\"}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: response.completed\ndata: {\"response\":{\"id\":\"resp_1\",\"status\":\"completed\",\"usage\":{\"input_tokens\":10,\"output_tokens\":5}}}\n\n")
		flusher.Flush()
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/responses", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	body := `{
		"model": "test-model",
		"max_tokens": 1024,
		"stream": true,
		"messages": [{"role": "user", "content": "What's the weather in SF?"}],
		"tools": [{"name": "get_weather", "description": "Get weather", "input_schema": {"type": "object", "properties": {"city": {"type": "string"}}}}]
	}`
	w := dispatchWithTarget(t, handler, body, planner)

	bodyStr := w.Body.String()
	assert.Contains(t, bodyStr, "tool_use")
	assert.Contains(t, bodyStr, "call_xyz")
	assert.Contains(t, bodyStr, "get_weather")
	assert.Contains(t, bodyStr, "input_json_delta")
	assert.Contains(t, bodyStr, "city")
}

// --- Table-driven error status matrix ---

// expectedAnthropicStatus maps an upstream HTTP status code to the expected
// client-facing status and Anthropic error type, per mapUpstreamStatusToAnthropic.
func expectedAnthropicStatus(upstreamStatus int) (int, string) {
	switch upstreamStatus {
	case 400:
		return 400, ErrTypeInvalidRequest
	case 401:
		return 401, ErrTypeAuthentication
	case 403:
		return 403, ErrTypePermission
	case 404:
		return 404, ErrTypeNotFound
	case 429:
		return 429, ErrTypeRateLimit
	case 500, 502, 503, 504, 529:
		return 529, ErrTypeOverloaded
	default:
		if upstreamStatus >= 500 {
			return 529, ErrTypeOverloaded
		}
		return upstreamStatus, ErrTypeAPI
	}
}

func TestE2E_UpstreamErrors(t *testing.T) {
	statusCodes := []int{400, 401, 403, 404, 429, 500, 502, 503, 504, 529}

	protocols := []struct {
		name string
		path string // upstream URL path suffix
	}{
		{"native", "/v1/messages"},
		{"to_chat", "/v1/chat/completions"},
		{"to_responses", "/v1/responses"},
	}

	modes := []struct {
		name   string
		stream bool
	}{
		{"non_stream", false},
		{"stream", true},
	}

	for _, proto := range protocols {
		for _, mode := range modes {
			for _, code := range statusCodes {
				t.Run(fmt.Sprintf("%s_%s_%d", proto.name, mode.name, code), func(t *testing.T) {
					upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(code)
						fmt.Fprintf(w, `{"error":{"message":"upstream error %d"}}`, code)
					}))
					defer upstream.Close()

					target := makeMessagesTarget(upstream.URL+proto.path, "")
					planner := &fakePlanner{target: target}
					handler := makeTestHandlerWithPlanner(planner)

					body := validMessagesBody("test-model")
					if mode.stream {
						body = strings.Replace(body, `"max_tokens": 1024`, `"max_tokens": 1024, "stream": true`, 1)
					}
					w := dispatchWithTarget(t, handler, body, planner)

					expectedStatus, expectedErrType := expectedAnthropicStatus(code)
					assert.Equal(t, expectedStatus, w.Code, "upstream %d should map to %d", code, expectedStatus)

					bodyStr := w.Body.String()
					assert.NotContains(t, bodyStr, "message_stop", "error response must not contain message_stop")

					var errResp map[string]any
					require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp), "body: %s", bodyStr)
					assert.Equal(t, "error", errResp["type"])
					errObj, ok := errResp["error"].(map[string]any)
					require.True(t, ok)
					assert.Equal(t, expectedErrType, errObj["type"])
				})
			}
		}
	}
}

// --- Mid-stream error tests (SSE already started, then error) ---

func TestE2E_MidStreamError_ToChat(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher := w.(http.Flusher)
		// Send valid chunks first.
		fmt.Fprintf(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"model\":\"test\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hi\"},\"finish_reason\":null}]}\n\n")
		flusher.Flush()
		// Then send an SSE error event.
		fmt.Fprintf(w, "event: error\ndata: {\"error\":{\"message\":\"mid-stream failure\"}}\n\n")
		flusher.Flush()
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/chat/completions", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	body := `{
		"model": "test-model",
		"max_tokens": 1024,
		"stream": true,
		"messages": [{"role": "user", "content": "Hello"}]
	}`
	w := dispatchWithTarget(t, handler, body, planner)

	// Mid-stream error: SSE already started, so status is 200 and error is
	// delivered as an SSE error event.
	assert.Equal(t, 200, w.Code)
	bodyStr := w.Body.String()
	assert.Contains(t, bodyStr, "event: error")
	assert.Contains(t, bodyStr, "overloaded_error")
	assert.NotContains(t, bodyStr, "message_stop")
}

func TestE2E_MidStreamError_ToResponses(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher := w.(http.Flusher)
		// Send valid events first.
		fmt.Fprintf(w, "event: response.created\ndata: {\"response\":{\"id\":\"resp_1\",\"status\":\"in_progress\"}}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: response.output_text.delta\ndata: {\"delta\":\"Hi\"}\n\n")
		flusher.Flush()
		// Then send a response.failed event.
		fmt.Fprintf(w, "event: response.failed\ndata: {\"error\":{\"message\":\"mid-stream failure\"}}\n\n")
		flusher.Flush()
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/responses", "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)
	body := `{
		"model": "test-model",
		"max_tokens": 1024,
		"stream": true,
		"messages": [{"role": "user", "content": "Hello"}]
	}`
	w := dispatchWithTarget(t, handler, body, planner)

	// Mid-stream error: SSE already started, so status is 200 and error is
	// delivered as an SSE error event.
	assert.Equal(t, 200, w.Code)
	bodyStr := w.Body.String()
	assert.Contains(t, bodyStr, "event: error")
	assert.Contains(t, bodyStr, "overloaded_error")
	assert.NotContains(t, bodyStr, "message_stop")
}

// --- Billing verification tests ---

func TestE2E_Billing_TokensRecorded_Native_NonStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{
			"id": "msg_123",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "Hello!"}],
			"model": "test-model",
			"stop_reason": "end_turn",
			"usage": {
				"input_tokens": 10,
				"output_tokens": 5,
				"cache_read_input_tokens": 3,
				"cache_creation_input_tokens": 2
			}
		}`)
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/messages", "")
	planner := &fakePlanner{target: target}
	handler, recorder, limiter, metrics := makeTestHandlerWithPlannerAndFakes(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	require.Equal(t, 200, w.Code)

	// Billing runs in a goroutine; wait for it.
	// Since runPostProcessAsync is async, we need to poll for a short time.
	require.Eventually(t, func() bool {
		return recorder.recorded && limiter.committed
	}, 2*time.Second, 10*time.Millisecond)

	assert.Equal(t, int64(10), recorder.inputTokens)
	assert.Equal(t, int64(5), recorder.outputTokens)
	assert.Equal(t, int64(3), recorder.cachedPromptTokens)
	assert.Equal(t, int64(2), recorder.cacheCreationTokens)

	assert.Equal(t, int64(10), limiter.inputTokens)
	assert.Equal(t, int64(5), limiter.outputTokens)
	assert.Equal(t, int64(3), limiter.cachedPromptTokens)
	assert.Equal(t, int64(2), limiter.cacheCreationTokens)

	assert.True(t, metrics.usageRecorded)
	assert.Equal(t, int64(10), metrics.inputTokens)
	assert.Equal(t, int64(5), metrics.outputTokens)
	assert.Equal(t, int64(3), metrics.cachedPromptTokens)
}

func TestE2E_Billing_TokensRecorded_Native_Stream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher := w.(http.Flusher)
		fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"test\",\"content\":[],\"usage\":{\"input_tokens\":10,\"output_tokens\":0,\"cache_read_input_tokens\":4,\"cache_creation_input_tokens\":1}}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi!\"}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":5}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		flusher.Flush()
	}))
	defer upstream.Close()

	body := strings.Replace(validMessagesBody("test-model"), `"max_tokens": 1024`, `"max_tokens": 1024, "stream": true`, 1)
	target := makeMessagesTarget(upstream.URL+"/v1/messages", "")
	planner := &fakePlanner{target: target}
	handler, recorder, limiter, _ := makeTestHandlerWithPlannerAndFakes(planner)
	w := dispatchWithTarget(t, handler, body, planner)

	require.Equal(t, 200, w.Code)

	require.Eventually(t, func() bool {
		return recorder.recorded && limiter.committed
	}, 2*time.Second, 10*time.Millisecond)

	assert.Equal(t, int64(10), recorder.inputTokens)
	assert.Equal(t, int64(5), recorder.outputTokens)
	assert.Equal(t, int64(4), recorder.cachedPromptTokens)
	assert.Equal(t, int64(1), recorder.cacheCreationTokens)

	assert.Equal(t, int64(4), limiter.cachedPromptTokens)
	assert.Equal(t, int64(1), limiter.cacheCreationTokens)
}

func TestE2E_Billing_TokensRecorded_ToChat_NonStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{
			"id": "chatcmpl-1",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "Hello via chat!"},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 8,
				"total_tokens": 18,
				"prompt_tokens_details": {"cached_tokens": 3}
			}
		}`)
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/chat/completions", "")
	planner := &fakePlanner{target: target}
	handler, recorder, limiter, _ := makeTestHandlerWithPlannerAndFakes(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	require.Equal(t, 200, w.Code)

	require.Eventually(t, func() bool {
		return recorder.recorded && limiter.committed
	}, 2*time.Second, 10*time.Millisecond)

	assert.Equal(t, int64(10), recorder.inputTokens)
	assert.Equal(t, int64(8), recorder.outputTokens)
	assert.Equal(t, int64(3), recorder.cachedPromptTokens)
	// Chat adapter doesn't track cache creation tokens.
	assert.Equal(t, int64(0), recorder.cacheCreationTokens)

	assert.Equal(t, int64(3), limiter.cachedPromptTokens)
}

func TestE2E_Billing_TokensRecorded_ToChat_Stream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher := w.(http.Flusher)
		chunks := []string{
			`data: {"id":"1","object":"chat.completion.chunk","model":"test","choices":[{"index":0,"delta":{"role":"assistant","content":"Hi"},"finish_reason":null}]}`,
			`data: {"id":"1","object":"chat.completion.chunk","model":"test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13,"prompt_tokens_details":{"cached_tokens":5}}}`,
			`data: [DONE]`,
		}
		for _, chunk := range chunks {
			fmt.Fprintf(w, "%s\n\n", chunk)
			flusher.Flush()
		}
	}))
	defer upstream.Close()

	body := strings.Replace(validMessagesBody("test-model"), `"max_tokens": 1024`, `"max_tokens": 1024, "stream": true`, 1)
	target := makeMessagesTarget(upstream.URL+"/v1/chat/completions", "")
	planner := &fakePlanner{target: target}
	handler, recorder, limiter, _ := makeTestHandlerWithPlannerAndFakes(planner)
	w := dispatchWithTarget(t, handler, body, planner)

	require.Equal(t, 200, w.Code)

	require.Eventually(t, func() bool {
		return recorder.recorded && limiter.committed
	}, 2*time.Second, 10*time.Millisecond)

	assert.Equal(t, int64(10), recorder.inputTokens)
	assert.Equal(t, int64(3), recorder.outputTokens)
	assert.Equal(t, int64(5), recorder.cachedPromptTokens)

	assert.Equal(t, int64(5), limiter.cachedPromptTokens)
}

func TestE2E_Billing_TokensRecorded_ToResponses_NonStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{
			"id": "resp_1",
			"object": "response",
			"created_at": 1234567890,
			"status": "completed",
			"model": "test-model",
			"output": [{
				"type": "message",
				"role": "assistant",
				"content": [{"type": "output_text", "text": "Hello via responses!"}]
			}],
			"usage": {
				"input_tokens": 10,
				"output_tokens": 7,
				"total_tokens": 17,
				"input_tokens_details": {
					"cached_tokens": 3,
					"cached_creation_tokens": 2
				}
			}
		}`)
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/responses", "")
	planner := &fakePlanner{target: target}
	handler, recorder, limiter, _ := makeTestHandlerWithPlannerAndFakes(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	require.Equal(t, 200, w.Code)

	require.Eventually(t, func() bool {
		return recorder.recorded && limiter.committed
	}, 2*time.Second, 10*time.Millisecond)

	assert.Equal(t, int64(10), recorder.inputTokens)
	assert.Equal(t, int64(7), recorder.outputTokens)
	assert.Equal(t, int64(3), recorder.cachedPromptTokens)
	assert.Equal(t, int64(2), recorder.cacheCreationTokens)

	assert.Equal(t, int64(3), limiter.cachedPromptTokens)
	assert.Equal(t, int64(2), limiter.cacheCreationTokens)
}

func TestE2E_Billing_TokensRecorded_ToResponses_Stream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher := w.(http.Flusher)
		events := []string{
			`event: response.created\ndata: {"type":"response.created","response":{"id":"resp_1","status":"in_progress","model":"test"}}`,
			`event: response.output_text.delta\ndata: {"type":"response.output_text.delta","delta":"Hello"}`,
			`event: response.completed\ndata: {"type":"response.completed","response":{"id":"resp_1","status":"completed","model":"test","usage":{"input_tokens":10,"output_tokens":2,"input_tokens_details":{"cached_tokens":6,"cached_creation_tokens":1}}}}`,
		}
		for _, e := range events {
			e = strings.ReplaceAll(e, `\n`, "\n")
			fmt.Fprint(w, e+"\n\n")
			flusher.Flush()
		}
	}))
	defer upstream.Close()

	body := strings.Replace(validMessagesBody("test-model"), `"max_tokens": 1024`, `"max_tokens": 1024, "stream": true`, 1)
	target := makeMessagesTarget(upstream.URL+"/v1/responses", "")
	planner := &fakePlanner{target: target}
	handler, recorder, limiter, _ := makeTestHandlerWithPlannerAndFakes(planner)
	w := dispatchWithTarget(t, handler, body, planner)

	require.Equal(t, 200, w.Code)

	require.Eventually(t, func() bool {
		return recorder.recorded && limiter.committed
	}, 2*time.Second, 10*time.Millisecond)

	assert.Equal(t, int64(10), recorder.inputTokens)
	assert.Equal(t, int64(2), recorder.outputTokens)
	assert.Equal(t, int64(6), recorder.cachedPromptTokens)
	assert.Equal(t, int64(1), recorder.cacheCreationTokens)

	assert.Equal(t, int64(6), limiter.cachedPromptTokens)
	assert.Equal(t, int64(1), limiter.cacheCreationTokens)
}

func TestE2E_Billing_NotRecorded_OnError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, `{"error":{"message":"internal error"}}`)
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL+"/v1/messages", "")
	planner := &fakePlanner{target: target}
	handler, recorder, limiter, _ := makeTestHandlerWithPlannerAndFakes(planner)
	w := dispatchWithTarget(t, handler, validMessagesBody("test-model"), planner)

	assert.Equal(t, 529, w.Code)

	// Usage recording should NOT fire on error status.
	// But the usage limiter DOES commit (even on error, to prevent abuse).
	require.Eventually(t, func() bool {
		return limiter.committed
	}, 2*time.Second, 10*time.Millisecond)
	assert.False(t, recorder.recorded, "usage should not be recorded on error status")
}

// --- Claude Code compatibility tests ---

// TestE2E_ClaudeCode_NativeFallback verifies that when a Messages request is
// sent to an external (non-CSGHub-hosted) upstream whose URL does not end in a
// recognizable protocol path (e.g. /v1/messages), the request takes the native
// passthrough path instead of being adapted to Chat.  This is the core fix for
// Claude Code compatibility: Claude Code sends standard Anthropic tools
// (name/description/input_schema without a "type" field) and expects them to
// reach the upstream unchanged.  If the request went through the chat adapter,
// tools would get "type":"function" injected, which Anthropic-native upstreams
// reject.
func TestE2E_ClaudeCode_NativeFallback(t *testing.T) {
	var receivedBody map[string]json.RawMessage
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The upstream URL has no protocol path; the gateway should still
		// forward to it natively (no /v1/chat/completions appended).
		bodyData := readBody(r.Body)
		require.NoError(t, json.Unmarshal(bodyData, &receivedBody))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{
			"id": "msg_cc",
			"type": "message",
			"role": "assistant",
			"content": [{"type": "text", "text": "Hello from Claude Code upstream!"}],
			"model": "test-model",
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`)
	}))
	defer upstream.Close()

	// External upstream with a bare URL (no /v1/messages path).
	// No protocol metadata set — the gateway must default to native Messages.
	target := makeMessagesTarget(upstream.URL, "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)

	body := `{
		"model": "test-model",
		"max_tokens": 32000,
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [{"name": "Bash", "description": "Run a command", "input_schema": {"type": "object", "properties": {"command": {"type": "string"}}}}],
		"thinking": {"type": "adaptive"},
		"context_management": {"edits": [{"keep": "all", "type": "clear_thinking_20251015"}]},
		"output_config": {"effort": "high"},
		"metadata": {"user_id": "test-user-123"},
		"system": [{"type": "text", "text": "You are Claude Code.", "cache_control": {"type": "ephemeral"}}]
	}`
	w := dispatchWithTarget(t, handler, body, planner)

	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())

	// Verify the request was forwarded natively — tools should NOT have
	// "type":"function" injected by the chat adapter.
	toolsRaw, ok := receivedBody["tools"]
	require.True(t, ok, "tools should be present in forwarded request")
	var tools []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(toolsRaw, &tools))
	require.Len(t, tools, 1)
	// The tool must NOT have a "type" field (chat adapter injects "type":"function").
	_, hasType := tools[0]["type"]
	assert.False(t, hasType, "tools should not have 'type' field on native path (chat adapter injects type:function)")

	// Verify Claude Code-specific fields are preserved on native path.
	_, ok = receivedBody["context_management"]
	assert.True(t, ok, "context_management should be preserved on native path")
	_, ok = receivedBody["output_config"]
	assert.True(t, ok, "output_config should be preserved on native path")

	// Verify thinking field preserves "adaptive" type.
	thinkingRaw, ok := receivedBody["thinking"]
	require.True(t, ok, "thinking should be preserved on native path")
	var thinking struct {
		Type string `json:"type"`
	}
	require.NoError(t, json.Unmarshal(thinkingRaw, &thinking))
	assert.Equal(t, "adaptive", thinking.Type, "thinking.type should be 'adaptive'")
}

// TestE2E_ClaudeCode_NativeFallback_Stream verifies the native fallback also
// works for streaming requests with Claude Code-style tools.
func TestE2E_ClaudeCode_NativeFallback_Stream(t *testing.T) {
	var receivedBody map[string]json.RawMessage
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyData := readBody(r.Body)
		require.NoError(t, json.Unmarshal(bodyData, &receivedBody))

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		flusher := w.(http.Flusher)
		fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"test\",\"content\":[],\"usage\":{\"input_tokens\":5,\"output_tokens\":0}}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi!\"}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		flusher.Flush()
	}))
	defer upstream.Close()

	target := makeMessagesTarget(upstream.URL, "")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)

	body := `{
		"model": "test-model",
		"max_tokens": 32000,
		"stream": true,
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [{"name": "Bash", "description": "Run a command", "input_schema": {"type": "object"}}],
		"thinking": {"type": "adaptive"},
		"system": [{"type": "text", "text": "You are Claude Code.", "cache_control": {"type": "ephemeral"}}]
	}`
	w := dispatchWithTarget(t, handler, body, planner)

	require.Equal(t, 200, w.Code)
	bodyStr := w.Body.String()
	assert.Contains(t, bodyStr, "message_start")
	assert.Contains(t, bodyStr, "Hi!")
	assert.Contains(t, bodyStr, "message_stop")

	// Verify tools were forwarded without type:function injection.
	toolsRaw, ok := receivedBody["tools"]
	require.True(t, ok)
	var tools []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(toolsRaw, &tools))
	_, hasType := tools[0]["type"]
	assert.False(t, hasType, "streaming tools should not have 'type' field on native path")
}

// TestE2E_ClaudeCode_CSGHubHosted_StillChat verifies that CSGHub-hosted models
// (vLLM/SGLang) still default to Chat protocol even when the client sends a
// Messages request.  The native fallback must NOT apply to CSGHub-hosted models.
func TestE2E_ClaudeCode_CSGHubHosted_StillChat(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CSGHub-hosted → must go to /v1/chat/completions, not native.
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{
			"id": "chatcmpl-1",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Hello!"}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 5, "completion_tokens": 4, "total_tokens": 9}
		}`)
	}))
	defer upstream.Close()

	target := &types.ModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
			InternalModelInfo: types.InternalModelInfo{
				SvcName: "test-svc",
			},
		},
		Upstream: commonType.UpstreamConfig{
			URL:      upstream.URL,
			Provider: "test",
		},
		Target:    upstream.URL,
		ModelName: "test-model",
	}
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)

	body := `{
		"model": "test-model",
		"max_tokens": 32000,
		"messages": [{"role": "user", "content": "hi"}],
		"tools": [{"name": "Bash", "description": "Run a command", "input_schema": {"type": "object"}}]
	}`
	w := dispatchWithTarget(t, handler, body, planner)

	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
	var resp types.AnthropicMessagesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Hello!", resp.Content[0].Text)
}

// TestE2E_ClaudeCode_MetadataProtocol_OverridesFallback verifies that explicit
// protocol metadata in the upstream config takes priority over the native
// fallback.  An external upstream with metadata protocol=chat should still use
// the chat adapter even though the URL has no recognizable protocol path.
func TestE2E_ClaudeCode_MetadataProtocol_OverridesFallback(t *testing.T) {
	var receivedBody map[string]json.RawMessage
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyData := readBody(r.Body)
		require.NoError(t, json.Unmarshal(bodyData, &receivedBody))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{
			"id": "chatcmpl-1",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "Hello!"}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 5, "completion_tokens": 4, "total_tokens": 9}
		}`)
	}))
	defer upstream.Close()

	// External upstream with bare URL (no protocol path), metadata declares chat.
	// The fallback must NOT override the explicit metadata declaration.
	target := makeMessagesTarget(upstream.URL, "chat")
	planner := &fakePlanner{target: target}
	handler := makeTestHandlerWithPlanner(planner)

	body := `{
		"model": "test-model",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": "Hello"}],
		"tools": [{"name": "Bash", "description": "Run a command", "input_schema": {"type": "object"}}]
	}`
	w := dispatchWithTarget(t, handler, body, planner)

	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())

	// If the request went through the chat adapter, tools will have
	// "type":"function" injected.  If native, tools keep the Anthropic format
	// (no "type" field).  Metadata protocol=chat should force the adapter.
	toolsRaw, ok := receivedBody["tools"]
	require.True(t, ok, "tools should be present")
	var tools []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(toolsRaw, &tools))
	require.Len(t, tools, 1)
	_, hasType := tools[0]["type"]
	assert.True(t, hasType, "tools should have 'type' field — metadata protocol=chat should force chat adapter")
}

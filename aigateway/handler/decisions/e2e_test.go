package decisions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	"opencsg.com/csghub-server/aigateway/handler/protocol"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/proxy"
	commonType "opencsg.com/csghub-server/common/types"
)

// --- Test doubles ---

// fakePlanner implements plan.Planner: it runs protocol.ResolveRouting on the
// configured target, skipping balance/admission/safety unless configured.
type fakePlanner struct {
	target     *types.ModelTarget
	planErr    error
	errorCode  types.PlanErrorCategory
	sensitive  *types.SafetyDecision
	balanceErr error
}

func (f *fakePlanner) Plan(c *gin.Context, meta *types.RequestMetadata) (*types.RequestPlan, error) {
	if f.target == nil && f.planErr != nil {
		return &types.RequestPlan{ErrorCode: f.errorCode}, f.planErr
	}
	pl := &types.RequestPlan{ModelTarget: f.target}
	decision, err := protocol.ResolveRouting(types.Protocol(meta.Protocol), protocol.RoutingTarget{
		ModelID:          f.target.Model.ID,
		Target:           f.target.Target,
		CSGHubHosted:     f.target.Model.SvcName != "",
		RuntimeFramework: f.target.Model.RuntimeFramework,
		ImageID:          f.target.Model.ImageID,
		ProtocolOverride: f.target.Upstream.MetadataProtocol(),
	})
	if err != nil {
		pl.ErrorCode = types.PlanErrUnknown
		return pl, err
	}
	pl.RouteMode = string(decision.Mode)
	pl.AdapterKind = string(decision.AdapterKind)
	pl.UpstreamProtocol = string(decision.UpstreamProtocol)
	if decision.Mode == protocol.ModeDisabled {
		pl.ErrorCode = types.PlanErrDisabled
		pl.BackendURL = f.target.Target
		return pl, fmt.Errorf("protocol %s is not available for this model", meta.Protocol)
	}
	if f.balanceErr != nil {
		pl.ErrorCode = types.PlanErrInsufficientBalance
		return pl, f.balanceErr
	}
	pl.BalanceOK = true
	pl.BackendURL = decision.BackendURL
	if pl.BackendURL == "" {
		pl.BackendURL = f.target.Target
	}
	if f.sensitive != nil && f.sensitive.IsSensitive {
		pl.Safety = f.sensitive
		pl.ErrorCode = types.PlanErrSensitive
		return pl, fmt.Errorf("content blocked due to safety policy")
	}
	return pl, nil
}

// fakeProxyExecutor mimics the production bridge: a real HTTP round trip to
// the backend URL with the request as prepared by the handler, writing the
// upstream response through the provided writer.  Transport failures mimic
// the proxy's ErrorHandler (502, no body).
type fakeProxyExecutor struct{}

func (f *fakeProxyExecutor) ServeProxy(c *gin.Context, backendURL, host string, responseWriter types.HTTPResponseWriter) {
	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, backendURL, c.Request.Body)
	if err != nil {
		responseWriter.WriteHeader(http.StatusBadGateway)
		return
	}
	req.Header = c.Request.Header.Clone()
	if host != "" {
		req.Host = host
	}
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
}

// realProxyExecutor runs the production reverse proxy (builder/proxy) so
// tests can exercise transport-level behavior the RoundTrip fake skips, e.g.
// 1xx informational responses forwarded via Got1xxResponse.
type realProxyExecutor struct{}

func (f *realProxyExecutor) ServeProxy(c *gin.Context, backendURL, host string, responseWriter types.HTTPResponseWriter) {
	rp, err := proxy.NewReverseProxy(backendURL, proxy.WithoutAcceptEncoding())
	if err != nil {
		responseWriter.WriteHeader(http.StatusBadGateway)
		return
	}
	apiPath := ""
	if parsed, perr := url.Parse(backendURL); perr == nil && parsed.Path != "" {
		apiPath = parsed.Path
	}
	rp.ServeHTTP(responseWriter, c.Request, apiPath, host)
}

// fakeProxyStatusExecutor mimics the proxy's ErrorHandler, which answers
// transport-level failures (including client disconnects) with a bare status
// and no body.
type fakeProxyStatusExecutor struct{ status int }

func (f *fakeProxyStatusExecutor) ServeProxy(c *gin.Context, backendURL, host string, responseWriter types.HTTPResponseWriter) {
	responseWriter.WriteHeader(f.status)
}

type fakeUsageRecorder struct {
	recorded        bool
	inputTokens     int64
	outputTokens    int64
	totalTokens     int64
	cachedTokens    int64
	cacheWrite      int64
	reasoningTokens int64
	targetModelName string
}

func (f *fakeUsageRecorder) RecordUsage(ctx context.Context, nsUUID string, model *types.Model, targetModelName string, usage *token.Usage, tokenID int64) error {
	f.recorded = true
	f.inputTokens = usage.PromptTokens
	f.outputTokens = usage.CompletionTokens
	f.totalTokens = usage.TotalTokens
	f.cachedTokens = usage.CachedPromptTokens
	f.cacheWrite = usage.CacheCreationPromptTokens
	f.reasoningTokens = usage.ReasoningTokens
	f.targetModelName = targetModelName
	return nil
}

type fakeMetricsRecorder struct {
	usageRecorded bool
	inputTokens   int64
	outputTokens  int64
	errorRecorded bool
	errorType     string
	errorMessage  string
}

func (f *fakeMetricsRecorder) RecordTokenUsage(c *gin.Context, inputTokens, outputTokens, cachedPromptTokens int64) {
	if inputTokens == 0 && outputTokens == 0 {
		return
	}
	f.usageRecorded = true
	f.inputTokens = inputTokens
	f.outputTokens = outputTokens
}

func (f *fakeMetricsRecorder) SetMetricsError(c *gin.Context, errorType, errorMessage string) {
	f.errorRecorded = true
	f.errorType = errorType
	f.errorMessage = errorMessage
}

// --- Test helpers ---

func makeTestHandler(planner *fakePlanner) (*Handler, *fakeUsageRecorder, *fakeMetricsRecorder) {
	recorder := &fakeUsageRecorder{}
	metrics := &fakeMetricsRecorder{}
	h := New(Deps{
		ProxyExecutor:   &fakeProxyExecutor{},
		UsageRecorder:   recorder,
		MetricsRecorder: metrics,
		ErrorRecorder:   metrics,
	})
	return h, recorder, metrics
}

func makeDecisionsTarget(upstreamURL, protocolOverride string) *types.ModelTarget {
	var meta *commonType.UpstreamMetadata
	if protocolOverride != "" {
		meta = &commonType.UpstreamMetadata{Protocol: protocolOverride}
	}
	return &types.ModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "decision-model"},
		},
		Upstream: commonType.UpstreamConfig{
			URL:        upstreamURL,
			Provider:   "openai",
			AuthHeader: "Bearer upstream-key",
			Metadata:   meta,
		},
		Target:    upstreamURL,
		ModelName: "decision-model-resolved",
	}
}

func validDecisionsBody(model string) string {
	return fmt.Sprintf(`{
		"model": "%s",
		"input": [{"role":"user","content":[{"type":"input_text","text":"The package arrived damaged."}]}],
		"questions": [{"type":"predicate","name":"damaged","instructions":"Is the item damaged?"}]
	}`, model)
}

const decisionsSuccessJSON = `{
	"model": "decision-model-resolved",
	"answers": [{"type":"predicate","name":"damaged","value":true}],
	"usage": {
		"input_tokens": 42, "output_tokens": 3, "total_tokens": 45,
		"input_tokens_details": {"cached_tokens": 7, "cache_write_tokens": 2},
		"output_tokens_details": {"reasoning_tokens": 1}
	}
}`

// dispatchWithTarget runs the handler through the production Orchestrator.
func dispatchWithTarget(t *testing.T, handler *Handler, body string, planner *fakePlanner) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/decisions", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	orch := plan.NewOrchestrator(planner, nil)
	orch.Dispatch(c, handler, handler)
	return w
}

// --- Happy path ---

func TestE2E_Native_200(t *testing.T) {
	var upstreamBody map[string]any
	var upstreamAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/decisions", r.URL.Path)
		upstreamAuth = r.Header.Get("Authorization")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&upstreamBody))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, decisionsSuccessJSON)
	}))
	defer upstream.Close()

	target := makeDecisionsTarget(upstream.URL+"/v1/decisions", "")
	planner := &fakePlanner{target: target}
	handler, recorder, metrics := makeTestHandler(planner)
	w := dispatchWithTarget(t, handler, validDecisionsBody("decision-model"), planner)

	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
	assert.Equal(t, "Bearer upstream-key", upstreamAuth)
	assert.Equal(t, "decision-model-resolved", upstreamBody["model"], "model is remapped to the upstream name")
	assert.JSONEq(t, decisionsSuccessJSON, w.Body.String(), "the raw upstream body is forwarded unchanged")

	assert.True(t, metrics.usageRecorded)
	assert.Equal(t, int64(42), metrics.inputTokens)
	assert.Equal(t, int64(3), metrics.outputTokens)

	require.Eventually(t, func() bool { return recorder.recorded }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, int64(42), recorder.inputTokens)
	assert.Equal(t, int64(3), recorder.outputTokens)
	assert.Equal(t, int64(45), recorder.totalTokens)
	assert.Equal(t, int64(7), recorder.cachedTokens)
	assert.Equal(t, int64(2), recorder.cacheWrite)
	assert.Equal(t, int64(1), recorder.reasoningTokens)
	assert.Equal(t, "decision-model-resolved", recorder.targetModelName)
}

func TestE2E_RawBody_OnlyModelPatched(t *testing.T) {
	var gotRaw []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRaw, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, decisionsSuccessJSON)
	}))
	defer upstream.Close()

	clientInput := `[{"role":"user","content":[{"type":"input_text","text":"damaged"}]}]`
	clientQuestions := `[{"type":"predicate","name":"damaged","custom":"kept"}]`
	body := fmt.Sprintf(`{"trace_id":"d-1","input":%s,"questions":%s,"model":"decision-model","extension":{"a":[1,2]}}`, clientInput, clientQuestions)
	target := makeDecisionsTarget(upstream.URL+"/v1/decisions", "")
	planner := &fakePlanner{target: target}
	handler, _, _ := makeTestHandler(planner)
	w := dispatchWithTarget(t, handler, body, planner)

	require.Equal(t, 200, w.Code)

	// The override path guarantees semantic preservation: the model is
	// replaced and every other field — including unknown extensions and the
	// exact value bytes inside each field — survives (key order/top-level
	// whitespace may change; duplicate keys are invalid JSON and collapse).
	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(gotRaw, &got))
	assert.Equal(t, `"decision-model-resolved"`, string(got["model"]))
	assert.Equal(t, clientInput, string(got["input"]), "input value bytes are preserved")
	assert.Equal(t, clientQuestions, string(got["questions"]), "questions value bytes are preserved")
	assert.Equal(t, `"d-1"`, string(got["trace_id"]), "unknown fields are preserved")
	assert.Equal(t, `{"a":[1,2]}`, string(got["extension"]), "extension fields are preserved")
}

func TestE2E_RawBody_FastPathForwardsClientBytes(t *testing.T) {
	// Without an upstream model_name override the handler forwards the
	// client's original bytes without any JSON round trip.
	var gotRaw []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRaw, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, decisionsSuccessJSON)
	}))
	defer upstream.Close()

	body := "{ \"trace_id\" : \"d-1\",\n  \"input\": [{\"role\":\"user\"}],\n  \"questions\" : [{\"type\":\"predicate\",\"name\":\"x\"}],\n  \"model\" : \"decision-model\" }"
	target := makeDecisionsTarget(upstream.URL+"/v1/decisions", "")
	target.ModelName = ""
	planner := &fakePlanner{target: target}
	handler, _, _ := makeTestHandler(planner)
	w := dispatchWithTarget(t, handler, body, planner)

	require.Equal(t, 200, w.Code)
	assert.Equal(t, body, string(gotRaw), "the no-override fast path forwards the client's bytes untouched")
}

func TestE2E_UpstreamHeadersPreserved(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.decisions+json; charset=utf-8")
		w.Header().Set("X-Upstream-Request-Id", "abc-123")
		fmt.Fprint(w, decisionsSuccessJSON)
	}))
	defer upstream.Close()

	target := makeDecisionsTarget(upstream.URL+"/v1/decisions", "")
	planner := &fakePlanner{target: target}
	handler, _, _ := makeTestHandler(planner)
	w := dispatchWithTarget(t, handler, validDecisionsBody("decision-model"), planner)

	require.Equal(t, 200, w.Code)
	// B6: the upstream's response headers reach the client unchanged; the
	// gateway must not normalize the vendor Content-Type.
	assert.Equal(t, "application/vnd.decisions+json; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, "abc-123", w.Header().Get("X-Upstream-Request-Id"))
	assert.Equal(t, fmt.Sprintf("%d", len(decisionsSuccessJSON)), w.Header().Get("Content-Length"))
}

func TestE2E_HostOverride(t *testing.T) {
	var gotHost string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, decisionsSuccessJSON)
	}))
	defer upstream.Close()

	target := makeDecisionsTarget(upstream.URL+"/v1/decisions", "")
	target.Host = "decisions.internal.svc"
	planner := &fakePlanner{target: target}
	handler, _, _ := makeTestHandler(planner)
	w := dispatchWithTarget(t, handler, validDecisionsBody("decision-model"), planner)

	require.Equal(t, 200, w.Code)
	assert.Equal(t, "decisions.internal.svc", gotHost)
}

// --- Upstream error mapping ---

func TestE2E_UpstreamErrors(t *testing.T) {
	cases := []struct {
		upstreamStatus int
		expectedStatus int
		expectedType   string
	}{
		{400, 400, "invalid_request_error"},
		{401, 401, "authentication_error"},
		{403, 403, "permission_error"},
		{404, 404, "not_found_error"},
		{429, 429, "rate_limit_error"},
		{500, 502, "api_error"},
		{503, 502, "api_error"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("upstream_%d", tc.upstreamStatus), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.upstreamStatus)
				fmt.Fprintf(w, `{"error":{"message":"upstream error %d"}}`, tc.upstreamStatus)
			}))
			defer upstream.Close()

			target := makeDecisionsTarget(upstream.URL+"/v1/decisions", "")
			planner := &fakePlanner{target: target}
			handler, recorder, metrics := makeTestHandler(planner)
			w := dispatchWithTarget(t, handler, validDecisionsBody("decision-model"), planner)

			assert.Equal(t, tc.expectedStatus, w.Code)
			var errResp map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
			errObj := errResp["error"].(map[string]any)
			assert.Equal(t, tc.expectedType, errObj["type"])
			assert.Contains(t, errObj["message"], fmt.Sprintf("upstream error %d", tc.upstreamStatus))
			assert.True(t, metrics.errorRecorded)
			assert.False(t, recorder.recorded, "billing must not run on error status")
		})
	}
}

func TestE2E_MalformedSuccess_Rejected(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"empty object", `{}`},
		{"usage missing", `{"model":"m","answers":[{"type":"predicate","name":"x","value":true}]}`},
		{"answers missing", `{"model":"m","usage":{"input_tokens":1}}`},
		{"answers not array", `{"model":"m","answers":{"x":1},"usage":{"input_tokens":1}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tc.body)
			}))
			defer upstream.Close()

			target := makeDecisionsTarget(upstream.URL+"/v1/decisions", "")
			planner := &fakePlanner{target: target}
			handler, recorder, metrics := makeTestHandler(planner)
			w := dispatchWithTarget(t, handler, validDecisionsBody("decision-model"), planner)

			assert.Equal(t, http.StatusBadGateway, w.Code)
			assert.Contains(t, w.Body.String(), "api_error")
			assert.False(t, metrics.usageRecorded, "malformed success must not be metered")
			assert.False(t, recorder.recorded, "malformed success must not be billed")
		})
	}
}

func TestE2E_UnreachableUpstream_502(t *testing.T) {
	target := makeDecisionsTarget("http://127.0.0.1:1/v1/decisions", "")
	planner := &fakePlanner{target: target}
	handler, _, metrics := makeTestHandler(planner)
	w := dispatchWithTarget(t, handler, validDecisionsBody("decision-model"), planner)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.True(t, metrics.errorRecorded)
}

func TestE2E_EarlyHints_103Then200(t *testing.T) {
	// The production reverse proxy forwards 1xx informational responses
	// (Got1xxResponse -> WriteHeader(103)) before the final 200. A 103 must
	// not lock the recorded status, otherwise the client sees a 502 and the
	// successful usage is skipped.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", "</styles.css>; rel=preload")
		w.WriteHeader(http.StatusEarlyHints)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, decisionsSuccessJSON)
	}))
	defer upstream.Close()

	target := makeDecisionsTarget(upstream.URL+"/v1/decisions", "")
	planner := &fakePlanner{target: target}
	handler, recorder, metrics := makeTestHandler(planner)
	handler.ProxyExecutor = &realProxyExecutor{}
	w := dispatchWithTarget(t, handler, validDecisionsBody("decision-model"), planner)

	require.Equal(t, 200, w.Code, "a 103 Early Hints must not lock the final status")
	assert.JSONEq(t, decisionsSuccessJSON, w.Body.String())
	assert.True(t, metrics.usageRecorded, "the successful exchange must still be metered")
	require.Eventually(t, func() bool { return recorder.recorded }, 2*time.Second, 10*time.Millisecond)
}

func TestE2E_ClientDisconnect_499(t *testing.T) {
	planner := &fakePlanner{target: makeDecisionsTarget("http://upstream.invalid/v1/decisions", "")}
	handler, recorder, metrics := makeTestHandler(planner)
	handler.ProxyExecutor = &fakeProxyStatusExecutor{status: 499}

	w := dispatchWithTarget(t, handler, validDecisionsBody("decision-model"), planner)

	assert.Equal(t, 499, w.Code)
	assert.True(t, metrics.errorRecorded, "the disconnect outcome is recorded")
	assert.Empty(t, metrics.errorType, "a disconnect is not an upstream error class")
	assert.NotEmpty(t, metrics.errorMessage)
	assert.False(t, recorder.recorded, "billing never runs for a disconnected client")
}

func TestE2E_SensitiveContent(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called for sensitive content")
	}))
	defer upstream.Close()

	planner := &fakePlanner{
		target:    makeDecisionsTarget(upstream.URL+"/v1/decisions", ""),
		sensitive: &types.SafetyDecision{IsSensitive: true, Message: "content blocked"},
	}
	handler, _, _ := makeTestHandler(planner)
	w := dispatchWithTarget(t, handler, validDecisionsBody("decision-model"), planner)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "content_policy_violation")
	assert.Contains(t, w.Body.String(), "content blocked")
}

// --- Plan error rendering ---

func TestE2E_PlanErrors(t *testing.T) {
	cases := []struct {
		name           string
		planErr        error
		errorCode      types.PlanErrorCategory
		expectedStatus int
		expectedType   string
	}{
		{"model_not_found", fmt.Errorf("model 'x' not found"), types.PlanErrModelNotFound, 404, "model_not_found"},
		{"model_unavailable", fmt.Errorf("model unavailable"), types.PlanErrModelUnavailable, 503, "model_unavailable"},
		{"insufficient_balance", fmt.Errorf("insufficient balance"), types.PlanErrInsufficientBalance, 402, "insufficient_balance"},
		{"internal", fmt.Errorf("boom"), types.PlanErrInternal, 500, "internal_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			planner := &fakePlanner{planErr: tc.planErr, errorCode: tc.errorCode}
			handler, _, _ := makeTestHandler(planner)
			w := dispatchWithTarget(t, handler, validDecisionsBody("decision-model"), planner)

			assert.Equal(t, tc.expectedStatus, w.Code)
			var errResp map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
			errObj := errResp["error"].(map[string]any)
			assert.Equal(t, tc.expectedType, errObj["type"])
		})
	}
}

func TestE2E_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"no model", `{"input":[{"role":"user"}],"questions":[{"type":"predicate","name":"x"}]}`},
		{"no input", `{"model":"m","questions":[{"type":"predicate","name":"x"}]}`},
		{"null input", `{"model":"m","input":null,"questions":[{"type":"predicate","name":"x"}]}`},
		{"no questions", `{"model":"m","input":[{"role":"user"}]}`},
		{"malformed json", `{"model": `},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			planner := &fakePlanner{target: makeDecisionsTarget("http://upstream/v1/decisions", "")}
			handler, _, _ := makeTestHandler(planner)
			w := dispatchWithTarget(t, handler, tc.body, planner)
			assert.Equal(t, 400, w.Code)
			assert.Contains(t, w.Body.String(), "invalid_request_error")
		})
	}
}

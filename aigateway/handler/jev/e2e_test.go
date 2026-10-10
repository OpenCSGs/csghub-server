package jev

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
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	commonType "opencsg.com/csghub-server/common/types"
)

// --- Test doubles ---

// fakePlanner implements plan.Planner.  It mimics the real Planner by running
// protocol.ResolveRouting on the provided ModelTarget, but skips balance,
// admission, and sensitive checks unless explicitly configured.
type fakePlanner struct {
	target     *types.ModelTarget
	planErr    error
	errorCode  types.PlanErrorCategory
	sensitive  *types.SafetyDecision
	balanceErr error
}

func (f *fakePlanner) Plan(c *gin.Context, meta *types.RequestMetadata) (*types.RequestPlan, error) {
	if f.target == nil && f.planErr != nil {
		p := &types.RequestPlan{ErrorCode: f.errorCode}
		return p, f.planErr
	}

	pl := &types.RequestPlan{ModelTarget: f.target}

	// Run routing just like the real planner.
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

	// Sensitive check.
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

type fakeUsageRecorder struct {
	recorded        bool
	inputTokens     int64
	outputTokens    int64
	totalTokens     int64
	targetModelName string
}

func (f *fakeUsageRecorder) RecordUsage(ctx context.Context, nsUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string, tokenID int64) error {
	f.recorded = true
	f.inputTokens = usage.PromptTokens
	f.outputTokens = usage.CompletionTokens
	f.totalTokens = usage.TotalTokens
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
	// Mirror the production bridge: zero usage is not recorded.
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

func makeSystemOneTarget(upstreamURL, protocolOverride string) *types.ModelTarget {
	var meta *commonType.UpstreamMetadata
	if protocolOverride != "" {
		meta = &commonType.UpstreamMetadata{Protocol: protocolOverride}
	}
	return &types.ModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "jev-1.13"},
		},
		Upstream: commonType.UpstreamConfig{
			URL:        upstreamURL,
			Provider:   "TypeSafe",
			AuthHeader: "Bearer upstream-key",
			Metadata:   meta,
		},
		Target:    upstreamURL,
		ModelName: "typesafe/jev-1.13",
	}
}

func validSystemOneBody(model string) string {
	return fmt.Sprintf(`{
		"model": "%s",
		"state": "I was charged twice for my subscription.",
		"questions": {
			"refund": {
				"type": "noul",
				"instructions": "Is the customer asking for money back?"
			}
		}
	}`, model)
}

// dispatchWithTarget runs the handler through the production Orchestrator
// with a fake Planner configured for the given target.
func dispatchWithTarget(t *testing.T, handler *Handler, body string, planner *fakePlanner) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/systemone", strings.NewReader(body))
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
		assert.Equal(t, "/v1/systemone", r.URL.Path)
		upstreamAuth = r.Header.Get("Authorization")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&upstreamBody))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{
			"id": "gen-1",
			"model": "typesafe/jev-1.13-20260917",
			"provider": "TypeSafe",
			"answers": {"refund": {"type": "noul", "noul": 0.98}},
			"usage": {"input_tokens": 275, "output_tokens": 20, "cost": 0.00003}
		}`)
	}))
	defer upstream.Close()

	target := makeSystemOneTarget(upstream.URL+"/v1/systemone", "")
	planner := &fakePlanner{target: target}
	handler, recorder, metrics := makeTestHandler(planner)
	w := dispatchWithTarget(t, handler, validSystemOneBody("jev-1.13"), planner)

	require.Equal(t, 200, w.Code)

	// The upstream request carries the resolved model name and the
	// configured upstream credential.
	assert.Equal(t, "Bearer upstream-key", upstreamAuth)
	assert.Equal(t, "typesafe/jev-1.13", upstreamBody["model"])

	// The forwarded response keeps the upstream's Content-Type.
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// The raw upstream body is forwarded unchanged.
	assert.JSONEq(t, `{
		"id": "gen-1",
		"model": "typesafe/jev-1.13-20260917",
		"provider": "TypeSafe",
		"answers": {"refund": {"type": "noul", "noul": 0.98}},
		"usage": {"input_tokens": 275, "output_tokens": 20, "cost": 0.00003}
	}`, w.Body.String())

	// Metrics and billing run from the parsed usage.
	assert.True(t, metrics.usageRecorded)
	assert.Equal(t, int64(275), metrics.inputTokens)
	assert.Equal(t, int64(20), metrics.outputTokens)

	require.Eventually(t, func() bool {
		return recorder.recorded
	}, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, int64(275), recorder.inputTokens)
	assert.Equal(t, int64(20), recorder.outputTokens)
	assert.Equal(t, int64(295), recorder.totalTokens)
	assert.Equal(t, "typesafe/jev-1.13", recorder.targetModelName)
}

func TestE2E_UnknownResponseFieldsForwarded(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id": "gen-2",
			"model": "typesafe/jev-1.13",
			"provider": "TypeSafe",
			"answers": {"refund": {"type": "noul", "noul": 0.5, "extra_key": "kept"}},
			"usage": {"input_tokens": 1, "output_tokens": 1},
			"provider_note": "passthrough"
		}`)
	}))
	defer upstream.Close()

	target := makeSystemOneTarget(upstream.URL+"/v1/systemone", "")
	planner := &fakePlanner{target: target}
	handler, _, _ := makeTestHandler(planner)
	w := dispatchWithTarget(t, handler, validSystemOneBody("jev-1.13"), planner)

	require.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "extra_key")
	assert.Contains(t, w.Body.String(), "kept")
	assert.Contains(t, w.Body.String(), "provider_note")
	assert.Contains(t, w.Body.String(), "passthrough")
}

func TestE2E_StructuredState_Passthrough(t *testing.T) {
	// A structured (object) state must pass through to the upstream
	// unchanged and still flow through the full success path.
	var gotState map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		gotState = body["state"].(map[string]any)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id": "gen-4",
			"model": "typesafe/jev-1.13",
			"provider": "TypeSafe",
			"answers": {"refund": {"type": "noul", "noul": 0.9}},
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`)
	}))
	defer upstream.Close()

	target := makeSystemOneTarget(upstream.URL+"/v1/systemone", "")
	planner := &fakePlanner{target: target}
	handler, recorder, metrics := makeTestHandler(planner)
	body := `{
		"model": "jev-1.13",
		"state": {"ticket": {"id": "T-1"}, "order": {"amount": 42}, "policy": "refund"},
		"questions": {"refund": {"type": "noul", "instructions": "Is the customer asking for money back?"}}
	}`
	w := dispatchWithTarget(t, handler, body, planner)

	require.Equal(t, 200, w.Code, "body: %s", w.Body.String())
	assert.Equal(t, "T-1", gotState["ticket"].(map[string]any)["id"])
	assert.Equal(t, float64(42), gotState["order"].(map[string]any)["amount"])
	assert.Equal(t, "refund", gotState["policy"])

	assert.True(t, metrics.usageRecorded)
	assert.Equal(t, int64(10), metrics.inputTokens)
	require.Eventually(t, func() bool {
		return recorder.recorded
	}, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, int64(15), recorder.totalTokens)
}

func TestE2E_MalformedSuccess_Rejected(t *testing.T) {
	// A 2xx response without the protocol-required fields must surface as
	// an upstream error (502), never as a zero-usage success.
	cases := []struct {
		name string
		body string
	}{
		{"empty object", `{}`},
		{"usage missing", `{"id": "gen-3", "model": "m", "answers": {}}`},
		{"answers missing", `{"id": "gen-3", "model": "m", "usage": {"input_tokens": 1}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tc.body)
			}))
			defer upstream.Close()

			target := makeSystemOneTarget(upstream.URL+"/v1/systemone", "")
			planner := &fakePlanner{target: target}
			handler, recorder, metrics := makeTestHandler(planner)
			w := dispatchWithTarget(t, handler, validSystemOneBody("jev-1.13"), planner)

			assert.Equal(t, http.StatusBadGateway, w.Code)
			assert.Contains(t, w.Body.String(), "api_error")
			assert.False(t, metrics.usageRecorded, "malformed success must not be metered as usage")
			assert.False(t, recorder.recorded, "malformed success must not be billed")
		})
	}
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

			target := makeSystemOneTarget(upstream.URL+"/v1/systemone", "")
			planner := &fakePlanner{target: target}
			handler, recorder, metrics := makeTestHandler(planner)
			w := dispatchWithTarget(t, handler, validSystemOneBody("jev-1.13"), planner)

			assert.Equal(t, tc.expectedStatus, w.Code)
			var errResp map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
			errObj, ok := errResp["error"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tc.expectedType, errObj["type"])
			assert.Contains(t, errObj["message"], fmt.Sprintf("upstream error %d", tc.upstreamStatus))

			assert.True(t, metrics.errorRecorded)
			assert.False(t, recorder.recorded, "billing must not run on error status")
		})
	}
}

func TestE2E_UnreachableUpstream_502(t *testing.T) {
	// A transport failure (proxy ErrorHandler writes an empty 502) renders
	// as a gateway 502 api_error.
	target := makeSystemOneTarget("http://127.0.0.1:1/v1/systemone", "")
	planner := &fakePlanner{target: target}
	handler, _, metrics := makeTestHandler(planner)
	w := dispatchWithTarget(t, handler, validSystemOneBody("jev-1.13"), planner)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	var errResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	errObj := errResp["error"].(map[string]any)
	assert.Equal(t, "api_error", errObj["type"])
	assert.True(t, metrics.errorRecorded)
	assert.Equal(t, "api_error", metrics.errorType)
}

func TestE2E_HostOverride(t *testing.T) {
	// The upstream host override reaches the proxy request's Host header.
	var gotHost string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"answers": {"q": {"type": "noul"}}, "usage": {"input_tokens": 1, "output_tokens": 1}}`)
	}))
	defer upstream.Close()

	target := makeSystemOneTarget(upstream.URL+"/v1/systemone", "")
	target.Host = "jev.internal.svc"
	planner := &fakePlanner{target: target}
	handler, _, _ := makeTestHandler(planner)
	w := dispatchWithTarget(t, handler, validSystemOneBody("jev-1.13"), planner)

	require.Equal(t, 200, w.Code)
	assert.Equal(t, "jev.internal.svc", gotHost)
}

func TestE2E_BadUpstreamResponse_502(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html>not json</html>`)
	}))
	defer upstream.Close()

	target := makeSystemOneTarget(upstream.URL+"/v1/systemone", "")
	planner := &fakePlanner{target: target}
	handler, _, _ := makeTestHandler(planner)
	w := dispatchWithTarget(t, handler, validSystemOneBody("jev-1.13"), planner)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "non-decodable")
}

// --- Raw-body passthrough ---

func TestPatchJevRawModel(t *testing.T) {
	raw := []byte(`{"model":"jev-1.13","state":"s","questions":{"q":{"type":"noul"}},"trace_id":"t1"}`)
	out, err := patchJevRawModel(raw, "typesafe/jev-1.13")
	require.NoError(t, err)

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out, &payload))
	assert.Equal(t, `"typesafe/jev-1.13"`, string(payload["model"]), "only the model is rewritten")
	// Every other value stays byte-identical to what the client sent.
	assert.Equal(t, `"s"`, string(payload["state"]))
	assert.Equal(t, `{"q":{"type":"noul"}}`, string(payload["questions"]))
	assert.Equal(t, `"t1"`, string(payload["trace_id"]))
}

func TestUpstreamRequestBody_TypedFallback(t *testing.T) {
	// Programmatically built requests (no raw body) keep the typed path.
	req := &types.JevRequest{Model: "jev-1.13", State: json.RawMessage(`"s"`)}
	out, err := upstreamRequestBody(req, "typesafe/jev-1.13")
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(out, &body))
	assert.Equal(t, "typesafe/jev-1.13", body["model"])
}

func TestE2E_RawBody_FastPathForwardsClientBytes(t *testing.T) {
	// Without an upstream model_name override, the upstream receives the
	// client's bytes exactly as sent — whitespace, key order, and unknown
	// fields included — with no JSON round trip.
	var gotRaw []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRaw, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"answers": {"q": {"type": "noul"}}, "usage": {"input_tokens": 1, "output_tokens": 1}}`)
	}))
	defer upstream.Close()

	clientBody := `{"trace_id":"e2e-raw",  "questions" : {"q":{"type":"noul","instructions":"i"}},"state":"s","model":"jev-1.13"}`
	target := makeSystemOneTarget(upstream.URL+"/v1/systemone", "")
	target.ModelName = ""
	planner := &fakePlanner{target: target}
	handler, _, _ := makeTestHandler(planner)
	w := dispatchWithTarget(t, handler, clientBody, planner)

	require.Equal(t, 200, w.Code)
	require.Equal(t, clientBody, string(gotRaw), "fast path must forward the client's bytes untouched")
}

func TestE2E_RawBody_OnlyModelPatched(t *testing.T) {
	// With an override, only the model value changes; every other value is
	// byte-identical to what the client sent.
	var got map[string]json.RawMessage
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &got))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"answers": {"q": {"type": "noul"}}, "usage": {"input_tokens": 1, "output_tokens": 1}}`)
	}))
	defer upstream.Close()

	clientState := `{"ticket":{"id":"T-1"},"policy":"refund"}`
	clientQuestions := `{"q":{"type":"noul","instructions":"score","custom":"kept"}}`
	body := fmt.Sprintf(`{"trace_id":"t1","state":%s,"questions":%s,"model":"jev-1.13"}`, clientState, clientQuestions)
	target := makeSystemOneTarget(upstream.URL+"/v1/systemone", "")
	planner := &fakePlanner{target: target}
	handler, _, _ := makeTestHandler(planner)
	w := dispatchWithTarget(t, handler, body, planner)

	require.Equal(t, 200, w.Code)
	assert.Equal(t, `"typesafe/jev-1.13"`, string(got["model"]))
	assert.Equal(t, clientState, string(got["state"]), "state is forwarded verbatim")
	assert.Equal(t, clientQuestions, string(got["questions"]), "questions are forwarded verbatim")
	assert.Equal(t, `"t1"`, string(got["trace_id"]), "unknown fields are forwarded verbatim")
}

// fakeProxyStatusExecutor mimics the proxy's ErrorHandler, which answers
// transport-level failures (including client disconnects) with a bare
// status and no body.
type fakeProxyStatusExecutor struct{ status int }

func (f *fakeProxyStatusExecutor) ServeProxy(c *gin.Context, backendURL, host string, responseWriter types.HTTPResponseWriter) {
	responseWriter.WriteHeader(f.status)
}

func TestE2E_ClientDisconnect_499(t *testing.T) {
	// The proxy answers a mid-call client disconnect with 499 and no body.
	// The status must be propagated unchanged, the outcome recorded on
	// metrics (with an empty error class) and trace, and nothing may panic
	// on the empty error type - the disconnect path is a routine gateway
	// event.
	planner := &fakePlanner{target: makeSystemOneTarget("http://upstream.invalid/v1/systemone", "")}
	handler, recorder, metrics := makeTestHandler(planner)
	handler.ProxyExecutor = &fakeProxyStatusExecutor{status: 499}

	w := dispatchWithTarget(t, handler, validSystemOneBody("jev-1.13"), planner)

	assert.Equal(t, 499, w.Code)
	assert.True(t, metrics.errorRecorded, "the disconnect outcome is recorded on the metrics")
	assert.Empty(t, metrics.errorType, "a disconnect is not an upstream error class")
	assert.NotEmpty(t, metrics.errorMessage)
	assert.False(t, recorder.recorded, "billing never runs for a disconnected client")
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
		{"queue_timeout", fmt.Errorf("queue timeout"), types.PlanErrQueueTimeout, 408, "timeout_error"},
		{"internal", fmt.Errorf("boom"), types.PlanErrInternal, 500, "internal_error"},
		{"disabled", fmt.Errorf("protocol disabled"), types.PlanErrDisabled, 400, "unsupported_feature"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			planner := &fakePlanner{planErr: tc.planErr, errorCode: tc.errorCode}
			handler, _, _ := makeTestHandler(planner)
			w := dispatchWithTarget(t, handler, validSystemOneBody("jev-1.13"), planner)

			assert.Equal(t, tc.expectedStatus, w.Code)
			var errResp map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
			errObj, ok := errResp["error"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tc.expectedType, errObj["type"])
		})
	}
}

func TestE2E_SensitiveContent(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called for sensitive content")
	}))
	defer upstream.Close()

	planner := &fakePlanner{
		target:    makeSystemOneTarget(upstream.URL+"/v1/systemone", ""),
		sensitive: &types.SafetyDecision{IsSensitive: true, Message: "content blocked"},
	}
	handler, _, _ := makeTestHandler(planner)
	w := dispatchWithTarget(t, handler, validSystemOneBody("jev-1.13"), planner)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "content_policy_violation")
	assert.Contains(t, w.Body.String(), "content blocked")
}

func TestE2E_Disabled_NoAdapter(t *testing.T) {
	// A chat upstream cannot serve a systemone request: the planner rejects
	// the request and the handler renders the disabled error.
	target := makeSystemOneTarget("http://127.0.0.1:1/v1/chat/completions", "")
	planner := &fakePlanner{target: target}
	handler, _, _ := makeTestHandler(planner)
	w := dispatchWithTarget(t, handler, validSystemOneBody("jev-1.13"), planner)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "unsupported_feature")
}

// --- Request validation ---

func TestE2E_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"no model", `{"state": "s", "questions": {"q": {"type": "noul"}}}`},
		{"no state", `{"model": "jev-1.13", "questions": {"q": {"type": "noul"}}}`},
		{"no questions", `{"model": "jev-1.13", "state": "s"}`},
		{"question without type", `{"model": "jev-1.13", "state": "s", "questions": {"q": {}}}`},
		{"choice without criteria", `{"model": "jev-1.13", "state": "s", "questions": {"q": {"type": "choice"}}}`},
		{"malformed json", `{"model": `},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			planner := &fakePlanner{target: makeSystemOneTarget("http://upstream/v1/systemone", "")}
			handler, _, _ := makeTestHandler(planner)
			w := dispatchWithTarget(t, handler, tc.body, planner)

			assert.Equal(t, 400, w.Code)
			assert.Contains(t, w.Body.String(), "invalid_request_error")
		})
	}
}

// --- Routing-mode guard ---

func TestE2E_UnsupportedRouteMode(t *testing.T) {
	// An adapter-mode plan must never reach the invoker for systemone.
	planner := &fakePlanner{target: makeSystemOneTarget("http://upstream/v1/systemone", "")}
	handler, _, _ := makeTestHandler(planner)

	// Bypass the fake planner and dispatch with a hand-built adapter plan.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/systemone", strings.NewReader(validSystemOneBody("jev-1.13")))
	c.Request.Header.Set("Content-Type", "application/json")

	meta := &types.RequestMetadata{
		Protocol: "systemone",
		Task:     "systemone",
		Model:    "jev-1.13",
		ParsedBody: &types.JevRequest{
			Model:     "jev-1.13",
			State:     json.RawMessage(`"s"`),
			Questions: map[string]types.JevQuestion{"q": {Type: "noul"}},
		},
	}
	p := &types.RequestPlan{
		ModelTarget: makeSystemOneTarget("http://upstream/v1/systemone", ""),
		RouteMode:   string(protocol.ModeAdapter),
		BackendURL:  "http://upstream/v1/systemone",
	}
	require.NoError(t, handler.Execute(c, meta, p))
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "unsupported route mode")
}

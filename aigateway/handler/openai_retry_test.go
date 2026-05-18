package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mocktoken "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/token"
	commontypes "opencsg.com/csghub-server/common/types"

	"opencsg.com/csghub-server/aigateway/types"
)

type testCommonResponseWriter struct {
	headers    http.Header
	body       bytes.Buffer
	statusCode int
}

type testChatAttemptFailureReporter struct {
	mu     sync.Mutex
	doneCh chan struct{}
	events []ChatAttemptFailureEvent
}

func newTestChatAttemptFailureReporter() *testChatAttemptFailureReporter {
	return &testChatAttemptFailureReporter{
		doneCh: make(chan struct{}, 10),
	}
}

func (r *testChatAttemptFailureReporter) ReportChatAttemptFailure(_ context.Context, event ChatAttemptFailureEvent) error {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
	r.doneCh <- struct{}{}
	return nil
}

func (r *testChatAttemptFailureReporter) Wait() {
	<-r.doneCh
}

func (r *testChatAttemptFailureReporter) Events() []ChatAttemptFailureEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]ChatAttemptFailureEvent, len(r.events))
	copy(cp, r.events)
	return cp
}

func (r *testChatAttemptFailureReporter) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

func newTestCommonResponseWriter() *testCommonResponseWriter {
	return &testCommonResponseWriter{
		headers: make(http.Header),
	}
}

func (w *testCommonResponseWriter) Header() http.Header {
	return w.headers
}

func (w *testCommonResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *testCommonResponseWriter) Write(data []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return w.body.Write(data)
}

func (w *testCommonResponseWriter) Flush() {}

func (w *testCommonResponseWriter) ClearBuffer() {}

func TestApplyChatFallbackTarget(t *testing.T) {
	headers := make(http.Header)
	modelTarget := &resolvedModelTarget{
		Model: &types.Model{
			BaseModel:         types.BaseModel{ID: "test-model"},
			ExternalModelInfo: types.ExternalModelInfo{AuthHead: `{"Authorization":"Bearer primary-token"}`, Provider: "primary-provider"},
			Upstreams: []commontypes.UpstreamConfig{
				{
					URL:     "https://primary.example.com/v1/chat/completions",
					Enabled: true,
				},
				{
					URL:       "https://fallback.example.com/v1/chat/completions",
					Enabled:   true,
					ModelName: "fallback-model",
				},
			},
		},
		ModelName: "test-model",
		Target:    "https://primary.example.com/v1/chat/completions",
	}

	applyChatFallbackTarget(context.Background(), headers, modelTarget, chatAttemptTarget{
		Upstream: commontypes.UpstreamConfig{
			URL:        "https://fallback.example.com/v1/chat/completions",
			Enabled:    true,
			Provider:   "fallback-provider",
			AuthHeader: `{"Authorization":"Bearer fallback-token"}`,
			ModelName:  "fallback-model",
		},
	}, nil, nil)

	require.Equal(t, "https://fallback.example.com/v1/chat/completions", modelTarget.Target)
	require.Equal(t, "https://fallback.example.com/v1/chat/completions", modelTarget.Model.Endpoint)
	require.Equal(t, "https://fallback.example.com/v1/chat/completions", modelTarget.Upstream.URL)
	require.Equal(t, "fallback-model", modelTarget.ModelName)
	require.Equal(t, "fallback-provider", modelTarget.Model.Provider)
	require.Equal(t, `{"Authorization":"Bearer fallback-token"}`, modelTarget.Model.AuthHead)
	require.Equal(t, "Bearer fallback-token", headers.Get("Authorization"))
}

func TestExecuteChatProxyAttempt_ReturnsUsageLimitError(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil

	expectedErr := errors.New("usage limited")
	modelTarget := &resolvedModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
			Endpoint:  "https://primary.example.com/v1/chat/completions",
		},
		ModelName: "test-model",
		Target:    "https://primary.example.com/v1/chat/completions",
	}
	tester.mocks.openAIComp.EXPECT().
		CheckUsageLimit(mock.Anything, "user-1", modelTarget.Model, modelTarget.Target).
		Return(expectedErr).
		Once()

	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{"message":"hello"}`)))
	writer := newTestCommonResponseWriter()
	chatReq := &ChatCompletionRequest{Model: "test-model"}

	retryWriter, err := tester.handler.executeChatProxyAttempt(c, writer, modelTarget, "user-1", chatReq)

	require.Nil(t, retryWriter)
	require.ErrorIs(t, err, expectedErr)
	require.Equal(t, 0, writer.statusCode)
	require.Empty(t, writer.body.String())
}

func TestExecuteChatProxyAttempt_ProxiesRequestAfterUsageLimitCheck(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil

	var receivedPath string
	var receivedBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(`{"status":"ok"}`))
		require.NoError(t, err)
	}))
	defer upstream.Close()

	targetURL := upstream.URL + "/v1/chat/completions"
	modelTarget := &resolvedModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
			Endpoint:  targetURL,
		},
		ModelName: "test-model",
		Target:    targetURL,
	}
	tester.mocks.openAIComp.EXPECT().
		CheckUsageLimit(mock.Anything, "user-1", modelTarget.Model, targetURL).
		Return(nil).
		Once()

	requestBody := []byte(`{"message":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(requestBody))
	writer := newTestCommonResponseWriter()
	chatReq := &ChatCompletionRequest{
		Model: "test-model",
	}

	retryWriter, err := tester.handler.executeChatProxyAttempt(c, writer, modelTarget, "user-1", chatReq)

	require.NoError(t, err)
	require.NotNil(t, retryWriter)
	require.Equal(t, "/v1/chat/completions", receivedPath)
	var forwardedBody map[string]any
	require.NoError(t, json.Unmarshal([]byte(receivedBody), &forwardedBody))
	require.Equal(t, "test-model", forwardedBody["model"])
	require.Equal(t, http.StatusOK, retryWriter.StatusCode())
	require.True(t, retryWriter.StreamStarted())
}

func TestRetryChatWithFallback_ReturnsNilWithoutFallbackTargets(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{}`)))

	modelTarget := &resolvedModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
			Endpoint:  "https://primary.example.com/v1/chat/completions",
		},
		ModelName:      "test-model",
		Target:         "https://primary.example.com/v1/chat/completions",
		AttemptTargets: []chatAttemptTarget{{Upstream: commontypes.UpstreamConfig{URL: "https://primary.example.com/v1/chat/completions", Enabled: true}}},
	}

	err := tester.handler.retryChatWithFallback(c, newTestCommonResponseWriter(), modelTarget, "user-1", &ChatCompletionRequest{Model: "test-model"}, nil, nil)

	require.NoError(t, err)
}

func TestRetryChatWithFallback_ReplaysLastRetryableFallbackResponse(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil

	lastFallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, err := w.Write([]byte(`fallback unavailable`))
		require.NoError(t, err)
	}))
	defer lastFallback.Close()

	lastFallbackURL := lastFallback.URL + "/v1/chat/completions"
	modelTarget := &resolvedModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
			Endpoint:  "https://primary.example.com/v1/chat/completions",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: lastFallbackURL, Enabled: true},
			},
		},
		ModelName: "test-model",
		Target:    "https://primary.example.com/v1/chat/completions",
		AttemptTargets: []chatAttemptTarget{
			{Upstream: commontypes.UpstreamConfig{URL: "https://primary.example.com/v1/chat/completions", Enabled: true}},
			{Upstream: commontypes.UpstreamConfig{URL: lastFallbackURL, Enabled: true}},
		},
	}
	tester.mocks.openAIComp.EXPECT().
		CheckUsageLimit(mock.Anything, "user-1", modelTarget.Model, lastFallbackURL).
		Return(nil).
		Once()

	body := []byte(`{"message":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	writer := newTestCommonResponseWriter()

	err := tester.handler.retryChatWithFallback(c, writer, modelTarget, "user-1", &ChatCompletionRequest{Model: "test-model"}, nil, nil)

	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, writer.statusCode)
	require.Equal(t, `fallback unavailable`, writer.body.String())
}

func TestRetryChatWithFallback_ContinuesUntilNextFallbackSucceeds(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil

	firstFallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, err := w.Write([]byte(`bad gateway`))
		require.NoError(t, err)
	}))
	defer firstFallback.Close()

	secondFallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`ok from second fallback`))
		require.NoError(t, err)
	}))
	defer secondFallback.Close()

	firstFallbackURL := firstFallback.URL + "/v1/chat/completions"
	secondFallbackURL := secondFallback.URL + "/v1/chat/completions"
	modelTarget := &resolvedModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
			Endpoint:  "https://primary.example.com/v1/chat/completions",
			Upstreams: []commontypes.UpstreamConfig{
				{URL: firstFallbackURL, Enabled: true},
				{URL: secondFallbackURL, Enabled: true},
			},
		},
		ModelName: "test-model",
		Target:    "https://primary.example.com/v1/chat/completions",
		AttemptTargets: []chatAttemptTarget{
			{Upstream: commontypes.UpstreamConfig{URL: "https://primary.example.com/v1/chat/completions", Enabled: true}},
			{Upstream: commontypes.UpstreamConfig{URL: firstFallbackURL, Enabled: true}},
			{Upstream: commontypes.UpstreamConfig{URL: secondFallbackURL, Enabled: true}},
		},
	}
	tester.mocks.openAIComp.EXPECT().
		CheckUsageLimit(mock.Anything, "user-1", modelTarget.Model, firstFallbackURL).
		Return(nil).
		Once()
	tester.mocks.openAIComp.EXPECT().
		CheckUsageLimit(mock.Anything, "user-1", modelTarget.Model, secondFallbackURL).
		Return(nil).
		Once()

	body := []byte(`{"message":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	writer := newTestCommonResponseWriter()

	err := tester.handler.retryChatWithFallback(c, writer, modelTarget, "user-1", &ChatCompletionRequest{Model: "test-model"}, nil, nil)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, writer.statusCode)
	require.Equal(t, `ok from second fallback`, writer.body.String())
	require.Equal(t, secondFallbackURL, modelTarget.Target)
	require.Equal(t, secondFallbackURL, modelTarget.Model.Endpoint)
}

func TestRetryChatWithFallback_UsesFallbackModelName(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil

	var fallbackModelName string
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		payload := map[string]any{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		if model, ok := payload["model"].(string); ok {
			fallbackModelName = model
		}
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`ok`))
		require.NoError(t, err)
	}))
	defer fallback.Close()

	fallbackURL := fallback.URL + "/v1/chat/completions"
	modelTarget := &resolvedModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "logical-model"},
			Endpoint:  "https://primary.example.com/v1/chat/completions",
			Upstreams: []commontypes.UpstreamConfig{
				{
					URL:       fallbackURL,
					Enabled:   true,
					ModelName: "provider-fallback-model",
				},
			},
		},
		ModelName: "logical-model",
		Target:    "https://primary.example.com/v1/chat/completions",
		AttemptTargets: []chatAttemptTarget{
			{Upstream: commontypes.UpstreamConfig{URL: "https://primary.example.com/v1/chat/completions", Enabled: true}},
			{Upstream: commontypes.UpstreamConfig{URL: fallbackURL, Enabled: true, ModelName: "provider-fallback-model"}},
		},
	}
	tester.mocks.openAIComp.EXPECT().
		CheckUsageLimit(mock.Anything, "user-1", modelTarget.Model, fallbackURL).
		Return(nil).
		Once()

	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{}`)))
	err := tester.handler.retryChatWithFallback(
		c,
		newTestCommonResponseWriter(),
		modelTarget,
		"user-1",
		&ChatCompletionRequest{Model: "logical-model"},
		nil,
		nil,
	)

	require.NoError(t, err)
	require.Equal(t, "provider-fallback-model", fallbackModelName)
	require.Equal(t, "provider-fallback-model", modelTarget.ModelName)

	tokenCounter := mocktoken.NewMockCounter(t)
	var wg sync.WaitGroup
	wg.Add(2)
	tester.mocks.openAIComp.EXPECT().
		CommitUsageLimit(mock.Anything, "user-1", modelTarget.Model, tokenCounter).
		RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, counter token.Counter) error {
			wg.Done()
			return nil
		}).
		Once()
	tester.mocks.openAIComp.EXPECT().
		RecordUsage(mock.Anything, "user-1", modelTarget.Model, "provider-fallback-model", tokenCounter, "api-key").
		RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, targetModelName string, counter token.Counter, apikey string) error {
			wg.Done()
			return nil
		}).
		Once()

	tester.handler.runChatPostProcessAsync(c.Request.Context(), chatPostProcessInput{
		NSUUID:          "user-1",
		ApiKey:          "api-key",
		Model:           modelTarget.Model,
		TargetModelName: modelTarget.ModelName,
		TokenCounter:    tokenCounter,
	})
	wg.Wait()
}

func TestRetryChatWithFallback_ReportsFallbackAttemptFailure(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil
	reporter := newTestChatAttemptFailureReporter()
	tester.handler.SetChatAttemptFailureReporter(reporter)

	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, err := w.Write([]byte(`fallback unavailable`))
		require.NoError(t, err)
	}))
	defer fallback.Close()
	fallbackURL := fallback.URL + "/v1/chat/completions"
	modelTarget := &resolvedModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "logical-model"},
			Endpoint:  "https://primary.example.com/v1/chat/completions",
		},
		ModelName: "logical-model",
		Target:    "https://primary.example.com/v1/chat/completions",
		AttemptTargets: []chatAttemptTarget{
			{Upstream: commontypes.UpstreamConfig{URL: "https://primary.example.com/v1/chat/completions", Enabled: true}},
			{Upstream: commontypes.UpstreamConfig{URL: fallbackURL, Enabled: true}},
		},
	}
	tester.mocks.openAIComp.EXPECT().
		CheckUsageLimit(mock.Anything, "user-1", modelTarget.Model, fallbackURL).
		Return(nil).
		Once()

	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{}`)))
	err := tester.handler.retryChatWithFallback(
		c,
		newTestCommonResponseWriter(),
		modelTarget,
		"user-1",
		&ChatCompletionRequest{Model: "logical-model"},
		nil,
		nil,
	)

	require.NoError(t, err)
	reporter.Wait()
	events := reporter.Events()
	require.Len(t, events, 1)
	require.Equal(t, chatAttemptPhaseFallback, events[0].Phase)
	require.Equal(t, 1, events[0].FallbackAttempt)
	require.Equal(t, fallbackURL, events[0].Target)
	require.Equal(t, http.StatusServiceUnavailable, events[0].StatusCode)
	require.True(t, events[0].Retryable)
}

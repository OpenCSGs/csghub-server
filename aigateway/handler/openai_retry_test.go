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
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mocktoken "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/token"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
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

type testGenerationRecorderWithMutex struct {
	mu         sync.Mutex
	doneCh     chan struct{}
	usage      *types.TokenUsage
	response   *types.GenerationResponse
	firstChunk *types.GenerationFirstChunk
	errorCode  string
	ended      bool
	events     []string
}

type testLLMTracerWithMutex struct {
	mu                sync.Mutex
	recorder          *testGenerationRecorderWithMutex
	embeddingRecorder *testEmbeddingRecorderWithMutex
	starts            []types.GenerationStart
	embeddingStarts   []types.EmbeddingStart
	streams           []bool
}

type testEmbeddingRecorderWithMutex struct {
	mu        sync.Mutex
	result    *types.EmbeddingResult
	errorCode string
	ended     bool
	events    []string
}

func (t *testLLMTracerWithMutex) StartGeneration(ctx context.Context, input types.GenerationStart) (context.Context, llmtrace.GenerationRecorder) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.starts = append(t.starts, input)
	t.streams = append(t.streams, false)
	return ctx, t.recorder
}

func (t *testLLMTracerWithMutex) StartStreamingGeneration(ctx context.Context, input types.GenerationStart) (context.Context, llmtrace.GenerationRecorder) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.starts = append(t.starts, input)
	t.streams = append(t.streams, true)
	return ctx, t.recorder
}

func (t *testLLMTracerWithMutex) StartEmbedding(ctx context.Context, input types.EmbeddingStart) (context.Context, llmtrace.EmbeddingRecorder) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.embeddingStarts = append(t.embeddingStarts, input)
	return ctx, t.embeddingRecorder
}

func (t *testLLMTracerWithMutex) Shutdown(context.Context) error {
	return nil
}

func (t *testLLMTracerWithMutex) Starts() []types.GenerationStart {
	t.mu.Lock()
	defer t.mu.Unlock()
	starts := make([]types.GenerationStart, len(t.starts))
	copy(starts, t.starts)
	return starts
}

func (t *testLLMTracerWithMutex) EmbeddingStarts() []types.EmbeddingStart {
	t.mu.Lock()
	defer t.mu.Unlock()
	starts := make([]types.EmbeddingStart, len(t.embeddingStarts))
	copy(starts, t.embeddingStarts)
	return starts
}

func (t *testLLMTracerWithMutex) Streams() []bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	streams := make([]bool, len(t.streams))
	copy(streams, t.streams)
	return streams
}

func (r *testEmbeddingRecorderWithMutex) SetResult(result types.EmbeddingResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.result = &result
	r.events = append(r.events, "result")
}

func (r *testEmbeddingRecorderWithMutex) SetError(_ error, code string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errorCode = code
	r.events = append(r.events, "error")
}

func (r *testEmbeddingRecorderWithMutex) End() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ended = true
	r.events = append(r.events, "end")
}

func (r *testEmbeddingRecorderWithMutex) snapshot() (*types.EmbeddingResult, string, bool, []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	events := append([]string(nil), r.events...)
	return r.result, r.errorCode, r.ended, events
}

func (r *testGenerationRecorderWithMutex) SetUsage(usage types.TokenUsage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.usage = &usage
	r.events = append(r.events, "usage")
}

func (r *testGenerationRecorderWithMutex) SetResponse(response types.GenerationResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.response = &response
	r.events = append(r.events, "response")
}

func (r *testGenerationRecorderWithMutex) SetFirstChunk(firstChunk types.GenerationFirstChunk) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.firstChunk = &firstChunk
	r.events = append(r.events, "first_chunk")
}

func (r *testGenerationRecorderWithMutex) SetError(_ error, code string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errorCode = code
	r.events = append(r.events, "error")
}

func (r *testGenerationRecorderWithMutex) End() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ended = true
	r.events = append(r.events, "end")
	if r.doneCh != nil {
		close(r.doneCh)
		r.doneCh = nil
	}
}

func (r *testGenerationRecorderWithMutex) snapshot() (*types.TokenUsage, bool, []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	events := append([]string(nil), r.events...)
	return r.usage, r.ended, events
}

func (r *testGenerationRecorderWithMutex) traceSnapshot() (*types.GenerationResponse, *types.GenerationFirstChunk, string, bool, []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	events := append([]string(nil), r.events...)
	return r.response, r.firstChunk, r.errorCode, r.ended, events
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

	applyChatFallbackTarget(context.Background(), headers, modelTarget,
		commontypes.UpstreamConfig{
			URL:        "https://fallback.example.com/v1/chat/completions",
			Enabled:    true,
			Provider:   "fallback-provider",
			AuthHeader: `{"Authorization":"Bearer fallback-token"}`,
			ModelName:  "fallback-model",
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

func TestExecuteChatProxyAttempt_RewritesResponsesURLForChatRequest(t *testing.T) {
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

	responsesURL := upstream.URL + "/v1/responses"
	chatURL := upstream.URL + "/v1/chat/completions"
	modelTarget := &resolvedModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
			Endpoint:  responsesURL,
		},
		Upstream:  commontypes.UpstreamConfig{ID: 1, URL: responsesURL, Enabled: true},
		ModelName: "provider-model",
		Target:    responsesURL,
	}
	applyChatCompletionsEndpointCompatibility(c.Request.Context(), modelTarget)

	tester.mocks.openAIComp.EXPECT().
		CheckUsageLimit(mock.Anything, "user-1", modelTarget.Model, chatURL).
		Return(nil).
		Once()

	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{"message":"hello"}`)))
	writer := newTestCommonResponseWriter()
	chatReq := &ChatCompletionRequest{
		Model: "test-model",
	}

	retryWriter, err := tester.handler.executeChatProxyAttempt(c, writer, modelTarget, "user-1", chatReq)

	require.NoError(t, err)
	require.NotNil(t, retryWriter)
	require.Equal(t, chatURL, modelTarget.Target)
	require.Equal(t, chatURL, modelTarget.Model.Endpoint)
	require.Equal(t, chatURL, modelTarget.Upstream.URL)
	require.Equal(t, "/v1/chat/completions", receivedPath)
	var forwardedBody map[string]any
	require.NoError(t, json.Unmarshal([]byte(receivedBody), &forwardedBody))
	require.Equal(t, "provider-model", forwardedBody["model"])
	require.Equal(t, http.StatusOK, retryWriter.StatusCode())
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
		AttemptTargets: nil,
	}

	_, err := tester.handler.retryChatWithFallback(c, newTestCommonResponseWriter(), modelTarget, "user-1", &ChatCompletionRequest{Model: "test-model"}, nil, nil)

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
		AttemptTargets: []commontypes.UpstreamConfig{
			{URL: lastFallbackURL, Enabled: true},
		},
	}
	tester.mocks.openAIComp.EXPECT().
		CheckUsageLimit(mock.Anything, "user-1", modelTarget.Model, lastFallbackURL).
		Return(nil).
		Once()

	body := []byte(`{"message":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	writer := newTestCommonResponseWriter()

	_, err := tester.handler.retryChatWithFallback(c, writer, modelTarget, "user-1", &ChatCompletionRequest{Model: "test-model"}, nil, nil)

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
		AttemptTargets: []commontypes.UpstreamConfig{
			{URL: firstFallbackURL, Enabled: true},
			{URL: secondFallbackURL, Enabled: true},
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

	_, err := tester.handler.retryChatWithFallback(c, writer, modelTarget, "user-1", &ChatCompletionRequest{Model: "test-model"}, nil, nil)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, writer.statusCode)
	require.Equal(t, `ok from second fallback`, writer.body.String())
	require.Equal(t, secondFallbackURL, modelTarget.Target)
	require.Equal(t, secondFallbackURL, modelTarget.Model.Endpoint)
}

func TestRetryChatWithFallback_RewritesResponsesFallbackURLForChatRequest(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil

	var receivedPath string
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`ok from fallback`))
		require.NoError(t, err)
	}))
	defer fallback.Close()

	responsesURL := fallback.URL + "/v1/responses"
	chatURL := fallback.URL + "/v1/chat/completions"
	modelTarget := &resolvedModelTarget{
		Model: &types.Model{
			BaseModel: types.BaseModel{ID: "test-model"},
			Endpoint:  "https://primary.example.com/v1/chat/completions",
			Upstreams: []commontypes.UpstreamConfig{
				{ID: 2, URL: responsesURL, Enabled: true},
			},
		},
		ModelName: "test-model",
		Target:    "https://primary.example.com/v1/chat/completions",
		AttemptTargets: []commontypes.UpstreamConfig{
			{ID: 2, URL: responsesURL, Enabled: true, ModelName: "fallback-model"},
		},
	}
	tester.mocks.openAIComp.EXPECT().
		CheckUsageLimit(mock.Anything, "user-1", modelTarget.Model, chatURL).
		Return(nil).
		Once()

	body := []byte(`{"message":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	writer := newTestCommonResponseWriter()

	_, err := tester.handler.retryChatWithFallback(c, writer, modelTarget, "user-1", &ChatCompletionRequest{Model: "test-model"}, nil, nil)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, writer.statusCode)
	require.Equal(t, `ok from fallback`, writer.body.String())
	require.Equal(t, "/v1/chat/completions", receivedPath)
	require.Equal(t, chatURL, modelTarget.Target)
	require.Equal(t, chatURL, modelTarget.Model.Endpoint)
	require.Equal(t, chatURL, modelTarget.Upstream.URL)
	require.Equal(t, "fallback-model", modelTarget.ModelName)
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
		AttemptTargets: []commontypes.UpstreamConfig{
			{URL: fallbackURL, Enabled: true, ModelName: "provider-fallback-model"},
		},
	}
	tester.mocks.openAIComp.EXPECT().
		CheckUsageLimit(mock.Anything, "user-1", modelTarget.Model, fallbackURL).
		Return(nil).
		Once()

	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{}`)))
	_, err := tester.handler.retryChatWithFallback(
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
	tokenCounter.EXPECT().
		Usage(mock.Anything).
		Return(&token.Usage{}, nil).
		Once()
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
		RecordUsageFromTokenUsage(mock.Anything, "user-1", modelTarget.Model, "provider-fallback-model", mock.Anything, "api-key").
		RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
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

func TestRunChatPostProcessAsync_RecordsTraceUsageBeforeAccounting(t *testing.T) {
	tester, c, _ := setupTest(t)
	tester.mocks.openAIComp.ExpectedCalls = nil

	model := &types.Model{
		BaseModel: types.BaseModel{
			ID: "model-1",
		},
		ExternalModelInfo: types.ExternalModelInfo{
			Provider: "deepseek",
		},
	}
	tokenCounter := mocktoken.NewMockCounter(t)
	tokenCounter.EXPECT().
		Usage(mock.Anything).
		Return(&token.Usage{
			PromptTokens:     11,
			CompletionTokens: 7,
			TotalTokens:      18,
			ReasoningTokens:  3,
		}, nil).
		Once()

	recorder := &testGenerationRecorderWithMutex{}
	var wg sync.WaitGroup
	wg.Add(2)
	tester.mocks.openAIComp.EXPECT().
		CommitUsageLimit(mock.Anything, "user-1", model, tokenCounter).
		RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, counter token.Counter) error {
			usage, ended, events := recorder.snapshot()
			require.True(t, ended)
			require.Equal(t, []string{"usage", "end"}, events)
			require.NotNil(t, usage)
			require.Equal(t, int64(11), usage.InputTokens)
			require.Equal(t, int64(7), usage.OutputTokens)
			require.Equal(t, int64(18), usage.TotalTokens)
			require.Equal(t, int64(3), usage.ReasoningTokens)
			wg.Done()
			return nil
		}).
		Once()
	tester.mocks.openAIComp.EXPECT().
		RecordUsageFromTokenUsage(mock.Anything, "user-1", model, "target-model", mock.Anything, "api-key").
		RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
			wg.Done()
			return nil
		}).
		Once()

	tester.handler.runChatPostProcessAsync(c.Request.Context(), chatPostProcessInput{
		NSUUID:          "user-1",
		ApiKey:          "api-key",
		Model:           model,
		TargetModelName: "target-model",
		TokenCounter:    tokenCounter,
		Trace: chatTracePostProcessInput{
			Recorder: recorder,
		},
	})
	wg.Wait()
}

func TestRunChatPostProcessAsync_RecordsTraceCompletionBeforeUsage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tester, c, _ := setupTest(t)
		tester.mocks.openAIComp.ExpectedCalls = nil

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "model-1",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				Provider: "deepseek",
			},
		}
		tokenCounter := mocktoken.NewMockCounter(t)
		tokenCounter.EXPECT().
			Usage(mock.Anything).
			Return(&token.Usage{
				PromptTokens:     5,
				CompletionTokens: 8,
				TotalTokens:      13,
			}, nil).
			Once()

		recorder := &testGenerationRecorderWithMutex{}
		firstWriteAt := time.Now()
		tester.mocks.openAIComp.EXPECT().
			CommitUsageLimit(mock.Anything, "user-1", model, tokenCounter).
			Return(nil).
			Once()
		tester.mocks.openAIComp.EXPECT().
			RecordUsageFromTokenUsage(mock.Anything, "user-1", model, "target-model", mock.Anything, "api-key").
			RunAndReturn(func(ctx context.Context, userUUID string, model *types.Model, targetModelName string, usage *token.Usage, apikey string) error {
				_, _, _, ended, events := recorder.traceSnapshot()
				require.True(t, ended)
				require.Equal(t, []string{"first_chunk", "response", "error", "usage", "end"}, events)
				return nil
			}).
			Once()

		tester.handler.runChatPostProcessAsync(c.Request.Context(), chatPostProcessInput{
			NSUUID:          "user-1",
			ApiKey:          "api-key",
			Model:           model,
			TargetModelName: "target-model",
			TokenCounter:    tokenCounter,
			Trace: chatTracePostProcessInput{
				Recorder:     recorder,
				Completion:   true,
				Stream:       true,
				FirstWriteAt: firstWriteAt,
				StatusCode:   http.StatusBadGateway,
			},
		})
		synctest.Wait()
		response, firstChunk, errorCode, ended, events := recorder.traceSnapshot()
		require.True(t, ended)
		require.Equal(t, []string{"first_chunk", "response", "error", "usage", "end"}, events)
		require.NotNil(t, response)
		require.Equal(t, "deepseek", response.Provider)
		require.Equal(t, "target-model", response.Model)
		require.Equal(t, "target-model", response.ResponseModel)
		require.NotNil(t, firstChunk)
		require.Equal(t, firstWriteAt, firstChunk.At)
		require.Equal(t, types.TraceErrUpstreamError, errorCode)
	})
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
		AttemptTargets: []commontypes.UpstreamConfig{
			{URL: fallbackURL, Enabled: true},
		},
	}
	tester.mocks.openAIComp.EXPECT().
		CheckUsageLimit(mock.Anything, "user-1", modelTarget.Model, fallbackURL).
		Return(nil).
		Once()

	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{}`)))
	_, err := tester.handler.retryChatWithFallback(
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

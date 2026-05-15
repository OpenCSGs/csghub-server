package handler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"opencsg.com/csghub-server/aigateway/types"
)

type asyncTestChatAttemptFailureReporter struct {
	mu     sync.Mutex
	doneCh chan struct{}
	events []ChatAttemptFailureEvent
}

func newAsyncTestChatAttemptFailureReporter() *asyncTestChatAttemptFailureReporter {
	return &asyncTestChatAttemptFailureReporter{
		doneCh: make(chan struct{}, 10),
	}
}

func (r *asyncTestChatAttemptFailureReporter) ReportChatAttemptFailure(_ context.Context, event ChatAttemptFailureEvent) error {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
	r.doneCh <- struct{}{}
	return nil
}

func (r *asyncTestChatAttemptFailureReporter) wait(t *testing.T) {
	t.Helper()
	select {
	case <-r.doneCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for async report")
	}
}

func (r *asyncTestChatAttemptFailureReporter) eventsSnapshot() []ChatAttemptFailureEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]ChatAttemptFailureEvent, len(r.events))
	copy(cp, r.events)
	return cp
}

func (r *asyncTestChatAttemptFailureReporter) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

func TestReportChatAttemptResult_Failure(t *testing.T) {
	reporter := newAsyncTestChatAttemptFailureReporter()
	h := &OpenAIHandlerImpl{
		chatAttemptFailureReporter: reporter,
	}

	h.reportChatAttemptResult(context.Background(), chatAttemptReportParams{
		UpstreamID:      42,
		Phase:           chatAttemptPhasePrimary,
		RequestModelID:  "gpt-4(openai)",
		ModelName:       "gpt-4",
		Provider:        "openai",
		Endpoint:        "https://api.openai.com/v1/chat/completions",
		Target:          "https://api.openai.com/v1/chat/completions",
		SessionKeyHash:  "abc123",
		StatusCode:      503,
		Retryable:       true,
		FallbackAttempt: 0,
		Model:           &types.Model{BaseModel: types.BaseModel{ID: "gpt-4"}},
	})

	reporter.wait(t)
	events := reporter.eventsSnapshot()
	require.Len(t, events, 1)
	evt := events[0]
	require.Equal(t, int64(42), evt.UpstreamID)
	require.Equal(t, chatAttemptPhasePrimary, evt.Phase)
	require.Equal(t, "gpt-4(openai)", evt.ModelID)
	require.Equal(t, "gpt-4", evt.ModelName)
	require.Equal(t, "openai", evt.Provider)
	require.Equal(t, "https://api.openai.com/v1/chat/completions", evt.Endpoint)
	require.Equal(t, "https://api.openai.com/v1/chat/completions", evt.Target)
	require.Equal(t, "abc123", evt.SessionKeyHash)
	require.Equal(t, 503, evt.StatusCode)
	require.True(t, evt.Retryable)
	require.Equal(t, 0, evt.FallbackAttempt)
}

func TestReportChatAttemptResult_FailureFallback(t *testing.T) {
	reporter := newAsyncTestChatAttemptFailureReporter()
	h := &OpenAIHandlerImpl{
		chatAttemptFailureReporter: reporter,
	}

	h.reportChatAttemptResult(context.Background(), chatAttemptReportParams{
		UpstreamID:      99,
		Phase:           chatAttemptPhaseFallback,
		ModelName:       "fallback-model",
		Provider:        "fallback-provider",
		Endpoint:        "https://fallback.example.com/v1/chat/completions",
		Target:          "https://fallback.example.com/v1/chat/completions",
		SessionKeyHash:  "hash456",
		StatusCode:      500,
		Retryable:       false,
		FallbackAttempt: 2,
		Model:           &types.Model{BaseModel: types.BaseModel{ID: "fallback-model"}, ExternalModelInfo: types.ExternalModelInfo{FormatModelID: "fallback-model(fallback-provider)"}},
	})

	reporter.wait(t)
	events := reporter.eventsSnapshot()
	require.Len(t, events, 1)
	evt := events[0]
	require.Equal(t, chatAttemptPhaseFallback, evt.Phase)
	require.Equal(t, "fallback-model(fallback-provider)", evt.ModelID)
	require.Equal(t, 500, evt.StatusCode)
	require.False(t, evt.Retryable)
	require.Equal(t, 2, evt.FallbackAttempt)
}

func TestReportChatAttemptResult_Success(t *testing.T) {
	reporter := newAsyncTestChatAttemptFailureReporter()
	h := &OpenAIHandlerImpl{
		chatAttemptFailureReporter: reporter,
		availabilityManager:        &stubAvailabilityManager{},
	}

	h.reportChatAttemptResult(context.Background(), chatAttemptReportParams{
		UpstreamID:     10,
		Phase:          chatAttemptPhasePrimary,
		RequestModelID: "claude-3(anthropic)",
		ModelName:      "claude-3",
		Provider:       "anthropic",
		Endpoint:       "https://api.anthropic.com/v1/chat/completions",
		Target:         "https://api.anthropic.com/v1/chat/completions",
		SessionKeyHash: "def789",
		StatusCode:     200,
		Retryable:      false,
		Model:          &types.Model{BaseModel: types.BaseModel{ID: "claude-3"}},
	})

	time.Sleep(50 * time.Millisecond)
	require.Equal(t, 0, reporter.len(), "success should not report failure events")
}

func TestReportChatAttemptResult_NilHandler(t *testing.T) {
	var h *OpenAIHandlerImpl
	require.NotPanics(t, func() {
		h.reportChatAttemptResult(context.Background(), chatAttemptReportParams{
			StatusCode: 503,
		})
	})
}

func TestReportChatAttemptResult_NilModel(t *testing.T) {
	reporter := newAsyncTestChatAttemptFailureReporter()
	h := &OpenAIHandlerImpl{
		chatAttemptFailureReporter: reporter,
	}

	h.reportChatAttemptResult(context.Background(), chatAttemptReportParams{
		UpstreamID:     1,
		Phase:          chatAttemptPhasePrimary,
		StatusCode:     429,
		Retryable:      true,
		RequestModelID: "",
		Model:          nil,
	})

	reporter.wait(t)
	events := reporter.eventsSnapshot()
	require.Len(t, events, 1)
	require.Equal(t, "", events[0].ModelID)
}

func TestReportChatAttemptResult_EmptyRequestModelID_FallsBackToFormatModelID(t *testing.T) {
	reporter := newAsyncTestChatAttemptFailureReporter()
	h := &OpenAIHandlerImpl{
		chatAttemptFailureReporter: reporter,
	}

	h.reportChatAttemptResult(context.Background(), chatAttemptReportParams{
		UpstreamID:     1,
		Phase:          chatAttemptPhasePrimary,
		StatusCode:     500,
		RequestModelID: "",
		Model:          &types.Model{BaseModel: types.BaseModel{ID: "base-id"}, ExternalModelInfo: types.ExternalModelInfo{FormatModelID: "formatted-id"}},
	})

	reporter.wait(t)
	events := reporter.eventsSnapshot()
	require.Len(t, events, 1)
	require.Equal(t, "formatted-id", events[0].ModelID)
}

func TestReportChatAttemptResult_EmptyRequestModelID_FallsBackToModelID(t *testing.T) {
	reporter := newAsyncTestChatAttemptFailureReporter()
	h := &OpenAIHandlerImpl{
		chatAttemptFailureReporter: reporter,
	}

	h.reportChatAttemptResult(context.Background(), chatAttemptReportParams{
		UpstreamID:     1,
		Phase:          chatAttemptPhasePrimary,
		StatusCode:     500,
		RequestModelID: "",
		Model:          &types.Model{BaseModel: types.BaseModel{ID: "base-id"}},
	})

	reporter.wait(t)
	events := reporter.eventsSnapshot()
	require.Len(t, events, 1)
	require.Equal(t, "base-id", events[0].ModelID)
}

func TestReportChatAttemptResult_StatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantFail   bool
	}{
		{name: "200 OK", statusCode: 200, wantFail: false},
		{name: "201 Created", statusCode: 201, wantFail: false},
		{name: "301 Moved", statusCode: 301, wantFail: false},
		{name: "399 boundary below 400", statusCode: 399, wantFail: false},
		{name: "400 Bad Request", statusCode: 400, wantFail: true},
		{name: "401 Unauthorized", statusCode: 401, wantFail: true},
		{name: "429 Too Many Requests", statusCode: 429, wantFail: true},
		{name: "500 Internal Server Error", statusCode: 500, wantFail: true},
		{name: "502 Bad Gateway", statusCode: 502, wantFail: true},
		{name: "503 Service Unavailable", statusCode: 503, wantFail: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := newAsyncTestChatAttemptFailureReporter()
			h := &OpenAIHandlerImpl{
				chatAttemptFailureReporter: reporter,
				availabilityManager:        &stubAvailabilityManager{},
			}

			h.reportChatAttemptResult(context.Background(), chatAttemptReportParams{
				StatusCode: tt.statusCode,
			})

			if tt.wantFail {
				reporter.wait(t)
				require.Equal(t, 1, reporter.len(), "expected failure event for status %d", tt.statusCode)
			} else {
				time.Sleep(50 * time.Millisecond)
				require.Equal(t, 0, reporter.len(), "expected no failure event for status %d", tt.statusCode)
			}
		})
	}
}

func TestReportChatAttemptResult_NonBlocking(t *testing.T) {
	reporter := newAsyncTestChatAttemptFailureReporter()
	h := &OpenAIHandlerImpl{
		chatAttemptFailureReporter: reporter,
	}

	done := make(chan struct{})
	go func() {
		h.reportChatAttemptResult(context.Background(), chatAttemptReportParams{
			UpstreamID:     7,
			Phase:          chatAttemptPhasePrimary,
			StatusCode:     502,
			Retryable:      true,
			RequestModelID: "test-model(openai)",
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reportChatAttemptResult should return immediately (non-blocking)")
	}

	reporter.wait(t)
	events := reporter.eventsSnapshot()
	require.Len(t, events, 1)
	require.Equal(t, int64(7), events[0].UpstreamID)
}

type stubAvailabilityManager struct{}

func (s *stubAvailabilityManager) Start(_ context.Context) error { return nil }
func (s *stubAvailabilityManager) Stop() error                   { return nil }
func (s *stubAvailabilityManager) RecordRequestResult(_ context.Context, _ int64, _ string, _ bool, _ error) error {
	return nil
}
func (s *stubAvailabilityManager) IsAvailable(_ context.Context, _ int64) (bool, error) {
	return true, nil
}
func (s *stubAvailabilityManager) GetCircuitState(_ context.Context, _ int64) (*types.ProviderCircuitStatus, error) {
	return &types.ProviderCircuitStatus{CircuitState: types.CircuitStateClosed}, nil
}

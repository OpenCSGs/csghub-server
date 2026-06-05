package handler

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	commontypes "opencsg.com/csghub-server/common/types"
)

type testRetryResponseWriter struct {
	headers    http.Header
	body       bytes.Buffer
	statusCode int
}

func newTestRetryResponseWriter() *testRetryResponseWriter {
	return &testRetryResponseWriter{
		headers: make(http.Header),
	}
}

func (w *testRetryResponseWriter) Header() http.Header {
	return w.headers
}

func (w *testRetryResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *testRetryResponseWriter) Write(data []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return w.body.Write(data)
}

func (w *testRetryResponseWriter) Flush() {}

func (w *testRetryResponseWriter) ClearBuffer() {}

func TestShouldRetryChatAttempt(t *testing.T) {
	require.False(t, shouldRetryChatAttempt(http.StatusOK, false))
	require.False(t, shouldRetryChatAttempt(http.StatusBadRequest, false))
	require.True(t, shouldRetryChatAttempt(http.StatusUnauthorized, false))
	require.True(t, shouldRetryChatAttempt(http.StatusNotFound, false))
	require.True(t, shouldRetryChatAttempt(http.StatusTooManyRequests, false))
	require.True(t, shouldRetryChatAttempt(http.StatusBadGateway, false))
	require.True(t, shouldRetryChatAttempt(http.StatusServiceUnavailable, false))
	require.True(t, shouldRetryChatAttempt(http.StatusGatewayTimeout, false))
	require.False(t, shouldRetryChatAttempt(http.StatusBadGateway, true))
}

func TestChatRetryResponseWriter_BuffersFailedAttemptUntilReplay(t *testing.T) {
	downstream := newTestRetryResponseWriter()
	writer := newChatRetryResponseWriter(downstream)

	writer.WriteHeader(http.StatusBadGateway)
	_, err := writer.Write([]byte(`gateway error body`))

	require.NoError(t, err)
	require.Equal(t, 0, downstream.statusCode)
	require.Empty(t, downstream.body.String())
	require.False(t, writer.StreamStarted())
	require.NoError(t, writer.ReplayBufferedResponse())
	require.Equal(t, http.StatusBadGateway, downstream.statusCode)
	require.Equal(t, `gateway error body`, downstream.body.String())
}

func TestBuildChatAttemptTargets(t *testing.T) {
	targets := buildChatAttemptTargets(
		commontypes.UpstreamConfig{ID: 1, URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true, ModelName: "provider-model-b"},
		[]commontypes.UpstreamConfig{
			{ID: 1, URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true, ModelName: "provider-model-b"},
			{ID: 2, URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true, ModelName: "provider-model-a"},
		},
		2,
	)
	require.Equal(t, []commontypes.UpstreamConfig{
		{ID: 2, URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true, ModelName: "provider-model-a"},
	}, targets)
}

func TestBuildChatAttemptTargets_RespectMaxFallbackAttempts(t *testing.T) {
	targets := buildChatAttemptTargets(
		commontypes.UpstreamConfig{ID: 1, URL: "https://api.example.com/node-d/v1/chat/completions", Enabled: true},
		[]commontypes.UpstreamConfig{
			{ID: 1, URL: "https://api.example.com/node-d/v1/chat/completions", Enabled: true},
			{ID: 2, URL: "https://api.example.com/node-c/v1/chat/completions", Enabled: true, ModelName: "provider-model-c"},
			{ID: 3, URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true},
			{ID: 4, URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true},
			{ID: 5, URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true},
		},
		2,
	)
	require.Equal(t, []commontypes.UpstreamConfig{
		{ID: 2, URL: "https://api.example.com/node-c/v1/chat/completions", Enabled: true, ModelName: "provider-model-c"},
		{ID: 3, URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true},
	}, targets)
}

func TestBuildChatAttemptTargets_DisableFallbacks(t *testing.T) {
	targets := buildChatAttemptTargets(
		commontypes.UpstreamConfig{ID: 1, URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true},
		[]commontypes.UpstreamConfig{
			{ID: 1, URL: "https://api.example.com/node-b/v1/chat/completions", Enabled: true},
			{ID: 2, URL: "https://api.example.com/node-a/v1/chat/completions", Enabled: true},
		},
		0,
	)
	require.Empty(t, targets)
}

func TestSessionKeyDigest(t *testing.T) {
	digest1 := sessionKeyDigest("session-1")
	digest2 := sessionKeyDigest("session-1")
	digest3 := sessionKeyDigest("session-2")

	require.NotEmpty(t, digest1)
	require.Equal(t, digest1, digest2)
	require.NotEqual(t, digest1, digest3)
}

func TestRetryWriterTTFTMs(t *testing.T) {
	startTime := time.Now()

	// Test nil writer
	require.Equal(t, int64(0), retryWriterTTFTMs(nil, startTime))

	// Test writer with zero firstWriteAt
	downstream := newTestRetryResponseWriter()
	writer := newChatRetryResponseWriter(downstream)
	require.Equal(t, int64(0), retryWriterTTFTMs(writer, startTime))

	// Test writer with firstWriteAt
	time.Sleep(10 * time.Millisecond) // Add a small delay to ensure TTFT > 0
	_, err := writer.Write([]byte("first token"))
	require.NoError(t, err)
	ttftMs := retryWriterTTFTMs(writer, startTime)
	require.Greater(t, ttftMs, int64(0))
	require.Less(t, ttftMs, int64(1000)) // Should be less than 1 second
}

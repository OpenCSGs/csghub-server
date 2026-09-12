package handler

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
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

func TestRewriteResponsesURLToChatCompletions(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		rewritten bool
	}{
		{
			name:      "openai responses",
			input:     "https://api.openai.com/v1/responses",
			expected:  "https://api.openai.com/v1/chat/completions",
			rewritten: true,
		},
		{
			name:      "azure responses with query",
			input:     "https://example.openai.azure.com/openai/deployments/gpt/responses?api-version=2026-01-01-preview",
			expected:  "https://example.openai.azure.com/openai/deployments/gpt/chat/completions?api-version=2026-01-01-preview",
			rewritten: true,
		},
		{
			name:      "trailing slash",
			input:     "https://api.openai.com/v1/responses/",
			expected:  "https://api.openai.com/v1/chat/completions",
			rewritten: true,
		},
		{
			name:      "chat completions unchanged",
			input:     "https://api.openai.com/v1/chat/completions",
			expected:  "https://api.openai.com/v1/chat/completions",
			rewritten: false,
		},
		{
			name:      "responses extra unchanged",
			input:     "https://api.openai.com/v1/responses-extra",
			expected:  "https://api.openai.com/v1/responses-extra",
			rewritten: false,
		},
		{
			name:      "invalid unchanged",
			input:     "://bad-url",
			expected:  "://bad-url",
			rewritten: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, rewritten := rewriteResponsesURLToChatCompletions(tt.input)

			require.Equal(t, tt.rewritten, rewritten)
			require.Equal(t, tt.expected, actual)
		})
	}
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

// TestResolveProxyPathFromModelEndpoint pins the proxy-path contract of the
// locally modified resolveProxyPathFromModelEndpoint:
//
//   - The extracted endpoint path replaces the client request path in the
//     reverse-proxy Director (builder/proxy/reverse_proxy.go only rewrites
//     req.URL.Path when the returned api path is non-empty).
//   - Storage contract: external upstream endpoints must be stored either as
//     the full endpoint URL (path includes the terminal segment such as
//     /v1/chat/completions, e.g.
//     https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions) or
//     as a bare host. A base URL with a path prefix but no terminal segment
//     (e.g. https://host/v1) has its path replace the client path as-is,
//     which drops /v1/chat/completions upstream — a documented limitation.
//   - An endpoint that is non-empty but has no usable path logs the
//     "endpoint has wrong struct" warning; empty/whitespace endpoints are
//     silently ignored.
func TestResolveProxyPathFromModelEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantPath string
		wantWarn bool
	}{
		{
			name:     "empty endpoint keeps client path",
			endpoint: "",
			wantPath: "",
			wantWarn: false,
		},
		{
			name:     "whitespace endpoint keeps client path without warning",
			endpoint: "   ",
			wantPath: "",
			wantWarn: false,
		},
		{
			name:     "bare host keeps client path and warns",
			endpoint: "https://api.openai.com",
			wantPath: "",
			wantWarn: true,
		},
		{
			name:     "host with port and no path keeps client path and warns",
			endpoint: "http://127.0.0.1:8080",
			wantPath: "",
			wantWarn: true,
		},
		{
			name:     "openai style base url extracts /v1",
			endpoint: "https://api.openai.com/v1",
			wantPath: "/v1",
			wantWarn: false,
		},
		{
			name:     "internal deploy target base url extracts /v1",
			endpoint: "http://svc-e2e-running.svc.cluster.local:8080/v1",
			wantPath: "/v1",
			wantWarn: false,
		},
		{
			// The fixed scenario: the upstream lives under a non-/v1 path
			// prefix, so the client path /v1/chat/completions must NOT be
			// forwarded as-is.
			name:     "dashscope style full endpoint url extracts full prefix path",
			endpoint: "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
			wantPath: "/compatible-mode/v1/chat/completions",
			wantWarn: false,
		},
		{
			name:     "deep prefix full endpoint url extracts full prefix path",
			endpoint: "https://host.example.com/api/v1/chat/completions",
			wantPath: "/api/v1/chat/completions",
			wantWarn: false,
		},
		{
			// Documented limitation: this base-URL form replaces the client
			// path with /compatible-mode/v1, dropping /chat/completions.
			name:     "base url with non-v1 prefix and no terminal segment extracts prefix path",
			endpoint: "https://dashscope.aliyuncs.com/compatible-mode/v1",
			wantPath: "/compatible-mode/v1",
			wantWarn: false,
		},
		{
			name:     "responses terminated endpoint extracts full path",
			endpoint: "https://host.example.com/api/v1/responses",
			wantPath: "/api/v1/responses",
			wantWarn: false,
		},
		{
			name:     "query string is not part of the path",
			endpoint: "https://host.example.com/api/v1/chat/completions?api-version=2026-01-01",
			wantPath: "/api/v1/chat/completions",
			wantWarn: false,
		},
		{
			name:     "trailing slash is preserved",
			endpoint: "https://host.example.com/v1/chat/completions/",
			wantPath: "/v1/chat/completions/",
			wantWarn: false,
		},
		{
			name:     "root path extracts slash",
			endpoint: "https://host.example.com/",
			wantPath: "/",
			wantWarn: false,
		},
		{
			name:     "invalid url keeps client path and warns",
			endpoint: "://bad-url",
			wantPath: "",
			wantWarn: true,
		},
		{
			name:     "scheme-less host with path is not a parseable endpoint and warns",
			endpoint: "host.example.com/v1",
			wantPath: "",
			wantWarn: true,
		},
		{
			name:     "scheme-less host port path has no url path and warns",
			endpoint: "myhost:8080/v1",
			wantPath: "",
			wantWarn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })

			got := resolveProxyPathFromModelEndpoint(tt.endpoint, "test-model")

			require.Equal(t, tt.wantPath, got)
			require.Equal(t, tt.wantWarn, strings.Contains(buf.String(), "endpoint has wrong struct"))
			if tt.wantWarn {
				require.Contains(t, buf.String(), "model=test-model")
			}
		})
	}
}

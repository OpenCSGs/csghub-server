package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
)

func TestResponsesAdapterNonStreamWriterFinalizeOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	var captured *types.ResponsesResponse
	var mu sync.Mutex
	counter := &captureCounter{
		onResponse: func(r *types.ResponsesResponse) {
			mu.Lock()
			captured = r
			mu.Unlock()
		},
	}

	w := newResponsesAdapterNonStreamWriter(ctx.Writer, "public-model", counter, nil, "")
	w.Header().Set("Content-Encoding", "identity")
	chatBody := []byte(`{
		"id":"chat-1",
		"object":"chat.completion",
		"created":1700000000,
		"model":"upstream-model",
		"choices":[{"index":0,"message":{"role":"assistant","content":"hello world"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":3,"completion_tokens":5,"total_tokens":8}
	}`)
	_, err := w.Write(chatBody)
	require.NoError(t, err)

	err = w.Finalize(http.StatusOK)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, captured)
	require.Equal(t, "hello world", captured.OutputText)
	require.Equal(t, "public-model", captured.Model)
	require.Equal(t, int64(3), captured.Usage.InputTokens)

	body := rec.Body.String()
	require.Contains(t, body, `"output_text":"hello world"`)
	require.Contains(t, body, `"model":"public-model"`)
	require.Contains(t, body, `"object":"response"`)
	require.Contains(t, body, `resp_agw_adapter_`, "id should be wrapped with adapter prefix")
}

func TestResponsesAdapterNonStreamWriterFinalizePassesThroughErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	counter := &captureCounter{}
	w := newResponsesAdapterNonStreamWriter(ctx.Writer, "public-model", counter, nil, "")
	w.Header().Set("X-Trace", "abc")
	_, _ = w.Write([]byte(`{"error":"boom"}`))

	err := w.Finalize(http.StatusBadGateway)
	require.NoError(t, err)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Equal(t, "abc", rec.Header().Get("X-Trace"))
	require.Equal(t, `{"error":"boom"}`, rec.Body.String())

	mu := &counter.mu
	mu.Lock()
	defer mu.Unlock()
	require.Nil(t, counter.lastResp, "Response() should not be invoked on error passthrough")
}

func TestResponsesAdapterNonStreamWriterModeratesRefusalAndToolCalls(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	moderation := mockcomp.NewMockModeration(t)
	moderation.EXPECT().CheckText(
		mock.MatchedBy(func(ctx context.Context) bool {
			deadline, ok := ctx.Deadline()
			return ok && time.Until(deadline) <= responsespkg.ModerationTimeout
		}),
		mock.MatchedBy(func(req types.TextModerationRequest) bool {
			return req.Key == "session-1" &&
				req.Phase == types.TextModerationPhaseResponse &&
				req.Mode == types.TextModerationModeNonStream &&
				strings.Contains(req.Content, "blocked refusal") &&
				strings.Contains(req.Content, "delete_file") &&
				strings.Contains(req.Content, `{"path":"/tmp/a"}`)
		}),
	).Return(&rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil).Once()

	w := newResponsesAdapterNonStreamWriter(ctx.Writer, "public-model", nil, moderation, "session-1")
	chatBody := []byte(`{
		"id":"chat-1",
		"object":"chat.completion",
		"created":1700000000,
		"model":"upstream-model",
		"choices":[{
			"index":0,
			"message":{
				"role":"assistant",
				"content":"",
				"refusal":"blocked refusal",
				"tool_calls":[{
					"id":"call_1",
					"type":"function",
					"function":{"name":"delete_file","arguments":"{\"path\":\"/tmp/a\"}"}
				}]
			},
			"finish_reason":"stop"
		}]
	}`)
	_, err := w.Write(chatBody)
	require.NoError(t, err)
	require.NoError(t, w.Finalize(http.StatusOK))

	body := rec.Body.String()
	require.Contains(t, body, responsespkg.BlockedMessage)
	require.NotContains(t, body, "blocked refusal")
}

func TestResponsesAdapterNonStreamWriterFinalizeSurfacesMalformedChatBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	w := newResponsesAdapterNonStreamWriter(ctx.Writer, "public-model", &captureCounter{}, nil, "")
	_, _ = w.Write([]byte(`not-json`))

	err := w.Finalize(http.StatusOK)
	require.Error(t, err)
}

func TestDecodeResponsesAdapterChatBodyEmpty(t *testing.T) {
	_, err := decodeResponsesAdapterChatBody(nil)
	require.Error(t, err)
}

func TestDecodeResponsesAdapterChatBodyIdentity(t *testing.T) {
	w := newBufferCommonResponseWriter()
	w.Header().Set("Content-Encoding", "identity")
	_, _ = w.Write([]byte("body"))
	body, err := decodeResponsesAdapterChatBody(w)
	require.NoError(t, err)
	require.Equal(t, []byte("body"), body)
}

func TestDecodeResponsesAdapterChatBodyGzip(t *testing.T) {
	w := newBufferCommonResponseWriter()
	encoded, err := encodeGzip(t, []byte("body"))
	require.NoError(t, err)
	w.Header().Set("Content-Encoding", "gzip")
	_, _ = w.Write(encoded)
	body, err := decodeResponsesAdapterChatBody(w)
	require.NoError(t, err)
	require.Equal(t, []byte("body"), body)
}

type captureCounter struct {
	mu         sync.Mutex
	lastResp   *types.ResponsesResponse
	onResponse func(*types.ResponsesResponse)
}

func (c *captureCounter) Request(_ *types.ResponsesRequest) {}
func (c *captureCounter) Response(r *types.ResponsesResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastResp = r
	if c.onResponse != nil {
		c.onResponse(r)
	}
}
func (c *captureCounter) AppendEvent(_ types.ResponsesStreamEvent) {}
func (c *captureCounter) Usage(_ context.Context) (*token.Usage, error) {
	return nil, nil
}

func encodeGzip(t *testing.T, data []byte) ([]byte, error) {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(data); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

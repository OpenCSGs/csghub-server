package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResponsesNativeNonStreamWriterRewritesResponseID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mapper, err := newResponsesIDMapper("test-secret")
	require.NoError(t, err)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	transformer := newResponsesNativePayloadTransformer(mapper, responsesIDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7}, "", nil)
	writer := newResponsesNativeNonStreamWriter(ctx.Writer, transformer)
	writer.WriteHeader(200)
	_, err = writer.Write([]byte(`{"id":"resp_upstream_1","object":"response","status":"completed"}`))
	require.NoError(t, err)
	require.NoError(t, writer.Finalize())

	body := w.Body.String()
	require.Contains(t, body, "resp_agw_v1.")
	require.NotContains(t, body, `"id":"resp_upstream_1"`)

	start := strings.Index(body, "resp_agw_v1.")
	require.NotEqual(t, -1, start)
	end := strings.Index(body[start:], `"`)
	require.NotEqual(t, -1, end)
	id := body[start : start+end]
	claims, err := mapper.Unwrap(id, "namespace-1")
	require.NoError(t, err)
	require.Equal(t, "resp_upstream_1", claims.UpstreamResponseID)
}

func TestResponsesNativeNonStreamWriterReusesWrappedResponseID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mapper, err := newResponsesIDMapper("test-secret")
	require.NoError(t, err)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	transformer := newResponsesNativePayloadTransformer(mapper, responsesIDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7}, "", nil)
	writer := newResponsesNativeNonStreamWriter(ctx.Writer, transformer)
	writer.WriteHeader(200)
	_, err = writer.Write([]byte(`{"id":"resp_upstream_1","response_id":"resp_upstream_1","object":"response"}`))
	require.NoError(t, err)
	require.NoError(t, writer.Finalize())

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, body["id"], body["response_id"])
	require.Contains(t, body["id"], "resp_agw_v1.")
}

func TestResponsesNativeNonStreamWriterEchoesPreviousResponseID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mapper, err := newResponsesIDMapper("test-secret")
	require.NoError(t, err)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	transformer := newResponsesNativePayloadTransformer(
		mapper,
		responsesIDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7},
		"resp_agw_v1.public_previous",
		nil,
	)
	writer := newResponsesNativeNonStreamWriter(ctx.Writer, transformer)
	writer.WriteHeader(200)
	_, err = writer.Write([]byte(`{"id":"resp_upstream_2","object":"response","status":"completed"}`))
	require.NoError(t, err)
	require.NoError(t, writer.Finalize())

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "resp_agw_v1.public_previous", body["previous_response_id"])
}

func TestResponsesNativeStreamWriterRewritesResponseID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mapper, err := newResponsesIDMapper("test-secret")
	require.NoError(t, err)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	transformer := newResponsesNativePayloadTransformer(mapper, responsesIDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7}, "", nil)
	writer := newResponsesNativeStreamWriter(ctx.Writer, transformer)
	writer.WriteHeader(200)

	_, err = writer.Write([]byte("event: response.created\n" + `data: {"type":"response.created","response":{"id":"resp_upstream_1","object":"response","status":"in_progress"}}` + "\n\n"))
	require.NoError(t, err)

	body := w.Body.String()
	require.Contains(t, body, "event: response.created")
	require.Contains(t, body, "resp_agw_v1.")
	require.NotContains(t, body, "resp_upstream_1")
	require.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
}

func TestResponsesNativeStreamWriterEmitsErrorAndStopsStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mapper, err := newResponsesIDMapper("test-secret")
	require.NoError(t, err)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	transformer := newResponsesNativePayloadTransformer(mapper, responsesIDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7}, "", nil)
	writer := newResponsesNativeStreamWriter(ctx.Writer, transformer)
	writer.WriteHeader(200)

	_, err = writer.Write([]byte("event: error\n" + `data: {"error":{"message":"rate limited","type":"rate_limit_error","code":"rate_limit_exceeded"}}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte("event: response.completed\n" + `data: {"type":"response.completed"}` + "\n\n"))
	require.NoError(t, err)

	body := w.Body.String()
	require.Contains(t, body, "event: error")
	require.Contains(t, body, `"message":"rate limited"`)
	require.Contains(t, body, `"type":"rate_limit_error"`)
	require.Contains(t, body, `"code":"rate_limit_exceeded"`)
	require.NotContains(t, body, "event: response.completed")
}

func TestResponsesNativeStreamWriterPassthroughUpstreamJSONError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mapper, err := newResponsesIDMapper("test-secret")
	require.NoError(t, err)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	transformer := newResponsesNativePayloadTransformer(mapper, responsesIDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7}, "", nil)
	writer := newResponsesNativeStreamWriter(ctx.Writer, transformer)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusTooManyRequests)
	_, err = writer.Write([]byte(`{"error":{"message":"rate limited","type":"rate_limit_error"}}`))
	require.NoError(t, err)

	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	require.Contains(t, w.Body.String(), `"message":"rate limited"`)
}

func TestResponsesNativeStreamWriterRewritesResponseIDWithCRLF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mapper, err := newResponsesIDMapper("test-secret")
	require.NoError(t, err)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	transformer := newResponsesNativePayloadTransformer(mapper, responsesIDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7}, "", nil)
	writer := newResponsesNativeStreamWriter(ctx.Writer, transformer)
	writer.WriteHeader(200)

	_, err = writer.Write([]byte("event: response.created\r\n" + `data: {"type":"response.created","response":{"id":"resp_upstream_1","object":"response","status":"in_progress"}}` + "\r\n\r\n"))
	require.NoError(t, err)

	body := w.Body.String()
	require.Contains(t, body, "event: response.created")
	require.Contains(t, body, "resp_agw_v1.")
	require.NotContains(t, body, "resp_upstream_1")
	require.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
}

func TestParseResponsesUsageAcceptsDetailOnlyUsage(t *testing.T) {
	usage, ok := parseResponsesUsage(map[string]any{
		"input_tokens_details": map[string]any{
			"cached_tokens":          float64(12),
			"cached_creation_tokens": float64(3),
		},
	})
	require.True(t, ok)
	require.NotNil(t, usage.InputTokensDetails)
	require.Equal(t, int64(12), usage.InputTokensDetails.CachedTokens)
	require.Equal(t, int64(3), usage.InputTokensDetails.CachedCreationTokens)
}

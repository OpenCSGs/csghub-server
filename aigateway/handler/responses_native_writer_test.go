package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
)

func TestResponsesNativeNonStreamWriterRewritesResponseID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mapper, err := responsespkg.NewIDMapper("test-secret")
	require.NoError(t, err)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	transformer := newResponsesNativePayloadTransformer(mapper, responsespkg.IDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7}, "", nil)
	writer := newResponsesNativeNonStreamWriter(ctx.Writer, transformer, nil, "")
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
	mapper, err := responsespkg.NewIDMapper("test-secret")
	require.NoError(t, err)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	transformer := newResponsesNativePayloadTransformer(mapper, responsespkg.IDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7}, "", nil)
	writer := newResponsesNativeNonStreamWriter(ctx.Writer, transformer, nil, "")
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
	mapper, err := responsespkg.NewIDMapper("test-secret")
	require.NoError(t, err)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	transformer := newResponsesNativePayloadTransformer(
		mapper,
		responsespkg.IDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7},
		"resp_agw_v1.public_previous",
		nil,
	)
	writer := newResponsesNativeNonStreamWriter(ctx.Writer, transformer, nil, "")
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
	mapper, err := responsespkg.NewIDMapper("test-secret")
	require.NoError(t, err)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	transformer := newResponsesNativePayloadTransformer(mapper, responsespkg.IDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7}, "", nil)
	writer := newResponsesNativeStreamWriter(ctx.Writer, transformer, nil, "")
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
	mapper, err := responsespkg.NewIDMapper("test-secret")
	require.NoError(t, err)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	transformer := newResponsesNativePayloadTransformer(mapper, responsespkg.IDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7}, "", nil)
	writer := newResponsesNativeStreamWriter(ctx.Writer, transformer, nil, "")
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

func TestResponsesNativeStreamWriterFinalizeDoesNotAppendSensitiveAfterError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mapper, err := responsespkg.NewIDMapper("test-secret")
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	transformer := newResponsesNativePayloadTransformer(mapper, responsespkg.IDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7}, "", nil)

	moderation := mockcomp.NewMockModeration(t)
	moderation.EXPECT().CheckText(
		mock.Anything,
		types.TextModerationRequest{
			Content: "safe first chunk",
			Key:     "session-1",
			Phase:   types.TextModerationPhaseResponse,
			Mode:    types.TextModerationModeStream,
		},
	).Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
	moderation.EXPECT().CloseStreamCheck(mock.Anything, "session-1").
		Return(&rpc.CheckResult{IsSensitive: true, Reason: "async sensitive"}, nil).
		Once()

	writer := newResponsesNativeStreamWriter(ctx.Writer, transformer, moderation, "session-1")
	writer.WriteHeader(http.StatusOK)
	_, err = writer.Write([]byte("event: response.output_text.delta\n" + `data: {"type":"response.output_text.delta","delta":"safe first chunk"}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte("event: error\n" + `data: {"error":{"message":"rate limited","type":"rate_limit_error","code":"rate_limit_exceeded"}}` + "\n\n"))
	require.NoError(t, err)
	require.NoError(t, writer.Finalize())

	body := rec.Body.String()
	require.Contains(t, body, "event: error")
	require.Contains(t, body, `"message":"rate limited"`)
	require.NotContains(t, body, responsespkg.BlockedMessage)
	require.NotContains(t, body, "data: [DONE]")
	require.Equal(t, 1, strings.Count(body, "event: error"))
}

func TestResponsesNativeStreamWriterClosesModerationOnceOnDone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mapper, err := responsespkg.NewIDMapper("test-secret")
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	transformer := newResponsesNativePayloadTransformer(mapper, responsespkg.IDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7}, "", nil)

	moderation := mockcomp.NewMockModeration(t)
	moderation.EXPECT().CheckText(
		mock.Anything,
		types.TextModerationRequest{
			Content: "safe first chunk",
			Key:     "session-1",
			Phase:   types.TextModerationPhaseResponse,
			Mode:    types.TextModerationModeStream,
		},
	).Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
	moderation.EXPECT().CloseStreamCheck(mock.Anything, "session-1").
		Return(&rpc.CheckResult{IsSensitive: false}, nil).
		Once()

	writer := newResponsesNativeStreamWriter(ctx.Writer, transformer, moderation, "session-1")
	writer.WriteHeader(http.StatusOK)
	_, err = writer.Write([]byte("event: response.output_text.delta\n" + `data: {"type":"response.output_text.delta","delta":"safe first chunk"}` + "\n\n"))
	require.NoError(t, err)
	_, err = writer.Write([]byte("data: [DONE]\n\n"))
	require.NoError(t, err)
	require.NoError(t, writer.Finalize())

	body := rec.Body.String()
	require.Contains(t, body, "data: [DONE]")
	require.NotContains(t, body, responsespkg.BlockedMessage)
}

func TestResponsesNativeStreamEventTextExtractsRefusalPart(t *testing.T) {
	text, ok := responsesStreamEventText([]byte(`{
		"type":"response.content_part.added",
		"part":{"type":"refusal","refusal":"blocked refusal"}
	}`))

	require.True(t, ok)
	require.Equal(t, "blocked refusal", text)
}

func TestResponsesNativeNonStreamWriterModeratesRefusalContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mapper, err := responsespkg.NewIDMapper("test-secret")
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	transformer := newResponsesNativePayloadTransformer(mapper, responsespkg.IDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7}, "", nil)

	moderation := mockcomp.NewMockModeration(t)
	moderation.EXPECT().CheckText(
		mock.MatchedBy(func(ctx context.Context) bool {
			deadline, ok := ctx.Deadline()
			return ok && time.Until(deadline) <= responsespkg.ModerationTimeout
		}),
		types.TextModerationRequest{
			Content: "blocked refusal",
			Key:     "session-1",
			Phase:   types.TextModerationPhaseResponse,
			Mode:    types.TextModerationModeNonStream,
		},
	).Return(&rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil).Once()

	writer := newResponsesNativeNonStreamWriter(ctx.Writer, transformer, moderation, "session-1")
	writer.WriteHeader(http.StatusOK)
	_, err = writer.Write([]byte(`{
		"id":"resp_upstream_1",
		"object":"response",
		"status":"completed",
		"output":[{
			"type":"message",
			"status":"completed",
			"role":"assistant",
			"content":[{"type":"refusal","refusal":"blocked refusal"}]
		}]
	}`))
	require.NoError(t, err)
	require.NoError(t, writer.Finalize())

	body := rec.Body.String()
	require.Contains(t, body, responsespkg.BlockedMessage)
	require.NotContains(t, body, "blocked refusal")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
}

func TestResponsesNativeStreamWriterPassthroughUpstreamJSONError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mapper, err := responsespkg.NewIDMapper("test-secret")
	require.NoError(t, err)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	transformer := newResponsesNativePayloadTransformer(mapper, responsespkg.IDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7}, "", nil)
	writer := newResponsesNativeStreamWriter(ctx.Writer, transformer, nil, "")
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
	mapper, err := responsespkg.NewIDMapper("test-secret")
	require.NoError(t, err)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	transformer := newResponsesNativePayloadTransformer(mapper, responsespkg.IDClaims{NamespaceUUID: "namespace-1", UpstreamID: 7}, "", nil)
	writer := newResponsesNativeStreamWriter(ctx.Writer, transformer, nil, "")
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

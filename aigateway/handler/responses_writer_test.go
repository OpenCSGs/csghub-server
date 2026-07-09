package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/token"
)

func TestBufferCommonResponseWriterStatusDefaultsOK(t *testing.T) {
	w := newBufferCommonResponseWriter()
	require.Equal(t, http.StatusOK, w.StatusCode())
}

func TestBufferCommonResponseWriterTracksStatus(t *testing.T) {
	w := newBufferCommonResponseWriter()
	w.WriteHeader(404)
	require.Equal(t, 404, w.StatusCode())
}

func TestBufferCommonResponseWriterBuffersBody(t *testing.T) {
	w := newBufferCommonResponseWriter()
	n, err := w.Write([]byte("hello"))
	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, "hello", w.body.String())
	require.Equal(t, "hello", w.body.String())
}

func TestBufferCommonResponseWriterHeaderReturnsMutable(t *testing.T) {
	w := newBufferCommonResponseWriter()
	w.Header().Set("X-Test", "1")
	require.Equal(t, "1", w.Header().Get("X-Test"))
}

func TestBufferCommonResponseWriterClearBufferResetsBody(t *testing.T) {
	w := newBufferCommonResponseWriter()
	_, _ = w.Write([]byte("body"))
	w.ClearBuffer()
	require.Equal(t, 0, w.body.Len())
}

func TestNewResponsesAdapterResponseWriterStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	w := newResponsesAdapterResponseWriter(ctx.Writer, true, "public-model", token.NewResponsesTokenCounter(nil), nil, "")
	_, ok := w.(*responsesAdapterStreamWriter)
	require.True(t, ok)
}

func TestNewResponsesAdapterResponseWriterNonStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	w := newResponsesAdapterResponseWriter(ctx.Writer, false, "public-model", token.NewResponsesTokenCounter(nil), nil, "")
	_, ok := w.(*responsesAdapterNonStreamWriter)
	require.True(t, ok)
}

func TestNewResponsesModerationSessionIDUsesFullUUID(t *testing.T) {
	sessionID := newResponsesModerationSessionID()

	parsed, err := uuid.Parse(sessionID)
	require.NoError(t, err)
	require.Equal(t, sessionID, parsed.String())
	require.Len(t, sessionID, 36)
}

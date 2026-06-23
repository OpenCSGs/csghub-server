package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChatRetryResponseWriter_ReplaysJSONErrorToStreamWrapper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	downstream := newStreamResponseWriter(ctx.Writer, nil, nil, nil)
	retryWriter := newChatRetryResponseWriter(downstream)

	errorBody := `{"error":{"message":"bad gateway","type":"api_error"}}`
	retryWriter.Header().Set("Content-Type", "application/json")
	retryWriter.WriteHeader(http.StatusBadGateway)
	_, err := retryWriter.Write([]byte(errorBody))
	require.NoError(t, err)
	require.False(t, retryWriter.StreamStarted())
	require.NoError(t, retryWriter.ReplayBufferedResponse())

	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.Equal(t, errorBody, rec.Body.String())
}

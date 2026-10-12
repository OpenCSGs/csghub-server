package handler

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInternalServiceProxyOAuthCallbackLogsAreSafe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/fedap/oauth/callback", r.URL.Path)
		assert.Equal(t, "code-secret", r.URL.Query().Get("code"))
		assert.Equal(t, "state-secret", r.URL.Query().Get("state"))
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	h, err := NewInternalServiceProxyHandler(upstream.URL)
	require.NoError(t, err)
	router := gin.New()
	router.GET("/federation-adapter/oauth/callback", h.ProxyToApi("/api/v1/fedap/oauth/callback"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/federation-adapter/oauth/callback?code=code-secret&state=state-secret", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.NotContains(t, logs.String(), "code-secret")
	require.NotContains(t, logs.String(), "state-secret")
	require.Contains(t, logs.String(), "proxy user request")
}

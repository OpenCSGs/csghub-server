package rpc

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/common/errorx"
)

func setupTestCsgbotClient(server *httptest.Server) *CsgbotSvcHttpClientImpl {
	return &CsgbotSvcHttpClientImpl{
		hc: &HttpClient{
			endpoint: server.URL,
			hc:       server.Client(),
			logger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		},
	}
}

func TestDeleteWorkspaceFiles_Success(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	contentID := "test-agent-name"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request path
		expectedPath := "/api/v1/csgbot/codeAgent/" + contentID
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)

		// Verify headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, userUUID, r.Header.Get("user_uuid"))
		assert.Equal(t, username, r.Header.Get("user_name"))
		assert.Equal(t, token, r.Header.Get("user_token"))

		// Return success
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	err := client.DeleteWorkspaceFiles(context.Background(), userUUID, username, token, contentID)

	// Verify result
	assert.NoError(t, err)
}

func TestDeleteWorkspaceFiles_InternalServerError(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	contentID := "test-agent-name"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	err := client.DeleteWorkspaceFiles(context.Background(), userUUID, username, token, contentID)

	// Verify result - should return RemoteSvcFail error
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errorx.ErrRemoteServiceFail))
}

func TestNewCsgbotSvcHttpClient(t *testing.T) {
	endpoint := "http://test-endpoint.com"
	client := NewCsgbotSvcHttpClient(endpoint)

	// Verify the client is created
	assert.NotNil(t, client)

	// Verify it implements the interface
	var _ CsgbotSvcClient = client
}

package rpc

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestAgentHubClient(server *httptest.Server) *AgentHubSvcClientImpl {
	return &AgentHubSvcClientImpl{
		hc: &HttpClient{
			endpoint: server.URL,
			hc:       server.Client(),
			logger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		},
		token: "test-token",
	}
}

func TestAgentHubSvcClientImpl_DeleteAgentInstance_Success(t *testing.T) {
	contentID := "test-content-id"
	userUUID := "test-user-uuid"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/opencsg/flows/delete?token=test-token", r.URL.String())
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, userUUID, r.Header.Get("user_uuid"))

		var req DeleteAgentInstanceRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, []string{contentID}, req.IDs)

		resp := DeleteAgentInstanceResponse{
			IDs:   []string{contentID},
			Total: 1,
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := setupTestAgentHubClient(server)

	err := client.DeleteAgentInstance(context.Background(), userUUID, contentID)
	require.NoError(t, err)
}

func TestAgentHubSvcClientImpl_DeleteAgentInstance_Non200Status(t *testing.T) {
	contentID := "test-content-id"
	userUUID := "test-user-uuid"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := setupTestAgentHubClient(server)

	err := client.DeleteAgentInstance(context.Background(), userUUID, contentID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete agent instance in agenthub, status code: 500")
}

func TestAgentHubSvcClientImpl_DeleteAgentInstance_ReadBodyError(t *testing.T) {
	contentID := "test-content-id"
	userUUID := "test-user-uuid"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set Content-Length to force an error when reading
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		// Close the connection immediately to cause a read error
	}))
	defer server.Close()

	client := setupTestAgentHubClient(server)

	err := client.DeleteAgentInstance(context.Background(), userUUID, contentID)
	require.Error(t, err)
}

func TestAgentHubSvcClientImpl_CreateAgentInstance_Success(t *testing.T) {
	userUUID := "test-user-uuid"
	req := &CreateAgentInstanceRequest{
		Name:        "Test Instance",
		Description: "Test Description",
		Data:        json.RawMessage(`{"nodes": [], "edges": []}`),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/opencsg/flows/?token=test-token", r.URL.String())
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, userUUID, r.Header.Get("user_uuid"))

		var requestBody CreateAgentInstanceRequest
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		require.NoError(t, err)
		assert.Equal(t, req.Name, requestBody.Name)
		assert.Equal(t, req.Description, requestBody.Description)

		resp := CreateAgentInstanceResponse{
			ID:          "new-flow-id",
			Name:        "Test Instance",
			Description: "Test Description",
			Data:        json.RawMessage(`{"nodes": [], "edges": []}`),
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	}))
	defer server.Close()

	client := setupTestAgentHubClient(server)

	resp, err := client.CreateAgentInstance(context.Background(), userUUID, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "new-flow-id", resp.ID)
	assert.Equal(t, "Test Instance", resp.Name)
	assert.Equal(t, "Test Description", resp.Description)
}

func TestAgentHubSvcClientImpl_CreateAgentInstance_NilRequest(t *testing.T) {
	userUUID := "test-user-uuid"

	client := setupTestAgentHubClient(httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	defer client.hc.hc.CloseIdleConnections()

	resp, err := client.CreateAgentInstance(context.Background(), userUUID, nil)
	require.Error(t, err)
	require.Nil(t, resp)
	assert.Contains(t, err.Error(), "create agent instance request is nil")
}

func TestAgentHubSvcClientImpl_CreateAgentInstance_Non200Status(t *testing.T) {
	userUUID := "test-user-uuid"
	req := &CreateAgentInstanceRequest{
		Name:        "Test Instance",
		Description: "Test Description",
		Data:        json.RawMessage(`{}`),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := setupTestAgentHubClient(server)

	resp, err := client.CreateAgentInstance(context.Background(), userUUID, req)
	require.Error(t, err)
	require.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to create agent instance in agenthub")
}

func TestAgentHubSvcClientImpl_CreateAgentInstance_ReadBodyError(t *testing.T) {
	userUUID := "test-user-uuid"
	req := &CreateAgentInstanceRequest{
		Name:        "Test Instance",
		Description: "Test Description",
		Data:        json.RawMessage(`{}`),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set Content-Length to force an error when reading
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		// Close the connection immediately to cause a read error
	}))
	defer server.Close()

	client := setupTestAgentHubClient(server)

	resp, err := client.CreateAgentInstance(context.Background(), userUUID, req)
	require.Error(t, err)
	require.Nil(t, resp)
}

func TestAgentHubSvcClientImpl_UpdateAgentInstance_Success(t *testing.T) {
	contentID := "test-content-id"
	userUUID := "test-user-uuid"
	mcpEnabled := true
	locked := false
	req := &UpdateAgentInstanceRequest{
		Name:        "Updated Instance",
		Description: "Updated Description",
		Data:        json.RawMessage(`{"nodes": [{"id": "1"}], "edges": []}`),
		FolderID:    "folder-id",
		MCPEnabled:  &mcpEnabled,
		Locked:      &locked,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/opencsg/flows/"+contentID+"?token=test-token", r.URL.String())
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, userUUID, r.Header.Get("user_uuid"))

		var requestBody UpdateAgentInstanceRequest
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		require.NoError(t, err)
		assert.Equal(t, req.Name, requestBody.Name)
		assert.Equal(t, req.Description, requestBody.Description)
		assert.Equal(t, req.FolderID, requestBody.FolderID)
		assert.NotNil(t, requestBody.MCPEnabled)
		assert.True(t, *requestBody.MCPEnabled)
		assert.NotNil(t, requestBody.Locked)
		assert.False(t, *requestBody.Locked)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := setupTestAgentHubClient(server)

	err := client.UpdateAgentInstance(context.Background(), userUUID, contentID, req)
	require.NoError(t, err)
}

func TestAgentHubSvcClientImpl_UpdateAgentInstance_NilRequest(t *testing.T) {
	contentID := "test-content-id"
	userUUID := "test-user-uuid"

	client := setupTestAgentHubClient(httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	defer client.hc.hc.CloseIdleConnections()

	err := client.UpdateAgentInstance(context.Background(), userUUID, contentID, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update agent instance request is nil")
}

func TestAgentHubSvcClientImpl_UpdateAgentInstance_Non200Status(t *testing.T) {
	contentID := "test-content-id"
	userUUID := "test-user-uuid"
	req := &UpdateAgentInstanceRequest{
		Name:        "Updated Instance",
		Description: "Updated Description",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := setupTestAgentHubClient(server)

	err := client.UpdateAgentInstance(context.Background(), userUUID, contentID, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update agent instance in agenthub")
}

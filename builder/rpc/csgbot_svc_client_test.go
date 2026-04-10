package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func setupTestCsgbotClient(server *httptest.Server) *CsgbotSvcHttpClientImpl {
	hc := &HttpClient{
		endpoint:   server.URL,
		hc:         server.Client(),
		logger:     slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		retry:      1, // no retry for tests
		retryDelay: 0,
	}
	return &CsgbotSvcHttpClientImpl{hc: hc}
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

func TestUpdateWorkspaceFiles_Success(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	agentName := "test-agent-name"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/api/v1/csgbot/codeAgent/updateCode"
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, userUUID, r.Header.Get("user_uuid"))
		assert.Equal(t, username, r.Header.Get("user_name"))
		assert.Equal(t, token, r.Header.Get("user_token"))

		var req UpdateWorkspaceFilesRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, agentName, req.AgentName)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	err := client.UpdateWorkspaceFiles(context.Background(), userUUID, username, token, agentName)

	assert.NoError(t, err)
}

func TestUpdateWorkspaceFiles_InternalServerError(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	agentName := "test-agent-name"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	err := client.UpdateWorkspaceFiles(context.Background(), userUUID, username, token, agentName)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, errorx.ErrRemoteServiceFail))
}

func TestNewCsgbotSvcHttpClient(t *testing.T) {
	endpoint := "http://test-endpoint.com"
	client := NewCsgbotSvcHttpClient(endpoint)

	assert.NotNil(t, client)

	httpClient, ok := client.(*CsgbotSvcHttpClientImpl)
	require.True(t, ok)
	assert.Equal(t, endpoint, httpClient.hc.endpoint)
	assert.Len(t, httpClient.hc.authOpts, 0)
}

func TestCreateKnowledgeBase_Success(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	req := &CreateKnowledgeBaseRequest{
		Name:        "Test KB",
		Description: "Test description",
	}

	expectedResponse := CreateKnowledgeBaseResponse{
		ID:          "test-content-id",
		Name:        "Test KB",
		Description: "",
		IsComponent: false,
		Webhook:     false,
		Tags:        []string{"tag1"},
		Locked:      false,
		McpEnabled:  false,
		AccessType:  "PRIVATE",
		UserUUID:    userUUID,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request path
		expectedPath := "/api/v1/csgbot/langflow/flows/rag"
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		// Verify headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, userUUID, r.Header.Get("user_uuid"))
		assert.Equal(t, username, r.Header.Get("user_name"))
		assert.Equal(t, token, r.Header.Get("user_token"))

		// Return success with response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "test-content-id",
			"name": "Test KB",
			"description": "",
			"data": {},
			"is_component": false,
			"created_at": "2025-01-01T00:00:00Z",
			"updated_at": "2025-01-01T00:00:00Z",
			"webhook": false,
			"tags": ["tag1"],
			"locked": false,
			"mcp_enabled": false,
			"access_type": "PRIVATE",
			"user_id": "test-user-uuid",
			"folder_id": "test-folder-id"
		}`))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	resp, err := client.CreateKnowledgeBase(context.Background(), userUUID, username, token, req)

	// Verify result
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, expectedResponse.ID, resp.ID)
	assert.Equal(t, expectedResponse.Name, resp.Name)
	assert.Equal(t, expectedResponse.Description, resp.Description)
	assert.Equal(t, expectedResponse.UserUUID, resp.UserUUID)
}

func TestCreateKnowledgeBase_DescriptionIsEmpty(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	req := &CreateKnowledgeBaseRequest{
		Name:        "Test KB",
		Description: "",
	}

	expectedResponse := CreateKnowledgeBaseResponse{
		ID:          "test-content-id",
		Name:        "Test KB",
		Description: "",
		IsComponent: false,
		Webhook:     false,
		Tags:        []string{"tag1"},
		Locked:      false,
		McpEnabled:  false,
		AccessType:  "PRIVATE",
		UserUUID:    userUUID,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request path
		expectedPath := "/api/v1/csgbot/langflow/flows/rag"
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		// Verify headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, userUUID, r.Header.Get("user_uuid"))
		assert.Equal(t, username, r.Header.Get("user_name"))
		assert.Equal(t, token, r.Header.Get("user_token"))

		// Return success with response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "test-content-id",
			"name": "Test KB",
			"description": "",
			"data": {},
			"is_component": false,
			"created_at": "2025-01-01T00:00:00Z",
			"updated_at": "2025-01-01T00:00:00Z",
			"webhook": false,
			"tags": ["tag1"],
			"locked": false,
			"mcp_enabled": false,
			"access_type": "PRIVATE",
			"user_id": "test-user-uuid",
			"folder_id": "test-folder-id"
		}`))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	resp, err := client.CreateKnowledgeBase(context.Background(), userUUID, username, token, req)

	// Verify result
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, expectedResponse.ID, resp.ID)
	assert.Equal(t, expectedResponse.Name, resp.Name)
	assert.Equal(t, expectedResponse.Description, resp.Description)
	assert.Equal(t, expectedResponse.UserUUID, resp.UserUUID)
}

func TestCreateKnowledgeBase_NilRequest(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	resp, err := client.CreateKnowledgeBase(context.Background(), userUUID, username, token, nil)

	// Verify result
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, errorx.ErrBadRequest))
}

func TestCreateKnowledgeBase_Non200Status(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	req := &CreateKnowledgeBaseRequest{
		Name:        "Test KB",
		Description: "",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	resp, err := client.CreateKnowledgeBase(context.Background(), userUUID, username, token, req)

	// Verify result
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, errorx.ErrRemoteServiceFail))
}

func TestCreateKnowledgeBase_ReadBodyError(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	req := &CreateKnowledgeBaseRequest{
		Name:        "Test KB",
		Description: "",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1")
		w.WriteHeader(http.StatusOK)
		// Close the connection immediately to cause read error
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	resp, err := client.CreateKnowledgeBase(context.Background(), userUUID, username, token, req)

	// Verify result
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestDeleteKnowledgeBase_Success(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	contentID := "test-content-id"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request path
		expectedPath := "/api/v1/csgbot/langflow/flows/rag/delete"
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		// Verify headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, userUUID, r.Header.Get("user_uuid"))
		assert.Equal(t, username, r.Header.Get("user_name"))
		assert.Equal(t, token, r.Header.Get("user_token"))

		// Verify request body
		var reqBody DeleteKnowledgeBaseRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		assert.NoError(t, err)
		assert.Equal(t, []string{contentID}, reqBody.IDs)

		// Return success with response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ids": ["test-content-id"],
			"total": 1
		}`))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	err := client.DeleteKnowledgeBase(context.Background(), userUUID, username, token, contentID)

	// Verify result
	assert.NoError(t, err)
}

func TestDeleteKnowledgeBase_Non200Status(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	contentID := "test-content-id"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	err := client.DeleteKnowledgeBase(context.Background(), userUUID, username, token, contentID)

	// Verify result
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errorx.ErrRemoteServiceFail))
}

func TestDeleteKnowledgeBase_ReadBodyError(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	contentID := "test-content-id"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1")
		w.WriteHeader(http.StatusOK)
		// Close the connection immediately to cause read error
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	err := client.DeleteKnowledgeBase(context.Background(), userUUID, username, token, contentID)

	// Verify result
	assert.Error(t, err)
}

func TestDeleteKnowledgeBase_UnmarshalError(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	contentID := "test-content-id"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	err := client.DeleteKnowledgeBase(context.Background(), userUUID, username, token, contentID)

	// Verify result
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal response error")
}

func TestDeleteKnowledgeBase_TotalMismatch(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	contentID := "test-content-id"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ids": ["test-content-id"],
			"total": 2
		}`))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	err := client.DeleteKnowledgeBase(context.Background(), userUUID, username, token, contentID)

	// Verify result
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "total: 2")
}

func TestDeleteKnowledgeBase_EmptyIDs(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	contentID := "test-content-id"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ids": [],
			"total": 1
		}`))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	err := client.DeleteKnowledgeBase(context.Background(), userUUID, username, token, contentID)

	// Verify result
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "response IDs is empty")
}

func TestDeleteKnowledgeBase_ContentIDMismatch(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	contentID := "test-content-id"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ids": ["different-content-id"],
			"total": 1
		}`))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	err := client.DeleteKnowledgeBase(context.Background(), userUUID, username, token, contentID)

	// Verify result
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "content ID mismatch")
}

func TestUpdateKnowledgeBase_Success(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	contentID := "test-content-id"
	name := "Updated KB Name"
	description := "Updated description"
	req := &types.UpdateAgentKnowledgeBaseRequest{
		Name:        &name,
		Description: &description,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request path
		expectedPath := "/api/v1/csgbot/langflow/flows/rag/" + contentID
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, http.MethodPatch, r.Method)

		// Verify headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, userUUID, r.Header.Get("user_uuid"))
		assert.Equal(t, username, r.Header.Get("user_name"))
		assert.Equal(t, token, r.Header.Get("user_token"))

		// Verify request body
		var reqBody UpdateKnowledgeBaseRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		assert.NoError(t, err)
		assert.Equal(t, name, reqBody.Name)
		assert.Equal(t, description, reqBody.Description)

		// Return success
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	err := client.UpdateKnowledgeBase(context.Background(), userUUID, username, token, contentID, req)

	// Verify result
	assert.NoError(t, err)
}

func TestUpdateKnowledgeBase_SuccessWithNameOnly(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	contentID := "test-content-id"
	name := "Updated KB Name"
	req := &types.UpdateAgentKnowledgeBaseRequest{
		Name: &name,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request path
		expectedPath := "/api/v1/csgbot/langflow/flows/rag/" + contentID
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, http.MethodPatch, r.Method)

		// Verify headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, userUUID, r.Header.Get("user_uuid"))
		assert.Equal(t, username, r.Header.Get("user_name"))
		assert.Equal(t, token, r.Header.Get("user_token"))

		// Verify request body
		var reqBody UpdateKnowledgeBaseRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		assert.NoError(t, err)
		assert.Equal(t, name, reqBody.Name)
		assert.Empty(t, reqBody.Description)

		// Return success
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	err := client.UpdateKnowledgeBase(context.Background(), userUUID, username, token, contentID, req)

	// Verify result
	assert.NoError(t, err)
}

func TestUpdateKnowledgeBase_SuccessWithDescriptionOnly(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	contentID := "test-content-id"
	description := "Updated description"
	req := &types.UpdateAgentKnowledgeBaseRequest{
		Description: &description,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request path
		expectedPath := "/api/v1/csgbot/langflow/flows/rag/" + contentID
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, http.MethodPatch, r.Method)

		// Verify headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, userUUID, r.Header.Get("user_uuid"))
		assert.Equal(t, username, r.Header.Get("user_name"))
		assert.Equal(t, token, r.Header.Get("user_token"))

		// Verify request body
		var reqBody UpdateKnowledgeBaseRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		assert.NoError(t, err)
		assert.Empty(t, reqBody.Name)
		assert.Equal(t, description, reqBody.Description)

		// Return success
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	err := client.UpdateKnowledgeBase(context.Background(), userUUID, username, token, contentID, req)

	// Verify result
	assert.NoError(t, err)
}

func TestUpdateKnowledgeBase_SuccessWithEmptyRequest(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	contentID := "test-content-id"
	req := &types.UpdateAgentKnowledgeBaseRequest{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request path
		expectedPath := "/api/v1/csgbot/langflow/flows/rag/" + contentID
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, http.MethodPatch, r.Method)

		// Verify headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, userUUID, r.Header.Get("user_uuid"))
		assert.Equal(t, username, r.Header.Get("user_name"))
		assert.Equal(t, token, r.Header.Get("user_token"))

		// Verify request body is empty
		var reqBody UpdateKnowledgeBaseRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		assert.NoError(t, err)
		assert.Empty(t, reqBody.Name)
		assert.Empty(t, reqBody.Description)

		// Return success
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	err := client.UpdateKnowledgeBase(context.Background(), userUUID, username, token, contentID, req)

	// Verify result
	assert.NoError(t, err)
}

func TestUpdateKnowledgeBase_NilRequest(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	contentID := "test-content-id"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	err := client.UpdateKnowledgeBase(context.Background(), userUUID, username, token, contentID, nil)

	// Verify result
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errorx.ErrBadRequest))
}

func TestUpdateKnowledgeBase_Non200Status(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	contentID := "test-content-id"
	name := "Updated KB Name"
	req := &types.UpdateAgentKnowledgeBaseRequest{
		Name: &name,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	// Execute the test
	err := client.UpdateKnowledgeBase(context.Background(), userUUID, username, token, contentID, req)

	// Verify result
	assert.Error(t, err)
	assert.True(t, errors.Is(err, errorx.ErrRemoteServiceFail))
}

func TestChat_Success(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	agentName := "test-agent"
	sessionID := "session-123"
	req := &CsgbotChatRequest{
		Messages: []CsgbotChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream: false,
	}

	expectedBody := `{"data":"hello response"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/api/v1/csgbot/" + agentName + "/chat"
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.NotEmpty(t, r.Header.Get(types.CSGBotHeaderRequestID))
		assert.Equal(t, userUUID, r.Header.Get(types.CSGBotHeaderUserUUID))
		assert.Equal(t, username, r.Header.Get(types.CSGBotHeaderUserName))
		assert.Equal(t, token, r.Header.Get(types.CSGBotHeaderUserToken))
		assert.Equal(t, agentName, r.Header.Get(types.CSGBotHeaderAgentName))
		assert.Equal(t, sessionID, r.Header.Get(types.CSGBotHeaderSessionID))
		assert.Empty(t, r.Header.Get("Accept"))

		var chatReq CsgbotChatRequest
		err := json.NewDecoder(r.Body).Decode(&chatReq)
		require.NoError(t, err)
		assert.Equal(t, req.Messages, chatReq.Messages)
		assert.Equal(t, req.Stream, chatReq.Stream)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(expectedBody))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	body, err := client.Chat(context.Background(), userUUID, username, token, agentName, sessionID, req)

	require.NoError(t, err)
	require.NotNil(t, body)
	defer body.Close()
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, expectedBody, string(data))
}

func TestChat_Success_IncludesModelInJSONBody(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	agentName := "test-agent"
	sessionID := "session-123"
	req := &CsgbotChatRequest{
		Model: "gemini:infini-ai",
		Messages: []CsgbotChatMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream: false,
	}

	expectedBody := `{"data":"hello response"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var chatReq CsgbotChatRequest
		err := json.NewDecoder(r.Body).Decode(&chatReq)
		require.NoError(t, err)
		assert.Equal(t, "gemini:infini-ai", chatReq.Model)
		assert.Equal(t, req.Messages, chatReq.Messages)
		assert.Equal(t, req.Stream, chatReq.Stream)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(expectedBody))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	body, err := client.Chat(context.Background(), userUUID, username, token, agentName, sessionID, req)
	require.NoError(t, err)
	require.NotNil(t, body)
	defer body.Close()
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, expectedBody, string(data))
}

func TestChat_Success_Stream(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	agentName := "stream-agent"
	sessionID := "session-456"
	req := &CsgbotChatRequest{
		Messages: []CsgbotChatMessage{
			{Role: "user", Content: "Stream me"},
		},
		Stream: true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"chunk\":\"hello\"}\n\n"))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	body, err := client.Chat(context.Background(), userUUID, username, token, agentName, sessionID, req)

	require.NoError(t, err)
	require.NotNil(t, body)
	defer body.Close()
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Contains(t, string(data), "data:")
}

func TestChat_NilRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	body, err := client.Chat(context.Background(), "user", "name", "token", "agent", "session", nil)

	assert.Error(t, err)
	assert.Nil(t, body)
	assert.True(t, errors.Is(err, errorx.ErrBadRequest))
	assert.Contains(t, err.Error(), "chat request is nil")
}

func TestChat_Non200Status(t *testing.T) {
	req := &CsgbotChatRequest{
		Messages: []CsgbotChatMessage{{Role: "user", Content: "test"}},
		Stream:   false,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	body, err := client.Chat(context.Background(), "user", "name", "token", "agent", "session", req)

	assert.Error(t, err)
	assert.Nil(t, body)
	assert.True(t, errors.Is(err, errorx.ErrRemoteServiceFail))
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "internal server error")
}

func TestChat_HttpError(t *testing.T) {
	req := &CsgbotChatRequest{
		Messages: []CsgbotChatMessage{{Role: "user", Content: "test"}},
		Stream:   false,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	client := setupTestCsgbotClient(server)
	server.Close() // Close server before request to simulate connection error

	body, err := client.Chat(context.Background(), "user", "name", "token", "agent", "session", req)

	assert.Error(t, err)
	assert.Nil(t, body)
	assert.True(t, errors.Is(err, errorx.ErrRemoteServiceFail))
	assert.Contains(t, err.Error(), "failed to call csgbot chat")
}

func TestChat_RequestBody(t *testing.T) {
	req := &CsgbotChatRequest{
		Messages: []CsgbotChatMessage{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: []map[string]any{{"type": "text", "text": "Hi"}}},
		},
		Stream: true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var chatReq CsgbotChatRequest
		err := json.NewDecoder(r.Body).Decode(&chatReq)
		require.NoError(t, err)
		assert.Len(t, chatReq.Messages, 2)
		assert.True(t, chatReq.Stream)

		// Verify content types (string vs array)
		assert.Equal(t, "Hello", chatReq.Messages[0].Content)
		content, ok := chatReq.Messages[1].Content.([]any)
		require.True(t, ok)
		assert.Len(t, content, 1)
		block, ok := content[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "text", block["type"])
		assert.Equal(t, "Hi", block["text"])

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	body, err := client.Chat(context.Background(), "u", "n", "t", "a", "s", req)
	require.NoError(t, err)
	require.NotNil(t, body)
	defer body.Close()
	data, _ := io.ReadAll(body)
	assert.Equal(t, "ok", strings.TrimSpace(string(data)))
}

func TestChatLangflow_Success(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	flowID := "flow-123"
	sessionID := "session-123"
	req := &LangflowSchedulerChatRequest{
		InputValue: "hello",
		InputType:  "chat",
		OutputType: "chat",
		SessionID:  sessionID,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/csgbot/langflow/flows/run/"+flowID, r.URL.Path)
		assert.Equal(t, "stream=true", r.URL.RawQuery)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))
		assert.NotEmpty(t, r.Header.Get(types.CSGBotHeaderRequestID))
		assert.Equal(t, userUUID, r.Header.Get(types.CSGBotHeaderUserUUID))
		assert.Equal(t, username, r.Header.Get(types.CSGBotHeaderUserName))
		assert.Equal(t, token, r.Header.Get(types.CSGBotHeaderUserToken))
		assert.Equal(t, sessionID, r.Header.Get(types.CSGBotHeaderSessionID))

		var got LangflowSchedulerChatRequest
		err := json.NewDecoder(r.Body).Decode(&got)
		require.NoError(t, err)
		assert.Equal(t, *req, got)

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"answer\":\"hello\",\"isFinal\":true}\n\n"))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	body, err := client.ChatLangflow(context.Background(), userUUID, username, token, flowID, sessionID, req)
	require.NoError(t, err)
	require.NotNil(t, body)
	defer body.Close()

	data, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Contains(t, string(data), "\"answer\":\"hello\"")
}

func TestChatLangflow_Non200Status(t *testing.T) {
	req := &LangflowSchedulerChatRequest{
		InputValue: "hello",
		InputType:  "chat",
		OutputType: "chat",
		SessionID:  "session-123",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("bad gateway"))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	body, err := client.ChatLangflow(context.Background(), "user", "name", "token", "flow-123", "session-123", req)
	assert.Error(t, err)
	assert.Nil(t, body)
	assert.True(t, errors.Is(err, errorx.ErrRemoteServiceFail))
	assert.Contains(t, err.Error(), "failed to call csgbot chat with status 502")
}

func TestChatCodeAgent_Success(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	req := &CodeAgentSchedulerChatRequest{
		RequestID: "req-123",
		Query:     "optimize code",
		AgentName: "custom-agent",
		Stream:    true,
		StreamMode: map[string]any{
			"mode": "general",
		},
		MaxLoop:       1,
		SearchEngines: []string{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/csgbot/codeAgent", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))
		assert.NotEmpty(t, r.Header.Get(types.CSGBotHeaderRequestID))
		assert.Equal(t, userUUID, r.Header.Get(types.CSGBotHeaderUserUUID))
		assert.Equal(t, username, r.Header.Get(types.CSGBotHeaderUserName))
		assert.Equal(t, token, r.Header.Get(types.CSGBotHeaderUserToken))

		var got CodeAgentSchedulerChatRequest
		err := json.NewDecoder(r.Body).Decode(&got)
		require.NoError(t, err)
		assert.Equal(t, req.RequestID, got.RequestID)
		assert.Equal(t, req.Query, got.Query)
		assert.Equal(t, req.AgentName, got.AgentName)
		assert.Equal(t, req.Stream, got.Stream)
		assert.Equal(t, req.MaxLoop, got.MaxLoop)

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"answer\":\"ok\",\"isFinal\":true}\n\n"))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	body, err := client.ChatCodeAgent(context.Background(), userUUID, username, token, req)
	require.NoError(t, err)
	require.NotNil(t, body)
	defer body.Close()

	data, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Contains(t, string(data), "\"answer\":\"ok\"")
}

func TestChatCodeAgent_Non200Status(t *testing.T) {
	req := &CodeAgentSchedulerChatRequest{
		RequestID: "req-123",
		Query:     "optimize code",
		AgentName: "custom-agent",
		Stream:    true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	body, err := client.ChatCodeAgent(context.Background(), "user", "name", "token", req)
	assert.Error(t, err)
	assert.Nil(t, body)
	assert.True(t, errors.Is(err, errorx.ErrRemoteServiceFail))
	assert.Contains(t, err.Error(), "failed to call csgbot chat with status 500")
}

func TestCreateOpenClaw_Success(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	req := CreateOpenClawRequest{"field": "value"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/api/v1/csgbot/openclaw/create"
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, userUUID, r.Header.Get(types.CSGBotHeaderUserUUID))
		assert.Equal(t, username, r.Header.Get(types.CSGBotHeaderUserName))
		assert.Equal(t, token, r.Header.Get(types.CSGBotHeaderUserToken))

		var body CreateOpenClawRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		assert.NoError(t, err)
		assert.Equal(t, "value", body["field"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "oc-12345",
			"metadata": {"endpoint": "https://oc.example.com"}
		}`))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	resp, err := client.CreateOpenClaw(context.Background(), userUUID, username, token, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "oc-12345", resp.ID)
	assert.Equal(t, "https://oc.example.com", resp.Metadata["endpoint"])
}

func TestCreateOpenClaw_NilRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, "{}", string(body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id": "oc-67890"}`))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	resp, err := client.CreateOpenClaw(context.Background(), "test-uuid", "testuser", "test-token", nil)

	assert.NoError(t, err)
	assert.Equal(t, "oc-67890", resp.ID)
}

func TestCreateOpenClaw_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	resp, err := client.CreateOpenClaw(context.Background(), "test-uuid", "testuser", "test-token", CreateOpenClawRequest{})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, errors.Is(err, errorx.ErrRemoteServiceFail))
}

func TestCreateOpenClaw_UnmarshalError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	resp, err := client.CreateOpenClaw(context.Background(), "test-uuid", "testuser", "test-token", CreateOpenClawRequest{})

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestDeleteOpenClaw_Success(t *testing.T) {
	userUUID := "test-user-uuid"
	username := "test-username"
	token := "test-token"
	contentID := "oc-12345"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/api/v1/csgbot/openclaw/" + contentID
		assert.Equal(t, expectedPath, r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, userUUID, r.Header.Get(types.CSGBotHeaderUserUUID))
		assert.Equal(t, username, r.Header.Get(types.CSGBotHeaderUserName))
		assert.Equal(t, token, r.Header.Get(types.CSGBotHeaderUserToken))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	err := client.DeleteOpenClaw(context.Background(), userUUID, username, token, contentID)

	assert.NoError(t, err)
}

func TestDeleteOpenClaw_FailureWithError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success": false, "error": "sandbox not found"}`))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	err := client.DeleteOpenClaw(context.Background(), "test-uuid", "testuser", "test-token", "oc-12345")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sandbox not found")
}

func TestDeleteOpenClaw_FailureNoError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success": false}`))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	err := client.DeleteOpenClaw(context.Background(), "test-uuid", "testuser", "test-token", "oc-12345")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown error")
}

func TestDeleteOpenClaw_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	err := client.DeleteOpenClaw(context.Background(), "test-uuid", "testuser", "test-token", "oc-12345")

	assert.Error(t, err)
	assert.True(t, errors.Is(err, errorx.ErrRemoteServiceFail))
}

func TestDeleteOpenClaw_UnmarshalError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	client := setupTestCsgbotClient(server)

	err := client.DeleteOpenClaw(context.Background(), "test-uuid", "testuser", "test-token", "oc-12345")

	assert.Error(t, err)
}

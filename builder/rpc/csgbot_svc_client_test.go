package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
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

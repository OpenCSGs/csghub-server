package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestLangflowAdapter_Name(t *testing.T) {
	config := &config.Config{}
	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	adapter := NewLangflowAdapter(config, mockAgentComponent)

	assert.Equal(t, "langflow", adapter.Name())
}

func TestLangflowAdapter_GetHost_Comprehensive(t *testing.T) {
	t.Run("success with trailing slash", func(t *testing.T) {
		config := &config.Config{}
		config.Agent.AgentHubServiceHost = "http://localhost:8080/"
		mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
		adapter := NewLangflowAdapter(config, mockAgentComponent)

		host, err := adapter.GetHost(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "http://localhost:8080", host)
	})

	t.Run("success without trailing slash", func(t *testing.T) {
		config := &config.Config{}
		config.Agent.AgentHubServiceHost = "http://localhost:8080"
		mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
		adapter := NewLangflowAdapter(config, mockAgentComponent)

		host, err := adapter.GetHost(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "http://localhost:8080", host)
	})

	t.Run("success with multiple trailing slashes", func(t *testing.T) {
		config := &config.Config{}
		config.Agent.AgentHubServiceHost = "http://localhost:8080///"
		mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
		adapter := NewLangflowAdapter(config, mockAgentComponent)

		host, err := adapter.GetHost(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "http://localhost:8080//", host)
	})
}

func TestLangflowAdapter_PrepareProxyContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("non-POST request", func(t *testing.T) {
		config := &config.Config{}
		config.Agent.AgentHubServiceToken = "test-token"
		mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
		adapter := NewLangflowAdapter(config, mockAgentComponent)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest("GET", "http://localhost/api/v1/opencsg/run/flow-123", nil)
		ctx.Request = req
		ctx.Set("currentUserUUID", "test-user-uuid")

		err := adapter.PrepareProxyContext(ctx, "/api/v1/opencsg/run/flow-123")
		assert.NoError(t, err)

		// Should set token and user_uuid header
		assert.Contains(t, ctx.Request.URL.RawQuery, "token=test-token")
		assert.Equal(t, "test-user-uuid", ctx.Request.Header.Get("user_uuid"))
	})

	t.Run("POST request to non-run endpoint", func(t *testing.T) {
		config := &config.Config{}
		config.Agent.AgentHubServiceToken = "test-token"
		mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
		adapter := NewLangflowAdapter(config, mockAgentComponent)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest("POST", "http://localhost/api/v1/flows", nil)
		ctx.Request = req
		ctx.Set("currentUserUUID", "test-user-uuid")

		err := adapter.PrepareProxyContext(ctx, "/api/v1/flows")
		assert.NoError(t, err)

		// Should set token and user_uuid header
		assert.Contains(t, ctx.Request.URL.RawQuery, "token=test-token")
		assert.Equal(t, "test-user-uuid", ctx.Request.Header.Get("user_uuid"))
	})

	t.Run("POST request to run endpoint without stream", func(t *testing.T) {
		config := &config.Config{}
		config.Agent.AgentHubServiceToken = "test-token"
		mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
		adapter := NewLangflowAdapter(config, mockAgentComponent)

		// Setup request with valid JSON body
		chatReq := types.LangflowChatRequest{
			InputValue: "test message",
			InputType:  "chat",
			OutputType: "chat",
		}
		jsonData, _ := json.Marshal(chatReq)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest("POST", "http://localhost/api/v1/opencsg/run/flow-123", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		ctx.Request = req
		ctx.Set("currentUserUUID", "test-user-uuid")

		// Mock session creation
		sessionUUID := "test-session-uuid"
		mockAgentComponent.On("CreateSession", mock.Anything, "test-user-uuid", mock.AnythingOfType("*types.CreateAgentInstanceSessionRequest")).Return(sessionUUID, nil)

		err := adapter.PrepareProxyContext(ctx, "/api/v1/opencsg/run/flow-123")
		assert.NoError(t, err)

		// Should set token and user_uuid header
		assert.Contains(t, ctx.Request.URL.RawQuery, "token=test-token")
		assert.Equal(t, "test-user-uuid", ctx.Request.Header.Get("user_uuid"))

		// Should not set stream headers
		assert.Empty(t, ctx.Writer.Header().Get("Cache-Control"))
		assert.Empty(t, ctx.Writer.Header().Get("Connection"))

		mockAgentComponent.AssertExpectations(t)
	})

	t.Run("POST request to run endpoint with stream", func(t *testing.T) {
		config := &config.Config{}
		config.Agent.AgentHubServiceToken = "test-token"
		mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
		adapter := NewLangflowAdapter(config, mockAgentComponent)

		// Setup request with valid JSON body and stream=true
		chatReq := types.LangflowChatRequest{
			InputValue: "test message",
			InputType:  "chat",
			OutputType: "chat",
		}
		jsonData, _ := json.Marshal(chatReq)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest("POST", "http://localhost/api/v1/opencsg/run/flow-123?stream=true", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		ctx.Request = req
		ctx.Set("currentUserUUID", "test-user-uuid")

		// Mock session creation
		sessionUUID := "test-session-uuid"
		mockAgentComponent.On("CreateSession", mock.Anything, "test-user-uuid", mock.AnythingOfType("*types.CreateAgentInstanceSessionRequest")).Return(sessionUUID, nil)

		err := adapter.PrepareProxyContext(ctx, "/api/v1/opencsg/run/flow-123")
		assert.NoError(t, err)

		// Should set token and user_uuid header
		assert.Contains(t, ctx.Request.URL.RawQuery, "token=test-token")
		assert.Equal(t, "test-user-uuid", ctx.Request.Header.Get("user_uuid"))

		// Should set stream headers
		assert.Equal(t, "no-cache", ctx.Writer.Header().Get("Cache-Control"))
		assert.Equal(t, "keep-alive", ctx.Writer.Header().Get("Connection"))

		mockAgentComponent.AssertExpectations(t)
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		config := &config.Config{}
		config.Agent.AgentHubServiceToken = "test-token"
		mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
		adapter := NewLangflowAdapter(config, mockAgentComponent)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest("POST", "http://localhost/api/v1/opencsg/run/flow-123", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		ctx.Request = req
		ctx.Set("currentUserUUID", "test-user-uuid")

		err := adapter.PrepareProxyContext(ctx, "/api/v1/opencsg/run/flow-123")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parse request body of run flow request")
	})

	t.Run("session creation failure", func(t *testing.T) {
		config := &config.Config{}
		config.Agent.AgentHubServiceToken = "test-token"
		mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
		adapter := NewLangflowAdapter(config, mockAgentComponent)

		// Setup request with valid JSON body
		chatReq := types.LangflowChatRequest{
			InputValue: "test message",
			InputType:  "chat",
			OutputType: "chat",
		}
		jsonData, _ := json.Marshal(chatReq)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest("POST", "http://localhost/api/v1/opencsg/run/flow-123", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		ctx.Request = req
		ctx.Set("currentUserUUID", "test-user-uuid")

		// Mock session creation failure
		mockAgentComponent.On("CreateSession", mock.Anything, "test-user-uuid", mock.AnythingOfType("*types.CreateAgentInstanceSessionRequest")).Return("", assert.AnError)

		err := adapter.PrepareProxyContext(ctx, "/api/v1/opencsg/run/flow-123")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "create langflow agent session")

		mockAgentComponent.AssertExpectations(t)
	})

	t.Run("with existing session ID", func(t *testing.T) {
		config := &config.Config{}
		config.Agent.AgentHubServiceToken = "test-token"
		mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
		adapter := NewLangflowAdapter(config, mockAgentComponent)

		// Setup request with existing session ID
		existingSessionID := "existing-session-id"
		chatReq := types.LangflowChatRequest{
			InputValue: "test message",
			InputType:  "chat",
			OutputType: "chat",
			SessionID:  &existingSessionID,
		}
		jsonData, _ := json.Marshal(chatReq)

		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest("POST", "http://localhost/api/v1/opencsg/run/flow-123", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		ctx.Request = req
		ctx.Set("currentUserUUID", "test-user-uuid")

		// Mock session creation
		sessionUUID := "test-session-uuid"
		mockAgentComponent.On("CreateSession", mock.Anything, "test-user-uuid", mock.AnythingOfType("*types.CreateAgentInstanceSessionRequest")).Return(sessionUUID, nil)

		err := adapter.PrepareProxyContext(ctx, "/api/v1/opencsg/run/flow-123")
		assert.NoError(t, err)

		// Verify the request body was updated with the new session UUID
		bodyBytes, err := io.ReadAll(ctx.Request.Body)
		assert.NoError(t, err)
		var updatedReq types.LangflowChatRequest
		err = json.Unmarshal(bodyBytes, &updatedReq)
		assert.NoError(t, err)
		assert.Equal(t, sessionUUID, *updatedReq.SessionID)

		mockAgentComponent.AssertExpectations(t)
	})
}

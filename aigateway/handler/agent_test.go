package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestNewAgentProxyHandler(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: func() *config.Config {
				cfg := &config.Config{}
				cfg.Agent.AgentHubServiceHost = "http://localhost:8080"
				cfg.Agent.AgentHubServiceToken = "test-token"
				return cfg
			}(),
			wantErr: false,
		},
		{
			name: "empty host",
			config: func() *config.Config {
				cfg := &config.Config{}
				cfg.Agent.AgentHubServiceHost = ""
				cfg.Agent.AgentHubServiceToken = "test-token"
				return cfg
			}(),
			wantErr: false, // Current implementation doesn't validate empty host
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip this test as it requires real component creation with NATS
			// In a real test environment, you would mock the component creation
			t.Skip("Skipping test that requires real component creation with NATS connection")

			handler, err := NewAgentProxyHandler(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, handler)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, handler)
				assert.IsType(t, &AgentProxyHandlerImpl{}, handler)
			}
		})
	}
}

func TestAgentProxyHandlerImpl_ProxyToApi_Basic(t *testing.T) {
	// Test basic functionality with langflow adapter
	config := &config.Config{}
	config.Agent.AgentHubServiceHost = "http://localhost:8080"
	config.Agent.AgentHubServiceToken = "test-token"

	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	handler := &AgentProxyHandlerImpl{
		config: config,
		adapters: map[string]AgentAdapter{
			"langflow": NewLangflowAdapter(config, mockAgentComponent),
		},
	}

	// Setup request
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("GET", "http://localhost/api/v1/test", nil)
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "type", Value: "langflow"}}
	ctx.Set("currentUserUUID", "test-user-uuid")

	// Execute handler
	handlerFunc := handler.ProxyToApi("/api/v1/test")
	handlerFunc(ctx)

	// Verify user UUID header was set
	assert.Equal(t, "test-user-uuid", ctx.Request.Header.Get("user_uuid"))

	// Verify token was added to query
	assert.Contains(t, ctx.Request.URL.RawQuery, "token=test-token")

	mockAgentComponent.AssertExpectations(t)
}

func TestAgentProxyHandlerImpl_ProxyToApi_CodeAdapter(t *testing.T) {
	// Skip this test as it requires real user service HTTP calls
	// In a real test environment, you would mock the user service client
	t.Skip("Skipping test that requires real user service HTTP calls")
}

func TestAgentProxyHandlerImpl_ProxyToApi_StreamHeaders(t *testing.T) {
	// Test stream headers are set correctly for langflow run flow request
	config := &config.Config{}
	config.Agent.AgentHubServiceHost = "http://localhost:8080"
	config.Agent.AgentHubServiceToken = "test-token"

	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	handler := &AgentProxyHandlerImpl{
		config: config,
		adapters: map[string]AgentAdapter{
			"langflow": NewLangflowAdapter(config, mockAgentComponent),
		},
	}

	// Setup POST request to run flow endpoint with stream=true
	chatReq := types.LangflowChatRequest{
		InputValue: "test message",
		InputType:  "chat",
		OutputType: "chat",
	}
	jsonData, _ := json.Marshal(chatReq)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("POST", "http://localhost/api/v1/opencsg/run/flow-id?stream=true", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "type", Value: "langflow"}}
	ctx.Set("currentUserUUID", "test-user-uuid")

	// Mock session creation
	sessionUUID := "test-session-uuid"
	mockAgentComponent.On("CreateSession", mock.Anything, "test-user-uuid", mock.AnythingOfType("*types.CreateAgentInstanceSessionRequest")).Return(sessionUUID, nil)

	handlerFunc := handler.ProxyToApi("/api/v1/opencsg/run/flow-id")
	handlerFunc(ctx)

	// Verify stream headers are set by the adapter
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))

	mockAgentComponent.AssertExpectations(t)
}

func TestAgentProxyHandlerImpl_ProxyToApi_WithPathParams(t *testing.T) {
	// Test path parameter substitution
	config := &config.Config{}
	config.Agent.AgentHubServiceHost = "http://localhost:8080"
	config.Agent.AgentHubServiceToken = "test-token"

	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	handler := &AgentProxyHandlerImpl{
		config: config,
		adapters: map[string]AgentAdapter{
			"langflow": NewLangflowAdapter(config, mockAgentComponent),
		},
	}

	// Setup request with path parameters
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("GET", "http://localhost/api/v1/flows/123/run", nil)
	ctx.Request = req
	ctx.Params = gin.Params{
		{Key: "type", Value: "langflow"},
		{Key: "id", Value: "123"},
		{Key: "action", Value: "run"},
	}
	ctx.Set("currentUserUUID", "test-user-uuid")

	handlerFunc := handler.ProxyToApi("/api/v1/flows/%s/%s", "id", "action")
	handlerFunc(ctx)

	// Verify path was formatted correctly
	assert.Equal(t, "test-user-uuid", ctx.Request.Header.Get("user_uuid"))
	assert.Contains(t, ctx.Request.URL.RawQuery, "token=test-token")

	mockAgentComponent.AssertExpectations(t)
}

func TestAgentProxyHandlerImpl_ProxyToApi_QueryParameters(t *testing.T) {
	// Test that token is added to query parameters
	config := &config.Config{}
	config.Agent.AgentHubServiceHost = "http://localhost:8080"
	config.Agent.AgentHubServiceToken = "test-token"

	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	handler := &AgentProxyHandlerImpl{
		config: config,
		adapters: map[string]AgentAdapter{
			"langflow": NewLangflowAdapter(config, mockAgentComponent),
		},
	}

	// Setup request with existing query parameters
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("GET", "http://localhost/api/v1/test?param1=value1&param2=value2", nil)
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "type", Value: "langflow"}}
	ctx.Set("currentUserUUID", "test-user-uuid")

	handlerFunc := handler.ProxyToApi("/api/v1/test")
	handlerFunc(ctx)

	// Verify that token was added to existing query parameters
	parsedQuery, _ := url.ParseQuery(ctx.Request.URL.RawQuery)
	assert.Equal(t, "test-token", parsedQuery.Get("token"))
	assert.Equal(t, "value1", parsedQuery.Get("param1"))
	assert.Equal(t, "value2", parsedQuery.Get("param2"))

	mockAgentComponent.AssertExpectations(t)
}

func TestAgentProxyHandlerImpl_ProxyToApi_UnsupportedAgentType(t *testing.T) {
	// Test unsupported agent type
	config := &config.Config{}
	config.Agent.AgentHubServiceHost = "http://localhost:8080"
	config.Agent.AgentHubServiceToken = "test-token"

	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	handler := &AgentProxyHandlerImpl{
		config: config,
		adapters: map[string]AgentAdapter{
			"langflow": NewLangflowAdapter(config, mockAgentComponent),
		},
	}

	// Setup request with unsupported agent type
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("GET", "http://localhost/api/v1/test", nil)
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "type", Value: "unsupported"}}
	ctx.Set("currentUserUUID", "test-user-uuid")

	handlerFunc := handler.ProxyToApi("/api/v1/test")
	handlerFunc(ctx)

	// Verify error response
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "unsupported agent type: unsupported")

	mockAgentComponent.AssertExpectations(t)
}

// Test LangflowAdapter methods
func TestLangflowAdapter_GetHost(t *testing.T) {
	config := &config.Config{}
	config.Agent.AgentHubServiceHost = "http://localhost:8080"
	config.Agent.AgentHubServiceToken = "test-token"

	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	adapter := NewLangflowAdapter(config, mockAgentComponent)

	host, err := adapter.GetHost(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:8080", host)

	// Test with trailing slash
	config.Agent.AgentHubServiceHost = "http://localhost:8080/"
	adapter = NewLangflowAdapter(config, mockAgentComponent)
	host, err = adapter.GetHost(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:8080", host)
}

func TestLangflowAdapter_PrepareResponseWriter_NonRunFlow(t *testing.T) {
	config := &config.Config{}
	config.Agent.AgentHubServiceHost = "http://localhost:8080"
	config.Agent.AgentHubServiceToken = "test-token"

	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	adapter := NewLangflowAdapter(config, mockAgentComponent)

	// Setup non-run flow request
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("GET", "http://localhost/api/v1/flows", nil)
	ctx.Request = req
	ctx.Set("currentUserUUID", "test-user-uuid")

	err := adapter.PrepareProxyContext(ctx, "/api/v1/flows")
	assert.NoError(t, err)

	// Verify headers and query params were set
	assert.Equal(t, "test-user-uuid", ctx.Request.Header.Get("user_uuid"))
	assert.Contains(t, ctx.Request.URL.RawQuery, "token=test-token")

	mockAgentComponent.AssertExpectations(t)
}

func TestLangflowAdapter_PrepareResponseWriter_RunFlow(t *testing.T) {
	config := &config.Config{}
	config.Agent.AgentHubServiceHost = "http://localhost:8080"
	config.Agent.AgentHubServiceToken = "test-token"

	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	adapter := NewLangflowAdapter(config, mockAgentComponent)

	// Setup run flow request with proper JSON body
	chatReq := types.LangflowChatRequest{
		InputValue: "test message",
		InputType:  "chat",
		OutputType: "chat",
	}
	jsonData, _ := json.Marshal(chatReq)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("POST", "http://localhost/api/v1/opencsg/run/flow-id", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	ctx.Set("currentUserUUID", "test-user-uuid")

	// Mock session creation
	sessionUUID := "test-session-uuid"
	mockAgentComponent.On("CreateSession", mock.Anything, "test-user-uuid", mock.AnythingOfType("*types.CreateAgentInstanceSessionRequest")).Return(sessionUUID, nil)

	err := adapter.PrepareProxyContext(ctx, "/api/v1/opencsg/run/flow-id")
	assert.NoError(t, err)

	// Verify headers and query params were set
	assert.Equal(t, "test-user-uuid", ctx.Request.Header.Get("user_uuid"))
	assert.Contains(t, ctx.Request.URL.RawQuery, "token=test-token")

	mockAgentComponent.AssertExpectations(t)
}

// Test CodeAdapter methods
func TestCodeAdapter_GetHost(t *testing.T) {
	config := &config.Config{}
	config.CSGBot.Host = "localhost"
	config.CSGBot.Port = 8080

	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	adapter := NewCodeAdapter(config, mockAgentComponent)

	host, err := adapter.GetHost(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "localhost:8080", host)
}

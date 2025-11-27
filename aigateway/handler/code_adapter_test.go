package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/common/types"
)

func TestCodeAgentRequest_JSONBinding(t *testing.T) {
	// Test JSON that matches the provided example
	jsonData := `{
		"request_id": "e6516e77a4c3asdasdsa1sssadasdasd123",
		"query": "你好",
		"max_loop": 1,
		"search_engines": [],
		"stream": true,
		"agent_name": "FinanceCodeMaster",
		"stream_mode": {
			"mode": "general",
			"token": 5,
			"time": 5
		}
	}`

	var req types.CodeAgentRequest
	err := json.Unmarshal([]byte(jsonData), &req)
	assert.NoError(t, err)
	assert.Equal(t, "e6516e77a4c3asdasdsa1sssadasdasd123", req.RequestID)
	assert.Equal(t, "你好", req.Query)
	assert.Equal(t, 1, req.MaxLoop)
	assert.Equal(t, true, req.Stream)
	assert.Equal(t, "FinanceCodeMaster", req.AgentName)
	assert.NotNil(t, req.StreamMode)
	assert.Equal(t, "general", req.StreamMode.Mode)
	assert.Equal(t, 5, req.StreamMode.Token)
	assert.Equal(t, 5, req.StreamMode.Time)
}

func TestCodeAgentRequest_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a test request with missing required fields
	jsonData := `{
		"query": "",
		"max_loop": -1
	}`

	req, _ := http.NewRequest("POST", "/test", bytes.NewBufferString(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	var codeReq types.CodeAgentRequest
	err := c.ShouldBindJSON(&codeReq)

	// Should have validation errors for required fields and min values
	assert.Error(t, err)
}

func TestCodeAgentRequest_GinBinding_UserExample(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Test the user's provided example with Gin binding
	jsonData := `{
		"request_id": "e6516e77a4c3asd",
		"query": "你好",
		"stream": true,
		"agent_name": "FinanceCodeMaster"
	}`

	req, _ := http.NewRequest("POST", "/test", bytes.NewBufferString(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	var codeReq types.CodeAgentRequest
	err := c.ShouldBindJSON(&codeReq)

	// Should NOT have validation errors for the user's example
	assert.NoError(t, err)
	assert.Equal(t, "e6516e77a4c3asd", codeReq.RequestID)
	assert.Equal(t, "你好", codeReq.Query)
	assert.Equal(t, 0, codeReq.MaxLoop) // Should be 0 when not provided
	assert.Equal(t, true, codeReq.Stream)
	assert.Equal(t, "FinanceCodeMaster", codeReq.AgentName)
	assert.Nil(t, codeReq.StreamMode)
}

func TestCodeAgentRequest_MinimalValid(t *testing.T) {
	// Test minimal valid request
	jsonData := `{
		"request_id": "test-123",
		"query": "test query",
		"max_loop": 1,
		"stream": false,
		"agent_name": "TestAgent"
	}`

	var req types.CodeAgentRequest
	err := json.Unmarshal([]byte(jsonData), &req)
	assert.NoError(t, err)
	assert.Equal(t, "test-123", req.RequestID)
	assert.Equal(t, "test query", req.Query)
	assert.Equal(t, 1, req.MaxLoop)
	assert.Equal(t, false, req.Stream)
	assert.Equal(t, "TestAgent", req.AgentName)
	assert.Nil(t, req.StreamMode) // Should be nil when not provided
}

func TestCodeAgentRequest_UserProvidedExample(t *testing.T) {
	// Test the user's provided example (with syntax fixed)
	jsonData := `{
		"request_id": "e6516e77a4c3asd",
		"query": "你好",
		"stream": true,
		"agent_name": "FinanceCodeMaster"
	}`

	var req types.CodeAgentRequest
	err := json.Unmarshal([]byte(jsonData), &req)
	assert.NoError(t, err)
	assert.Equal(t, "e6516e77a4c3asd", req.RequestID)
	assert.Equal(t, "你好", req.Query)
	assert.Equal(t, 0, req.MaxLoop) // Should be 0 when not provided
	assert.Equal(t, true, req.Stream)
	assert.Equal(t, "FinanceCodeMaster", req.AgentName)
	assert.Nil(t, req.StreamMode) // Should be nil when not provided
}

func TestCodeAgentRequest_WithSearchEngines(t *testing.T) {
	// Test request with search engines
	jsonData := `{
		"request_id": "test-123",
		"query": "test query",
		"max_loop": 1,
		"search_engines": ["google", "bing", "duckduckgo"],
		"stream": false,
		"agent_name": "TestAgent"
	}`

	var req types.CodeAgentRequest
	err := json.Unmarshal([]byte(jsonData), &req)
	assert.NoError(t, err)
	assert.Equal(t, "test-123", req.RequestID)
	assert.Equal(t, "test query", req.Query)
	assert.Equal(t, 1, req.MaxLoop)
	assert.Equal(t, []string{"google", "bing", "duckduckgo"}, req.SearchEngines)
	assert.Equal(t, false, req.Stream)
	assert.Equal(t, "TestAgent", req.AgentName)
	assert.Nil(t, req.StreamMode)
}

func TestCodeAgentRequest_WithHistory(t *testing.T) {
	// Test request with conversation history
	jsonData := `{
		"request_id": "test-123",
		"query": "test query",
		"max_loop": 1,
		"stream": false,
		"agent_name": "TestAgent",
		"history": [
			{
				"role": "user",
				"content": "Hello"
			},
			{
				"role": "assistant", 
				"content": "Hi there!"
			}
		]
	}`

	var req types.CodeAgentRequest
	err := json.Unmarshal([]byte(jsonData), &req)
	assert.NoError(t, err)
	assert.Equal(t, "test-123", req.RequestID)
	assert.Equal(t, "test query", req.Query)
	assert.Equal(t, 1, req.MaxLoop)
	assert.Equal(t, false, req.Stream)
	assert.Equal(t, "TestAgent", req.AgentName)
	assert.Len(t, req.History, 2)
	assert.Equal(t, "user", req.History[0].Role)
	assert.Equal(t, "Hello", req.History[0].Content)
	assert.Equal(t, "assistant", req.History[1].Role)
	assert.Equal(t, "Hi there!", req.History[1].Content)
	assert.Nil(t, req.StreamMode)
}

func TestCodeAgentRequest_StreamModeValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("valid stream mode", func(t *testing.T) {
		jsonData := `{
			"query": "test query",
			"agent_name": "TestAgent",
			"stream_mode": {
				"mode": "general",
				"token": 5,
				"time": 10
			}
		}`

		req, _ := http.NewRequest("POST", "/test", bytes.NewBufferString(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		var codeReq types.CodeAgentRequest
		err := c.ShouldBindJSON(&codeReq)

		assert.NoError(t, err)
		assert.NotNil(t, codeReq.StreamMode)
		assert.Equal(t, "general", codeReq.StreamMode.Mode)
		assert.Equal(t, 5, codeReq.StreamMode.Token)
		assert.Equal(t, 10, codeReq.StreamMode.Time)
	})

	t.Run("invalid stream mode - missing required mode", func(t *testing.T) {
		jsonData := `{
			"query": "test query",
			"agent_name": "TestAgent",
			"stream_mode": {
				"token": 5,
				"time": 10
			}
		}`

		req, _ := http.NewRequest("POST", "/test", bytes.NewBufferString(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		var codeReq types.CodeAgentRequest
		err := c.ShouldBindJSON(&codeReq)

		assert.Error(t, err)
	})

	t.Run("invalid stream mode - token too small", func(t *testing.T) {
		jsonData := `{
			"query": "test query",
			"agent_name": "TestAgent",
			"stream_mode": {
				"mode": "general",
				"token": 0,
				"time": 10
			}
		}`

		req, _ := http.NewRequest("POST", "/test", bytes.NewBufferString(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		var codeReq types.CodeAgentRequest
		err := c.ShouldBindJSON(&codeReq)

		assert.Error(t, err)
	})

	t.Run("invalid stream mode - time too small", func(t *testing.T) {
		jsonData := `{
			"query": "test query",
			"agent_name": "TestAgent",
			"stream_mode": {
				"mode": "general",
				"token": 5,
				"time": 0
			}
		}`

		req, _ := http.NewRequest("POST", "/test", bytes.NewBufferString(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		var codeReq types.CodeAgentRequest
		err := c.ShouldBindJSON(&codeReq)

		assert.Error(t, err)
	})
}

func TestCodeAgentRequest_EdgeCases(t *testing.T) {
	t.Run("empty search engines", func(t *testing.T) {
		jsonData := `{
			"query": "test query",
			"agent_name": "TestAgent",
			"search_engines": []
		}`

		var req types.CodeAgentRequest
		err := json.Unmarshal([]byte(jsonData), &req)
		assert.NoError(t, err)
		assert.Empty(t, req.SearchEngines)
	})

	t.Run("empty history", func(t *testing.T) {
		jsonData := `{
			"query": "test query",
			"agent_name": "TestAgent",
			"history": []
		}`

		var req types.CodeAgentRequest
		err := json.Unmarshal([]byte(jsonData), &req)
		assert.NoError(t, err)
		assert.Empty(t, req.History)
	})

	t.Run("max loop boundary values", func(t *testing.T) {
		gin.SetMode(gin.TestMode)

		// Test max_loop = 1 (minimum valid value)
		jsonData := `{
			"query": "test query",
			"agent_name": "TestAgent",
			"max_loop": 1
		}`

		req, _ := http.NewRequest("POST", "/test", bytes.NewBufferString(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		var codeReq types.CodeAgentRequest
		err := c.ShouldBindJSON(&codeReq)

		assert.NoError(t, err)
		assert.Equal(t, 1, codeReq.MaxLoop)
	})

	t.Run("max loop zero (should be valid due to omitempty)", func(t *testing.T) {
		jsonData := `{
			"query": "test query",
			"agent_name": "TestAgent",
			"max_loop": 0
		}`

		var req types.CodeAgentRequest
		err := json.Unmarshal([]byte(jsonData), &req)
		assert.NoError(t, err)
		assert.Equal(t, 0, req.MaxLoop)
	})
}

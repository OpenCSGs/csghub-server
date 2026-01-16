package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mock_sensitive "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/moderation/component"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/types"
)

func TestSensitiveHandler_Image(t *testing.T) {

	mockSensitiveComponent := mock_sensitive.NewMockSensitiveComponent(t)
	handler := &SensitiveHandler{
		c: mockSensitiveComponent,
	}

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.POST("/api/v1/moderation/image", handler.Image)

	// Create a test CheckResult
	successResult := &sensitive.CheckResult{
		IsSensitive: false,
		Reason:      "",
	}

	t.Run("success with image_url", func(t *testing.T) {
		// Prepare request body
		reqBody := map[string]interface{}{
			"scenario":  "profilePhotoCheck",
			"image_url": "https://example.com/image.jpg",
		}
		reqBodyBytes, _ := json.Marshal(reqBody)

		// Set mock expectation
		mockSensitiveComponent.EXPECT().PassImageURLCheck(
			mock.Anything,
			types.ScenarioImageProfileCheck,
			"https://example.com/image.jpg",
		).Return(successResult, nil).Once()

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/image", bytes.NewBuffer(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		var response struct {
			Data *sensitive.CheckResult `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.False(t, response.Data.IsSensitive)
	})

	t.Run("success with oss_bucket_name and oss_object_name", func(t *testing.T) {
		// Prepare request body
		reqBody := map[string]interface{}{
			"scenario":        "baselineCheck",
			"oss_bucket_name": "test-bucket",
			"oss_object_name": "test-object.jpg",
		}
		reqBodyBytes, _ := json.Marshal(reqBody)

		// Set mock expectation
		mockSensitiveComponent.EXPECT().PassImageCheck(
			mock.Anything,
			types.ScenarioImageBaseLineCheck,
			"test-bucket",
			"test-object.jpg",
		).Return(successResult, nil).Once()

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/image", bytes.NewBuffer(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		var response struct {
			Data *sensitive.CheckResult `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.False(t, response.Data.IsSensitive)
	})

	t.Run("bad request missing both image_url and oss parameters", func(t *testing.T) {
		// Prepare request body (missing required parameters)
		reqBody := map[string]interface{}{
			"scenario": "profilePhotoCheck",
		}
		reqBodyBytes, _ := json.Marshal(reqBody)

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/image", bytes.NewBuffer(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("bad request invalid json format", func(t *testing.T) {
		// Prepare request body (invalid JSON format)
		invalidJSON := `{"scenario": "profilePhotoCheck", "image_url": "https://example.com/image.jpg",}`

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/image", bytes.NewBufferString(invalidJSON))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("server error from sensitive component", func(t *testing.T) {
		// Prepare request body
		reqBody := map[string]interface{}{
			"scenario":  "profilePhotoCheck",
			"image_url": "https://example.com/image.jpg",
		}
		reqBodyBytes, _ := json.Marshal(reqBody)

		// Set mock expectation to return error
		expectedErr := assert.AnError
		mockSensitiveComponent.EXPECT().PassImageURLCheck(
			mock.Anything,
			types.ScenarioImageProfileCheck,
			"https://example.com/image.jpg",
		).Return(nil, expectedErr).Once()

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/image", bytes.NewBuffer(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("sensitive content detected", func(t *testing.T) {
		// Prepare request body
		reqBody := map[string]interface{}{
			"scenario":  "profilePhotoCheck",
			"image_url": "https://example.com/sensitive.jpg",
		}
		reqBodyBytes, _ := json.Marshal(reqBody)

		// Create result for sensitive content
		sensitiveResult := &sensitive.CheckResult{
			IsSensitive: true,
			Reason:      "contains sensitive content",
		}

		// Set mock expectation
		mockSensitiveComponent.EXPECT().PassImageURLCheck(
			mock.Anything,
			types.ScenarioImageProfileCheck,
			"https://example.com/sensitive.jpg",
		).Return(sensitiveResult, nil).Once()

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/image", bytes.NewBuffer(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		var response struct {
			Data *sensitive.CheckResult `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response.Data.IsSensitive)
		assert.Equal(t, "contains sensitive content", response.Data.Reason)
	})
}

func TestSensitiveHandler_LlmResp(t *testing.T) {
	mockSensitiveComponent := mock_sensitive.NewMockSensitiveComponent(t)
	handler := &SensitiveHandler{
		c: mockSensitiveComponent,
	}

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.POST("/api/v1/moderation/llm_resp", handler.LlmResp)

	// Create a test CheckResult
	successResult := &sensitive.CheckResult{
		IsSensitive: false,
		Reason:      "",
	}

	t.Run("success", func(t *testing.T) {
		// Prepare request body
		reqBody := map[string]interface{}{
			"Service": "LLMResponseModeration",
			"ServiceParameters": map[string]interface{}{
				"content":   "This is a safe response",
				"sessionId": "test-session-123",
			},
		}
		reqBodyBytes, _ := json.Marshal(reqBody)

		// Set mock expectation
		mockSensitiveComponent.EXPECT().PassStreamCheck(
			mock.Anything,
			types.ScenarioLLMResModeration,
			"This is a safe response",
			"test-session-123",
		).Return(successResult, nil).Once()

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/llm_resp", bytes.NewBuffer(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		var response struct {
			Data *sensitive.CheckResult `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.False(t, response.Data.IsSensitive)
	})

	t.Run("bad request invalid json format", func(t *testing.T) {
		// Prepare request body (invalid JSON format)
		invalidJSON := `{"Service": "LLMResponseModeration", "ServiceParameters": {"content": "test", "sessionId": "test-session"},}`

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/llm_resp", bytes.NewBufferString(invalidJSON))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("server error from sensitive component", func(t *testing.T) {
		// Prepare request body
		reqBody := map[string]interface{}{
			"Service": "LLMResponseModeration",
			"ServiceParameters": map[string]interface{}{
				"content":   "This is a test response",
				"sessionId": "test-session-123",
			},
		}
		reqBodyBytes, _ := json.Marshal(reqBody)

		// Set mock expectation to return error
		expectedErr := assert.AnError
		mockSensitiveComponent.EXPECT().PassStreamCheck(
			mock.Anything,
			types.ScenarioLLMResModeration,
			"This is a test response",
			"test-session-123",
		).Return(nil, expectedErr).Once()

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/llm_resp", bytes.NewBuffer(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("sensitive content detected", func(t *testing.T) {
		// Prepare request body
		reqBody := map[string]interface{}{
			"Service": "LLMResponseModeration",
			"ServiceParameters": map[string]interface{}{
				"content":   "This is sensitive content",
				"sessionId": "test-session-123",
			},
		}
		reqBodyBytes, _ := json.Marshal(reqBody)

		// Create result for sensitive content
		sensitiveResult := &sensitive.CheckResult{
			IsSensitive: true,
			Reason:      "contains sensitive content",
		}

		// Set mock expectation
		mockSensitiveComponent.EXPECT().PassStreamCheck(
			mock.Anything,
			types.ScenarioLLMResModeration,
			"This is sensitive content",
			"test-session-123",
		).Return(sensitiveResult, nil).Once()

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/llm_resp", bytes.NewBuffer(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		var response struct {
			Data *sensitive.CheckResult `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response.Data.IsSensitive)
		assert.Equal(t, "contains sensitive content", response.Data.Reason)
	})
}

func TestSensitiveHandler_LlmPrompt(t *testing.T) {
	mockSensitiveComponent := mock_sensitive.NewMockSensitiveComponent(t)
	handler := &SensitiveHandler{
		c: mockSensitiveComponent,
	}

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.POST("/api/v1/moderation/llm_prompt", handler.LlmPrompt)

	// Create a test CheckResult
	successResult := &sensitive.CheckResult{
		IsSensitive: false,
		Reason:      "",
	}

	t.Run("success", func(t *testing.T) {
		// Prepare request body
		reqBody := map[string]interface{}{
			"Service": "LLMPromptModeration",
			"ServiceParameters": map[string]interface{}{
				"content":   "This is a safe prompt",
				"accountId": "test-account-123",
			},
		}
		reqBodyBytes, _ := json.Marshal(reqBody)

		// Set mock expectation
		mockSensitiveComponent.EXPECT().PassLLMQueryCheck(
			mock.Anything,
			types.ScenarioLLMQueryModeration,
			"This is a safe prompt",
			"test-account-123",
		).Return(successResult, nil).Once()

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/llm_prompt", bytes.NewBuffer(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		var response struct {
			Data *sensitive.CheckResult `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.False(t, response.Data.IsSensitive)
	})

	t.Run("bad request invalid json format", func(t *testing.T) {
		// Prepare request body (invalid JSON format)
		invalidJSON := `{"Service": "LLMPromptModeration", "ServiceParameters": {"content": "test", "accountId": "test-account"},}`

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/llm_prompt", bytes.NewBufferString(invalidJSON))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("server error from sensitive component", func(t *testing.T) {
		// Prepare request body
		reqBody := map[string]interface{}{
			"Service": "LLMPromptModeration",
			"ServiceParameters": map[string]interface{}{
				"content":   "This is a test prompt",
				"accountId": "test-account-123",
			},
		}
		reqBodyBytes, _ := json.Marshal(reqBody)

		// Set mock expectation to return error
		expectedErr := assert.AnError
		mockSensitiveComponent.EXPECT().PassLLMQueryCheck(
			mock.Anything,
			types.ScenarioLLMQueryModeration,
			"This is a test prompt",
			"test-account-123",
		).Return(nil, expectedErr).Once()

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/llm_prompt", bytes.NewBuffer(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("sensitive content detected", func(t *testing.T) {
		// Prepare request body
		reqBody := map[string]interface{}{
			"Service": "LLMPromptModeration",
			"ServiceParameters": map[string]interface{}{
				"content":   "This is sensitive prompt",
				"accountId": "test-account-123",
			},
		}
		reqBodyBytes, _ := json.Marshal(reqBody)

		// Create result for sensitive content
		sensitiveResult := &sensitive.CheckResult{
			IsSensitive: true,
			Reason:      "contains sensitive content",
		}

		// Set mock expectation
		mockSensitiveComponent.EXPECT().PassLLMQueryCheck(
			mock.Anything,
			types.ScenarioLLMQueryModeration,
			"This is sensitive prompt",
			"test-account-123",
		).Return(sensitiveResult, nil).Once()

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/llm_prompt", bytes.NewBuffer(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		var response struct {
			Data *sensitive.CheckResult `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response.Data.IsSensitive)
		assert.Equal(t, "contains sensitive content", response.Data.Reason)
	})
}

func TestSensitiveHandler_Text(t *testing.T) {
	mockSensitiveComponent := mock_sensitive.NewMockSensitiveComponent(t)
	handler := &SensitiveHandler{
		c: mockSensitiveComponent,
	}

	// Set up Gin router
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.POST("/api/v1/moderation/text", handler.Text)

	// Create a test CheckResult
	successResult := &sensitive.CheckResult{
		IsSensitive: false,
		Reason:      "",
	}

	t.Run("success", func(t *testing.T) {
		// Prepare request body
		reqBody := map[string]interface{}{
			"scenario": types.ScenarioCommentDetection,
			"text":     "This is a safe text content",
		}
		reqBodyBytes, _ := json.Marshal(reqBody)

		// Set mock expectation
		mockSensitiveComponent.EXPECT().PassTextCheck(
			mock.Anything,
			types.ScenarioCommentDetection,
			"This is a safe text content",
		).Return(successResult, nil).Once()

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/text", bytes.NewBuffer(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		var response struct {
			Data *sensitive.CheckResult `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.False(t, response.Data.IsSensitive)
	})

	t.Run("bad request invalid json format", func(t *testing.T) {
		// Prepare request body (invalid JSON format)
		invalidJSON := `{"scenario": "textProfileCheck", "text": "test",}`

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/text", bytes.NewBufferString(invalidJSON))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("server error from sensitive component", func(t *testing.T) {
		// Prepare request body
		reqBody := map[string]interface{}{
			"scenario": types.ScenarioCommentDetection,
			"text":     "This is a test text",
		}
		reqBodyBytes, _ := json.Marshal(reqBody)

		// Set mock expectation to return error
		expectedErr := assert.AnError
		mockSensitiveComponent.EXPECT().PassTextCheck(
			mock.Anything,
			types.ScenarioCommentDetection,
			"This is a test text",
		).Return(nil, expectedErr).Once()

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/text", bytes.NewBuffer(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("sensitive content detected", func(t *testing.T) {
		// Prepare request body
		reqBody := map[string]interface{}{
			"scenario": types.ScenarioCommentDetection,
			"text":     "This is sensitive text content",
		}
		reqBodyBytes, _ := json.Marshal(reqBody)

		// Create result for sensitive content
		sensitiveResult := &sensitive.CheckResult{
			IsSensitive: true,
			Reason:      "contains sensitive content",
		}

		// Set mock expectation
		mockSensitiveComponent.EXPECT().PassTextCheck(
			mock.Anything,
			types.ScenarioCommentDetection,
			"This is sensitive text content",
		).Return(sensitiveResult, nil).Once()

		// Create request
		req, _ := http.NewRequest("POST", "/api/v1/moderation/text", bytes.NewBuffer(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")

		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify result
		assert.Equal(t, http.StatusOK, w.Code)
		var response struct {
			Data *sensitive.CheckResult `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response.Data.IsSensitive)
		assert.Equal(t, "contains sensitive content", response.Data.Reason)
	})
}

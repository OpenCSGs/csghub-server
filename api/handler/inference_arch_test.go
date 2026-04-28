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
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/common/types"
)

func TestInferenceArchHandler_GetInferenceArch(t *testing.T) {
	// Create a mock inference arch component
	mockComponent := mockcomponent.NewMockInferenceArchComponent(t)

	// Set up the mock to return a test inference arch
	testArch := &types.InferenceArch{
		ID:       1,
		Patterns: "test-pattern",
	}

	mockComponent.On("GetInferenceArch", mock.Anything).Return(testArch, nil)

	// Create the handler with the mock component
	handler := &InferenceArchHandler{
		inferenceArch: mockComponent,
	}

	// Create a test request
	req, err := http.NewRequest("GET", "/api/v1/inference-arch", nil)
	assert.NoError(t, err)

	// Create a response recorder
	recorder := httptest.NewRecorder()

	// Create a gin context
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req

	// Test the handler
	handler.GetInferenceArch(c)

	// Check the response
	assert.Equal(t, http.StatusOK, recorder.Code)

	// Parse the response
	var response types.InferenceArchResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, 0, response.Code)
	assert.Equal(t, "OK", response.Msg)
	assert.NotNil(t, response.Data)
	assert.Equal(t, "test-pattern", response.Data.Patterns)

	// Verify the mock was called
	mockComponent.AssertCalled(t, "GetInferenceArch", mock.Anything)
}

func TestInferenceArchHandler_UpdateInferenceArch(t *testing.T) {
	// Create a mock inference arch component
	mockComponent := mockcomponent.NewMockInferenceArchComponent(t)

	// Set up the mock to return a test inference arch
	testArch := &types.InferenceArch{
		ID:       1,
		Patterns: "updated-pattern",
	}

	mockComponent.On("UpdateInferenceArch", mock.Anything, mock.Anything).Return(testArch, nil)

	// Create the handler with the mock component
	handler := &InferenceArchHandler{
		inferenceArch: mockComponent,
	}

	// Create a test request body
	reqBody := &types.CreateInferenceArchReq{
		Patterns: "updated-pattern",
	}

	reqBodyBytes, err := json.Marshal(reqBody)
	assert.NoError(t, err)

	// Create a test request
	req, err := http.NewRequest("PUT", "/api/v1/inference-arch", bytes.NewBuffer(reqBodyBytes))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Create a response recorder
	recorder := httptest.NewRecorder()

	// Create a gin context
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req

	// Test the handler
	handler.UpdateInferenceArch(c)

	// Check the response
	assert.Equal(t, http.StatusOK, recorder.Code)

	// Parse the response
	var response types.InferenceArchResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, 0, response.Code)
	assert.Equal(t, "OK", response.Msg)
	assert.NotNil(t, response.Data)
	assert.Equal(t, "updated-pattern", response.Data.Patterns)

	// Verify the mock was called
	mockComponent.AssertCalled(t, "UpdateInferenceArch", mock.Anything, mock.Anything)
}

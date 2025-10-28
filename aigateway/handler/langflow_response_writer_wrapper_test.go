package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/common/types"
)

func TestNewLangflowResponseWriterWrapper(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)

	wrapper := NewLangflowResponseWriterWrapper(ctx.Writer, true, mockAgentComponent)

	assert.NotNil(t, wrapper)
	assert.Equal(t, ctx.Writer, wrapper.internalWritter)
	assert.True(t, wrapper.useStream)
	assert.Equal(t, mockAgentComponent, wrapper.agentComponent)
	assert.NotNil(t, wrapper.eventStreamDecoder)
}

func TestLangflowResponseWriterWrapper_Header(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	wrapper := NewLangflowResponseWriterWrapper(ctx.Writer, false, mockAgentComponent)

	// Test that Header() returns the same header as the internal writer
	header := wrapper.Header()
	assert.Equal(t, w.Header(), header)

	// Test that modifications to the header affect the internal writer
	header.Set("Content-Type", "application/json")
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestLangflowResponseWriterWrapper_WriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	wrapper := NewLangflowResponseWriterWrapper(ctx.Writer, false, mockAgentComponent)

	wrapper.WriteHeader(200)
	assert.Equal(t, 200, w.Code)
}

func TestLangflowResponseWriterWrapper_Flush(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	wrapper := NewLangflowResponseWriterWrapper(ctx.Writer, false, mockAgentComponent)

	wrapper.Flush()
	// httptest.ResponseRecorder doesn't have a direct way to check if Flush was called
	// but we can verify the wrapper doesn't panic
	assert.NotNil(t, wrapper)
}

func TestLangflowResponseWriterWrapper_Hijack(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	wrapper := NewLangflowResponseWriterWrapper(ctx.Writer, false, mockAgentComponent)

	// httptest.ResponseRecorder doesn't implement http.Hijacker, so this will panic
	// We test that the wrapper properly delegates to the internal writer
	assert.Panics(t, func() {
		_, _, _ = wrapper.Hijack()
	})
}

func TestLangflowResponseWriterWrapper_Write_NonStream(t *testing.T) {
	tests := []struct {
		name           string
		input          []byte
		expectedCalls  int
		expectError    bool
		expectedOutput string
	}{
		{
			name: "valid RunLangflowAgentInstanceResponse",
			input: []byte(`{
				"session_id": "test-session",
				"outputs": [
					{
						"outputs": [
							{
								"results": {
									"message": {
										"text": "Hello World"
									}
								}
							}
						]
					}
				]
			}`),
			expectedCalls: 1,
			expectError:   false,
			expectedOutput: `{
				"session_id": "test-session",
				"outputs": [
					{
						"outputs": [
							{
								"results": {
									"message": {
										"text": "Hello World"
									}
								}
							}
						]
					}
				]
			}`,
		},
		{
			name:           "invalid JSON",
			input:          []byte(`invalid json`),
			expectedCalls:  0,
			expectError:    false,
			expectedOutput: `invalid json`,
		},
		{
			name: "empty outputs",
			input: []byte(`{
				"session_id": "test-session",
				"outputs": []
			}`),
			expectedCalls: 0,
			expectError:   false,
			expectedOutput: `{
				"session_id": "test-session",
				"outputs": []
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
			wrapper := NewLangflowResponseWriterWrapper(ctx.Writer, false, mockAgentComponent)

			n, err := wrapper.Write(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, len(tt.input), n)
			assert.Equal(t, tt.expectedOutput, w.Body.String())
			mockAgentComponent.AssertExpectations(t)
		})
	}
}

func TestLangflowResponseWriterWrapper_Write_Stream(t *testing.T) {
	tests := []struct {
		name           string
		input          []byte
		expectedCalls  int
		expectedOutput string
	}{
		{
			name:           "token event",
			input:          []byte(`{"event":"token","data":{"chunk":"Hello"}}`),
			expectedCalls:  0,
			expectedOutput: `{"event":"token","data":{"chunk":"Hello"}}`,
		},
		{
			name:           "add_message event",
			input:          []byte(`{"event":"add_message","data":{"message":"Test message"}}`),
			expectedCalls:  0,
			expectedOutput: `{"event":"add_message","data":{"message":"Test message"}}`,
		},
		{
			name:           "end event with valid data",
			input:          []byte(`{"event":"end","data":{"result":{"session_id":"test-session","outputs":[{"outputs":[{"results":{"message":{"text":"Final message"}}}]}]}}}`),
			expectedCalls:  1,
			expectedOutput: `{"event":"end","data":{"result":{"session_id":"test-session","outputs":[{"outputs":[{"results":{"message":{"text":"Final message"}}}]}]}}}`,
		},
		{
			name:           "end event with invalid data",
			input:          []byte(`{"event":"end","data":"invalid"}`),
			expectedCalls:  0,
			expectedOutput: `{"event":"end","data":"invalid"}`,
		},
		{
			name:           "unknown event",
			input:          []byte(`{"event":"unknown","data":{"test":"value"}}`),
			expectedCalls:  0,
			expectedOutput: `{"event":"unknown","data":{"test":"value"}}`,
		},
		{
			name:           "multiple events",
			input:          []byte(`{"event":"token","data":{"chunk":"Hello"}}{"event":"end","data":{"result":{"session_id":"test-session","outputs":[{"outputs":[{"results":{"message":{"text":"Final"}}}]}]}}}`),
			expectedCalls:  1,
			expectedOutput: `{"event":"token","data":{"chunk":"Hello"}}{"event":"end","data":{"result":{"session_id":"test-session","outputs":[{"outputs":[{"results":{"message":{"text":"Final"}}}]}]}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
			wrapper := NewLangflowResponseWriterWrapper(ctx.Writer, true, mockAgentComponent)

			n, err := wrapper.Write(tt.input)

			assert.NoError(t, err)
			assert.Equal(t, len(tt.input), n)
			assert.Equal(t, tt.expectedOutput, w.Body.String())
			mockAgentComponent.AssertExpectations(t)
		})
	}
}

func TestExtractLangflowMessage(t *testing.T) {
	tests := []struct {
		name     string
		outputs  []types.LangflowOutputs
		expected string
	}{
		{
			name:     "empty outputs",
			outputs:  []types.LangflowOutputs{},
			expected: "",
		},
		{
			name:     "nil outputs",
			outputs:  nil,
			expected: "",
		},
		{
			name: "empty inner outputs",
			outputs: []types.LangflowOutputs{
				{Outputs: []types.LangflowInnerOutput{}},
			},
			expected: "",
		},
		{
			name: "valid message",
			outputs: []types.LangflowOutputs{
				{
					Outputs: []types.LangflowInnerOutput{
						{
							Results: &types.LangflowResults{
								Message: types.LangflowMessage{
									Text: "Hello World",
								},
							},
						},
					},
				},
			},
			expected: "Hello World",
		},
		{
			name: "nil results",
			outputs: []types.LangflowOutputs{
				{
					Outputs: []types.LangflowInnerOutput{
						{Results: nil},
					},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLangflowMessage(tt.outputs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLangflowResponseWriterWrapper_WriteInternal(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	wrapper := NewLangflowResponseWriterWrapper(ctx.Writer, false, mockAgentComponent)

	testData := []byte("test data")
	wrapper.writeInternal(testData)

	assert.Equal(t, "test data", w.Body.String())
}

func TestLangflowResponseWriterWrapper_StreamWrite_ComplexScenario(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	wrapper := NewLangflowResponseWriterWrapper(ctx.Writer, true, mockAgentComponent)

	// Simulate a complex streaming scenario
	streamData := `{"event":"start","data":{"message":"Starting"}}{"event":"token","data":{"chunk":"Hello"}}{"event":"token","data":{"chunk":" World"}}{"event":"end","data":{"result":{"session_id":"session-123","outputs":[{"outputs":[{"results":{"message":{"text":"Hello World"}}}]}]}}}`

	n, err := wrapper.Write([]byte(streamData))

	assert.NoError(t, err)
	assert.Equal(t, len(streamData), n)
	assert.Equal(t, streamData, w.Body.String())
	mockAgentComponent.AssertExpectations(t)
}

func TestLangflowResponseWriterWrapper_NonStreamWrite_ComplexResponse(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	mockAgentComponent := mockcomponent.NewMockAgentComponent(t)
	wrapper := NewLangflowResponseWriterWrapper(ctx.Writer, false, mockAgentComponent)

	// Test with complex response structure
	responseData := `{
		"session_id": "complex-session",
		"outputs": [
			{
				"outputs": [
					{
						"results": {
							"message": {
								"timestamp": "2023-01-01T00:00:00Z",
								"sender": "assistant",
								"sender_name": "AI Assistant",
								"text_key": "response",
								"text": "This is a complex response with multiple fields"
							}
						}
					}
				]
			}
		]
	}`

	n, err := wrapper.Write([]byte(responseData))

	assert.NoError(t, err)
	assert.Equal(t, len(responseData), n)

	// Verify the response was written correctly
	var writtenResponse types.RunLangflowAgentInstanceResponse
	err = json.Unmarshal(w.Body.Bytes(), &writtenResponse)
	assert.NoError(t, err)
	assert.Equal(t, "complex-session", writtenResponse.SessionID)
	assert.Len(t, writtenResponse.Outputs, 1)

	mockAgentComponent.AssertExpectations(t)
}

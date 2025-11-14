package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/aigateway/token"
)

func TestResponseWriterWrapperEmbedding_Write(t *testing.T) {
	tests := []struct {
		name          string
		response      openai.CreateEmbeddingResponse
		withCounter   bool
		expectedBytes int
		expectedError bool
	}{
		{
			name: "successful write with token counter",
			response: openai.CreateEmbeddingResponse{
				Object: "embedding",
				Data: []openai.Embedding{
					{
						Object:    "embedding",
						Embedding: []float64{0.1, 0.2, 0.3},
						Index:     0,
					},
				},
				Model: "text-embedding-ada-002",
				Usage: openai.CreateEmbeddingResponseUsage{
					PromptTokens: 10,
					TotalTokens:  10,
				},
			},
			withCounter:   true,
			expectedError: false,
		},
		{
			name: "successful write without token counter",
			response: openai.CreateEmbeddingResponse{
				Object: "embedding",
				Data: []openai.Embedding{
					{
						Object:    "embedding",
						Embedding: []float64{0.1, 0.2, 0.3},
						Index:     0,
					},
				},
				Model: "text-embedding-ada-002",
				Usage: openai.CreateEmbeddingResponseUsage{
					PromptTokens: 10,
					TotalTokens:  10,
				},
			},
			withCounter:   false,
			expectedError: false,
		},
		{
			name:          "invalid json data",
			response:      openai.CreateEmbeddingResponse{},
			withCounter:   true,
			expectedError: false, // Should not return error even if unmarshal fails
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test http.ResponseWriter
			w := httptest.NewRecorder()
			wrapper := NewResponseWriterWrapperEmbedding(w)

			// Setup token counter if needed
			if tt.withCounter {
				counter := token.NewEmbeddingTokenCounter(nil)
				wrapper.WithTokenCounter(counter)
			}

			// Prepare test data
			var data []byte
			var err error
			if tt.name == "invalid json data" {
				data = []byte(`invalid json`)
			} else {
				data, err = json.Marshal(tt.response)
				assert.NoError(t, err)
			}

			// Execute Write method
			n, err := wrapper.Write(data)

			// Verify results
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(data), n)
				assert.Equal(t, data, w.Body.Bytes())
			}

			// Verify token counter if used
			if tt.withCounter && tt.name != "invalid json data" {
				counter := wrapper.tokenCounter
				usage, err := counter.Usage()
				assert.NoError(t, err)
				assert.Equal(t, tt.response.Usage.TotalTokens, usage.TotalTokens)
				assert.Equal(t, tt.response.Usage.PromptTokens, usage.PromptTokens)
			}
		})
	}
}

package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	mocktoken "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/token"
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

			// Setup token counter if needed
			var wrapper *ResponseWriterWrapperEmbedding
			var counter *mocktoken.MockEmbeddingTokenCounter = nil
			if tt.withCounter {
				counter = mocktoken.NewMockEmbeddingTokenCounter(t)
				wrapper = NewResponseWriterWrapperEmbedding(w, counter)
			} else {
				wrapper = NewResponseWriterWrapperEmbedding(w, nil)
			}

			// Prepare test data
			var data []byte
			var err error
			if tt.name == "invalid json data" {
				data = []byte(`invalid json`)
			} else if tt.withCounter {
				data, err = json.Marshal(tt.response)
				assert.NoError(t, err)
				var expectResp openai.CreateEmbeddingResponse
				err = json.Unmarshal(data, &expectResp)
				assert.NoError(t, err)
				counter.EXPECT().Embedding(expectResp.Usage).Return().Once()
				counter.EXPECT().Usage(context.Background()).Return(&token.Usage{
					TotalTokens:      expectResp.Usage.TotalTokens,
					PromptTokens:     expectResp.Usage.PromptTokens,
					CompletionTokens: 0,
				}, nil).Once()
			} else if !tt.withCounter {
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
				usage, err := counter.Usage(context.Background())
				assert.NoError(t, err)
				assert.Equal(t, tt.response.Usage.TotalTokens, usage.TotalTokens)
				assert.Equal(t, tt.response.Usage.PromptTokens, usage.PromptTokens)
			}
		})
	}
}

func TestResponseWriterWrapperEmbedding_Write_Gzip(t *testing.T) {
	// Create a test http.ResponseWriter
	w := httptest.NewRecorder()

	// Setup token counter
	counter := mocktoken.NewMockEmbeddingTokenCounter(t)
	wrapper := NewResponseWriterWrapperEmbedding(w, counter)

	// Create embedding response
	response := openai.CreateEmbeddingResponse{
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
	}

	// Marshal response to JSON
	jsonData, err := json.Marshal(response)
	assert.NoError(t, err)

	// Create gzip compressed data
	var gzippedData bytes.Buffer
	gzipWriter := gzip.NewWriter(&gzippedData)
	_, err = gzipWriter.Write(jsonData)
	assert.NoError(t, err)
	gzipWriter.Close()
	var expectResp openai.CreateEmbeddingResponse
	err = json.Unmarshal(jsonData, &expectResp)
	assert.NoError(t, err)
	// Set expectations for token counter
	counter.EXPECT().Embedding(expectResp.Usage).Return().Once()
	counter.EXPECT().Usage(context.Background()).Return(&token.Usage{
		TotalTokens:      expectResp.Usage.TotalTokens,
		PromptTokens:     expectResp.Usage.PromptTokens,
		CompletionTokens: 0,
	}, nil).Once()

	// Execute Write method with gzipped data
	n, err := wrapper.Write(gzippedData.Bytes())

	// Verify results
	assert.NoError(t, err)
	assert.Equal(t, len(gzippedData.Bytes()), n)
	assert.Equal(t, gzippedData.Bytes(), w.Body.Bytes())

	// Verify token counter was called with correct usage data
	usage, err := wrapper.tokenCounter.Usage(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, response.Usage.TotalTokens, usage.TotalTokens)
	assert.Equal(t, response.Usage.PromptTokens, usage.PromptTokens)
}

package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
			wrapper.CaptureEmbeddingUsage()

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

func TestResponseWriterWrapperEmbedding_StatusCode(t *testing.T) {
	w := httptest.NewRecorder()
	wrapper := NewResponseWriterWrapperEmbedding(w, nil)

	assert.Equal(t, http.StatusOK, wrapper.StatusCode())

	wrapper.WriteHeader(http.StatusTooManyRequests)

	assert.Equal(t, http.StatusTooManyRequests, wrapper.StatusCode())
}

func TestResponseWriterWrapperEmbedding_StatusCode_DefaultsToOKOnWrite(t *testing.T) {
	w := httptest.NewRecorder()
	wrapper := NewResponseWriterWrapperEmbedding(w, nil)

	_, err := wrapper.Write([]byte(`{"object":"list","data":[]}`))

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, wrapper.StatusCode())
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
	wrapper.CaptureEmbeddingUsage()

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

func TestResponseWriterWrapperEmbedding_CaptureUsageAfterChunkedWrites(t *testing.T) {
	w := httptest.NewRecorder()
	counter := mocktoken.NewMockEmbeddingTokenCounter(t)
	wrapper := NewResponseWriterWrapperEmbedding(w, counter)

	data := []byte(`{"object":"list","data":[{"object":"embedding","embedding":[0.1,0.2,0.3],"index":0}],"model":"text-embedding-ada-002","usage":{"prompt_tokens":10,"total_tokens":10}}`)
	counter.EXPECT().Embedding(mock.MatchedBy(func(usage openai.CreateEmbeddingResponseUsage) bool {
		return usage.PromptTokens == 10 && usage.TotalTokens == 10
	})).Return().Once()

	n, err := wrapper.Write(data[:60])
	assert.NoError(t, err)
	assert.Equal(t, 60, n)
	n, err = wrapper.Write(data[60:])
	assert.NoError(t, err)
	assert.Equal(t, len(data)-60, n)

	wrapper.CaptureEmbeddingUsage()

	assert.Equal(t, data, w.Body.Bytes())
}

func TestResponseWriterWrapperEmbedding_SkipsUsageCaptureWhenTruncated(t *testing.T) {
	w := httptest.NewRecorder()
	counter := mocktoken.NewMockEmbeddingTokenCounter(t)
	wrapper := newResponseWriterWrapperEmbedding(w, counter, 8)

	data := []byte(`{"object":"list","data":[],"usage":{"prompt_tokens":10,"total_tokens":10}}`)
	n, err := wrapper.Write(data)
	assert.NoError(t, err)
	assert.Equal(t, len(data), n)

	wrapper.CaptureEmbeddingUsage()

	assert.Equal(t, data, w.Body.Bytes())
}

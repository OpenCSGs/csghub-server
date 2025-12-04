package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/compress"
	"opencsg.com/csghub-server/builder/rpc"
)

func TestNonStreamResponseWriter_Write(t *testing.T) {
	// Create test context
	gin.SetMode(gin.TestMode)
	t.Run("basic write without processing", func(t *testing.T) {
		// Prepare test data
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		modComponent := component.NewMockModeration(t)
		nsw := newNonStreamResponseWriter(ctx.Writer, modComponent)
		// Execute write operation
		data := []byte("Hello, World!")
		n, err := nsw.Write(data)

		// Verify results
		require.NoError(t, err)
		require.Equal(t, len(data), n)
	})

	t.Run("write with valid JSON completion", func(t *testing.T) {
		// Prepare test data
		w := httptest.NewRecorder()
		w.Header().Set("Content-Encoding", "")
		ctx, _ := gin.CreateTestContext(w)
		modComponent := component.NewMockModeration(t)
		nsw := newNonStreamResponseWriter(ctx.Writer, modComponent)

		// Create valid ChatCompletion data
		completion := types.ChatCompletion{
			ChatCompletion: openai.ChatCompletion{
				Choices: []openai.ChatCompletionChoice{
					{
						Message: openai.ChatCompletionMessage{
							Content: "This is a test response",
						},
					},
				},
			},
		}

		data, _ := json.Marshal(completion)
		var expectCompletion types.ChatCompletion
		_ = json.Unmarshal(data, &expectCompletion)

		modComponent.EXPECT().CheckChatNonStreamResponse(mock.Anything, expectCompletion).Return(&rpc.CheckResult{IsSensitive: false}, nil)
		// Execute write operation
		n, err := nsw.Write(data)

		// Verify results
		require.NoError(t, err)
		require.Equal(t, len(data), n)
	})

	t.Run("write with empty choices", func(t *testing.T) {
		// Prepare test data
		w := httptest.NewRecorder()
		w.Header().Set("Content-Encoding", "")
		ctx, _ := gin.CreateTestContext(w)
		mockMod := component.NewMockModeration(t)
		nsw := newNonStreamResponseWriter(ctx.Writer, mockMod)

		// Create valid ChatCompletion data without choices
		completion := types.ChatCompletion{
			ChatCompletion: openai.ChatCompletion{
				Choices: []openai.ChatCompletionChoice{},
			},
		}
		data, _ := json.Marshal(completion)

		// Execute write operation
		n, err := nsw.Write(data)

		// Verify results
		require.NoError(t, err)
		require.Equal(t, len(data), n)
	})
}

func TestNonStreamResponseWriter_WithModeration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("moderation detects sensitive content", func(t *testing.T) {
		// Prepare test data
		w := httptest.NewRecorder()
		w.Header().Set("Content-Encoding", "")
		ctx, _ := gin.CreateTestContext(w)
		mockMod := component.NewMockModeration(t)
		nsw := newNonStreamResponseWriter(ctx.Writer, mockMod)

		// Create valid ChatCompletion data
		completion := types.ChatCompletion{
			ChatCompletion: openai.ChatCompletion{
				Choices: []openai.ChatCompletionChoice{
					{
						Message: openai.ChatCompletionMessage{
							Content: "sensitive content",
						},
					},
				},
			},
		}

		data, _ := json.Marshal(completion)
		var expectCompletion types.ChatCompletion
		_ = json.Unmarshal(data, &expectCompletion)
		mockMod.EXPECT().CheckChatNonStreamResponse(mock.Anything, expectCompletion).
			Return(&rpc.CheckResult{IsSensitive: true}, nil)
		// Execute write operation
		n, err := nsw.Write(data)

		// Verify results
		require.NoError(t, err)
		require.Equal(t, len(data), n)
	})

	t.Run("moderation returns error", func(t *testing.T) {
		// Prepare test data
		w := httptest.NewRecorder()
		w.Header().Set("Content-Encoding", "")
		ctx, _ := gin.CreateTestContext(w)
		mockMod := component.NewMockModeration(t)
		nsw := newNonStreamResponseWriter(ctx.Writer, mockMod)

		// Create valid ChatCompletion data
		completion := types.ChatCompletion{
			ChatCompletion: openai.ChatCompletion{
				Choices: []openai.ChatCompletionChoice{
					{
						Message: openai.ChatCompletionMessage{
							Content: "test content",
						},
					},
				},
			},
		}
		data, _ := json.Marshal(completion)
		var expectCompletion types.ChatCompletion
		_ = json.Unmarshal(data, &expectCompletion)
		mockMod.EXPECT().CheckChatNonStreamResponse(mock.Anything, expectCompletion).
			Return(nil, errors.New("some error"))

		// Execute write operation
		n, err := nsw.Write(data)

		// Verify results
		require.NoError(t, err)
		require.Equal(t, len(data), n)
	})
}

func TestNonStreamResponseWriter_compressedData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("compressed data processing", func(t *testing.T) {
		// Prepare test data
		w := httptest.NewRecorder()
		w.Header().Set("Content-Encoding", "gzip")
		ctx, _ := gin.CreateTestContext(w)
		mockMod := component.NewMockModeration(t)
		nsw := newNonStreamResponseWriter(ctx.Writer, mockMod)

		// Create valid ChatCompletion data and compress
		completion := types.ChatCompletion{
			ChatCompletion: openai.ChatCompletion{
				Choices: []openai.ChatCompletionChoice{
					{
						Message: openai.ChatCompletionMessage{
							Content: "test content",
						},
					},
				},
			},
		}
		jsonData, _ := json.Marshal(completion)
		compressedData, err := compress.Encode("gzip", jsonData)
		require.NoError(t, err)
		var expectCompletion types.ChatCompletion
		_ = json.Unmarshal(jsonData, &expectCompletion)
		mockMod.EXPECT().CheckChatNonStreamResponse(mock.Anything, expectCompletion).
			Return(&rpc.CheckResult{IsSensitive: false}, nil)
		// Execute write operation
		n, err := nsw.Write(compressedData)

		// Verify results
		require.NoError(t, err)
		require.Equal(t, len(compressedData), n)
	})

	t.Run("compressed data wrong", func(t *testing.T) {
		// Prepare test data
		w := httptest.NewRecorder()
		w.Header().Set("Content-Encoding", "gzip")
		ctx, _ := gin.CreateTestContext(w)
		nsw := newNonStreamResponseWriter(ctx.Writer, component.NewMockModeration(t))

		// Create valid ChatCompletion data and compress
		completion := types.ChatCompletion{
			ChatCompletion: openai.ChatCompletion{
				Choices: []openai.ChatCompletionChoice{
					{
						Message: openai.ChatCompletionMessage{
							Content: "test content",
						},
					},
				},
			},
		}
		jsonData, _ := json.Marshal(completion)

		// Execute write operation
		n, err := nsw.Write(jsonData)

		// Verify results
		require.NoError(t, err)
		require.Equal(t, len(jsonData), n)
	})

	t.Run("moderation with compressed data", func(t *testing.T) {
		// Prepare test data
		w := httptest.NewRecorder()
		w.Header().Set("Content-Encoding", "gzip")
		ctx, _ := gin.CreateTestContext(w)
		mockMod := component.NewMockModeration(t)
		nsw := newNonStreamResponseWriter(ctx.Writer, mockMod)

		// Create valid ChatCompletion data and compress
		completion := types.ChatCompletion{
			ChatCompletion: openai.ChatCompletion{
				Choices: []openai.ChatCompletionChoice{
					{
						Message: openai.ChatCompletionMessage{
							Content: "sensitive content",
						},
					},
				},
			},
		}
		jsonData, _ := json.Marshal(completion)
		compressedData, err := compress.Encode("gzip", jsonData)
		require.NoError(t, err)
		var expectCompletion types.ChatCompletion
		_ = json.Unmarshal(jsonData, &expectCompletion)
		mockMod.EXPECT().CheckChatNonStreamResponse(mock.Anything, expectCompletion).
			Return(&rpc.CheckResult{IsSensitive: true}, nil)

		// Execute write operation
		n, err := nsw.Write(compressedData)

		// Verify results
		require.NoError(t, err)
		require.Equal(t, len(compressedData), n)
	})
}

func TestNonStreamResponseWriter_multipleWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("multiple writes should not process multiple times", func(t *testing.T) {
		// Prepare test data
		w := httptest.NewRecorder()
		w.Header().Set("Content-Encoding", "")
		ctx, _ := gin.CreateTestContext(w)
		mockMod := component.NewMockModeration(t)
		nsw := newNonStreamResponseWriter(ctx.Writer, mockMod)

		// Create valid ChatCompletion data
		completion := types.ChatCompletion{
			ChatCompletion: openai.ChatCompletion{
				Choices: []openai.ChatCompletionChoice{
					{
						Message: openai.ChatCompletionMessage{
							Content: "test content",
						},
					},
				},
			},
		}
		data, _ := json.Marshal(completion)
		var expectCompletion types.ChatCompletion
		_ = json.Unmarshal(data, &expectCompletion)
		mockMod.EXPECT().CheckChatNonStreamResponse(mock.Anything, expectCompletion).
			Return(&rpc.CheckResult{IsSensitive: false}, nil)

		// Execute multiple write operations
		for i := 0; i < 3; i++ {
			n, err := nsw.Write(data)
			require.NoError(t, err)
			require.Equal(t, len(data), n)
		}

	})
}

func TestNonStreamResponseWriter_ClearBuffer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("clear buffer writes remaining data", func(t *testing.T) {
		// Prepare test data
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		nsw := newNonStreamResponseWriter(ctx.Writer, component.NewMockModeration(t))

		// Write some data without triggering processing
		data := []byte("incomplete data")
		datalen, err := nsw.Write(data)
		require.Equal(t, datalen, len(data))
		require.NoError(t, err)
		// Clear buffer
		nsw.ClearBuffer()

		// Verify data was written
		require.Contains(t, w.Body.String(), string(data))
	})
}

func TestNonStreamResponseWriter_HeaderAndWriteHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("header methods delegate to internal writer", func(t *testing.T) {
		// Prepare test data
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		nsw := newNonStreamResponseWriter(ctx.Writer, component.NewMockModeration(t))

		// Test Header method
		header := nsw.Header()
		header.Set("X-Test", "test value")
		require.Equal(t, "test value", w.Header().Get("X-Test"))

		// Test WriteHeader method
		nsw.WriteHeader(http.StatusOK)
		require.Equal(t, http.StatusOK, w.Code)
	})
}

// Helper method: implements Usage interface
type Usage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

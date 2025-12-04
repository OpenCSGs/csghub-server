package component

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mock_rpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mock_cache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/sensitive"
)

func TestSplitContentIntoChunksByWindow_Table(t *testing.T) {
	longLen := slidingWindowSize*2 + 10
	longStr := strings.Repeat("a", longLen)

	// build expected chunks for longStr
	var expectedLong []string
	for i := 0; i < longLen; i += slidingWindowSize {
		end := i + slidingWindowSize
		if end > longLen {
			end = longLen
		}
		expectedLong = append(expectedLong, longStr[i:end])
	}

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty", in: "", want: []string{}},
		{name: "simple sentences", in: "Hello world. How are you? I'm fine!", want: []string{"Hello world", "How are you", "I'm fine"}},
		{name: "leading/trailing/extra punctuation", in: "  .hello.. world!  ", want: []string{"hello", "world"}},
		{name: "long single sentence", in: longStr, want: expectedLong},
		{name: "mixed with long sentence and short", in: "short. " + longStr + "! tail?", want: append(append([]string{"short"}, expectedLong...), "tail")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitContentIntoChunksByWindow(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected result for %q:\ngot:  %#v\nwant: %#v", tt.name, got, tt.want)
			}
		})
	}
}

func TestModerationImpl_CheckLLMPrompt_WithoutCache(t *testing.T) {
	ctx := context.Background()
	key := "test-key"

	t.Run("short and not sensitive", func(t *testing.T) {
		mockClient := mock_rpc.NewMockModerationSvcClient(t)
		moderation := NewModerationImplWithClient(mockClient, nil)
		content := "this is a short and safe text"

		mockClient.EXPECT().PassLLMPromptCheck(ctx, content, key).Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()

		result, err := moderation.CheckChatPrompts(ctx, []openai.ChatCompletionMessageParamUnion{
			{
				OfSystem: &openai.ChatCompletionSystemMessageParam{
					Content: openai.ChatCompletionSystemMessageParamContentUnion{
						OfString: param.Opt[string]{Value: content},
					},
				},
			},
		}, key)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.IsSensitive)
		mockClient.AssertExpectations(t)
	})

	t.Run("short and sensitive", func(t *testing.T) {
		mockClient := mock_rpc.NewMockModerationSvcClient(t)
		moderation := NewModerationImplWithClient(mockClient, nil)
		content := "this is a short and sensitive text"

		mockClient.On("PassLLMPromptCheck", ctx, content, key).Return(&rpc.CheckResult{IsSensitive: true, Reason: "sensitive"}, nil).Once()

		result, err := moderation.CheckChatPrompts(ctx, []openai.ChatCompletionMessageParamUnion{
			{
				OfSystem: &openai.ChatCompletionSystemMessageParam{
					Content: openai.ChatCompletionSystemMessageParamContentUnion{
						OfString: param.Opt[string]{Value: content},
					},
				},
			},
		}, key)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.IsSensitive)
		mockClient.AssertExpectations(t)
	})
}

func TestModerationImpl_CheckChatStreamResponse(t *testing.T) {
	ctx := context.Background()
	uuid := "test-uuid"

	t.Run("should_return_non_sensitive_when_modSvcClient_is_nil", func(t *testing.T) {
		modImpl := &moderationImpl{
			modSvcClient: nil,
			cacheClient:  nil,
		}

		chunk := types.ChatCompletionChunk{
			Choices: []types.ChatCompletionChunkChoice{{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: "test content",
				},
			}},
		}

		result, err := modImpl.CheckChatStreamResponse(ctx, chunk, uuid)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, false, result.IsSensitive)
	})

	t.Run("should_return_non_sensitive_when_choices_is_empty", func(t *testing.T) {
		mockModClient := mock_rpc.NewMockModerationSvcClient(t)
		modImpl := &moderationImpl{
			modSvcClient: mockModClient,
			cacheClient:  nil,
		}

		chunk := types.ChatCompletionChunk{
			Choices: []types.ChatCompletionChunkChoice{},
		}

		result, err := modImpl.CheckChatStreamResponse(ctx, chunk, uuid)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, false, result.IsSensitive)
	})

	t.Run("should_return_non_sensitive_when_both_content_and_reasoning_are_empty", func(t *testing.T) {
		mockModClient := mock_rpc.NewMockModerationSvcClient(t)
		modImpl := &moderationImpl{
			modSvcClient: mockModClient,
			cacheClient:  nil,
		}

		chunk := types.ChatCompletionChunk{
			Choices: []types.ChatCompletionChunkChoice{{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content:          "",
					ReasoningContent: "",
				},
			}},
		}

		result, err := modImpl.CheckChatStreamResponse(ctx, chunk, uuid)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, false, result.IsSensitive)
	})

	t.Run("should_call_PassLLMRespCheck_and_return_non_sensitive_when_content_not_empty", func(t *testing.T) {
		mockModClient := mock_rpc.NewMockModerationSvcClient(t)
		mockModClient.EXPECT().PassLLMRespCheck(ctx, "test content", uuid).
			Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		modImpl := &moderationImpl{
			modSvcClient: mockModClient,
			cacheClient:  nil,
		}

		chunk := types.ChatCompletionChunk{
			Choices: []types.ChatCompletionChunkChoice{{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: "test content",
				},
			}},
		}

		result, err := modImpl.CheckChatStreamResponse(ctx, chunk, uuid)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, false, result.IsSensitive)
		mockModClient.AssertExpectations(t)
	})

	t.Run("should_call_PassLLMRespCheck_and_return_sensitive_when_content_is_sensitive", func(t *testing.T) {
		mockModClient := mock_rpc.NewMockModerationSvcClient(t)
		mockModClient.EXPECT().PassLLMRespCheck(ctx, "sensitive content", uuid).
			Return(&rpc.CheckResult{IsSensitive: true, Reason: "inappropriate language"}, nil).Once()
		modImpl := &moderationImpl{
			modSvcClient: mockModClient,
			cacheClient:  nil,
		}

		chunk := types.ChatCompletionChunk{
			Choices: []types.ChatCompletionChunkChoice{{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: "sensitive content",
				},
			}},
		}

		result, err := modImpl.CheckChatStreamResponse(ctx, chunk, uuid)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, true, result.IsSensitive)
		mockModClient.AssertExpectations(t)
	})

	t.Run("should_check_reasoning_content_when_content_is_whitespace", func(t *testing.T) {
		mockModClient := mock_rpc.NewMockModerationSvcClient(t)
		mockModClient.EXPECT().PassLLMRespCheck(ctx, "reasoning content", uuid).
			Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		modImpl := &moderationImpl{
			modSvcClient: mockModClient,
			cacheClient:  nil,
		}

		chunk := types.ChatCompletionChunk{
			Choices: []types.ChatCompletionChunkChoice{{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content:          "   ",
					ReasoningContent: "reasoning content",
				},
			}},
		}

		result, err := modImpl.CheckChatStreamResponse(ctx, chunk, uuid)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, false, result.IsSensitive)
		mockModClient.AssertExpectations(t)
	})

	t.Run("should_call_PassLLMRespCheck_when_reasoning_content_not_empty", func(t *testing.T) {
		mockModClient := mock_rpc.NewMockModerationSvcClient(t)
		mockModClient.EXPECT().PassLLMRespCheck(ctx, "reasoning content", uuid).
			Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		modImpl := &moderationImpl{
			modSvcClient: mockModClient,
			cacheClient:  nil,
		}

		chunk := types.ChatCompletionChunk{
			Choices: []types.ChatCompletionChunkChoice{{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content:          "",
					ReasoningContent: "reasoning content",
				},
			}},
		}

		result, err := modImpl.CheckChatStreamResponse(ctx, chunk, uuid)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, false, result.IsSensitive)
		mockModClient.AssertExpectations(t)
	})

	t.Run("should_return_error_when_PassLLMRespCheck_fails", func(t *testing.T) {
		mockModClient := mock_rpc.NewMockModerationSvcClient(t)
		mockModClient.EXPECT().PassLLMRespCheck(ctx, "test content", uuid).
			Return(nil, assert.AnError).Once()
		modImpl := &moderationImpl{
			modSvcClient: mockModClient,
			cacheClient:  nil,
		}

		chunk := types.ChatCompletionChunk{
			Choices: []types.ChatCompletionChunkChoice{{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content: "test content",
				},
			}},
		}

		result, err := modImpl.CheckChatStreamResponse(ctx, chunk, uuid)
		assert.Error(t, err)
		assert.Nil(t, result)
		mockModClient.AssertExpectations(t)
	})

	t.Run("should_return_default_result_when_both_content_and_reasoning_are_whitespace", func(t *testing.T) {
		mockModClient := mock_rpc.NewMockModerationSvcClient(t)
		modImpl := &moderationImpl{
			modSvcClient: mockModClient,
			cacheClient:  nil,
		}

		chunk := types.ChatCompletionChunk{
			Choices: []types.ChatCompletionChunkChoice{{
				Delta: types.ChatCompletionChunkChoiceDelta{
					Content:          "   ",
					ReasoningContent: "   ",
				},
			}},
		}

		result, err := modImpl.CheckChatStreamResponse(ctx, chunk, uuid)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, false, result.IsSensitive)
	})
}

func TestModerationImpl_CheckChatNonStreamResponse(t *testing.T) {
	ctx := context.Background()
	t.Run("should_call_PassLLMRespCheck_and_return_sensitive_when_content_is_sensitive", func(t *testing.T) {
		mockModClient := mock_rpc.NewMockModerationSvcClient(t)
		mockModClient.EXPECT().PassTextCheck(ctx, string(sensitive.ScenarioChatDetection), "sensitive content").
			Return(&rpc.CheckResult{IsSensitive: true, Reason: "inappropriate language"}, nil).Once()
		modImpl := &moderationImpl{
			modSvcClient: mockModClient,
			cacheClient:  nil,
		}

		completion := types.ChatCompletion{
			ChatCompletion: openai.ChatCompletion{
				Choices: []openai.ChatCompletionChoice{{
					Message: openai.ChatCompletionMessage{
						Content: "sensitive content",
					},
				}},
			},
		}

		result, err := modImpl.CheckChatNonStreamResponse(ctx, completion)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, true, result.IsSensitive)
		mockModClient.AssertExpectations(t)
	})
	t.Run("should_call_PassLLMRespCheck_and_return_not_sensitive_when_content_is_not_sensitive", func(t *testing.T) {
		mockModClient := mock_rpc.NewMockModerationSvcClient(t)
		mockModClient.EXPECT().PassTextCheck(ctx, string(sensitive.ScenarioChatDetection), "not sensitive content").
			Return(&rpc.CheckResult{IsSensitive: false, Reason: "appropriate language"}, nil).Once()
		modImpl := &moderationImpl{
			modSvcClient: mockModClient,
			cacheClient:  nil,
		}

		completion := types.ChatCompletion{
			ChatCompletion: openai.ChatCompletion{
				Choices: []openai.ChatCompletionChoice{{
					Message: openai.ChatCompletionMessage{
						Content: "not sensitive content",
					},
				}},
			},
		}

		result, err := modImpl.CheckChatNonStreamResponse(ctx, completion)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, false, result.IsSensitive)
		mockModClient.AssertExpectations(t)
	})
}

// TestModerationImpl_CheckLLMPrompt_CacheCheck 测试 moderation.go 中第231-255行的缓存检查逻辑
func TestModerationImpl_CheckLLMPrompt_CacheCheck(t *testing.T) {
	ctx := context.Background()
	key := "test-key"

	// case 1: cache hit
	t.Run("cache_client_exists_and_cache_has_sensitive_content", func(t *testing.T) {
		mockModClient := mock_rpc.NewMockModerationSvcClient(t)
		mockCacheClient := mock_cache.NewMockRedisClient(t)
		modImpl := &moderationImpl{
			cacheClient:  mockCacheClient,
			modSvcClient: mockModClient,
		}

		sensitiveChunk := "this is a sensitive chunk of content"
		safeContent := strings.Repeat("safe content. ", 200)
		testContent := sensitiveChunk + ". " + safeContent

		chunkHash := md5.Sum([]byte(sensitiveChunk))
		cacheKey := moderationCachePrefix + fmt.Sprintf("%x", chunkHash)

		sensitiveResult := &rpc.CheckResult{IsSensitive: true, Reason: "contains inappropriate content"}
		resultJSON, _ := json.Marshal(sensitiveResult)
		mockCacheClient.EXPECT().Get(ctx, cacheKey).Return(string(resultJSON), nil).Once()

		result, err := modImpl.CheckChatPrompts(ctx, []openai.ChatCompletionMessageParamUnion{
			{
				OfSystem: &openai.ChatCompletionSystemMessageParam{
					Content: openai.ChatCompletionSystemMessageParamContentUnion{
						OfString: param.Opt[string]{Value: testContent},
					},
				},
			},
		}, key)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.IsSensitive)
		assert.Equal(t, "contains inappropriate content", result.Reason)
		mockCacheClient.AssertExpectations(t)
		mockModClient.AssertExpectations(t)
	})

	// case 2: cache failed
	t.Run("cache_get_error_but_does_not_affect_overall_functionality", func(t *testing.T) {
		mockModClient := mock_rpc.NewMockModerationSvcClient(t)
		mockCacheClient := mock_cache.NewMockRedisClient(t)

		modImpl := &moderationImpl{
			cacheClient:  mockCacheClient,
			modSvcClient: mockModClient,
		}
		testChunk := "this is a test chunk of content"
		testContent := testChunk + ". " + strings.Repeat("y", slidingWindowSize*2)

		chunkHash := md5.Sum([]byte(testChunk))
		cacheKey1 := moderationCachePrefix + fmt.Sprintf("%x", chunkHash)
		cacheKey2 := moderationCachePrefix + fmt.Sprintf("%x", md5.Sum([]byte(strings.Repeat("y", slidingWindowSize))))

		mockCacheClient.EXPECT().Get(mock.Anything, cacheKey1).Return("", errors.New("cache error"))

		mockCacheClient.EXPECT().Get(mock.Anything, cacheKey2).Return("", errors.New("cache error"))

		mockModClient.EXPECT().PassLLMPromptCheck(mock.Anything, mock.Anything, key).
			Return(&rpc.CheckResult{IsSensitive: false}, nil)

		mockCacheClient.EXPECT().SetEx(mock.Anything, cacheKey1, mock.Anything, cacheTTL).
			Return(nil)
		mockCacheClient.EXPECT().SetEx(mock.Anything, cacheKey2, mock.Anything, cacheTTL).
			Return(nil)
		result, err := modImpl.CheckChatPrompts(ctx, []openai.ChatCompletionMessageParamUnion{
			{
				OfSystem: &openai.ChatCompletionSystemMessageParam{
					Content: openai.ChatCompletionSystemMessageParamContentUnion{
						OfString: param.Opt[string]{Value: testContent},
					},
				},
			},
		}, key)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.IsSensitive)
	})
}

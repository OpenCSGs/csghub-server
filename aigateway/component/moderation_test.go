package component

import (
	"context"
	"strings"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/common/config"
	commontypes "opencsg.com/csghub-server/common/types"
)

// MockModerationSvcClient is a mock of rpc.ModerationSvcClient
type MockModerationSvcClient struct {
	mock.Mock
}

func (m *MockModerationSvcClient) PassLLMRespCheck(ctx context.Context, req commontypes.LLMCheckRequest) (*rpc.CheckResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) != nil {
		return args.Get(0).(*rpc.CheckResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockModerationSvcClient) PassLLMPromptCheck(ctx context.Context, req commontypes.LLMCheckRequest) (*rpc.CheckResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) != nil {
		return args.Get(0).(*rpc.CheckResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockModerationSvcClient) PassTextCheck(ctx context.Context, scenario commontypes.SensitiveScenario, text string) (*rpc.CheckResult, error) {
	args := m.Called(ctx, scenario, text)
	if args.Get(0) != nil {
		return args.Get(0).(*rpc.CheckResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockModerationSvcClient) PassImageCheck(ctx context.Context, scenario commontypes.SensitiveScenario, ossBucketName, ossObjectName string) (*rpc.CheckResult, error) {
	args := m.Called(ctx, scenario, ossBucketName, ossObjectName)
	if args.Get(0) != nil {
		return args.Get(0).(*rpc.CheckResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockModerationSvcClient) PassImageURLCheck(ctx context.Context, scenario commontypes.SensitiveScenario, imageURL string) (*rpc.CheckResult, error) {
	args := m.Called(ctx, scenario, imageURL)
	if args.Get(0) != nil {
		return args.Get(0).(*rpc.CheckResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockModerationSvcClient) SubmitRepoCheck(ctx context.Context, repoType commontypes.RepositoryType, namespace, name string) error {
	args := m.Called(ctx, repoType, namespace, name)
	return args.Error(0)
}

// MockStreamChecker is a mock of StreamChecker
type MockStreamChecker struct {
	mock.Mock
}

func (m *MockStreamChecker) CheckChatStreamResponse(ctx context.Context, chunk types.ChatCompletionChunk, uuid string) (*rpc.CheckResult, error) {
	args := m.Called(ctx, chunk, uuid)
	if args.Get(0) != nil {
		return args.Get(0).(*rpc.CheckResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStreamChecker) CloseStreamCheck(ctx context.Context, uuid string) (*rpc.CheckResult, error) {
	args := m.Called(ctx, uuid)
	if args.Get(0) != nil {
		return args.Get(0).(*rpc.CheckResult), args.Error(1)
	}
	return nil, args.Error(1)
}

func TestSyncStreamChecker_CheckChatStreamResponse(t *testing.T) {
	ctx := context.Background()
	mockSvcClient := new(MockModerationSvcClient)

	modImpl := &moderationImpl{
		modSvcClient: mockSvcClient,
	}

	checker := &syncStreamChecker{
		modImpl: modImpl,
	}

	t.Run("empty chunk", func(t *testing.T) {
		res, err := checker.CheckChatStreamResponse(ctx, types.ChatCompletionChunk{}, "uuid-1")
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
	})

	t.Run("normal text check pass", func(t *testing.T) {
		chunk := types.ChatCompletionChunk{
			Choices: []types.ChatCompletionChunkChoice{
				{Delta: types.ChatCompletionChunkChoiceDelta{Content: "hello"}},
			},
		}

		mockSvcClient.On("PassLLMRespCheck", ctx, commontypes.LLMCheckRequest{
			Scenario:  commontypes.ScenarioLLMResModeration,
			Text:      "hello",
			SessionId: "uuid-2",
			Resumable: true,
			Stream:    true,
		}).Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		res, err := checker.CheckChatStreamResponse(ctx, chunk, "uuid-2")

		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
		mockSvcClient.AssertExpectations(t)
	})

	t.Run("sensitive text block", func(t *testing.T) {
		chunk := types.ChatCompletionChunk{
			Choices: []types.ChatCompletionChunkChoice{
				{Delta: types.ChatCompletionChunkChoiceDelta{Content: "bad words"}},
			},
		}

		mockSvcClient.On("PassLLMRespCheck", ctx, commontypes.LLMCheckRequest{
			Scenario:  commontypes.ScenarioLLMResModeration,
			Text:      "bad words",
			SessionId: "uuid-3",
			Resumable: true,
			Stream:    true,
		}).Return(&rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil).Once()
		res, err := checker.CheckChatStreamResponse(ctx, chunk, "uuid-3")

		assert.NoError(t, err)
		assert.True(t, res.IsSensitive)
		assert.Equal(t, "toxic", res.Reason)
		mockSvcClient.AssertExpectations(t)
	})
}

func TestSyncStreamChecker_CloseStreamCheck(t *testing.T) {
	checker := &syncStreamChecker{}
	res, err := checker.CloseStreamCheck(context.Background(), "uuid-1")
	assert.NoError(t, err)
	assert.False(t, res.IsSensitive)
}

func TestAsyncStreamChecker_CheckChatStreamResponse(t *testing.T) {
	ctx := context.Background()
	mockSvcClient := new(MockModerationSvcClient)

	modImpl := &moderationImpl{
		modSvcClient: mockSvcClient,
	}

	sessionCache, _ := lru.New[string, *sessionState](100)

	checker := &asyncStreamChecker{
		modImpl:      modImpl,
		sessionCache: sessionCache,
		maxChars:     10,
	}

	t.Run("empty chunk", func(t *testing.T) {
		res, err := checker.CheckChatStreamResponse(ctx, types.ChatCompletionChunk{}, "uuid-1")
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
	})

	t.Run("accumulate chunks", func(t *testing.T) {
		// First chunk - short, should not trigger check
		chunk1 := types.ChatCompletionChunk{
			Choices: []types.ChatCompletionChunkChoice{
				{Delta: types.ChatCompletionChunkChoiceDelta{Content: "hello"}},
			},
		}

		res1, err := checker.CheckChatStreamResponse(ctx, chunk1, "uuid-2")
		assert.NoError(t, err)
		assert.False(t, res1.IsSensitive)

		// Second chunk - total length > maxChars, should trigger async check
		chunk2 := types.ChatCompletionChunk{
			Choices: []types.ChatCompletionChunkChoice{
				{Delta: types.ChatCompletionChunkChoiceDelta{Content: " world"}},
			},
		}

		// Setup mock for the async call
		mockSvcClient.On("PassLLMRespCheck", mock.Anything, commontypes.LLMCheckRequest{
			Scenario:  commontypes.ScenarioLLMResModeration,
			Text:      "hello world",
			SessionId: "uuid-2",
			Resumable: true,
			Stream:    true,
		}).Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()

		res2, err := checker.CheckChatStreamResponse(ctx, chunk2, "uuid-2")
		assert.NoError(t, err)
		assert.False(t, res2.IsSensitive)

		// Wait a bit for the async goroutine to complete
		time.Sleep(100 * time.Millisecond)
		mockSvcClient.AssertExpectations(t)
	})

	t.Run("sensitive async result updates cache", func(t *testing.T) {
		chunk1 := types.ChatCompletionChunk{
			Choices: []types.ChatCompletionChunkChoice{
				{Delta: types.ChatCompletionChunkChoiceDelta{Content: "very bad word here"}},
			},
		}

		mockSvcClient.On("PassLLMRespCheck", mock.Anything, commontypes.LLMCheckRequest{
			Scenario:  commontypes.ScenarioLLMResModeration,
			Text:      "very bad word here",
			SessionId: "uuid-3",
			Resumable: true,
			Stream:    true,
		}).Return(&rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil).Once()

		res1, err := checker.CheckChatStreamResponse(ctx, chunk1, "uuid-3")
		assert.NoError(t, err)
		assert.False(t, res1.IsSensitive) // Initial response is always non-sensitive while async check runs

		// Wait for async check to complete and update cache
		time.Sleep(100 * time.Millisecond)
		mockSvcClient.AssertExpectations(t)

		// Next chunk should be blocked immediately based on cache
		chunk2 := types.ChatCompletionChunk{
			Choices: []types.ChatCompletionChunkChoice{
				{Delta: types.ChatCompletionChunkChoiceDelta{Content: "more"}},
			},
		}

		res2, err := checker.CheckChatStreamResponse(ctx, chunk2, "uuid-3")
		assert.NoError(t, err)
		assert.True(t, res2.IsSensitive)
		assert.Equal(t, "toxic", res2.Reason)
	})
}

func TestAsyncStreamChecker_CloseStreamCheck(t *testing.T) {
	ctx := context.Background()
	mockSvcClient := new(MockModerationSvcClient)

	modImpl := &moderationImpl{
		modSvcClient: mockSvcClient,
	}

	sessionCache, _ := lru.New[string, *sessionState](100)

	checker := &asyncStreamChecker{
		modImpl:      modImpl,
		sessionCache: sessionCache,
		maxChars:     10,
	}

	t.Run("not in cache", func(t *testing.T) {
		res, err := checker.CloseStreamCheck(ctx, "uuid-unknown")
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
	})

	t.Run("check remaining buffer", func(t *testing.T) {
		// Put something in buffer first
		chunk := types.ChatCompletionChunk{
			Choices: []types.ChatCompletionChunkChoice{
				{Delta: types.ChatCompletionChunkChoiceDelta{Content: "short"}},
			},
		}

		res1, err := checker.CheckChatStreamResponse(ctx, chunk, "uuid-4")
		assert.NoError(t, err)
		assert.False(t, res1.IsSensitive)

		// Close should trigger check on remaining "short"
		mockSvcClient.On("PassLLMRespCheck", ctx, commontypes.LLMCheckRequest{
			Scenario:  commontypes.ScenarioLLMResModeration,
			Text:      "short",
			SessionId: "uuid-4",
			Resumable: false,
			Stream:    true,
		}).Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()

		res2, err := checker.CloseStreamCheck(ctx, "uuid-4")
		assert.NoError(t, err)
		assert.False(t, res2.IsSensitive)
		mockSvcClient.AssertExpectations(t)

		// Verify it was removed from cache
		_, exists := sessionCache.Get("uuid-4")
		assert.False(t, exists)
	})

	t.Run("already marked sensitive", func(t *testing.T) {
		// Create a sensitive state directly in cache
		state := &sessionState{
			sensitive: true,
			reason:    "toxic",
		}
		sessionCache.Add("uuid-5", state)

		res, err := checker.CloseStreamCheck(ctx, "uuid-5")
		assert.NoError(t, err)
		assert.True(t, res.IsSensitive)
		assert.Equal(t, "toxic", res.Reason)
	})
}

func TestModerationImpl_CheckChatStreamResponse(t *testing.T) {
	ctx := context.Background()
	mockChecker := new(MockStreamChecker)
	modImpl := &moderationImpl{
		streamChecker: mockChecker,
	}

	chunk := types.ChatCompletionChunk{ID: "test-id"}
	uuid := "uuid-1"
	expectedResult := &rpc.CheckResult{IsSensitive: true, Reason: "toxic"}

	mockChecker.On("CheckChatStreamResponse", ctx, chunk, uuid).Return(expectedResult, nil).Once()

	res, err := modImpl.CheckChatStreamResponse(ctx, chunk, uuid)

	assert.NoError(t, err)
	assert.Equal(t, expectedResult, res)
	mockChecker.AssertExpectations(t)
}

func TestModerationImpl_CloseStreamCheck(t *testing.T) {
	ctx := context.Background()
	mockChecker := new(MockStreamChecker)
	modImpl := &moderationImpl{
		streamChecker: mockChecker,
	}

	uuid := "uuid-1"
	expectedResult := &rpc.CheckResult{IsSensitive: false}

	mockChecker.On("CloseStreamCheck", ctx, uuid).Return(expectedResult, nil).Once()

	res, err := modImpl.CloseStreamCheck(ctx, uuid)

	assert.NoError(t, err)
	assert.Equal(t, expectedResult, res)
	mockChecker.AssertExpectations(t)
}

func TestInitStreamChecker(t *testing.T) {
	t.Run("sync mode", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.SensitiveCheck.StreamCheckMode = StreamCheckModeSync
		modImpl := &moderationImpl{config: cfg}

		initStreamChecker(modImpl)

		_, ok := modImpl.streamChecker.(*syncStreamChecker)
		assert.True(t, ok)
	})

	t.Run("async mode", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.SensitiveCheck.StreamCheckMode = StreamCheckModeAsync
		modImpl := &moderationImpl{config: cfg}

		initStreamChecker(modImpl)

		checker, ok := modImpl.streamChecker.(*asyncStreamChecker)
		assert.True(t, ok)
		assert.NotNil(t, checker.sessionCache)
		assert.Equal(t, DefaultAsyncBufferMaxChars, checker.maxChars)
	})

	t.Run("async mode with custom max chars", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.SensitiveCheck.StreamCheckMode = StreamCheckModeAsync
		cfg.SensitiveCheck.AsyncBufferMaxChars = 100
		modImpl := &moderationImpl{config: cfg}

		initStreamChecker(modImpl)

		checker, ok := modImpl.streamChecker.(*asyncStreamChecker)
		assert.True(t, ok)
		assert.Equal(t, 100, checker.maxChars)
	})
}

func TestModerationImpl_checkLLMPrompt(t *testing.T) {
	ctx := context.Background()
	mockSvcClient := new(MockModerationSvcClient)

	modImpl := &moderationImpl{
		modSvcClient:     mockSvcClient,
		maxContentLength: 10,
	}

	t.Run("short content", func(t *testing.T) {
		mockSvcClient.ExpectedCalls = nil
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		
		res, err := modImpl.checkLLMPrompt(ctx, "short", "test-key", false)
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
	})

	t.Run("long content chunking", func(t *testing.T) {
		mockSvcClient.ExpectedCalls = nil
		// 20 chars, max length is 10, so it will be chunked
		// splitContentIntoChunksByWindow logic: if chunk size is maxContentLength (10)?
		// wait, splitContentIntoChunksByWindow splits by 2000! 
		// Actually, splitContentIntoChunksByWindow has slidingWindowSize = 2000 hardcoded in moderation.go
		
		// If we use 3000 chars, it will be chunked
		modImpl.maxContentLength = 2000
		longText := strings.Repeat("a", 3000)
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).Return(&rpc.CheckResult{IsSensitive: false}, nil)
		
		res, err := modImpl.checkLLMPrompt(ctx, longText, "test-key", false)
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
	})
}

func TestModerationImpl_CheckChatPrompts(t *testing.T) {
	ctx := context.Background()
	mockSvcClient := new(MockModerationSvcClient)

	modImpl := &moderationImpl{
		modSvcClient:     mockSvcClient,
		maxContentLength: 2000,
	}

	t.Run("nil modSvcClient", func(t *testing.T) {
		emptyModImpl := &moderationImpl{modSvcClient: nil}
		res, err := emptyModImpl.CheckChatPrompts(ctx, nil, "uuid", false)
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
	})

	t.Run("normal message", func(t *testing.T) {
		mockSvcClient.ExpectedCalls = nil
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		
		messages := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Hello"),
		}
		
		res, err := modImpl.CheckChatPrompts(ctx, messages, "uuid", false)
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
	})

	t.Run("sensitive message", func(t *testing.T) {
		mockSvcClient.ExpectedCalls = nil
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).Return(&rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil).Once()
		
		messages := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Bad words"),
		}
		
		res, err := modImpl.CheckChatPrompts(ctx, messages, "uuid", false)
		assert.NoError(t, err)
		assert.True(t, res.IsSensitive)
		assert.Equal(t, "toxic", res.Reason)
	})
}

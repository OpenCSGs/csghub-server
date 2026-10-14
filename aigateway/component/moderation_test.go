package component

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
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
	if rf, ok := args.Get(0).(func(context.Context, commontypes.LLMCheckRequest) (*rpc.CheckResult, error)); ok {
		return rf(ctx, req)
	}
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

func (m *MockModerationSvcClient) SubmitMediaModeration(ctx context.Context, req commontypes.MediaModerationRequest) (*commontypes.MediaModerationSubmission, error) {
	args := m.Called(ctx, req)
	if args.Get(0) != nil {
		return args.Get(0).(*commontypes.MediaModerationSubmission), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockModerationSvcClient) QueryMediaModerationResult(ctx context.Context, req commontypes.MediaModerationRequest) (*commontypes.MediaModerationResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) != nil {
		return args.Get(0).(*commontypes.MediaModerationResult), args.Error(1)
	}
	return nil, args.Error(1)
}

// MockStreamChecker is a mock of StreamChecker
type MockStreamChecker struct {
	mock.Mock
}

func (m *MockStreamChecker) CheckStreamResponseText(ctx context.Context, content string, sessionID string) (*rpc.CheckResult, error) {
	args := m.Called(ctx, content, sessionID)
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

func TestSyncStreamChecker_CheckStreamResponseText(t *testing.T) {
	ctx := context.Background()
	mockSvcClient := new(MockModerationSvcClient)

	modImpl := &moderationImpl{
		modSvcClient: mockSvcClient,
	}

	checker := &syncStreamChecker{
		modImpl: modImpl,
	}

	t.Run("empty chunk", func(t *testing.T) {
		res, err := checker.CheckStreamResponseText(ctx, "", "uuid-1")
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
	})

	t.Run("normal text check pass", func(t *testing.T) {
		mockSvcClient.On("PassLLMRespCheck", ctx, commontypes.LLMCheckRequest{
			Scenario:  commontypes.ScenarioLLMResModeration,
			Text:      "hello",
			SessionId: "uuid-2",
			Resumable: true,
			Stream:    true,
		}).Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		res, err := checker.CheckStreamResponseText(ctx, "hello", "uuid-2")

		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
		mockSvcClient.AssertExpectations(t)
	})

	t.Run("sensitive text block", func(t *testing.T) {
		mockSvcClient.On("PassLLMRespCheck", ctx, commontypes.LLMCheckRequest{
			Scenario:  commontypes.ScenarioLLMResModeration,
			Text:      "bad words",
			SessionId: "uuid-3",
			Resumable: true,
			Stream:    true,
		}).Return(&rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil).Once()
		res, err := checker.CheckStreamResponseText(ctx, "bad words", "uuid-3")

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

func TestAsyncStreamChecker_CheckStreamResponseText(t *testing.T) {
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
		res, err := checker.CheckStreamResponseText(ctx, "", "uuid-1")
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
	})

	t.Run("accumulate chunks", func(t *testing.T) {
		res1, err := checker.CheckStreamResponseText(ctx, "hello", "uuid-2")
		assert.NoError(t, err)
		assert.False(t, res1.IsSensitive)

		mockSvcClient.On("PassLLMRespCheck", mock.Anything, commontypes.LLMCheckRequest{
			Scenario:  commontypes.ScenarioLLMResModeration,
			Text:      "hello world",
			SessionId: "uuid-2",
			Resumable: true,
			Stream:    true,
		}).Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()

		res2, err := checker.CheckStreamResponseText(ctx, " world", "uuid-2")
		assert.NoError(t, err)
		assert.False(t, res2.IsSensitive)

		// Wait a bit for the async goroutine to complete
		time.Sleep(100 * time.Millisecond)
		mockSvcClient.AssertExpectations(t)
	})

	t.Run("sensitive async result updates cache", func(t *testing.T) {
		mockSvcClient.On("PassLLMRespCheck", mock.Anything, commontypes.LLMCheckRequest{
			Scenario:  commontypes.ScenarioLLMResModeration,
			Text:      "very bad word here",
			SessionId: "uuid-3",
			Resumable: true,
			Stream:    true,
		}).Return(&rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil).Once()

		res1, err := checker.CheckStreamResponseText(ctx, "very bad word here", "uuid-3")
		assert.NoError(t, err)
		assert.False(t, res1.IsSensitive) // Initial response is always non-sensitive while async check runs

		// Wait for async check to complete and update cache
		time.Sleep(100 * time.Millisecond)
		mockSvcClient.AssertExpectations(t)

		res2, err := checker.CheckStreamResponseText(ctx, "more", "uuid-3")
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
		res1, err := checker.CheckStreamResponseText(ctx, "short", "uuid-4")
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

	chunk := types.ChatCompletionChunk{
		ID: "test-id",
		Choices: []types.ChatCompletionChunkChoice{{
			Delta: types.ChatCompletionChunkChoiceDelta{
				Content: "hello",
			},
		}},
	}
	uuid := "uuid-1"
	expectedResult := &rpc.CheckResult{IsSensitive: true, Reason: "toxic"}

	mockChecker.On("CheckStreamResponseText", ctx, "hello", uuid).Return(expectedResult, nil).Once()

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

func TestModerationImpl_CheckTextPrompt(t *testing.T) {
	ctx := context.Background()

	t.Run("nil modSvcClient", func(t *testing.T) {
		modImpl := &moderationImpl{modSvcClient: nil}
		res, err := modImpl.CheckText(ctx, types.TextModerationRequest{
			Content: "anything",
			Key:     "uuid",
			Phase:   types.TextModerationPhasePrompt,
			Mode:    types.TextModerationModeNonStream,
		})
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
	})

	t.Run("empty content short-circuits without RPC", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		modImpl := &moderationImpl{modSvcClient: mockSvcClient, maxContentLength: 2000}
		res, err := modImpl.CheckText(ctx, types.TextModerationRequest{
			Content: "   \n  ",
			Key:     "uuid",
			Phase:   types.TextModerationPhasePrompt,
			Mode:    types.TextModerationModeNonStream,
		})
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
		mockSvcClient.AssertNotCalled(t, "PassLLMPromptCheck")
	})

	t.Run("sensitive content", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).
			Return(&rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil).Once()
		modImpl := &moderationImpl{modSvcClient: mockSvcClient, maxContentLength: 2000}

		res, err := modImpl.CheckText(ctx, types.TextModerationRequest{
			Content: "bad content",
			Key:     "uuid",
			Phase:   types.TextModerationPhasePrompt,
			Mode:    types.TextModerationModeNonStream,
		})
		assert.NoError(t, err)
		assert.True(t, res.IsSensitive)
		assert.Equal(t, "toxic", res.Reason)
	})
}

func TestModerationImpl_CheckTextResponse(t *testing.T) {
	ctx := context.Background()

	t.Run("nil modSvcClient", func(t *testing.T) {
		modImpl := &moderationImpl{modSvcClient: nil}
		res, err := modImpl.CheckText(ctx, types.TextModerationRequest{
			Content: "anything",
			Key:     "uuid",
			Phase:   types.TextModerationPhaseResponse,
			Mode:    types.TextModerationModeNonStream,
		})
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
	})

	t.Run("empty content short-circuits without RPC", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		modImpl := &moderationImpl{modSvcClient: mockSvcClient}
		res, err := modImpl.CheckText(ctx, types.TextModerationRequest{
			Content: "",
			Key:     "uuid",
			Phase:   types.TextModerationPhaseResponse,
			Mode:    types.TextModerationModeNonStream,
		})
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
		mockSvcClient.AssertNotCalled(t, "PassTextCheck")
	})

	t.Run("sensitive content", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		mockSvcClient.On("PassTextCheck", mock.Anything, commontypes.ScenarioChatDetection, "bad content").
			Return(&rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil).Once()
		modImpl := &moderationImpl{modSvcClient: mockSvcClient}

		res, err := modImpl.CheckText(ctx, types.TextModerationRequest{
			Content: "bad content",
			Key:     "uuid",
			Phase:   types.TextModerationPhaseResponse,
			Mode:    types.TextModerationModeNonStream,
		})
		assert.NoError(t, err)
		assert.True(t, res.IsSensitive)
		assert.Equal(t, "toxic", res.Reason)
	})
}

func TestModerationImpl_CheckTextStreamResponse(t *testing.T) {
	ctx := context.Background()

	t.Run("nil streamChecker short-circuits", func(t *testing.T) {
		modImpl := &moderationImpl{}
		res, err := modImpl.CheckText(ctx, types.TextModerationRequest{
			Content: "anything",
			Key:     "uuid",
			Phase:   types.TextModerationPhaseResponse,
			Mode:    types.TextModerationModeStream,
		})
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
	})

	t.Run("delegates to streamChecker with text", func(t *testing.T) {
		mockChecker := new(MockStreamChecker)
		modImpl := &moderationImpl{streamChecker: mockChecker}
		expected := &rpc.CheckResult{IsSensitive: true, Reason: "toxic"}

		mockChecker.On("CheckStreamResponseText", ctx, "chunk text", "uuid").Return(expected, nil).Once()

		res, err := modImpl.CheckText(ctx, types.TextModerationRequest{
			Content: "chunk text",
			Key:     "uuid",
			Phase:   types.TextModerationPhaseResponse,
			Mode:    types.TextModerationModeStream,
		})
		assert.NoError(t, err)
		assert.Equal(t, expected, res)
		mockChecker.AssertExpectations(t)
	})
}

func TestModerationImpl_checkLLMPromptChunkLimit(t *testing.T) {
	ctx := context.Background()

	t.Run("within one moderation call stays synchronous", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).
			Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		modImpl := &moderationImpl{
			modSvcClient:     mockSvcClient,
			maxContentLength: 100,
			promptMaxChunks:  3,
		}

		res, err := modImpl.checkLLMPrompt(ctx, strings.Repeat("a", 50), "key", false)
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
		mockSvcClient.AssertExpectations(t)
	})

	t.Run("over one call but within limit runs asynchronously", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		// 25 chars / maxContentLength 10 => 3 estimated calls <= limit 3.
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).
			Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		modImpl := &moderationImpl{
			modSvcClient:     mockSvcClient,
			maxContentLength: 10,
			promptMaxChunks:  3,
		}

		res, err := modImpl.checkLLMPrompt(ctx, strings.Repeat("a", 25), "key", false)
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)

		time.Sleep(100 * time.Millisecond)
		mockSvcClient.AssertExpectations(t)
	})

	t.Run("sensitive verdict from async check does not block the request", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).
			Return(&rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil).Once()
		modImpl := &moderationImpl{
			modSvcClient:     mockSvcClient,
			maxContentLength: 10,
			promptMaxChunks:  3,
		}

		res, err := modImpl.checkLLMPrompt(ctx, strings.Repeat("a", 25), "key", false)
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)

		time.Sleep(100 * time.Millisecond)
		mockSvcClient.AssertExpectations(t)
	})

	t.Run("over chunk limit skips moderation and proceeds", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		// 100 chars / maxContentLength 10 => 10 estimated calls > limit 3.
		modImpl := &moderationImpl{
			modSvcClient:     mockSvcClient,
			maxContentLength: 10,
			promptMaxChunks:  3,
		}

		res, err := modImpl.checkLLMPrompt(ctx, strings.Repeat("a", 100), "key", false)
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
		mockSvcClient.AssertNotCalled(t, "PassLLMPromptCheck")
	})

	t.Run("zero limit keeps legacy synchronous chunking", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		// A 25-char sentence fits one 2000-char chunk; it exceeds
		// maxContentLength 10, so one buffer flush makes a single call.
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).
			Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		modImpl := &moderationImpl{
			modSvcClient:     mockSvcClient,
			maxContentLength: 10,
		}

		res, err := modImpl.checkLLMPrompt(ctx, strings.Repeat("a", 25), "key", false)
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
		mockSvcClient.AssertExpectations(t)
	})
}

func TestModerationImpl_checkSingleChunkCache(t *testing.T) {
	ctx := context.Background()
	content := "some prompt content"
	expectedKey := moderationCachePromptPrefix + fmt.Sprintf("%x", md5.Sum([]byte(content)))

	t.Run("cache miss checks service and stores result under content hash", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		mockCache := mockcache.NewMockRedisClient(t)
		mockCache.On("Get", ctx, expectedKey).Return("", errors.New("redis: nil")).Once()
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).
			Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		cached, _ := json.Marshal(rpc.CheckResult{IsSensitive: false})
		mockCache.On("SetEx", ctx, expectedKey, string(cached), cacheTTL).Return(nil).Once()

		modImpl := &moderationImpl{
			modSvcClient: mockSvcClient,
			cacheClient:  mockCache,
		}
		res, err := modImpl.checkSingleChunk(ctx, content, "key", false)
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
	})

	t.Run("cache hit skips the service call", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		mockCache := mockcache.NewMockRedisClient(t)
		cached, _ := json.Marshal(rpc.CheckResult{IsSensitive: false})
		mockCache.On("Get", ctx, expectedKey).Return(string(cached), nil).Once()

		modImpl := &moderationImpl{
			modSvcClient: mockSvcClient,
			cacheClient:  mockCache,
		}
		res, err := modImpl.checkSingleChunk(ctx, content, "key", false)
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
		mockSvcClient.AssertNotCalled(t, "PassLLMPromptCheck")
	})
}

func TestPromptMaxChunksFromConfig(t *testing.T) {
	assert.Equal(t, 0, promptMaxChunksFromConfig(nil))
	cfg := &config.Config{}
	assert.Equal(t, 0, promptMaxChunksFromConfig(cfg))
	cfg.SensitiveCheck.PromptMaxChunks = 3
	assert.Equal(t, 3, promptMaxChunksFromConfig(cfg))
	cfg.SensitiveCheck.PromptMaxChunks = -1
	assert.Equal(t, 0, promptMaxChunksFromConfig(cfg))
}

func TestModerationImpl_PendingPromptVerdictGatesResponse(t *testing.T) {
	ctx := context.Background()

	newModImpl := func() *moderationImpl {
		mockSvcClient := new(MockModerationSvcClient)
		modImpl := &moderationImpl{
			modSvcClient:        mockSvcClient,
			maxContentLength:    10,
			promptMaxChunks:     3,
			pendingPromptChecks: newPendingPromptCheckCache(),
		}
		// 25 chars / maxContentLength 10 => 3 estimated calls => async path.
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).
			Return(&rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil).Once()
		// Response-text checks polled before the verdict lands pass through.
		mockSvcClient.On("PassTextCheck", mock.Anything, mock.Anything, mock.Anything).
			Return(&rpc.CheckResult{IsSensitive: false}, nil).Maybe()
		return modImpl
	}

	t.Run("async sensitive verdict blocks stream chunks", func(t *testing.T) {
		modImpl := newModImpl()
		streamCtx := types.WithModerationSessionID(ctx, "session-1")
		res, err := modImpl.checkLLMPrompt(streamCtx, strings.Repeat("a", 25), "key", true)
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)

		// The verdict lands asynchronously; once present it gates every
		// chunk check for the same session.
		assert.Eventually(t, func() bool {
			r, err := modImpl.CheckChatStreamResponse(streamCtx, types.ChatCompletionChunk{}, "session-1")
			return err == nil && r.IsSensitive && r.Reason == "toxic"
		}, time.Second, 10*time.Millisecond)

		// Peeking does not consume: a later chunk is still blocked, and the
		// close check reports the verdict and removes it.
		r, err := modImpl.CheckChatStreamResponse(streamCtx, types.ChatCompletionChunk{}, "session-1")
		assert.NoError(t, err)
		assert.True(t, r.IsSensitive)
		closeRes, err := modImpl.CloseStreamCheck(streamCtx, "session-1")
		assert.NoError(t, err)
		assert.True(t, closeRes.IsSensitive)

		// Consumed: a new stream with the same id is not blocked.
		r, err = modImpl.CheckChatStreamResponse(streamCtx, types.ChatCompletionChunk{}, "session-1")
		assert.NoError(t, err)
		assert.False(t, r.IsSensitive)
	})

	t.Run("async sensitive verdict blocks non-stream response once", func(t *testing.T) {
		modImpl := newModImpl()
		nonStreamCtx := types.WithModerationSessionID(ctx, "session-2")
		_, err := modImpl.checkLLMPrompt(nonStreamCtx, strings.Repeat("a", 25), "key", false)
		assert.NoError(t, err)

		completion := types.ChatCompletion{ChatCompletion: openai.ChatCompletion{
			Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: "hello"}}},
		}}

		assert.Eventually(t, func() bool {
			r, err := modImpl.CheckChatNonStreamResponse(nonStreamCtx, completion)
			return err == nil && r.IsSensitive
		}, time.Second, 10*time.Millisecond)

		// Consumed by the non-stream read: a second check passes through to
		// the (mocked) response-text path rather than the pending verdict.
		res, err := modImpl.CheckChatNonStreamResponse(nonStreamCtx, completion)
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
	})

	t.Run("empty-content completion with tool calls awaits and gates", func(t *testing.T) {
		// A non-stream agent response with empty content but tool calls must
		// not bypass the gate: the writer relies on this single call and the
		// client may execute the returned calls.
		mockSvcClient := new(MockModerationSvcClient)
		modImpl := &moderationImpl{
			modSvcClient:        mockSvcClient,
			maxContentLength:    10,
			promptMaxChunks:     3,
			pendingPromptChecks: newPendingPromptCheckCache(),
		}
		release := make(chan struct{})
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).
			Return(func(ctx context.Context, req commontypes.LLMCheckRequest) (*rpc.CheckResult, error) {
				<-release
				return &rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil
			}).Once()

		checkCtx := types.WithModerationSessionID(context.Background(), "session-tool")
		_, err := modImpl.checkLLMPrompt(checkCtx, strings.Repeat("a", 25), "key", false)
		assert.NoError(t, err)

		go func() {
			time.Sleep(100 * time.Millisecond)
			close(release)
		}()
		waitCtx, cancel := context.WithTimeout(
			types.WithModerationSessionID(context.Background(), "session-tool"), 5*time.Second)
		defer cancel()
		completion := types.ChatCompletion{ChatCompletion: openai.ChatCompletion{
			Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{
				ToolCalls: []openai.ChatCompletionMessageToolCallUnion{{
					Function: openai.ChatCompletionMessageFunctionToolCallFunction{Name: "run_shell", Arguments: "{\"cmd\":\"rm -rf /\"}"},
				}},
			}}},
		}}
		r, err := modImpl.CheckChatNonStreamResponse(waitCtx, completion)
		assert.NoError(t, err)
		assert.True(t, r.IsSensitive, "tool-call-only completion must await the verdict and gate")
		mockSvcClient.AssertExpectations(t)
	})

	t.Run("tool call text is submitted to response checking", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		modImpl := &moderationImpl{modSvcClient: mockSvcClient}
		mockSvcClient.On("PassTextCheck", mock.Anything, commontypes.ScenarioChatDetection, "run_shell\n{\"cmd\":\"ls\"}").
			Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()

		completion := types.ChatCompletion{ChatCompletion: openai.ChatCompletion{
			Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{
				ToolCalls: []openai.ChatCompletionMessageToolCallUnion{{
					Function: openai.ChatCompletionMessageFunctionToolCallFunction{Name: "run_shell", Arguments: "{\"cmd\":\"ls\"}"},
				}},
			}}},
		}}
		r, err := modImpl.CheckChatNonStreamResponse(context.Background(), completion)
		assert.NoError(t, err)
		assert.False(t, r.IsSensitive)
		mockSvcClient.AssertExpectations(t)
	})

	t.Run("detached response context still gates via the writer session id", func(t *testing.T) {
		// responses writers check with a context deliberately detached from
		// the request (ModerationContext), so the verdict must also be found
		// through the writer session id passed as Key.
		modImpl := newModImpl()
		promptCtx := types.WithModerationSessionID(ctx, "session-5")
		_, err := modImpl.checkLLMPrompt(promptCtx, strings.Repeat("a", 25), "key", false)
		assert.NoError(t, err)

		detachedCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		assert.Eventually(t, func() bool {
			r, err := modImpl.CheckText(detachedCtx, types.TextModerationRequest{
				Content: "hello",
				Key:     "session-5",
				Phase:   types.TextModerationPhaseResponse,
				Mode:    types.TextModerationModeNonStream,
			})
			return err == nil && r.IsSensitive
		}, time.Second, 10*time.Millisecond)
	})

	t.Run("verdict respects ModerationBypassSensitiveCheck at read time", func(t *testing.T) {
		modImpl := newModImpl()
		modImpl.config = &config.Config{}
		modImpl.config.AIGateway.ModerationBypassSensitiveCheck = true
		bypassCtx := types.WithModerationSessionID(ctx, "session-3")
		_, err := modImpl.checkLLMPrompt(bypassCtx, strings.Repeat("a", 25), "key", true)
		assert.NoError(t, err)

		assert.Eventually(t, func() bool {
			r, _ := modImpl.CheckChatStreamResponse(bypassCtx, types.ChatCompletionChunk{}, "session-3")
			return !r.IsSensitive
		}, time.Second, 10*time.Millisecond)
	})

	t.Run("other sessions are not affected", func(t *testing.T) {
		modImpl := newModImpl()
		otherCtx := types.WithModerationSessionID(ctx, "session-4")
		_, err := modImpl.checkLLMPrompt(otherCtx, strings.Repeat("a", 25), "key", true)
		assert.NoError(t, err)

		assert.Eventually(t, func() bool {
			_, ok := modImpl.peekPendingPromptVerdict(context.Background(), "session-4")
			return ok
		}, time.Second, 10*time.Millisecond)

		// A different request carries a different moderation session id in
		// its own context; it must not be blocked by session-4's verdict.
		otherRequestCtx := types.WithModerationSessionID(ctx, "session-other")
		r, err := modImpl.CheckChatStreamResponse(otherRequestCtx, types.ChatCompletionChunk{}, "session-other")
		assert.NoError(t, err)
		assert.False(t, r.IsSensitive)

		// A request context without any session id is not blocked either.
		r, err = modImpl.CheckChatStreamResponse(ctx, types.ChatCompletionChunk{}, "session-other")
		assert.NoError(t, err)
		assert.False(t, r.IsSensitive)
	})
}

func TestModerationImpl_AwaitPendingPromptVerdictAtFinalization(t *testing.T) {
	t.Run("response finalization waits for an in-flight async verdict", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		modImpl := &moderationImpl{
			modSvcClient:        mockSvcClient,
			maxContentLength:    10,
			promptMaxChunks:     3,
			pendingPromptChecks: newPendingPromptCheckCache(),
		}
		release := make(chan struct{})
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).
			Return(func(ctx context.Context, req commontypes.LLMCheckRequest) (*rpc.CheckResult, error) {
				<-release
				return &rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil
			}).Once()

		checkCtx := types.WithModerationSessionID(context.Background(), "session-wait")
		res, err := modImpl.checkLLMPrompt(checkCtx, strings.Repeat("a", 25), "key", false)
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)

		// The response reaches its finalization boundary while the verdict is
		// still in flight; it must wait for the check and gate on the verdict.
		go func() {
			time.Sleep(100 * time.Millisecond)
			close(release)
		}()
		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		r, err := modImpl.CheckText(waitCtx, types.TextModerationRequest{
			Content: "hello",
			Key:     "session-wait",
			Phase:   types.TextModerationPhaseResponse,
			Mode:    types.TextModerationModeNonStream,
		})
		assert.NoError(t, err)
		assert.True(t, r.IsSensitive, "finalization must gate on the awaited verdict")
		mockSvcClient.AssertExpectations(t)
	})

	t.Run("await grace expires without blocking or cancelling the body check", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		modImpl := &moderationImpl{
			modSvcClient:        mockSvcClient,
			maxContentLength:    10,
			promptMaxChunks:     3,
			pendingPromptChecks: newPendingPromptCheckCache(),
			pendingVerdictWait:  80 * time.Millisecond, // dedicated wait budget
		}
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).
			Return(func(ctx context.Context, req commontypes.LLMCheckRequest) (*rpc.CheckResult, error) {
				<-ctx.Done() // outlives the wait window
				return nil, ctx.Err()
			}).Once()
		// The body check must run on a live context after the wait expired.
		mockSvcClient.On("PassTextCheck", mock.Anything, mock.Anything, mock.Anything).
			Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()

		checkCtx := types.WithModerationSessionID(context.Background(), "session-grace")
		_, err := modImpl.checkLLMPrompt(checkCtx, strings.Repeat("a", 25), "key", false)
		assert.NoError(t, err)

		waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		r, err := modImpl.CheckText(waitCtx, types.TextModerationRequest{
			Content: "hello",
			Key:     "session-grace",
			Phase:   types.TextModerationPhaseResponse,
			Mode:    types.TextModerationModeNonStream,
		})
		assert.NoError(t, err)
		assert.False(t, r.IsSensitive, "grace expiry must let the response proceed")
		mockSvcClient.AssertExpectations(t)
	})
}

func TestModerationImpl_CheckImagePrompts_FullySynchronous(t *testing.T) {
	ctx := context.Background()

	t.Run("oversized image prompt is still checked (no skip tier)", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		// 100 chars / maxContentLength 10 => the chat path would skip (est 10
		// > 3); the image path must run the check synchronously instead: a
		// single 2000-char chunk flushed once.
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).
			Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		modImpl := &moderationImpl{
			modSvcClient:     mockSvcClient,
			maxContentLength: 10,
			promptMaxChunks:  3,
		}

		res, err := modImpl.CheckImagePrompts(ctx, strings.Repeat("a", 100), "key")
		assert.NoError(t, err)
		assert.False(t, res.IsSensitive)
		mockSvcClient.AssertExpectations(t)
	})

	t.Run("sensitive oversized image prompt is blocked synchronously", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).
			Return(&rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil).Once()
		modImpl := &moderationImpl{
			modSvcClient:     mockSvcClient,
			maxContentLength: 10,
			promptMaxChunks:  3,
		}

		res, err := modImpl.CheckImagePrompts(ctx, strings.Repeat("a", 100), "key")
		assert.NoError(t, err)
		assert.True(t, res.IsSensitive)
		assert.Equal(t, "toxic", res.Reason)
	})
}

func TestModerationImpl_AsyncTierReplay(t *testing.T) {
	ctx := context.Background()
	content := strings.Repeat("a", 25) // 25 chars / maxContentLength 10 => est 3 => async tier
	chunkKey := moderationCachePromptPrefix + fmt.Sprintf("%x", md5.Sum([]byte(content)))
	sensitiveJSON, _ := json.Marshal(rpc.CheckResult{IsSensitive: true, Reason: "toxic"})
	safeJSON, _ := json.Marshal(rpc.CheckResult{IsSensitive: false})

	t.Run("replayed sensitive prompt is blocked from the chunk cache", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		mockCache := mockcache.NewMockRedisClient(t)
		mockCache.On("Get", mock.Anything, chunkKey).Return("", errors.New("redis: nil")).Twice()
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).
			Return(&rpc.CheckResult{IsSensitive: true, Reason: "toxic"}, nil).Once()
		mockCache.On("SetEx", mock.Anything, chunkKey, string(sensitiveJSON), cacheTTL).Return(nil).Once()
		// Replay: the pre-scan finds the cached sensitive chunk verdict.
		mockCache.On("Get", mock.Anything, chunkKey).Return(string(sensitiveJSON), nil).Once()

		modImpl := &moderationImpl{
			modSvcClient:        mockSvcClient,
			cacheClient:         mockCache,
			maxContentLength:    10,
			promptMaxChunks:     3,
			pendingPromptChecks: newPendingPromptCheckCache(),
			pendingVerdictWait:  time.Second,
		}

		first, err := modImpl.checkLLMPrompt(ctx, content, "key", false)
		assert.NoError(t, err)
		assert.False(t, first.IsSensitive, "first pass is the async tier")

		time.Sleep(80 * time.Millisecond) // let the background check land

		second, err := modImpl.checkLLMPrompt(ctx, content, "key", false)
		assert.NoError(t, err)
		assert.True(t, second.IsSensitive, "replay must be blocked from the cached chunk verdict")
		mockSvcClient.AssertExpectations(t)
	})

	t.Run("replayed safe prompt returns without another moderation call", func(t *testing.T) {
		mockSvcClient := new(MockModerationSvcClient)
		mockCache := mockcache.NewMockRedisClient(t)
		mockCache.On("Get", mock.Anything, chunkKey).Return("", errors.New("redis: nil")).Twice()
		mockSvcClient.On("PassLLMPromptCheck", mock.Anything, mock.Anything).
			Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()
		mockCache.On("SetEx", mock.Anything, chunkKey, string(safeJSON), cacheTTL).Return(nil).Once()
		mockCache.On("Get", mock.Anything, chunkKey).Return(string(safeJSON), nil).Once()

		modImpl := &moderationImpl{
			modSvcClient:        mockSvcClient,
			cacheClient:         mockCache,
			maxContentLength:    10,
			promptMaxChunks:     3,
			pendingPromptChecks: newPendingPromptCheckCache(),
			pendingVerdictWait:  time.Second,
		}

		first, err := modImpl.checkLLMPrompt(ctx, content, "key", false)
		assert.NoError(t, err)
		assert.False(t, first.IsSensitive)

		time.Sleep(80 * time.Millisecond)

		second, err := modImpl.checkLLMPrompt(ctx, content, "key", false)
		assert.NoError(t, err)
		assert.False(t, second.IsSensitive, "fully cached safe replay passes without RPC")
		mockSvcClient.AssertExpectations(t)
	})
}

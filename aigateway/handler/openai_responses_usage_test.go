package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestNewResponsesTokenCounterWithNilModel(t *testing.T) {
	tester, _, _ := setupTest(t)

	counter := tester.handler.newResponsesTokenCounter(nil)
	require.NotNil(t, counter)

	counter.Request(&types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`)})
	counter.Response(&types.ResponsesResponse{OutputText: "hello"})

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.NotNil(t, usage)
}

func TestNewResponsesTokenCounterBuildsTokenizer(t *testing.T) {
	tester, _, _ := setupTest(t)
	tester.handler.config.AIGateway.ResponsesIDSecret = "responses-secret"

	model := &types.Model{BaseModel: types.BaseModel{ID: "m"}}
	modelTarget := &resolvedModelTarget{
		Model:     model,
		Target:    "http://example.com",
		Host:      "example.com",
		ModelName: "upstream-model",
	}

	counter := tester.handler.newResponsesTokenCounter(modelTarget)
	require.NotNil(t, counter)

	counter.Request(&types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`)})
	counter.Response(&types.ResponsesResponse{OutputText: "hello"})

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.NotNil(t, usage)
}

func TestNewResponsesTokenCounterUsesTokenizerTarget(t *testing.T) {
	tester, _, _ := setupTest(t)
	var tokenizePaths []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenizePaths = append(tokenizePaths, r.URL.Path)
		require.Equal(t, "/tokenize", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":3}`))
	}))
	defer upstream.Close()

	model := &types.Model{
		BaseModel: types.BaseModel{ID: "m"},
		InternalModelInfo: types.InternalModelInfo{
			ImageID: "vllm-local:latest",
		},
	}
	modelTarget := &resolvedModelTarget{
		Model:           model,
		Target:          upstream.URL + "/v1/responses",
		TokenizerTarget: upstream.URL,
		ModelName:       "upstream-model",
	}

	counter := tester.handler.newResponsesTokenCounter(modelTarget)
	counter.Request(&types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`)})
	counter.Response(&types.ResponsesResponse{OutputText: "ok"})

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(3), usage.PromptTokens)
	require.Equal(t, int64(3), usage.CompletionTokens)
	require.Equal(t, int64(6), usage.TotalTokens)
	require.Equal(t, []string{"/tokenize", "/tokenize"}, tokenizePaths)
}

func TestRecordResponsesUsageHappyPathCallsComponent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tester, c, _ := setupTest(t)
		tester.mocks.openAIComp.ExpectedCalls = nil
		c.Request = httptest.NewRequest("POST", "/v1/responses", nil)

		model := &types.Model{BaseModel: types.BaseModel{ID: "m"}}
		modelTarget := &resolvedModelTarget{Model: model, ModelName: "upstream"}

		counter := token.NewResponsesTokenCounter(&token.DumyTokenizer{})
		counter.Request(&types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`)})
		counter.Response(&types.ResponsesResponse{
			OutputText: "world",
			Output: []types.ResponsesOutputItem{{
				Type:    "message",
				Content: []types.ResponsesContentPart{{Type: "output_text", Text: "world"}},
			}},
		})

		var wg sync.WaitGroup
		wg.Add(2)
		var seenUsage *token.Usage
		tester.mocks.openAIComp.EXPECT().CommitUsageLimitFromUsage(mock.Anything, "testuuid", model, mock.Anything).
			RunAndReturn(func(_ context.Context, _ string, _ *types.Model, _ *token.Usage) error {
				wg.Done()
				return nil
			}).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(
			mock.Anything, "testuuid", model, "upstream", mock.Anything, "apikey",
		).RunAndReturn(func(_ context.Context, _ string, _ *types.Model, _ string, usage *token.Usage, _ string) error {
			seenUsage = usage
			wg.Done()
			return nil
		}).Once()

		tester.handler.recordResponsesUsageWithTrace(c, counter, nil, "testuuid", modelTarget, "apikey", nil, responsesTracePostProcessInput{StatusCode: http.StatusOK})

		synctest.Wait()
		require.NotNil(t, seenUsage)
		require.Greater(t, seenUsage.CompletionTokens, int64(0))
	})
}

func TestRecordResponsesUsagePublishesLLMLog(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tester, c, _ := setupTest(t)
		tester.mocks.openAIComp.ExpectedCalls = nil
		c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
		tester.handler.config.AIGateway.EnableLLMLog = true
		publisher := &captureLLMLogPublisher{}
		tester.handler.llmLogPublisher = publisher

		model := &types.Model{BaseModel: types.BaseModel{ID: "m"}}
		modelTarget := &resolvedModelTarget{Model: model, ModelName: "upstream"}

		counter := token.NewResponsesTokenCounter(&token.DumyTokenizer{})
		counter.Request(&types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`)})
		counter.Response(&types.ResponsesResponse{
			Object:     "response",
			OutputText: "world",
			Output: []types.ResponsesOutputItem{{
				Type: "message",
				Role: "assistant",
				Content: []types.ResponsesContentPart{{
					Type: "output_text",
					Text: "world",
				}},
			}},
		})
		recorder, err := responsespkg.NewLLMLogRecorder("req-1", "upstream", "testuuid", &types.ResponsesRequest{
			Model: "m",
			Input: json.RawMessage(`"hi"`),
		}, map[string]any{"api": "/v1/responses"})
		require.NoError(t, err)
		recorder.CaptureResponse(&types.ResponsesResponse{
			Object:     "response",
			OutputText: "world",
			Output: []types.ResponsesOutputItem{{
				Type:    "message",
				Role:    "assistant",
				Content: []types.ResponsesContentPart{{Type: "output_text", Text: "world"}},
			}},
		})

		var wg sync.WaitGroup
		wg.Add(3)
		tester.mocks.openAIComp.EXPECT().CommitUsageLimitFromUsage(mock.Anything, "testuuid", model, mock.Anything).
			RunAndReturn(func(_ context.Context, _ string, _ *types.Model, _ *token.Usage) error {
				wg.Done()
				return nil
			}).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(
			mock.Anything, "testuuid", model, "upstream", mock.Anything, "apikey",
		).RunAndReturn(func(_ context.Context, _ string, _ *types.Model, _ string, _ *token.Usage, _ string) error {
			wg.Done()
			return nil
		}).Once()
		publisher.onPublish = func(_ []byte) {
			wg.Done()
		}

		tester.handler.recordResponsesUsageWithTrace(c, counter, nil, "testuuid", modelTarget, "apikey", recorder, responsesTracePostProcessInput{StatusCode: http.StatusOK})

		synctest.Wait()
		require.NotNil(t, publisher.payload)
		var record commontypes.LLMLogRecord
		require.NoError(t, json.Unmarshal(publisher.payload, &record))
		require.Equal(t, "responses", record.SampleType)
		require.Equal(t, "upstream", record.ModelID)
		require.Equal(t, "testuuid", record.UserUUID)
		require.Equal(t, []string{"user", "assistant"}, llmLogRoles(record.Messages))
		require.Equal(t, "hi", record.Messages[0].Content)
		require.Equal(t, "world", record.Messages[1].Content)
		require.Greater(t, record.Usage.TotalTokens, int64(0))
	})
}

func TestRecordResponsesUsageRecordsLLMTrace(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tester, c, _ := setupTest(t)
		tester.mocks.openAIComp.ExpectedCalls = nil
		c.Request = httptest.NewRequest("POST", "/v1/responses", nil)

		model := &types.Model{BaseModel: types.BaseModel{ID: "m"}, ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"}}
		modelTarget := &resolvedModelTarget{Model: model, ModelName: "upstream"}
		counter := token.NewResponsesTokenCounter(&token.DumyTokenizer{})
		counter.Request(&types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`)})
		counter.Response(&types.ResponsesResponse{OutputText: "world"})
		recorder, err := responsespkg.NewLLMLogRecorder("req-1", "upstream", "testuuid", &types.ResponsesRequest{
			Model: "m",
			Input: json.RawMessage(`"hi"`),
		}, nil)
		require.NoError(t, err)
		recorder.CaptureResponse(&types.ResponsesResponse{
			ID:         "resp-1",
			Object:     "response",
			Status:     "completed",
			OutputText: "world",
		})

		traceRecorder := &testGenerationRecorderWithMutex{}
		traceInput := responsesTracePostProcessInput{
			Recorder:   traceRecorder,
			Completion: true,
			StatusCode: http.StatusOK,
		}

		var wg sync.WaitGroup
		wg.Add(2)
		tester.mocks.openAIComp.EXPECT().CommitUsageLimitFromUsage(mock.Anything, "testuuid", model, mock.Anything).
			RunAndReturn(func(_ context.Context, _ string, _ *types.Model, _ *token.Usage) error {
				wg.Done()
				return nil
			}).Once()
		tester.mocks.openAIComp.EXPECT().RecordUsageFromTokenUsage(
			mock.Anything, "testuuid", model, "upstream", mock.Anything, "apikey",
		).RunAndReturn(func(_ context.Context, _ string, _ *types.Model, _ string, _ *token.Usage, _ string) error {
			wg.Done()
			return nil
		}).Once()

		tester.handler.recordResponsesUsageWithTrace(c, counter, nil, "testuuid", modelTarget, "apikey", recorder, traceInput)

		synctest.Wait()
		usage, usageEnded, usageEvents := traceRecorder.snapshot()
		response, _, errorCode, ended, events := traceRecorder.traceSnapshot()
		require.NotNil(t, usage)
		require.NotNil(t, response)
		require.Equal(t, "openai", response.Provider)
		require.Equal(t, "upstream", response.Model)
		require.Equal(t, "resp-1", response.ResponseID)
		require.Empty(t, response.FinishReasons)
		require.Len(t, response.Input, 1)
		require.Len(t, response.Output, 1)
		require.Empty(t, errorCode)
		require.True(t, ended)
		require.True(t, usageEnded)
		require.Contains(t, events, "response")
		require.Contains(t, events, "end")
		require.Contains(t, usageEvents, "usage")
	})
}

func TestRecordResponsesUsageSkipsBillingOnErrorStatus(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tester, c, _ := setupTest(t)
		tester.mocks.openAIComp.ExpectedCalls = nil
		c.Request = httptest.NewRequest("POST", "/v1/responses", nil)

		model := &types.Model{BaseModel: types.BaseModel{ID: "m"}}
		modelTarget := &resolvedModelTarget{Model: model, ModelName: "upstream"}

		counter := token.NewResponsesTokenCounter(&token.DumyTokenizer{})
		counter.Request(&types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`)})
		counter.Response(&types.ResponsesResponse{OutputText: "world"})

		var billingCalled atomic.Bool
		tester.mocks.openAIComp.EXPECT().
			CommitUsageLimitFromUsage(mock.Anything, "testuuid", model, mock.Anything).
			Return(nil).
			Once()
		tester.mocks.openAIComp.EXPECT().
			RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, "upstream", mock.Anything, "apikey").
			Run(func(context.Context, string, *types.Model, string, *token.Usage, string) {
				billingCalled.Store(true)
			}).
			Maybe()

		tester.handler.recordResponsesUsageWithTrace(c, counter, nil, "testuuid", modelTarget, "apikey", nil, responsesTracePostProcessInput{
			StatusCode: http.StatusInternalServerError,
		})

		// synctest.Wait blocks until the async post-processing goroutine has
		// fully completed, so the billing flag is settled.
		synctest.Wait()
		require.False(t, billingCalled.Load())
	})
}

func TestRecordResponsesUsageSkipsBillingOnRedirectStatus(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tester, c, _ := setupTest(t)
		tester.mocks.openAIComp.ExpectedCalls = nil
		c.Request = httptest.NewRequest("POST", "/v1/responses", nil)

		model := &types.Model{BaseModel: types.BaseModel{ID: "m"}}
		modelTarget := &resolvedModelTarget{Model: model, ModelName: "upstream"}

		counter := token.NewResponsesTokenCounter(&token.DumyTokenizer{})
		counter.Request(&types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`)})
		counter.Response(&types.ResponsesResponse{OutputText: "world"})

		var billingCalled atomic.Bool
		tester.mocks.openAIComp.EXPECT().
			CommitUsageLimitFromUsage(mock.Anything, "testuuid", model, mock.Anything).
			Return(nil).
			Once()
		tester.mocks.openAIComp.EXPECT().
			RecordUsageFromTokenUsage(mock.Anything, "testuuid", model, "upstream", mock.Anything, "apikey").
			Run(func(context.Context, string, *types.Model, string, *token.Usage, string) {
				billingCalled.Store(true)
			}).
			Maybe()

		tester.handler.recordResponsesUsageWithTrace(c, counter, nil, "testuuid", modelTarget, "apikey", nil, responsesTracePostProcessInput{
			StatusCode: http.StatusFound,
		})

		// synctest.Wait blocks until the async post-processing goroutine has
		// fully completed, so the billing flag is settled.
		synctest.Wait()
		require.False(t, billingCalled.Load())
	})
}

func TestSetupResponsesCaptureGatesWhenDisabled(t *testing.T) {
	tester, c, _ := setupTest(t)
	modelTarget := &resolvedModelTarget{
		Model:     &types.Model{BaseModel: types.BaseModel{ID: "m"}},
		ModelName: "upstream",
	}
	req := &types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`)}

	tester.handler.config.AIGateway.EnableLLMLog = false
	tester.handler.llmLogPublisher = &captureLLMLogPublisher{}
	require.Nil(t, tester.handler.setupResponsesCapture(c, req, modelTarget, responsespkg.RoutingDecision{Mode: responsespkg.ResponsesModeChatAdapter}, "testuuid"))

	tester.handler.config.AIGateway.EnableLLMLog = true
	tester.handler.llmLogPublisher = nil
	require.Nil(t, tester.handler.setupResponsesCapture(c, req, modelTarget, responsespkg.RoutingDecision{Mode: responsespkg.ResponsesModeChatAdapter}, "testuuid"))

	tester.handler.config.AIGateway.EnableLLMLog = false
	tester.handler.llmTracer = &testLLMTracerWithMutex{}
	require.NotNil(t, tester.handler.setupResponsesCapture(c, req, modelTarget, responsespkg.RoutingDecision{Mode: responsespkg.ResponsesModeChatAdapter}, "testuuid"))
}

type captureLLMLogPublisher struct {
	payload   []byte
	onPublish func([]byte)
	err       error
}

func (p *captureLLMLogPublisher) PublishTrainingLog(message []byte) error {
	p.payload = append([]byte(nil), message...)
	if p.onPublish != nil {
		p.onPublish(message)
	}
	if p.err != nil {
		return p.err
	}
	return nil
}

func llmLogRoles(messages []commontypes.LLMLogMessage) []string {
	roles := make([]string, 0, len(messages))
	for _, msg := range messages {
		roles = append(roles, msg.Role)
	}
	return roles
}

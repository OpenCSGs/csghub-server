package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
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
				Type: "message",
				Content: []types.ResponsesContentPart{{Type: "output_text", Text: "world"}},
			}},
		})

		var wg sync.WaitGroup
		wg.Add(2)
		var seenUsage *token.Usage
		tester.mocks.openAIComp.EXPECT().CommitUsageLimit(mock.Anything, "testuuid", model, mock.Anything).
			RunAndReturn(func(_ context.Context, _ string, _ *types.Model, _ token.Counter) error {
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

		tester.handler.recordResponsesUsage(c, counter, "testuuid", modelTarget, "apikey")

		synctest.Wait()
		require.NotNil(t, seenUsage)
		require.Greater(t, seenUsage.CompletionTokens, int64(0))
	})
}
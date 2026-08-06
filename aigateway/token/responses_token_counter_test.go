package token

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestResponsesTokenCounterPrefersUpstreamUsage(t *testing.T) {
	counter := NewResponsesTokenCounter(&DumyTokenizer{})
	counter.Request(&types.ResponsesRequest{
		Model: "m",
		Input: json.RawMessage(`"prompt that should not be counted"`),
	})
	counter.Response(&types.ResponsesResponse{
		Usage: &types.ResponsesUsage{
			InputTokens:  5,
			OutputTokens: 7,
			TotalTokens:  12,
			InputTokensDetails: &types.ResponsesInputTokenDetails{
				CachedTokens: 3,
			},
			OutputTokensDetails: &types.ResponsesOutputTokenDetails{
				ReasoningTokens: 4,
			},
		},
		Output: []types.ResponsesOutputItem{{
			Type: "message",
			Content: []types.ResponsesContentPart{{
				Type: "output_text",
				Text: "output that should not be counted",
			}},
		}},
	})

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(5), usage.PromptTokens)
	require.Equal(t, int64(7), usage.CompletionTokens)
	require.Equal(t, int64(12), usage.TotalTokens)
	require.Equal(t, int64(3), usage.CachedPromptTokens)
	require.Equal(t, int64(4), usage.ReasoningTokens)
}

func TestResponsesTokenCounterEstimatesNonStreamResponse(t *testing.T) {
	counter := NewResponsesTokenCounter(&DumyTokenizer{})
	counter.Request(&types.ResponsesRequest{
		Model:        "m",
		Instructions: json.RawMessage(`"sys"`),
		Input:        json.RawMessage(`"hello"`),
	})
	counter.Response(&types.ResponsesResponse{
		OutputText: "hello world",
		Output: []types.ResponsesOutputItem{{
			Type: "message",
			Content: []types.ResponsesContentPart{{
				Type: "output_text",
				Text: "hello world",
			}},
		}},
	})

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(len("sys\nhello")), usage.PromptTokens)
	require.Equal(t, int64(len("hello world")), usage.CompletionTokens)
	require.Equal(t, usage.PromptTokens+usage.CompletionTokens, usage.TotalTokens)
}

func TestResponsesTokenCounterApproximatesUnsupportedTokenizer(t *testing.T) {
	counter := NewResponsesTokenCounter(NewTokenizerImpl("http://example.com", "", "m", "sglang", ""))
	counter.Request(&types.ResponsesRequest{
		Model:        "m",
		Instructions: json.RawMessage(`"sys"`),
		Input:        json.RawMessage(`"hello"`),
	})
	counter.Response(&types.ResponsesResponse{OutputText: "world"})

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, approxTokensByText("sys\nhello"), usage.PromptTokens)
	require.Equal(t, approxTokensByText("world"), usage.CompletionTokens)
	require.Equal(t, usage.PromptTokens+usage.CompletionTokens, usage.TotalTokens)
	require.Equal(t, "approx_chars_div_4", usage.Source)
	require.Equal(t, "unsupported_tokenizer", usage.SourceReason)
}

func TestResponsesTokenCounterApproximatesTokenizerError(t *testing.T) {
	counter := NewResponsesTokenCounter(responsesFailingTokenizer{err: errors.New("tokenize failed")})
	counter.Request(&types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hello"`)})
	counter.Response(&types.ResponsesResponse{OutputText: "world"})

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, "approx_chars_div_4", usage.Source)
	require.Equal(t, "tokenizer_error", usage.SourceReason)
}

func TestResponsesTokenCounterApproximatesCanceledTokenizer(t *testing.T) {
	counter := NewResponsesTokenCounter(responsesBlockingTokenizer{})
	counter.Request(&types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hello"`)})
	counter.Response(&types.ResponsesResponse{OutputText: "world"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	usage, err := counter.Usage(ctx)
	require.NoError(t, err)
	require.Equal(t, "approx_chars_div_4", usage.Source)
	require.Equal(t, "tokenizer_canceled", usage.SourceReason)
}

func TestResponsesTokenCounterEstimatesStreamEventsWithoutDoubleCountingDonePayloads(t *testing.T) {
	counter := NewResponsesTokenCounter(&DumyTokenizer{})
	counter.Request(&types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`)})
	counter.AppendEvent(types.ResponsesStreamEvent{
		Type:  "response.output_text.delta",
		Delta: "he",
	})
	counter.AppendEvent(types.ResponsesStreamEvent{
		Type:  "response.output_text.delta",
		Delta: "llo",
	})
	counter.AppendEvent(types.ResponsesStreamEvent{
		Type: "response.content_part.done",
		Part: json.RawMessage(`{"type":"output_text","text":"hello"}`),
	})
	counter.AppendEvent(types.ResponsesStreamEvent{
		Type: "response.completed",
		Response: &types.ResponsesResponse{
			OutputText: "hello",
			Output: []types.ResponsesOutputItem{{
				Type: "message",
				Content: []types.ResponsesContentPart{{
					Type: "output_text",
					Text: "hello",
				}},
			}},
		},
	})

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(2), usage.PromptTokens)
	require.Equal(t, int64(5), usage.CompletionTokens)
	require.Equal(t, int64(7), usage.TotalTokens)
}

func TestResponsesTokenCounterEstimatesReasoningStreamEventsWithoutDoubleCountingCompletedPayload(t *testing.T) {
	counter := NewResponsesTokenCounter(&DumyTokenizer{})
	counter.Request(&types.ResponsesRequest{Model: "m", Input: json.RawMessage(`"hi"`)})
	counter.AppendEvent(types.ResponsesStreamEvent{
		Type:  "response.reasoning_summary_text.delta",
		Delta: "think",
	})
	counter.AppendEvent(types.ResponsesStreamEvent{
		Type: "response.completed",
		Response: &types.ResponsesResponse{
			Output: []types.ResponsesOutputItem{{
				Type: "reasoning",
				Summary: []types.ResponsesSummaryPart{{
					Type: "summary_text",
					Text: "think",
				}},
			}},
		},
	})

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(2), usage.PromptTokens)
	require.Equal(t, int64(5), usage.CompletionTokens)
	require.Equal(t, int64(7), usage.TotalTokens)
}

type responsesFailingTokenizer struct {
	err error
}

func (t responsesFailingTokenizer) Encode(types.Message) (int64, error) {
	return 0, t.err
}

func (t responsesFailingTokenizer) EmbeddingEncode(string) (int64, error) {
	return 0, t.err
}

type responsesBlockingTokenizer struct{}

func (responsesBlockingTokenizer) Encode(types.Message) (int64, error) {
	select {}
}

func (responsesBlockingTokenizer) EmbeddingEncode(string) (int64, error) {
	select {}
}

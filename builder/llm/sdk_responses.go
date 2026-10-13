package llm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"opencsg.com/csghub-server/common/types"
)

// openaiSDKOptions builds the openai SDK request options: the shared HTTP
// client (retry + tracing), the upstream base URL, and the upstream auth
// headers. SDK-level retries are disabled because the shared client already
// retries transport errors.
func (c *Client) openaiSDKOptions(call *upstreamCall) []option.RequestOption {
	doer := c.client
	if call.host != "" {
		doer = hostOverrideDoer{inner: c.client, host: call.host}
	}
	// Never let SDK env fallback credentials reach the upstream: strip
	// SDK-set auth headers when the upstream config carries none.
	doer = stripEnvAuthDoer{inner: doer, headers: call.headers}
	opts := []option.RequestOption{
		option.WithHTTPClient(doer),
		option.WithBaseURL(call.endpoint),
		option.WithMaxRetries(0),
	}
	for k, v := range call.headers {
		opts = append(opts, option.WithHeader(k, v))
	}
	// Both SDKs resolve the request path against the base URL and drop a
	// query carried by the base itself, so endpoint query parameters (e.g.
	// Azure-style api-version) must be forwarded as explicit query options
	// to land on the final URL exactly once.
	appendEndpointQueryOptions(call.endpoint, func(k, v string) {
		opts = append(opts, option.WithQueryAdd(k, v))
	})
	return opts
}

// buildResponsesParams maps the chat-style request onto the native OpenAI
// Responses API request: system messages become instructions, the remaining
// messages become input items.
func buildResponsesParams(call *upstreamCall, data types.LLMReqBody) responses.ResponseNewParams {
	system, rest := splitSystemMessages(data.Messages)
	params := responses.ResponseNewParams{
		Model:       shared.ResponsesModel(call.model),
		Temperature: param.NewOpt(data.Temperature),
	}
	if system != "" {
		params.Instructions = param.NewOpt(system)
	}
	input := make(responses.ResponseInputParam, 0, len(rest))
	for _, m := range rest {
		input = append(input, responses.ResponseInputItemUnionParam{
			OfMessage: &responses.EasyInputMessageParam{
				Role:    responses.EasyInputMessageRole(m.Role),
				Content: responses.EasyInputMessageContentUnionParam{OfString: param.NewOpt(m.Content)},
			},
		})
	}
	params.Input = responses.ResponseNewParamsInputUnion{OfInputItemList: input}
	if data.MaxTokens > 0 {
		params.MaxOutputTokens = param.NewOpt(int64(data.MaxTokens))
	}
	return params
}

func (c *Client) chatViaResponses(ctx context.Context, call *upstreamCall, data types.LLMReqBody) (string, error) {
	client := openai.NewClient(c.openaiSDKOptions(call)...)
	resp, err := client.Responses.New(ctx, buildResponsesParams(call, data))
	if err != nil {
		return "", fmt.Errorf("do llm request via responses api, error: %w", err)
	}
	text := resp.OutputText()
	if strings.TrimSpace(text) == "" {
		return "", errInvalidSummary
	}
	return text, nil
}

func (c *Client) chatStreamViaResponses(ctx context.Context, call *upstreamCall, data types.LLMReqBody) (<-chan string, error) {
	client := openai.NewClient(c.openaiSDKOptions(call)...)
	stream := client.Responses.NewStreaming(ctx, buildResponsesParams(call, data))
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("do llm stream request via responses api, error: %w", err)
	}
	out := make(chan string, 2)
	go func() {
		defer close(out)
		defer stream.Close()
		for stream.Next() {
			event := stream.Current()
			if event.Type != "response.output_text.delta" {
				continue
			}
			delta := event.AsResponseOutputTextDelta().Delta
			if delta == "" {
				continue
			}
			emitChunk(ctx, out, delta)
		}
		if err := stream.Err(); err != nil {
			slog.Warn("responses api stream aborted", slog.Any("error", err))
			emitStreamError(ctx, out, err)
		}
	}()
	return out, nil
}

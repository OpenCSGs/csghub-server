package llm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"opencsg.com/csghub-server/common/types"
)

// messageOptions builds the anthropic SDK request options: the shared HTTP
// client (retry + tracing), the upstream base URL, and the upstream auth
// headers. SDK-level retries are disabled because the shared client already
// retries transport errors.
func (c *Client) messageOptions(call *upstreamCall) []option.RequestOption {
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
	// See openaiSDKOptions: the SDKs drop a query carried by the base URL
	// during path resolution, so endpoint query parameters (e.g.
	// Azure-style api-version) are forwarded as explicit query options.
	appendEndpointQueryOptions(call.endpoint, func(k, v string) {
		opts = append(opts, option.WithQueryAdd(k, v))
	})
	return opts
}

// buildMessagesParams maps the chat-style request onto the native Anthropic
// Messages API request: system messages become the system prompt, the
// remaining messages become user/assistant messages.
func buildMessagesParams(call *upstreamCall, data types.LLMReqBody) anthropic.MessageNewParams {
	system, rest := splitSystemMessages(data.Messages)
	params := anthropic.MessageNewParams{
		Model: anthropic.Model(call.model),
		// The Messages API requires max_tokens; internal callers usually
		// leave it unset.
		MaxTokens:   defaultAnthropicMaxTokens,
		Temperature: anthropic.Float(data.Temperature),
	}
	if data.MaxTokens > 0 {
		params.MaxTokens = int64(data.MaxTokens)
	}
	if system != "" {
		params.System = []anthropic.TextBlockParam{{Text: system}}
	}
	// Messages map 1:1 to user/assistant roles. The Anthropic Messages API
	// requires strict user/assistant alternation starting with a user
	// message; all current internal callers send a single user message, so
	// no alternation merging is done here. Revisit if a future caller
	// passes multi-turn histories.
	for _, m := range rest {
		block := anthropic.NewTextBlock(m.Content)
		if strings.EqualFold(m.Role, assistantRole) {
			params.Messages = append(params.Messages, anthropic.NewAssistantMessage(block))
			continue
		}
		params.Messages = append(params.Messages, anthropic.NewUserMessage(block))
	}
	return params
}

func (c *Client) chatViaMessages(ctx context.Context, call *upstreamCall, data types.LLMReqBody) (string, error) {
	client := anthropic.NewClient(c.messageOptions(call)...)
	msg, err := client.Messages.New(ctx, buildMessagesParams(call, data))
	if err != nil {
		return "", fmt.Errorf("do llm request via anthropic messages api, error: %w", err)
	}
	var b strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			b.WriteString(block.Text)
		}
	}
	if strings.TrimSpace(b.String()) == "" {
		return "", errInvalidSummary
	}
	return b.String(), nil
}

func (c *Client) chatStreamViaMessages(ctx context.Context, call *upstreamCall, data types.LLMReqBody) (<-chan string, error) {
	client := anthropic.NewClient(c.messageOptions(call)...)
	stream := client.Messages.NewStreaming(ctx, buildMessagesParams(call, data))
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("do llm stream request via anthropic messages api, error: %w", err)
	}
	out := make(chan string, 2)
	go func() {
		defer close(out)
		defer stream.Close()
		for stream.Next() {
			event := stream.Current()
			if event.Type != "content_block_delta" {
				continue
			}
			delta := event.AsContentBlockDelta().Delta
			if delta.Type != "text_delta" {
				continue
			}
			if text := delta.AsTextDelta().Text; text != "" {
				emitChunk(ctx, out, text)
			}
		}
		if err := stream.Err(); err != nil {
			slog.Warn("anthropic messages api stream aborted", slog.Any("error", err))
			emitStreamError(ctx, out, err)
		}
	}()
	return out, nil
}

package llm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/shared"

	"opencsg.com/csghub-server/common/types"
)

// buildChatCompletionsParams maps the chat-style request onto the native
// OpenAI Chat Completions API request, preserving every role (the chat
// protocol carries system messages inline).
func buildChatCompletionsParams(call *upstreamCall, data types.LLMReqBody) openai.ChatCompletionNewParams {
	params := openai.ChatCompletionNewParams{
		Model:       shared.ChatModel(call.model),
		Temperature: param.NewOpt(data.Temperature),
	}
	if data.MaxTokens > 0 {
		params.MaxTokens = param.NewOpt(int64(data.MaxTokens))
	}
	for _, m := range data.Messages {
		switch {
		case strings.EqualFold(m.Role, systemRole):
			params.Messages = append(params.Messages, openai.SystemMessage(m.Content))
		case strings.EqualFold(m.Role, assistantRole):
			params.Messages = append(params.Messages, openai.AssistantMessage(m.Content))
		default:
			params.Messages = append(params.Messages, openai.UserMessage(m.Content))
		}
	}
	return params
}

func (c *Client) chatViaChatCompletions(ctx context.Context, call *upstreamCall, data types.LLMReqBody) (string, error) {
	client := openai.NewClient(c.openaiSDKOptions(call)...)
	resp, err := client.Chat.Completions.New(ctx, buildChatCompletionsParams(call, data))
	if err != nil {
		return "", fmt.Errorf("do llm request via chat completions api, error: %w", err)
	}
	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		return "", errInvalidSummary
	}
	return resp.Choices[0].Message.Content, nil
}

func (c *Client) chatStreamViaChatCompletions(ctx context.Context, call *upstreamCall, data types.LLMReqBody) (<-chan string, error) {
	client := openai.NewClient(c.openaiSDKOptions(call)...)
	stream := client.Chat.Completions.NewStreaming(ctx, buildChatCompletionsParams(call, data))
	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("do llm stream request via chat completions api, error: %w", err)
	}
	out := make(chan string, 2)
	go func() {
		defer close(out)
		defer stream.Close()
		for stream.Next() {
			// Map the typed chunk onto the consumer contract shape: the
			// SDK's own JSON omits nothing (e.g. logprobs as an object),
			// which the types.LLMResponse-based consumers cannot parse.
			chunk := stream.Current()
			resp := types.LLMResponse{
				Id:      chunk.ID,
				Object:  string(chunk.Object),
				Created: chunk.Created,
				Model:   string(chunk.Model),
			}
			for _, choice := range chunk.Choices {
				resp.Choices = append(resp.Choices, types.LLMChoice{
					Index:        int(choice.Index),
					Delta:        types.LLMMessage{Role: string(choice.Delta.Role), Content: choice.Delta.Content},
					FinishReason: string(choice.FinishReason),
				})
			}
			sendLLMResponseChunk(ctx, out, resp)
		}
		if err := stream.Err(); err != nil {
			slog.Warn("chat completions api stream aborted", slog.Any("error", err))
			emitStreamError(ctx, out, err)
		}
	}()
	return out, nil
}

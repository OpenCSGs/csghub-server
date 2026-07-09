package token

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
)

var _ Counter = (*responsesTokenCounterImpl)(nil)

type ResponsesTokenCounter interface {
	Request(req *types.ResponsesRequest)
	Response(resp *types.ResponsesResponse)
	AppendEvent(event types.ResponsesStreamEvent)
	Usage(ctx context.Context) (*Usage, error)
}

type responsesTokenCounterImpl struct {
	request        *types.ResponsesRequest
	response       *types.ResponsesResponse
	usage          *types.ResponsesUsage
	outputText     strings.Builder
	refusalText    strings.Builder
	reasoningText  strings.Builder
	toolCallText   strings.Builder
	tokenizer      Tokenizer
	promptFallback strings.Builder
}

func NewResponsesTokenCounter(tokenizer Tokenizer) ResponsesTokenCounter {
	return &responsesTokenCounterImpl{tokenizer: tokenizer}
}

func (c *responsesTokenCounterImpl) Request(req *types.ResponsesRequest) {
	if req == nil {
		return
	}
	cp := *req
	c.request = &cp
	c.promptFallback.Reset()
	c.promptFallback.WriteString(types.ResponsesPromptText(req))
}

func (c *responsesTokenCounterImpl) Response(resp *types.ResponsesResponse) {
	if resp == nil {
		return
	}
	cp := *resp
	c.response = &cp
	if resp.Usage != nil {
		c.usage = resp.Usage
	}
	outputLen := c.outputText.Len()
	for _, item := range resp.Output {
		c.captureOutputItem(item)
	}
	if c.outputText.Len() == outputLen && resp.OutputText != "" {
		c.outputText.WriteString(resp.OutputText)
	}
}

func (c *responsesTokenCounterImpl) AppendEvent(event types.ResponsesStreamEvent) {
	if event.Response != nil {
		cp := *event.Response
		c.response = &cp
		if event.Response.Usage != nil {
			c.usage = event.Response.Usage
		}
		if c.outputText.Len() == 0 && c.refusalText.Len() == 0 && c.reasoningText.Len() == 0 && c.toolCallText.Len() == 0 {
			c.Response(event.Response)
		}
	}
	switch event.Type {
	case "response.output_text.delta":
		c.outputText.WriteString(event.Delta)
	case "response.refusal.delta":
		c.refusalText.WriteString(event.Delta)
	// The chat adapter emits reasoning_summary_text today; accept
	// reasoning_text for native/future Responses-compatible streams.
	case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
		c.reasoningText.WriteString(event.Delta)
	case "response.function_call_arguments.delta":
		c.toolCallText.WriteString(event.Delta)
	}
	if strings.HasSuffix(event.Type, ".added") && event.Item != nil {
		c.captureOutputItem(*event.Item)
	}
	if strings.HasSuffix(event.Type, ".added") && len(event.Part) > 0 {
		var part types.ResponsesContentPart
		if err := json.Unmarshal(event.Part, &part); err == nil {
			c.captureContentPart(part)
		}
	}
}

func (c *responsesTokenCounterImpl) Usage(ctx context.Context) (*Usage, error) {
	if usage := responsesUsageToTokenUsage(c.usage); usage != nil {
		return usage, nil
	}

	promptText := c.promptFallback.String()
	outputText := c.outputText.String() + c.refusalText.String() + c.reasoningText.String() + c.toolCallText.String()
	if strings.TrimSpace(promptText) == "" && strings.TrimSpace(outputText) == "" {
		return nil, errors.New("no responses usage found and no text available for fallback estimate")
	}

	if c.tokenizer == nil {
		promptTokens := approxTokensByText(promptText)
		completionTokens := approxTokensByText(outputText)
		totalTokens := promptTokens + completionTokens
		slog.WarnContext(ctx, "responses tokenizer unavailable, using approximate token usage",
			slog.Int64("prompt_tokens", promptTokens),
			slog.Int64("completion_tokens", completionTokens),
			slog.Int64("total_tokens", totalTokens))
		return &Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
		}, nil
	}

	promptTokens, err := c.tokenizer.Encode(types.Message{Role: string(types.RoleUser), Content: promptText})
	if err != nil {
		return nil, err
	}
	completionTokens, err := c.tokenizer.Encode(types.Message{Content: outputText})
	if err != nil {
		return nil, err
	}
	totalTokens := promptTokens + completionTokens
	return &Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
	}, nil
}

func (c *responsesTokenCounterImpl) captureOutputItem(item types.ResponsesOutputItem) {
	if item.Arguments != "" {
		c.toolCallText.WriteString(item.Arguments)
	}
	for _, part := range item.Content {
		c.captureContentPart(part)
	}
	for _, part := range item.Summary {
		c.reasoningText.WriteString(part.Text)
	}
}

func (c *responsesTokenCounterImpl) captureContentPart(part types.ResponsesContentPart) {
	switch part.Type {
	case "output_text", "text":
		c.outputText.WriteString(part.Text)
	case "refusal":
		c.refusalText.WriteString(part.Refusal)
	}
}

func responsesUsageToTokenUsage(usage *types.ResponsesUsage) *Usage {
	if usage == nil {
		return nil
	}
	tokenUsage := &Usage{
		PromptTokens:     responsesInputTokenCount(usage),
		CompletionTokens: responsesOutputTokenCount(usage),
		TotalTokens:      usage.TotalTokens,
	}
	if usage.InputTokensDetails != nil {
		tokenUsage.CachedPromptTokens = usage.InputTokensDetails.CachedTokens
		tokenUsage.CacheCreationPromptTokens = usage.InputTokensDetails.CachedCreationTokens
	}
	if usage.OutputTokensDetails != nil {
		tokenUsage.ReasoningTokens = usage.OutputTokensDetails.ReasoningTokens
	}
	if tokenUsage.TotalTokens == 0 {
		tokenUsage.TotalTokens = tokenUsage.PromptTokens + tokenUsage.CompletionTokens
	}
	if tokenUsage.PromptTokens == 0 && tokenUsage.CompletionTokens == 0 && tokenUsage.TotalTokens == 0 &&
		tokenUsage.CachedPromptTokens == 0 && tokenUsage.CacheCreationPromptTokens == 0 && tokenUsage.ReasoningTokens == 0 {
		return nil
	}
	return tokenUsage
}

func responsesInputTokenCount(usage *types.ResponsesUsage) int64 {
	if usage == nil {
		return 0
	}
	if usage.InputTokens != 0 {
		return usage.InputTokens
	}
	if usage.InputTokensDetails == nil {
		return 0
	}
	return usage.InputTokensDetails.CachedTokens +
		usage.InputTokensDetails.CachedCreationTokens +
		usage.InputTokensDetails.TextTokens +
		usage.InputTokensDetails.AudioTokens +
		usage.InputTokensDetails.ImageTokens
}

func responsesOutputTokenCount(usage *types.ResponsesUsage) int64 {
	if usage == nil {
		return 0
	}
	if usage.OutputTokens != 0 {
		return usage.OutputTokens
	}
	if usage.OutputTokensDetails == nil {
		return 0
	}
	return usage.OutputTokensDetails.TextTokens +
		usage.OutputTokensDetails.AudioTokens +
		usage.OutputTokensDetails.ImageTokens +
		usage.OutputTokensDetails.ReasoningTokens
}

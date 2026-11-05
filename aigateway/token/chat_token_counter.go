package token

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/openai/openai-go/v3"
	"opencsg.com/csghub-server/aigateway/types"
)

var _ Counter = (*chatTokenCounterImpl)(nil)

type ChatTokenCounter interface {
	AppendPrompts(prompts []openai.ChatCompletionMessageParamUnion)
	Completion(completion types.ChatCompletion)
	AppendCompletionChunk(chunk types.ChatCompletionChunk)
	Usage(c context.Context) (*Usage, error)
}

type chatTokenCounterImpl struct {
	prompts    []openai.ChatCompletionMessageParamUnion
	completion *types.ChatCompletion
	chunks     []types.ChatCompletionChunk
	tokenizer  Tokenizer
}

func (l *chatTokenCounterImpl) AppendPrompts(prompts []openai.ChatCompletionMessageParamUnion) {
	l.prompts = append(l.prompts, prompts...)
}

func NewLLMTokenCounter(tokenizer Tokenizer) ChatTokenCounter {
	return &chatTokenCounterImpl{
		completion: nil,
		tokenizer:  tokenizer,
	}
}

func (l *chatTokenCounterImpl) SetCreateParam(param CreateParam) {
	l.tokenizer = NewTokenizerImpl(param.Endpoint, param.Host, param.Model, param.ImageID, param.Provider)
}

// Completion implements LLMTokenCounter.
func (l *chatTokenCounterImpl) Completion(completion types.ChatCompletion) {
	l.completion = &completion
}

// AppendCompletionChunk implements LLMTokenCounter.
func (l *chatTokenCounterImpl) AppendCompletionChunk(chunk types.ChatCompletionChunk) {
	l.chunks = append(l.chunks, chunk)
}

/*
	prompt:

	<|im_start|>system\n
	You are a helpful assistant.\n
	<|im_end|>\n
	<im_start>user\n
	[request content]\n
	<|im_end|>\n
	<|im_start|>assistant\n

	completion:

	[response content]\n
	<|im_end|>\n
*/

// Usage implements LLMTokenCounter.
func (l *chatTokenCounterImpl) Usage(c context.Context) (*Usage, error) {
	if l.completion != nil {
		if l.completion.Usage.TotalTokens > 0 {
			return &Usage{
				PromptTokens:              l.completion.Usage.PromptTokens,
				CompletionTokens:          l.completion.Usage.CompletionTokens,
				TotalTokens:               l.completion.Usage.TotalTokens,
				CachedPromptTokens:        l.completion.Usage.PromptTokensDetails.CachedTokens,
				ReasoningTokens:           l.completion.Usage.CompletionTokensDetails.ReasoningTokens,
				CacheCreationPromptTokens: 0,
				Duration:                  completionUsageSeconds(l.completion.Usage),
			}, nil
		}
		slog.WarnContext(c, "chat completion usage not found, fallback to local token estimate")
	}

	var contentBuf strings.Builder
	if l.completion != nil {
		for _, choice := range l.completion.Choices {
			if choice.Message.Content != "" {
				contentBuf.WriteString(choice.Message.Content)
			}
		}
	}
	for _, chunk := range l.chunks {
		if len(chunk.Choices) != 0 {
			if chunk.Choices[0].Delta.Content != "" {
				contentBuf.WriteString(chunk.Choices[0].Delta.Content)
			}
			if chunk.Choices[0].Delta.ReasoningContent != "" {
				contentBuf.WriteString(chunk.Choices[0].Delta.ReasoningContent)
			} else if chunk.Choices[0].Delta.Reasoning != "" {
				contentBuf.WriteString(chunk.Choices[0].Delta.Reasoning)
			}
		}
		// contains usage data
		if chunk.Usage.TotalTokens > 0 {
			slog.Debug("llmTokenCounter generated", slog.String("content", contentBuf.String()))
			return &Usage{
				PromptTokens:              chunk.Usage.PromptTokens,
				CompletionTokens:          chunk.Usage.CompletionTokens,
				TotalTokens:               chunk.Usage.TotalTokens,
				CachedPromptTokens:        chunk.Usage.PromptTokensDetails.CachedTokens,
				ReasoningTokens:           chunk.Usage.CompletionTokensDetails.ReasoningTokens,
				CacheCreationPromptTokens: 0,
				Duration:                  completionUsageSeconds(chunk.Usage),
			}, nil
		}
	}

	if l.tokenizer == nil {
		promptTokens, completionTokens := approximatePromptAndCompletionTokens(l.prompts, contentBuf.String())
		promptTokens += promptImageEstimate(l.prompts)
		if promptTokens <= 0 && completionTokens <= 0 {
			return nil, errors.New("no usage found in completion, and tokenizer not set")
		}
		totalTokens := promptTokens + completionTokens
		slog.WarnContext(c, "tokenizer unavailable, using approximate token usage",
			slog.Int64("prompt_tokens", promptTokens),
			slog.Int64("completion_tokens", completionTokens),
			slog.Int64("total_tokens", totalTokens))
		return &Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
		}, nil
	}

	var totalTokens, completionTokens, promptTokens int64
	// completion
	completionTokens, err := l.tokenizer.Encode(types.Message{
		Content: contentBuf.String(),
	})
	if err != nil {
		slog.Debug("call tokenizer API for completion", slog.Any("error", err))
		if err.Error() == "unsupported tokenizer" {
			// call tiktoken tokenizer
			return nil, err
		} else {
			return nil, err
		}
	}
	// prompt
	for _, msg := range l.prompts {
		var content string
		contentType := msg.GetContent().AsAny()

		switch v := contentType.(type) {
		case *string:
			// Handle string content
			content = *v
		case *[]openai.ChatCompletionContentPartTextParam:
			// Handle text content parts array
			var textContent string
			for _, part := range *v {
				textContent += part.Text
			}
			content = textContent
		case *[]openai.ChatCompletionContentPartUnionParam:
			// Handle mixed content parts array
			var combinedContent string
			for _, part := range *v {
				switch {
				case part.OfText != nil:
					if part.GetText() != nil {
						combinedContent += *part.GetText()
					}
				case part.OfImageURL != nil:
					// For image content, we'll handle it in future
					slog.WarnContext(c, "image content is not supported yet",
						slog.Any("part", part))
				case part.OfInputAudio != nil:
					// For audio content, we'll handle it in future
					slog.WarnContext(c, "audio content is not supported yet",
						slog.Any("part", part))
				case part.OfFile != nil:
					// For file content, we'll handle it in future
					slog.WarnContext(c, "file content is not supported yet",
						slog.Any("part", part))

				default:
					// For other content types, we'll handle it in future
					slog.WarnContext(c, "other content type is not supported yet",
						slog.Any("part", part))
				}
			}
			content = combinedContent
		case *[]openai.ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion:
			// Handle assistant message content parts array
			slog.WarnContext(c, "assistant message content parts array is not supported yet",
				slog.Any("msg", msg))
			content = ""
		default:
			// Fallback to empty string if content type is not supported
			content = ""
		}

		tmpToken, err := l.tokenizer.Encode(types.Message{
			Content: content,
			Role:    *msg.GetRole(),
		})
		if err != nil {
			slog.Debug("call tokenizer API for prompt", slog.Any("error", err))
			if err.Error() == "unsupported tokenizer" {
				// call tiktoken tokenizer
				return nil, err
			} else {
				return nil, err
			}
		}
		promptTokens += tmpToken
	}
	// between prompt and response
	promptTokens += 3
	promptTokens += promptImageEstimate(l.prompts)
	totalTokens = promptTokens + completionTokens
	return &Usage{
		CompletionTokens: completionTokens,
		PromptTokens:     promptTokens,
		TotalTokens:      totalTokens,
	}, nil
}

// fallbackImageTokensPerImage estimates the prompt tokens of one image when
// the upstream returns no usage and the gateway must bill from a local
// estimate. It approximates the typical per-image token cost of hosted
// vision models (DeepSeek caps one image at 1024 tokens, Doubao
// detail=high at 1280, Qwen3-VL bills a 1024x1024 image at ~1024).
const fallbackImageTokensPerImage = int64(1024)

// promptImageEstimate returns the estimated prompt tokens for the images in
// the request messages; audio and file parts are not estimated.
func promptImageEstimate(messages []openai.ChatCompletionMessageParamUnion) int64 {
	return int64(countPromptImages(messages)) * fallbackImageTokensPerImage
}

func countPromptImages(messages []openai.ChatCompletionMessageParamUnion) int {
	count := 0
	for _, message := range messages {
		count += countImageParts(message.GetContent().AsAny())
	}
	return count
}

func countImageParts(content any) int {
	switch parts := content.(type) {
	case *[]openai.ChatCompletionContentPartUnionParam:
		if parts == nil {
			return 0
		}
		return countImagePartsInUnion(*parts)
	case []openai.ChatCompletionContentPartUnionParam:
		return countImagePartsInUnion(parts)
	default:
		return 0
	}
}

func countImagePartsInUnion(parts []openai.ChatCompletionContentPartUnionParam) int {
	count := 0
	for _, part := range parts {
		if part.OfImageURL != nil {
			count++
		}
	}
	return count
}

func approximatePromptAndCompletionTokens(prompts []openai.ChatCompletionMessageParamUnion, completion string) (int64, int64) {
	var promptText strings.Builder
	for _, msg := range prompts {
		contentType := msg.GetContent().AsAny()
		switch v := contentType.(type) {
		case *string:
			if v != nil {
				promptText.WriteString(*v)
			}
		case *[]openai.ChatCompletionContentPartTextParam:
			for _, part := range *v {
				promptText.WriteString(part.Text)
			}
		case *[]openai.ChatCompletionContentPartUnionParam:
			for _, part := range *v {
				if part.OfText != nil && part.GetText() != nil {
					promptText.WriteString(*part.GetText())
				}
			}
		}
		promptText.WriteByte('\n')
	}
	promptTokens := approxTokensByText(promptText.String())
	if promptTokens > 0 {
		promptTokens += 3
	}
	completionTokens := approxTokensByText(completion)
	return promptTokens, completionTokens
}

func completionUsageSeconds(usage openai.CompletionUsage) float64 {
	field, ok := usage.JSON.ExtraFields["seconds"]
	if ok && field.Valid() {
		return positiveJSONFloat(field.Raw())
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(usage.RawJSON()), &payload); err != nil {
		return 0
	}
	return positiveFloat(payload["seconds"])
}

func positiveJSONFloat(raw string) float64 {
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return 0
	}
	return positiveFloat(value)
}

func positiveFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		if typed > 0 {
			return typed
		}
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return 0
}

func approxTokensByText(content string) int64 {
	content = strings.TrimSpace(content)
	if content == "" {
		return 0
	}
	runeCount := utf8.RuneCountInString(content)
	// A conservative approximation for CJK/mixed content: about 1 token ~= 4 chars.
	return int64(math.Ceil(float64(runeCount) / 4.0))
}

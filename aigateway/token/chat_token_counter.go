package token

import (
	"errors"
	"log/slog"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
)

var _ Counter = (*ChatTokenCounter)(nil)

type ChatTokenCounter struct {
	prompts    []types.Message
	completion *types.ChatCompletion
	chunks     []types.ChatCompletionChunk
	tokenizer  Tokenizer
}

func (l *ChatTokenCounter) AppendPrompts(prompts ...types.Message) {
	l.prompts = append(l.prompts, prompts...)
}

func NewLLMTokenCounter(tokenizer Tokenizer) *ChatTokenCounter {
	return &ChatTokenCounter{
		completion: nil,
		tokenizer:  tokenizer,
	}
}

// Completion implements LLMTokenCounter.
func (l *ChatTokenCounter) Completion(completion types.ChatCompletion) {
	l.completion = &completion
}

// AppendCompletionChunk implements LLMTokenCounter.
func (l *ChatTokenCounter) AppendCompletionChunk(chunk types.ChatCompletionChunk) {
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
func (l *ChatTokenCounter) Usage() (*Usage, error) {
	if l.completion != nil {
		return &Usage{
			PromptTokens:     l.completion.Usage.PromptTokens,
			CompletionTokens: l.completion.Usage.CompletionTokens,
			TotalTokens:      l.completion.Usage.TotalTokens,
		}, nil
	}

	var contentBuf strings.Builder
	for _, chunk := range l.chunks {
		if len(chunk.Choices) != 0 {
			if chunk.Choices[0].Delta.Content != "" {
				contentBuf.WriteString(chunk.Choices[0].Delta.Content)
			}
			if chunk.Choices[0].Delta.ReasoningContent != "" {
				contentBuf.WriteString(chunk.Choices[0].Delta.ReasoningContent)
			}
		}
		// contains usage data
		if chunk.Usage.TotalTokens > 0 {
			slog.Debug("llmTokenCounter generated", slog.String("content", contentBuf.String()))
			return &Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}, nil
		}
	}

	if l.tokenizer == nil {
		return nil, errors.New("no usage found in completion, and tokenizer not set")
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
		tmpToken, err := l.tokenizer.Encode(types.Message{
			Content: msg.Content,
			Role:    msg.Role,
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
	totalTokens = promptTokens + completionTokens
	return &Usage{
		CompletionTokens: completionTokens,
		PromptTokens:     promptTokens,
		TotalTokens:      totalTokens,
	}, nil
}

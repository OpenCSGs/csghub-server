package token

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/openai/openai-go"
)

type LLMTokenCounter interface {
	AppendPrompts(prompts ...string)
	// set completion if not using stream
	Completion(openai.ChatCompletion)
	// record completion chunk if using stream
	AppendCompletionChunk(openai.ChatCompletionChunk)
	// get final usage
	Usage() (*openai.CompletionUsage, error)
}

var _ LLMTokenCounter = (*llmTokenCounter)(nil)

type llmTokenCounter struct {
	prompts    []string
	completion *openai.ChatCompletion
	chunks     []openai.ChatCompletionChunk
	tokenizer  Tokenizer
}

func (l *llmTokenCounter) AppendPrompts(prompts ...string) {
	l.prompts = append(l.prompts, prompts...)
}

func NewLLMTokenCounter(tokenizer Tokenizer) LLMTokenCounter {
	return &llmTokenCounter{
		completion: nil,
		tokenizer:  tokenizer,
	}
}

// Completion implements LLMTokenCounter.
func (l *llmTokenCounter) Completion(completion openai.ChatCompletion) {
	l.completion = &completion
}

// AppendCompletionChunk implements LLMTokenCounter.
func (l *llmTokenCounter) AppendCompletionChunk(chunk openai.ChatCompletionChunk) {
	l.chunks = append(l.chunks, chunk)
}

// Usage implements LLMTokenCounter.
func (l *llmTokenCounter) Usage() (*openai.CompletionUsage, error) {
	if l.completion != nil {
		return &l.completion.Usage, nil
	}

	var contentBuf strings.Builder
	for _, chunk := range l.chunks {
		contentBuf.WriteString(chunk.Choices[0].Delta.Content)
		// contains usage data
		if chunk.Usage.TotalTokens > 0 {
			slog.Debug("llmTokenCounter generated", slog.String("content", contentBuf.String()))
			return &chunk.Usage, nil
		}
	}
	slog.Debug("llmTokenCounter content generated", slog.String("content", contentBuf.String()))

	if l.tokenizer == nil {
		return nil, errors.New("no usage found in completion, and tokenizer not set")
	}

	//TODO: call tokenizer to calc token usage

	return nil, errors.New("no usage found")
}

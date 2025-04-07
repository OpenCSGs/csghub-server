package token

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/openai/openai-go"
	"opencsg.com/csghub-server/aigateway/types"
)

type LLMTokenCounter interface {
	AppendPrompts(prompts ...types.ChatMessage)
	// set completion if not using stream
	Completion(openai.ChatCompletion)
	// record completion chunk if using stream
	AppendCompletionChunk(openai.ChatCompletionChunk)
	// get final usage
	Usage() (*openai.CompletionUsage, error)
}

var _ LLMTokenCounter = (*llmTokenCounter)(nil)

type llmTokenCounter struct {
	prompts    []types.ChatMessage
	completion *openai.ChatCompletion
	chunks     []openai.ChatCompletionChunk
	tokenizer  Tokenizer
}

func (l *llmTokenCounter) AppendPrompts(prompts ...types.ChatMessage) {
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
			//slog.Debug("generated in resp", slog.Any("usage", chunk.Usage))
		}
	}

	if l.tokenizer == nil {
		return nil, errors.New("no usage found in completion, and tokenizer not set")
	}

	//TODO: call tokenizer to calc token usage
	var totalTokens, completionTokens, promptTokens int64
	// completion
	completionTokens, err := l.tokenizer.Encode(types.Message{
		Content: contentBuf.String(),
	})
	if err != nil {
		slog.Error("call tokenizer API for completion Error")
		return nil, err
	}
	// prompt
	for _, msg := range l.prompts {
		tmpToken, err := l.tokenizer.Encode(types.Message{
			Content: msg.Content,
			Role:    msg.Role,
		})
		if err != nil {
			slog.Error("call tokenizer API for prompt Error")
			return nil, err
		}
		promptTokens += tmpToken
	}
	// between prompt and response
	tmpToken, err := l.tokenizer.Encode(types.Message{
		Content: "",
		Role:    "assistant",
	})
	if err != nil {
		slog.Error("call tokenizer API for prompt Error")
		return nil, err
	}
	promptTokens += tmpToken
	totalTokens = promptTokens + completionTokens
	return &openai.CompletionUsage{
		CompletionTokens: completionTokens,
		PromptTokens:     promptTokens,
		TotalTokens:      totalTokens,
	}, nil
}

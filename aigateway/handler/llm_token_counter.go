package handler

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/openai/openai-go"
)

type Tokenizer interface {
	Encode(string) (int64, error)
}

var _ Tokenizer = (*dumyyTokenizer)(nil)

type dumyyTokenizer struct{}

// Encode implements Tokenizer.
func (d *dumyyTokenizer) Encode(s string) (int64, error) {
	return int64(len(s)), nil
}

type LLMTokenCounter interface {
	ChatReqeuestTokens(ChatCompletionRequest)
	// set completion if not using stream
	Completion(openai.ChatCompletion)
	// record completion chunk if using stream
	AppendCompletionChunk(openai.ChatCompletionChunk)
	// get final usage
	Usage() (*openai.CompletionUsage, error)
}

var _ LLMTokenCounter = (*llmTokenCounter)(nil)

type llmTokenCounter struct {
	req        *ChatCompletionRequest
	completion *openai.ChatCompletion
	chunks     []openai.ChatCompletionChunk
	tokenizer  Tokenizer
}

// ChatReqeuestTokens implements LLMTokenCounter.
func (l *llmTokenCounter) ChatReqeuestTokens(req ChatCompletionRequest) {
	l.req = &req
}

func NewLLMTokenCounter(tokenizer Tokenizer) LLMTokenCounter {
	return &llmTokenCounter{
		completion: nil,
		chunks:     make([]openai.ChatCompletionChunk, 0),
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

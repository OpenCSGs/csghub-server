package token

import (
	"errors"

	"github.com/openai/openai-go/v3"
)

var _ Counter = (*EmbeddingTokenCounter)(nil)

type EmbeddingTokenCounter struct {
	input     string
	usage     *openai.CreateEmbeddingResponseUsage
	tokenizer Tokenizer
}

func NewEmbeddingTokenCounter(tokenizer Tokenizer) *EmbeddingTokenCounter {
	return &EmbeddingTokenCounter{
		tokenizer: tokenizer,
	}
}

// Embedding implements EmbeddingTokenCounter.
func (l *EmbeddingTokenCounter) Embedding(resp openai.CreateEmbeddingResponseUsage) {
	l.usage = &resp
}

func (l *EmbeddingTokenCounter) Input(input string) {
	l.input = input
}

// Usage implements LLMTokenCounter.
func (l *EmbeddingTokenCounter) Usage() (*Usage, error) {
	if l.usage != nil {
		return &Usage{
			PromptTokens: l.usage.PromptTokens,
			TotalTokens:  l.usage.TotalTokens,
		}, nil
	}

	if l.tokenizer == nil {
		return nil, errors.New("no usage found in embedding response, and tokenizer not set")
	}

	tokenCount, err := l.tokenizer.EmbeddingEncode(l.input)
	if err != nil {
		return nil, err
	}
	return &Usage{
		TotalTokens:  tokenCount,
		PromptTokens: tokenCount,
	}, nil
}

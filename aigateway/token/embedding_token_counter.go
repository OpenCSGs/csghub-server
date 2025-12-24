package token

import (
	"context"
	"errors"

	"github.com/openai/openai-go/v3"
)

var _ Counter = (*EmbeddingTokenCounterImpl)(nil)

type EmbeddingTokenCounter interface {
	Embedding(resp openai.CreateEmbeddingResponseUsage)
	Input(input string)
	Usage(c context.Context) (*Usage, error)
}

type EmbeddingTokenCounterImpl struct {
	input     string
	usage     *openai.CreateEmbeddingResponseUsage
	tokenizer Tokenizer
}

func NewEmbeddingTokenCounter(tokenizer Tokenizer) EmbeddingTokenCounter {
	return &EmbeddingTokenCounterImpl{
		tokenizer: tokenizer,
	}
}

// Embedding implements EmbeddingTokenCounter.
func (l *EmbeddingTokenCounterImpl) Embedding(resp openai.CreateEmbeddingResponseUsage) {
	l.usage = &resp
}

func (l *EmbeddingTokenCounterImpl) Input(input string) {
	l.input = input
}

// Usage implements LLMTokenCounter.
func (l *EmbeddingTokenCounterImpl) Usage(c context.Context) (*Usage, error) {
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

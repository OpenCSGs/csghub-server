package token

import (
	"context"
	"errors"
	"unicode/utf8"
)

var _ Counter = (*AudioUsageCounter)(nil)

type AudioUsageCounter struct {
	text      string
	tokenizer Tokenizer
	usage     *Usage
}

func NewAudioUsageCounter(tokenizer Tokenizer) *AudioUsageCounter {
	return &AudioUsageCounter{
		tokenizer: tokenizer,
	}
}

func (c *AudioUsageCounter) Text(text string) {
	c.text = text
}

func (c *AudioUsageCounter) SetUsage(usage Usage) {
	c.usage = &usage
}

func (c *AudioUsageCounter) Usage(ctx context.Context) (*Usage, error) {
	if c.usage != nil {
		return c.usage, nil
	}
	if c.text == "" {
		return nil, errors.New("no transcription text found")
	}
	tokenCount := int64(utf8.RuneCountInString(c.text))
	if c.tokenizer != nil {
		var err error
		tokenCount, err = c.tokenizer.EmbeddingEncode(c.text)
		if err != nil {
			return nil, err
		}
	}
	return &Usage{
		TotalTokens:      tokenCount,
		CompletionTokens: tokenCount,
	}, nil
}

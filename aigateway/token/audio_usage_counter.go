package token

import (
	"context"
	"errors"
	"unicode/utf8"

	commontypes "opencsg.com/csghub-server/common/types"
)

var _ Counter = (*AudioUsageCounter)(nil)

type AudioUsageCounter struct {
	text      string
	tokenizer Tokenizer
	usage     *Usage
	duration  float64
}

func NewAudioUsageCounter(tokenizer Tokenizer) *AudioUsageCounter {
	return &AudioUsageCounter{
		tokenizer: tokenizer,
	}
}

func (c *AudioUsageCounter) Text(text string) {
	c.text = text
}

func (c *AudioUsageCounter) Duration(duration float64) {
	if duration > 0 {
		c.duration = duration
	}
}

func (c *AudioUsageCounter) SetUsage(usage Usage) {
	if usage.Duration == 0 {
		usage.Duration = c.duration
	}
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
		DataType:         string(commontypes.DataTypeAudio),
		Duration:         c.duration,
		CompletionRC:     1,
		CompletionDesc:   c.text,
	}, nil
}

package token

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/aigateway/types"
)

var _ Counter = (*ImageUsageCounter)(nil)

type ImageUsageCounter struct {
	usage *Usage
}

func NewImageUsageCounter() *ImageUsageCounter {
	return &ImageUsageCounter{}
}

func (c *ImageUsageCounter) ImageResponse(resp *types.ImageGenerationResponse) {
	if resp != nil {
		u := resp.Usage
		c.usage = &Usage{
			PromptTokens:     u.InputTokens,
			CompletionTokens: u.OutputTokens,
			TotalTokens:      u.TotalTokens,
		}
	}
}

func (c *ImageUsageCounter) Usage(ctx context.Context) (*Usage, error) {
	if c.usage == nil {
		return nil, fmt.Errorf("no usage data available")
	}
	return c.usage, nil
}

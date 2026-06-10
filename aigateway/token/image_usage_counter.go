package token

import (
	"context"
	"fmt"

	commontypes "opencsg.com/csghub-server/common/types"

	"opencsg.com/csghub-server/aigateway/types"
)

var _ Counter = (*ImageUsageCounter)(nil)

type ImageUsageCounter struct {
	usage      *Usage
	resolution string
	count      int64
}

func NewImageUsageCounter() *ImageUsageCounter {
	return &ImageUsageCounter{count: 1}
}

func (c *ImageUsageCounter) SetResolution(r string) {
	c.resolution = r
}

func (c *ImageUsageCounter) SetRequestDetails(resolution string, count int64) {
	c.resolution = resolution
	if count > 0 {
		c.count = count
	}
}

func (c *ImageUsageCounter) ImageResponse(resp *types.ImageGenerationResponse) {
	if resp != nil {
		u := resp.Usage
		resolution := string(resp.Size)
		if resolution == "" {
			resolution = c.resolution
		}
		count := int64(len(resp.Data))
		if count == 0 {
			count = c.count
		}
		c.usage = &Usage{
			PromptTokens:     u.InputTokens,
			CompletionTokens: u.OutputTokens,
			TotalTokens:      u.TotalTokens,
			DataType:         string(commontypes.DataTypeImage),
			Resolution:       resolution,
			CompletionRC:     count,
		}
	}
}

func (c *ImageUsageCounter) Usage(ctx context.Context) (*Usage, error) {
	if c.usage == nil {
		return nil, fmt.Errorf("no usage data available")
	}
	return c.usage, nil
}

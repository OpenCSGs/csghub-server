package token

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

var _ Counter = (*VideoUsageCounter)(nil)

type VideoUsageCounter struct {
	usage      *Usage
	resolution string
	duration   float64
}

func NewVideoUsageCounter(resolution string, duration float64) *VideoUsageCounter {
	return &VideoUsageCounter{
		resolution: resolution,
		duration:   duration,
	}
}

func (c *VideoUsageCounter) SetVideoCreated() {
	c.usage = &Usage{
		DataType:     string(commontypes.DataTypeVideo),
		Resolution:   c.resolution,
		Duration:     c.duration,
		CompletionRC: 1,
	}
}

func (c *VideoUsageCounter) VideoResponse(resp *types.VideoObject) {
	if resp == nil {
		c.SetVideoCreated()
		return
	}
	resolution := resp.Size
	if resolution == "" {
		resolution = c.resolution
	}
	duration := float64(resp.Seconds)
	if duration == 0 {
		duration = c.duration
	}
	usage := &Usage{
		DataType:       string(commontypes.DataTypeVideo),
		Resolution:     resolution,
		Duration:       duration,
		CompletionRC:   1,
		CompletionDesc: resp.Prompt,
	}
	if resp.Usage != nil {
		usage.PromptTokens = resp.Usage.InputTokens
		usage.CompletionTokens = resp.Usage.OutputTokens
		usage.TotalTokens = resp.Usage.TotalTokens
	}
	c.usage = usage
}

func (c *VideoUsageCounter) Usage(ctx context.Context) (*Usage, error) {
	if c.usage == nil {
		return nil, fmt.Errorf("no video usage data available")
	}
	return c.usage, nil
}

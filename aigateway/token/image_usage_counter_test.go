package token

import (
	"context"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestNewImageUsageCounter(t *testing.T) {
	c := NewImageUsageCounter()
	require.NotNil(t, c)
	_, err := c.Usage(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no usage data available")
}

func TestImageUsageCounter_Usage_NoData(t *testing.T) {
	c := NewImageUsageCounter()
	usage, err := c.Usage(context.Background())
	assert.Nil(t, usage)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no usage data available")
}

func TestImageUsageCounter_ImageResponse_Nil(t *testing.T) {
	c := NewImageUsageCounter()
	c.ImageResponse(nil)
	usage, err := c.Usage(context.Background())
	assert.Nil(t, usage)
	assert.Error(t, err)
}

func TestImageUsageCounter_ImageResponse_WithUsage(t *testing.T) {
	c := NewImageUsageCounter()
	resp := &types.ImageGenerationResponse{
		ImagesResponse: openai.ImagesResponse{
			Usage: openai.ImagesResponseUsage{
				InputTokens:  2,
				OutputTokens: 8,
				TotalTokens:  10,
			},
		},
	}
	c.ImageResponse(resp)
	usage, err := c.Usage(context.Background())
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, int64(2), usage.PromptTokens)
	assert.Equal(t, int64(8), usage.CompletionTokens)
	assert.Equal(t, int64(10), usage.TotalTokens)
}

func TestImageUsageCounter_ImageResponse_OverwritesPrevious(t *testing.T) {
	c := NewImageUsageCounter()
	c.ImageResponse(&types.ImageGenerationResponse{
		ImagesResponse: openai.ImagesResponse{
			Usage: openai.ImagesResponseUsage{
				InputTokens:  1,
				OutputTokens: 2,
				TotalTokens:  3,
			},
		},
	})
	c.ImageResponse(&types.ImageGenerationResponse{
		ImagesResponse: openai.ImagesResponse{
			Usage: openai.ImagesResponseUsage{
				InputTokens:  10,
				OutputTokens: 20,
				TotalTokens:  30,
			},
		},
	})
	usage, err := c.Usage(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(10), usage.PromptTokens)
	assert.Equal(t, int64(20), usage.CompletionTokens)
	assert.Equal(t, int64(30), usage.TotalTokens)
}

func TestImageUsageCounter_ImageResponse_ZeroUsage(t *testing.T) {
	c := NewImageUsageCounter()
	resp := &types.ImageGenerationResponse{
		ImagesResponse: openai.ImagesResponse{
			Usage: openai.ImagesResponseUsage{},
		},
	}
	c.ImageResponse(resp)
	usage, err := c.Usage(context.Background())
	require.NoError(t, err)
	require.NotNil(t, usage)
	assert.Equal(t, int64(0), usage.PromptTokens)
	assert.Equal(t, int64(0), usage.CompletionTokens)
	assert.Equal(t, int64(0), usage.TotalTokens)
}

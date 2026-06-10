package token

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestVideoUsageCounter_VideoResponse_PrefersResponseFields(t *testing.T) {
	counter := NewVideoUsageCounter("720x1280", 4)

	counter.VideoResponse(&types.VideoObject{
		Prompt:  "make a boat",
		Size:    "1280x720",
		Seconds: 8,
		Usage: &types.VideoUsage{
			InputTokens:  11,
			OutputTokens: 22,
			TotalTokens:  33,
		},
	})

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, string(commontypes.DataTypeVideo), usage.DataType)
	require.Equal(t, "1280x720", usage.Resolution)
	require.Equal(t, float64(8), usage.Duration)
	require.Equal(t, int64(1), usage.CompletionRC)
	require.Equal(t, int64(11), usage.PromptTokens)
	require.Equal(t, int64(22), usage.CompletionTokens)
	require.Equal(t, int64(33), usage.TotalTokens)
	require.Equal(t, "make a boat", usage.CompletionDesc)
}

func TestVideoUsageCounter_VideoResponse_FallsBackToRequestFields(t *testing.T) {
	counter := NewVideoUsageCounter("720x1280", 4)

	counter.VideoResponse(&types.VideoObject{})

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, string(commontypes.DataTypeVideo), usage.DataType)
	require.Equal(t, "720x1280", usage.Resolution)
	require.Equal(t, float64(4), usage.Duration)
	require.Equal(t, int64(1), usage.CompletionRC)
	require.Zero(t, usage.TotalTokens)
}

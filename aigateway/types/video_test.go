package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVideoObject_UnmarshalJSON_OpenAISecondsString(t *testing.T) {
	var video VideoObject
	err := json.Unmarshal([]byte(`{
		"id":"video_123",
		"object":"video",
		"size":"1280x720",
		"seconds":"8",
		"usage":{"input_tokens":3,"output_tokens":4,"total_tokens":7}
	}`), &video)

	require.NoError(t, err)
	require.Equal(t, "video_123", video.ID)
	require.Equal(t, "1280x720", video.Size)
	require.Equal(t, int64(8), video.Seconds)
	require.NotNil(t, video.Usage)
	require.Equal(t, int64(3), video.Usage.InputTokens)
	require.Equal(t, int64(4), video.Usage.OutputTokens)
	require.Equal(t, int64(7), video.Usage.TotalTokens)
}

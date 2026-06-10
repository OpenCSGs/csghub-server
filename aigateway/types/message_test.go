package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMessageParseContentInputAudio(t *testing.T) {
	message := Message{
		Content: []any{
			map[string]any{"type": "text", "text": "describe this audio"},
			map[string]any{
				"type": "input_audio",
				"input_audio": map[string]any{
					"data":   "audio-data",
					"format": "mp3",
				},
			},
		},
	}

	content := message.ParseContent()

	require.Len(t, content, 2)
	require.Equal(t, "text", content[0].Type)
	require.Equal(t, "input_audio", content[1].Type)
	require.NotNil(t, content[1].InputAudio)
	require.Equal(t, "audio-data", content[1].InputAudio.Data)
	require.Equal(t, "mp3", content[1].InputAudio.Format)
}

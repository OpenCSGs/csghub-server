package component

import (
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestGetBuiltInTaskFromTags_ASR(t *testing.T) {
	tests := []struct {
		name string
		tags []database.Tag
	}{
		{
			name: "canonical hf task",
			tags: []database.Tag{{Name: string(types.AutomaticSpeechRecognition)}},
		},
		{
			name: "legacy auto speech recognition tag",
			tags: []database.Tag{{Name: string(types.AutoSpeechRecognition)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, string(types.AutomaticSpeechRecognition), GetBuiltInTaskFromTags(tt.tags))
		})
	}
}

func TestGetBuiltInTaskFromTags_Image2Image(t *testing.T) {
	task := GetBuiltInTaskFromTags([]database.Tag{{Name: string(types.Image2Image)}})

	require.Equal(t, string(types.Image2Image), task)
}

func TestGetBuiltInTaskFromTags_TextRanking(t *testing.T) {
	task := GetBuiltInTaskFromTags([]database.Tag{{Name: string(types.TextRanking)}})

	require.Equal(t, string(types.TextRanking), task)
}

func TestGetBuiltInTaskFromTags_TextToAudio(t *testing.T) {
	task := GetBuiltInTaskFromTags([]database.Tag{{Name: string(types.TextToAudio)}})

	require.Equal(t, string(types.TextToAudio), task)
}

func TestGetBuiltInTaskFromTags_OpticalCharacterRecognition(t *testing.T) {
	task := GetBuiltInTaskFromTags([]database.Tag{{Name: string(types.OpticalCharacterRecognition)}})

	require.Equal(t, string(types.OpticalCharacterRecognition), task)
}

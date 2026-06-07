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

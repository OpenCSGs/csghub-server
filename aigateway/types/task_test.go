package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAutoSpeechRecognition(t *testing.T) {
	tests := []struct {
		name  string
		tasks []string
		want  bool
	}{
		{name: "legacy alias", tasks: []string{"auto-speech-recognition"}, want: true},
		{name: "canonical name", tasks: []string{"automatic-speech-recognition"}, want: true},
		{name: "mixed with other tasks", tasks: []string{"text-generation", "auto-speech-recognition"}, want: true},
		{name: "unrelated task", tasks: []string{"text-generation", "text-to-image"}, want: false},
		{name: "empty", tasks: nil, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, IsAutoSpeechRecognition(test.tasks))
		})
	}
}

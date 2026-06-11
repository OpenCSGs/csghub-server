package handler

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/token"
)

func TestResponseWriterWrapperAudio_UsesResponseUsage(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	w := NewResponseWriterWrapperAudio(recorder, counter)

	_, err := w.Write([]byte(`{"text":"hello","usage":{"prompt_tokens":371,"completion_tokens":52,"total_tokens":423,"seconds":9.2}}`))
	require.NoError(t, err)

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(423), usage.TotalTokens)
	require.Equal(t, int64(371), usage.PromptTokens)
	require.Equal(t, int64(52), usage.CompletionTokens)
	require.Equal(t, 9.2, usage.Duration)
}

func TestResponseWriterWrapperAudio_UsesResponseUsageFromChunkedJSON(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	w := NewResponseWriterWrapperAudio(recorder, counter)

	_, err := w.Write([]byte(`{"text":"hello","usage":`))
	require.NoError(t, err)
	_, err = w.Write([]byte(`{"prompt_tokens":371,"completion_tokens":52,"total_tokens":423}}`))
	require.NoError(t, err)

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(423), usage.TotalTokens)
	require.Equal(t, int64(371), usage.PromptTokens)
	require.Equal(t, int64(52), usage.CompletionTokens)
}

func TestResponseWriterWrapperAudio_CapturesDurationFromUsage(t *testing.T) {
	tests := []struct {
		name string
		body string
		want float64
	}{
		{
			name: "seconds",
			body: `{"text":"hello","usage":{"seconds":9.2}}`,
			want: 9.2,
		},
		{
			name: "top_level_duration",
			body: `{"text":"hello","duration":12.5}`,
			want: 12.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := token.NewAudioUsageCounter(nil)
			recorder := httptest.NewRecorder()
			w := NewResponseWriterWrapperAudio(recorder, counter)

			_, err := w.Write([]byte(tt.body))
			require.NoError(t, err)

			duration, ok := w.DurationSeconds()
			require.True(t, ok)
			require.Equal(t, tt.want, duration)

			usage, err := counter.Usage(context.Background())
			require.NoError(t, err)
			require.Equal(t, tt.want, usage.Duration)
		})
	}
}

func TestResponseWriterWrapperAudio_SkipsMissingDuration(t *testing.T) {
	counter := token.NewAudioUsageCounter(nil)
	recorder := httptest.NewRecorder()
	w := NewResponseWriterWrapperAudio(recorder, counter)

	_, err := w.Write([]byte(`{"text":"hello","usage":{"prompt_tokens":1,"total_tokens":1}}`))
	require.NoError(t, err)

	_, ok := w.DurationSeconds()
	require.False(t, ok)
}

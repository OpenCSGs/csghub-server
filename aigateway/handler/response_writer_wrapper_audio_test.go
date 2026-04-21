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

	_, err := w.Write([]byte(`{"text":"hello","usage":{"prompt_tokens":371,"completion_tokens":52,"total_tokens":423}}`))
	require.NoError(t, err)

	usage, err := counter.Usage(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(423), usage.TotalTokens)
	require.Equal(t, int64(371), usage.PromptTokens)
	require.Equal(t, int64(52), usage.CompletionTokens)
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

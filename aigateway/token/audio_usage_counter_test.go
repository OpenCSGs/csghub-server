package token

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAudioUsageCounter_Usage(t *testing.T) {
	counter := NewAudioUsageCounter(&DumyTokenizer{})
	counter.Text("hello")

	usage, err := counter.Usage(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(5), usage.TotalTokens)
	require.Equal(t, int64(5), usage.CompletionTokens)
}

func TestAudioUsageCounter_Usage_FallbackWithoutTokenizer(t *testing.T) {
	counter := NewAudioUsageCounter(nil)
	counter.Text("hello")

	usage, err := counter.Usage(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(5), usage.TotalTokens)
}

func TestAudioUsageCounter_Usage_ResponseUsage(t *testing.T) {
	counter := NewAudioUsageCounter(&DumyTokenizer{})
	counter.Text("hello")
	counter.SetUsage(Usage{
		TotalTokens:      423,
		PromptTokens:     371,
		CompletionTokens: 52,
	})

	usage, err := counter.Usage(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(423), usage.TotalTokens)
	require.Equal(t, int64(371), usage.PromptTokens)
	require.Equal(t, int64(52), usage.CompletionTokens)
}

func TestAudioUsageCounter_Usage_NoText(t *testing.T) {
	counter := NewAudioUsageCounter(&DumyTokenizer{})

	_, err := counter.Usage(context.Background())

	require.ErrorContains(t, err, "no transcription text found")
}

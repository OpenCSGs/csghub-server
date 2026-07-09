package token

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

type failingEmbeddingTokenizer struct{}

func (t failingEmbeddingTokenizer) Encode(types.Message) (int64, error) {
	return 0, errors.New("encode failed")
}

func (t failingEmbeddingTokenizer) EmbeddingEncode(string) (int64, error) {
	return 0, errors.New("embedding encode failed")
}

func TestAudioUsageCounter_Usage(t *testing.T) {
	counter := NewAudioUsageCounter(&DumyTokenizer{})
	counter.Text("hello")
	counter.Duration(3.5)

	usage, err := counter.Usage(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(5), usage.TotalTokens)
	require.Equal(t, int64(5), usage.CompletionTokens)
	require.Equal(t, 3.5, usage.Duration)
}

func TestAudioUsageCounter_Usage_FallbackWithoutTokenizer(t *testing.T) {
	counter := NewAudioUsageCounter(nil)
	counter.Text("hello")

	usage, err := counter.Usage(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(5), usage.TotalTokens)
}

func TestAudioUsageCounter_Usage_FallbackWhenTokenizerFails(t *testing.T) {
	counter := NewAudioUsageCounter(failingEmbeddingTokenizer{})
	counter.Text("hello")

	usage, err := counter.Usage(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(5), usage.TotalTokens)
	require.Equal(t, int64(5), usage.CompletionTokens)
}

func TestAudioUsageCounter_Usage_DurationOnly(t *testing.T) {
	counter := NewAudioUsageCounter(&DumyTokenizer{})
	counter.Duration(3.5)

	usage, err := counter.Usage(context.Background())

	require.NoError(t, err)
	require.Zero(t, usage.TotalTokens)
	require.Zero(t, usage.CompletionTokens)
	require.Equal(t, string(commontypes.DataTypeAudio), usage.DataType)
	require.Equal(t, 3.5, usage.Duration)
	require.Equal(t, int64(1), usage.CompletionRC)
	require.Empty(t, usage.CompletionDesc)
}

func TestAudioUsageCounter_Usage_ResponseUsage(t *testing.T) {
	counter := NewAudioUsageCounter(&DumyTokenizer{})
	counter.Text("hello")
	counter.SetUsage(Usage{
		TotalTokens:      423,
		PromptTokens:     371,
		CompletionTokens: 52,
		Duration:         9.2,
	})

	usage, err := counter.Usage(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(423), usage.TotalTokens)
	require.Equal(t, int64(371), usage.PromptTokens)
	require.Equal(t, int64(52), usage.CompletionTokens)
	require.Equal(t, 9.2, usage.Duration)
}

func TestAudioUsageCounter_Usage_CountOnly(t *testing.T) {
	counter := NewAudioUsageCounter(&DumyTokenizer{})

	usage, err := counter.Usage(context.Background())

	require.NoError(t, err)
	require.Zero(t, usage.TotalTokens)
	require.Zero(t, usage.CompletionTokens)
	require.Equal(t, string(commontypes.DataTypeAudio), usage.DataType)
	require.Zero(t, usage.Duration)
	require.Equal(t, int64(1), usage.CompletionRC)
	require.Empty(t, usage.CompletionDesc)
}

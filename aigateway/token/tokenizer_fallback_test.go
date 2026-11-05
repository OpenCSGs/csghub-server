package token

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

// useOfflineFallbackTokenizer swaps the fallback tokenizer factory for an
// offline implementation so tests do not download the cl100k_base BPE file,
// and restores the original factory after the test.
func useOfflineFallbackTokenizer(t *testing.T) {
	t.Helper()
	orig := fallbackTokenizerFactory
	fallbackTokenizerFactory = func() Tokenizer {
		return NewOfflineTiktokenTokenizer()
	}
	t.Cleanup(func() {
		fallbackTokenizerFactory = orig
	})
}

func TestNewTokenizerImplFallback(t *testing.T) {
	useOfflineFallbackTokenizer(t)

	t.Run("azure embedding model falls back instead of nil", func(t *testing.T) {
		tk := NewTokenizerImpl("", "", "text-embedding-3-small", "", "azure")
		require.NotNil(t, tk)

		count, err := tk.EmbeddingEncode("hello azure embedding")
		require.NoError(t, err)
		assert.Greater(t, count, int64(0))
	})

	t.Run("azure rerank input falls back instead of nil", func(t *testing.T) {
		tk := NewTokenizerImpl("", "", "some-rerank-model", "", "azure")
		require.NotNil(t, tk)

		// Mirrors the rerank handler input: query plus joined documents.
		count, err := tk.EmbeddingEncode("what is csghub?\nfirst document\nsecond document")
		require.NoError(t, err)
		assert.Greater(t, count, int64(0))
	})

	t.Run("no imageID and no provider falls back instead of nil", func(t *testing.T) {
		tk := NewTokenizerImpl("", "", "", "", "")
		require.NotNil(t, tk)
	})

	t.Run("gemini provider falls back to cl100k_base approximation", func(t *testing.T) {
		tk := NewTokenizerImpl("", "", "gemini-pro", "", "google")
		require.NotNil(t, tk)

		count, err := tk.EmbeddingEncode("hello gemini")
		require.NoError(t, err)
		assert.Greater(t, count, int64(0))
	})

	t.Run("fallback Encode follows OpenAI chat semantics", func(t *testing.T) {
		tk := NewTokenizerImpl("", "", "unknown-model", "", "some-provider")
		require.NotNil(t, tk)

		// tokensPerMessage = 3 + runeCount("user") + runeCount("Hello")
		count, err := tk.Encode(types.Message{Role: "user", Content: "Hello"})
		require.NoError(t, err)
		assert.Equal(t, int64(3+4+5), count)
	})
}

package token

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

// runeCountEncoder is a deterministic stand-in for cl100k in unit tests (no network, no BPE file).
// It counts Unicode code points so behavior stays stable and offline.
type runeCountEncoder struct{}

func (runeCountEncoder) TokenCount(s string) int {
	return utf8.RuneCountInString(s)
}

func testTiktokenTokenizer(t *testing.T) Tokenizer {
	t.Helper()
	tk := newTiktokenTokenizerForTest(runeCountEncoder{})
	require.NotNil(t, tk)
	return tk
}

func TestNewTiktokenTokenizerImpl(t *testing.T) {
	t.Run("builds tokenizer with local encoder (no network)", func(t *testing.T) {
		tk := newTiktokenTokenizerForTest(runeCountEncoder{})
		require.NotNil(t, tk)

		impl, ok := tk.(*tiktokenTokenizerImpl)
		require.True(t, ok)
		assert.NotNil(t, impl.enc)
	})

	t.Run("returns nil when encoder is nil", func(t *testing.T) {
		require.Nil(t, newTiktokenTokenizerForTest(nil))
	})
}

func TestTiktokenTokenizerImpl_Encode(t *testing.T) {
	tk := testTiktokenTokenizer(t)

	t.Run("empty message returns overhead only", func(t *testing.T) {
		msg := types.Message{}
		count, err := tk.Encode(msg)
		require.NoError(t, err)
		// Only tokensPerMessage = 3
		assert.Equal(t, int64(3), count)
	})

	t.Run("message with role only", func(t *testing.T) {
		msg := types.Message{
			Role: "user",
		}
		count, err := tk.Encode(msg)
		require.NoError(t, err)
		// 3 (overhead) + runeCount("user")
		assert.Equal(t, int64(3+4), count)
	})

	t.Run("message with content only", func(t *testing.T) {
		msg := types.Message{
			Content: "Hello",
		}
		count, err := tk.Encode(msg)
		require.NoError(t, err)
		assert.Equal(t, int64(3+5), count)
	})

	t.Run("message with role and content", func(t *testing.T) {
		msg := types.Message{
			Role:    "user",
			Content: "Hello, world!",
		}
		count, err := tk.Encode(msg)
		require.NoError(t, err)
		assert.Equal(t, int64(3+4+13), count)
	})

	t.Run("message with role, content and name", func(t *testing.T) {
		msg := types.Message{
			Role:    "user",
			Content: "Hello",
			Name:    "John",
		}
		count, err := tk.Encode(msg)
		require.NoError(t, err)
		assert.Equal(t, int64(3+4+5+1), count)
	})

	t.Run("empty content string is handled", func(t *testing.T) {
		msg := types.Message{
			Role:    "assistant",
			Content: "",
		}
		count, err := tk.Encode(msg)
		require.NoError(t, err)
		assert.Equal(t, int64(3+9), count)
	})

	t.Run("long content", func(t *testing.T) {
		msg := types.Message{
			Role:    "user",
			Content: "This is a longer message with more tokens to count properly using the tiktoken tokenizer.",
		}
		count, err := tk.Encode(msg)
		require.NoError(t, err)
		assert.Greater(t, count, int64(10))
	})

	t.Run("nil content is handled", func(t *testing.T) {
		msg := types.Message{
			Role:    "user",
			Content: nil,
		}
		count, err := tk.Encode(msg)
		require.NoError(t, err)
		assert.Equal(t, int64(3+4), count)
	})

	t.Run("non-string content is ignored", func(t *testing.T) {
		msg := types.Message{
			Role:    "user",
			Content: 12345, // non-string type
		}
		count, err := tk.Encode(msg)
		require.NoError(t, err)
		assert.Equal(t, int64(3+4), count)
	})
}

func TestTiktokenTokenizerImpl_EmbeddingEncode(t *testing.T) {
	tk := testTiktokenTokenizer(t)

	t.Run("empty string returns zero", func(t *testing.T) {
		count, err := tk.EmbeddingEncode("")
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("simple text", func(t *testing.T) {
		count, err := tk.EmbeddingEncode("Hello, world!")
		require.NoError(t, err)
		assert.Equal(t, int64(13), count)
	})

	t.Run("longer text", func(t *testing.T) {
		count, err := tk.EmbeddingEncode("The quick brown fox jumps over the lazy dog.")
		require.NoError(t, err)
		assert.Greater(t, count, int64(5))
	})

	t.Run("multiline text", func(t *testing.T) {
		text := `Line one
Line two
Line three`
		count, err := tk.EmbeddingEncode(text)
		require.NoError(t, err)
		assert.Greater(t, count, int64(0))
	})

	t.Run("unicode text", func(t *testing.T) {
		count, err := tk.EmbeddingEncode("Hello 世界 \U0001F30D")
		require.NoError(t, err)
		assert.Equal(t, int64(10), count)
	})
}

func TestTiktokenTokenizerImpl_WithNilTokenizer(t *testing.T) {
	t.Run("Encode returns error when tokenizer is nil", func(t *testing.T) {
		impl := &tiktokenTokenizerImpl{enc: nil}
		msg := types.Message{
			Role:    "user",
			Content: "Hello",
		}
		count, err := impl.Encode(msg)
		assert.Error(t, err)
		assert.Equal(t, int64(0), count)
		assert.Equal(t, errUnsupportedTokenizer, err)
	})

	t.Run("EmbeddingEncode returns error when tokenizer is nil", func(t *testing.T) {
		impl := &tiktokenTokenizerImpl{enc: nil}
		count, err := impl.EmbeddingEncode("Hello")
		assert.Error(t, err)
		assert.Equal(t, int64(0), count)
		assert.Equal(t, errUnsupportedTokenizer, err)
	})
}

func TestTiktokenTokenizerImpl_TokenCounts(t *testing.T) {
	tk := testTiktokenTokenizer(t)

	testCases := []struct {
		name     string
		content  string
		expected int64
	}{
		{"single char", "a", 1},
		{"short word", "hello", 5},
		{"two words", "hello world", 11},
		{"common phrase", "Hello, world!", 13},
		{"code snippet", "func main() {}", 14},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			count, err := tk.EmbeddingEncode(tc.content)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, count,
				"Content: %q expected %d tokens, got %d", tc.content, tc.expected, count)
		})
	}
}

func TestTiktokenTokenizerImpl_Consistency(t *testing.T) {
	tk := testTiktokenTokenizer(t)

	t.Run("same input produces same output", func(t *testing.T) {
		msg := types.Message{
			Role:    "user",
			Content: "Test message",
		}
		count1, err1 := tk.Encode(msg)
		count2, err2 := tk.Encode(msg)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, count1, count2)
	})

	t.Run("embedding is consistent", func(t *testing.T) {
		text := "Consistent test"
		count1, err1 := tk.EmbeddingEncode(text)
		count2, err2 := tk.EmbeddingEncode(text)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, count1, count2)
	})
}

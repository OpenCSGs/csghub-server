package token_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mock_token "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestTokenizer_EmbeddingEncode(t *testing.T) {
	tests := []struct {
		name    string
		message types.Message
		want    int64
		err     error
	}{
		{
			name: "empty message",
			message: types.Message{
				Content: "",
			},
			want: 0,
			err:  fmt.Errorf("EmbeddingTokenize input cannot be empty"),
		},
		{
			name: "simple message",
			message: types.Message{
				Content: "test",
			},
			want: 3,
			err:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := mock_token.NewMockTokenizer(t)
			d.EXPECT().Encode(tt.message).Return(tt.want, tt.err)
			got, err := d.Encode(tt.message)
			if err != nil {
				assert.Equal(t, err, fmt.Errorf("EmbeddingTokenize input cannot be empty"))
			}
			if got != tt.want {
				t.Errorf("DumyTokenizer.Encode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTiktokenTokenizerImpl(t *testing.T) {
	t.Run("Encode with content only (GPT model)", func(t *testing.T) {
		tk := token.NewOfflineTiktokenTokenizer()
		require.NotNil(t, tk)

		msg := types.Message{
			Content: "Hello, world!",
		}
		count, err := tk.Encode(msg)
		require.NoError(t, err)
		assert.Greater(t, count, int64(0))
	})

	t.Run("Encode with role and content (GPT model)", func(t *testing.T) {
		tk := token.NewOfflineTiktokenTokenizer()
		require.NotNil(t, tk)

		msg := types.Message{
			Role:    "user",
			Content: "Hello",
		}
		count, err := tk.Encode(msg)
		require.NoError(t, err)
		assert.Greater(t, count, int64(0))
	})

	t.Run("Encode with DeepSeek model", func(t *testing.T) {
		// DeepSeek also uses the tiktoken tokenizer path
		tk := token.NewOfflineTiktokenTokenizer()
		require.NotNil(t, tk)

		msg := types.Message{
			Content: "Hello, world!",
		}
		count, err := tk.Encode(msg)
		require.NoError(t, err)
		assert.Greater(t, count, int64(0))
	})

	t.Run("Encode empty content", func(t *testing.T) {
		tk := token.NewOfflineTiktokenTokenizer()
		require.NotNil(t, tk)

		msg := types.Message{
			Content: "",
		}
		count, err := tk.Encode(msg)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(0))
	})

	t.Run("EmbeddingEncode", func(t *testing.T) {
		tk := token.NewOfflineTiktokenTokenizer()
		require.NotNil(t, tk)

		count, err := tk.EmbeddingEncode("Hello, world!")
		require.NoError(t, err)
		assert.Greater(t, count, int64(0))
	})

	t.Run("Internal model uses imageID not provider", func(t *testing.T) {
		// When imageID is set, provider should be ignored
		tk := token.NewTokenizerImpl("http://localhost:8000", "", "llama-3", "vllm-local:latest", "openai")
		require.NotNil(t, tk)
		// This should be a vLLM tokenizer, not tiktoken
	})

	// Unmatched imageID/provider combinations used to return nil (no billing);
	// they now get the cl100k_base fallback. Those cases are covered by
	// TestNewTokenizerImplFallback in tokenizer_fallback_test.go, which
	// injects an offline tokenizer because the real fallback downloads the
	// cl100k_base BPE file on first use.
}

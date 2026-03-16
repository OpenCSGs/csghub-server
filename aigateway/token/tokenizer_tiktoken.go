package token

import (
	"log/slog"

	"github.com/pkoukk/tiktoken-go"
	"opencsg.com/csghub-server/aigateway/types"
)

// tiktokenTokenizerImpl implements Tokenizer using OpenAI-compatible rules
type tiktokenTokenizerImpl struct {
	tk *tiktoken.Tiktoken
}

// newTiktokenTokenizerImpl creates a tokenizer for billing / quota purpose.
// Token definition follows OpenAI chat completion semantics.
func newTiktokenTokenizerImpl() Tokenizer {
	// Platform-level billing tokenizer:
	// cl100k_base is the de-facto OpenAI standard
	tk, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		slog.Error("failed to initialize tiktoken cl100k_base", slog.Any("error", err))
		return nil
	}

	return &tiktokenTokenizerImpl{
		tk: tk,
	}
}

// Encode calculates the OpenAI-compatible token usage of a single chat message.
//
// IMPORTANT SEMANTICS:
//   - Includes per-message overhead (tokens_per_message = 3)
//   - Includes role / content / name tokens
//   - DOES NOT include assistant reply primer (+3)
//     → caller must add it once per request
func (t *tiktokenTokenizerImpl) Encode(message types.Message) (int64, error) {
	if t.tk == nil {
		return 0, errUnsupportedTokenizer
	}

	var tokens int64 = 0

	// OpenAI chat constants (cl100k_base)
	const tokensPerMessage = 3
	const tokensPerName = 1

	// Base overhead per message
	tokens += tokensPerMessage

	// Role
	if message.Role != "" {
		tokens += int64(len(t.tk.Encode(message.Role, nil, nil)))
	}

	// Content
	if message.Content != nil {
		switch v := message.Content.(type) {
		case string:
			if v != "" {
				tokens += int64(len(t.tk.Encode(v, nil, nil)))
			}
			// If multi-part content is supported in the future,
			// extend handling here explicitly.
		}
	}

	// Optional name
	if message.Name != "" {
		tokens += tokensPerName
	}

	return tokens, nil
}

// EmbeddingEncode calculates token usage for plain text content
// (used by embeddings or streaming completion deltas).
func (t *tiktokenTokenizerImpl) EmbeddingEncode(content string) (int64, error) {
	if t.tk == nil {
		return 0, errUnsupportedTokenizer
	}

	if content == "" {
		return 0, nil
	}

	return int64(len(t.tk.Encode(content, nil, nil))), nil
}

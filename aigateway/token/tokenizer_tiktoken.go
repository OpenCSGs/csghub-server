package token

import (
	"log/slog"

	"github.com/pkoukk/tiktoken-go"
	"opencsg.com/csghub-server/aigateway/types"
)

// pieceEncoder counts tokens for a single text fragment (role string, message body, etc.).
// Production uses cl100k_base via tiktoken; unit tests inject a local implementation.
type pieceEncoder interface {
	TokenCount(text string) int
}

type tiktokenPieceAdapter struct {
	tk *tiktoken.Tiktoken
}

func (a *tiktokenPieceAdapter) TokenCount(text string) int {
	if text == "" {
		return 0
	}
	return len(a.tk.Encode(text, nil, nil))
}

// tiktokenTokenizerImpl implements Tokenizer using OpenAI-compatible rules
type tiktokenTokenizerImpl struct {
	enc pieceEncoder
}

// newTiktokenTokenizerForTest builds a tokenizer with the given encoder (same package tests only).
// Passing nil returns nil.
func newTiktokenTokenizerForTest(enc pieceEncoder) Tokenizer {
	if enc == nil {
		return nil
	}
	return &tiktokenTokenizerImpl{enc: enc}
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
		enc: &tiktokenPieceAdapter{tk: tk},
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
	if t.enc == nil {
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
		tokens += int64(t.enc.TokenCount(message.Role))
	}

	// Content
	if message.Content != nil {
		switch v := message.Content.(type) {
		case string:
			if v != "" {
				tokens += int64(t.enc.TokenCount(v))
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
	if t.enc == nil {
		return 0, errUnsupportedTokenizer
	}

	if content == "" {
		return 0, nil
	}

	return int64(t.enc.TokenCount(content)), nil
}

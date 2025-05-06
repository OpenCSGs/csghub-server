package token

import (
	"errors"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
)

// unsupported tokenizer error
var errUnsupportedTokenizer = errors.New("unsupported tokenizer")

// use Message, compatible with text and images
type Tokenizer interface {
	Encode(types.Message) (int64, error)
	EmbeddingEncode(string) (int64, error)
}

func NewTokenizerImpl(endpoint, model, imageID string) Tokenizer {
	switch {
	case strings.Contains(imageID, "vllm-local"):
		return newVllmTokenizerImpl(endpoint, model)
	case strings.Contains(imageID, "llama.cpp"):
		return newLlamacppTokenizerImpl(endpoint, model)
	case strings.Contains(imageID, "tgi"):
		return newTGITokenizerImpl(endpoint, model)
	case strings.Contains(imageID, "sglang"):
		return newSGLangTokenizerImpl(endpoint, model)
	case strings.Contains(imageID, "tei"):
		return newTEITokenizerImpl(endpoint, model)
	default:
		return nil
	}
}

var _ Tokenizer = (*DumyTokenizer)(nil)

// dumyy tokenizer for testing only
type DumyTokenizer struct{}

// Encode implements Tokenizer.
func (d *DumyTokenizer) Encode(s types.Message) (int64, error) {
	return int64(len(s.Content.(string))), nil
}

func (d *DumyTokenizer) EmbeddingEncode(content string) (int64, error) {
	return int64(len(content)), nil
}

func parseTextMessage(message types.Message) string {
	// prompt
	if message.Role != "" {
		if message.Content != "" {
			// prompt content
			return "<|im_start|>" + message.Role + "\n" + message.Content.(string) + "<|im_end|>\n"
		} else {
			// prompt end, before response
			return "<|im_start|>" + message.Role + "\n"
		}
	} else {
		// response completion
		if message.Content != "" {
			return message.Content.(string) + "<|im_end|>"
		} else {
			return ""
		}
	}
}

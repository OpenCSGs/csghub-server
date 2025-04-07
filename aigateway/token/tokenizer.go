package token

import (
	"context"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/llm"
)

// use Message, compatible with text and images
type Tokenizer interface {
	Encode(types.Message) (int64, error)
}

func NewTokenizerImpl(endpoint, model, hardware, frameWork string) Tokenizer {
	switch strings.ToLower(frameWork) {
	case "vllm":
		return NewVllmTokenizerImpl(endpoint, model, hardware)
	case "llama.cpp":
		return NewLlamacppTokenizerImpl(endpoint, model, hardware)
	case "tgi":
		return NewTgiTokenizerImpl(endpoint, model, hardware)
	case "sglang":
		return NewSglangTokenizerImpl(endpoint, model, hardware)
	default:
		return &DumyTokenizer{}
	}
}

var _ Tokenizer = (*DumyTokenizer)(nil)

// dumyy tokenizer for testing only
type DumyTokenizer struct{}

// Encode implements Tokenizer.
func (d *DumyTokenizer) Encode(s types.Message) (int64, error) {
	return int64(len(s.Content.(string))), nil
}

type llmSvcClientWrapper struct {
	endpoint  string
	framework string
	model     string
	hc        llm.LLMSvcClient
}

func (client *llmSvcClientWrapper) Tokenize(ctx context.Context, content string) (int64, error) {
	return client.hc.Tokenize(ctx, nil, client.endpoint, client.framework, client.model, content)
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

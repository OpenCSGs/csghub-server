package token

import (
	"context"
	"log/slog"

	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/llm"
)

type VllmTokenizerImpl struct {
	tokens int64
	hc     *llmSvcClientWrapper
}

func NewVllmTokenizerImpl(endpoint, model, hardware string) Tokenizer {
	return &VllmTokenizerImpl{
		tokens: 0,
		hc: &llmSvcClientWrapper{
			endpoint:  endpoint,
			framework: "VLLM",
			model:     model,
			hc:        llm.NewClient(),
		},
	}
}

func (tk *VllmTokenizerImpl) Encode(message types.Message) (int64, error) {
	switch message.Content.(type) {
	case string:
		ctx := context.Background()
		slog.Debug("call tokenize api for", slog.String("content", message.Content.(string)), slog.String("role", message.Role))
		parsedMessage := parseTextMessage(message)
		if parsedMessage == "" {
			return 0, nil
		}
		tokenRes, err := tk.hc.Tokenize(ctx, parsedMessage)
		if err != nil {
			slog.Error("Call inference model", slog.Any("error", err))
			return 0, err
		}
		return tokenRes, nil
	default:
		return 0, nil
	}
}

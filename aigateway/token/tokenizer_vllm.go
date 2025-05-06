package token

import (
	"context"
	"encoding/json"
	"log/slog"

	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/llm"
)

type vllmTokenizerImpl struct {
	endpoint string
	model    string
	hc       llm.LLMSvcClient
}

func newVllmTokenizerImpl(endpoint, model string) Tokenizer {
	return &vllmTokenizerImpl{
		endpoint: endpoint,
		model:    model,
		hc:       llm.NewClient(),
	}
}

func (tk *vllmTokenizerImpl) Encode(message types.Message) (int64, error) {
	switch message.Content.(type) {
	case string:
		const path = "/tokenize"
		ctx := context.Background()
		slog.Debug("call tokenize api for", slog.String("content", message.Content.(string)), slog.String("role", message.Role))
		parsedMessage := parseTextMessage(message)
		if parsedMessage == "" {
			return 0, nil
		}
		req := &llm.VllmGPUTokenizeReq{
			Model:  tk.model,
			Prompt: parsedMessage,
		}
		tokenRespByte, err := tk.hc.Tokenize(ctx, tk.endpoint+path, req)
		if err != nil {
			slog.Error("Call inference model", slog.Any("error", err))
			return 0, err
		}
		var resp llm.VllmGPUTokenizeResponse
		err = json.Unmarshal(tokenRespByte, &resp)
		if err != nil {
			return 0, err
		}
		return resp.Count, nil
	default:
		return 0, nil
	}
}

func (tk *vllmTokenizerImpl) EmbeddingEncode(message string) (int64, error) {
	return 0, errUnsupportedTokenizer
}

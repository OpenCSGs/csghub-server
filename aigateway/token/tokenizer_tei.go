package token

import (
	"context"
	"encoding/json"
	"log/slog"

	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/llm"
)

type teiTokenizerImpl struct {
	endpoint string
	host     string
	model    string
	hc       llm.LLMSvcClient
}

func newTEITokenizerImpl(endpoint, host, model string) Tokenizer {
	return &teiTokenizerImpl{
		endpoint: endpoint,
		host:     host,
		model:    model,
		hc:       llm.NewClient(),
	}
}

func (tk *teiTokenizerImpl) Encode(message types.Message) (int64, error) {
	return 0, errUnsupportedTokenizer
}

func (tk *teiTokenizerImpl) EmbeddingEncode(message string) (int64, error) {
	const path = "/tokenize"
	ctx := context.Background()
	slog.Debug("call tokenize api for", slog.String("content", message))
	req := &llm.TEIEmbeddingTokenizeReq{
		AddSpecialTokens: false,
		Inputs:           &message,
	}
	tokenRespByte, err := tk.hc.Tokenize(ctx, tk.endpoint+path, tk.host, req)
	if err != nil {
		slog.Error("Call inference model", slog.Any("error", err))
		return 0, err
	}
	var resp llm.TEIEmbeddingTokenizeResponse
	err = json.Unmarshal(tokenRespByte, &resp)
	if err != nil {
		return 0, err
	}
	return int64(len(resp[0])), nil
}

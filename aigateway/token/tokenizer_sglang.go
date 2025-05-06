package token

import (
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/llm"
)

type sglangTokenizerImpl struct {
	endpoint string
	model    string
	hc       llm.LLMSvcClient
}

func newSGLangTokenizerImpl(endpoint, model string) Tokenizer {
	return &sglangTokenizerImpl{
		endpoint: endpoint,
		model:    model,
		hc:       llm.NewClient(),
	}
}

func (tk *sglangTokenizerImpl) Encode(message types.Message) (int64, error) {
	return 0, errUnsupportedTokenizer
}

func (tk *sglangTokenizerImpl) EmbeddingEncode(message string) (int64, error) {
	return 0, errUnsupportedTokenizer
}

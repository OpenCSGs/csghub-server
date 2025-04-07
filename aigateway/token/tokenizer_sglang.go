package token

import (
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/llm"
)

type SglangTokenizerImpl struct {
	tokens int64
	hc     *llmSvcClientWrapper
}

func NewSglangTokenizerImpl(endpoint, model, hardware string) Tokenizer {
	return &SglangTokenizerImpl{
		tokens: 0,
		hc: &llmSvcClientWrapper{
			endpoint:  endpoint,
			framework: "SGLang",
			model:     model,
			hc:        llm.NewClient(),
		},
	}
}

// tokenize API error
func (tk *SglangTokenizerImpl) Encode(message types.Message) (int64, error) {
	switch message.Content.(type) {
	case string:
		// TODO: local calculate token
		return 0, nil
	default:
		return 0, nil
	}
}

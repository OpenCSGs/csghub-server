package audio

import (
	"net/http"

	"opencsg.com/csghub-server/aigateway/types"
)

type OpenAICompatibleAdapter struct{}

func NewOpenAICompatibleAdapter() *OpenAICompatibleAdapter {
	return &OpenAICompatibleAdapter{}
}

func (a *OpenAICompatibleAdapter) Name() string {
	return "openai-compatible"
}

func (a *OpenAICompatibleAdapter) CanHandle(model *types.Model) bool {
	return model != nil
}

func (a *OpenAICompatibleAdapter) DurationFromHeader(header http.Header) (float64, bool) {
	return 0, false
}

package text2image

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
	commonTypes "opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/builder/compress"
)

var openAIProviders = []string{"openai", "azure", "infini-ai"}

type OpenAICompatibleAdapter struct{}

func NewOpenAICompatibleAdapter() *OpenAICompatibleAdapter {
	return &OpenAICompatibleAdapter{}
}

func (a *OpenAICompatibleAdapter) Name() string {
	return "openai-compatible"
}

func (a *OpenAICompatibleAdapter) CanHandle(model *types.Model) bool {
	if model == nil {
		return false
	}
	if model.Provider != "" {
		lowerProvider := strings.ToLower(model.Provider)
		return slices.Contains(openAIProviders, lowerProvider)
	}
	if strings.Contains(strings.ToLower(model.RuntimeFramework), frameworkHFInferenceToolkit) {
		return false
	}
	openAIFrameworks := []string{"vllm", "tgi", "text-generation-inference", "sglang"}
	lowerFramework := strings.ToLower(model.RuntimeFramework)
	if slices.ContainsFunc(openAIFrameworks, func(fw string) bool { return strings.Contains(lowerFramework, fw) }) {
		return true
	}
	if model.Task == string(commonTypes.Text2Image) {
		return true
	}
	return false
}

func (a *OpenAICompatibleAdapter) TransformRequest(ctx context.Context, openaiReq types.ImageGenerationRequest) ([]byte, error) {
	return json.Marshal(openaiReq)
}

func (a *OpenAICompatibleAdapter) NeedsResponseTransform() bool {
	return false
}

func (a *OpenAICompatibleAdapter) TransformResponse(ctx context.Context, respBody []byte, contentType string, encodingHeader string, opts *types.TransformResponseOptions) ([]byte, *types.ImageGenerationResponse, error) {
	decoded, err := compress.Decode(encodingHeader, respBody)
	if err != nil {
		decoded = respBody
	}
	var resp types.ImageGenerationResponse
	if err := json.Unmarshal(decoded, &resp); err != nil {
		return nil, nil, err
	}
	return decoded, &resp, nil
}

func (a *OpenAICompatibleAdapter) GetHeaders(model *types.Model, req *types.ImageGenerationRequest) map[string]string {
	return map[string]string{
		"Content-Type": "application/json",
	}
}

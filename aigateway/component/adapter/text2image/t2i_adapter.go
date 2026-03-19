package text2image

import (
	"context"

	"opencsg.com/csghub-server/aigateway/types"
)

type T2IAdapter interface {
	Name() string
	CanHandle(model *types.Model) bool
	TransformRequest(ctx context.Context, openaiReq types.ImageGenerationRequest) ([]byte, error)
	NeedsResponseTransform() bool
	// TransformResponse decodes respBody (using encodingHeader), transforms if needed, and returns the exact body to write and the parsed response.
	// opts may be nil; when non-nil, ResponseFormat and Storage can be used to e.g. upload image to S3 and return a presigned URL.
	TransformResponse(ctx context.Context, respBody []byte, contentType string, encodingHeader string, opts *types.TransformResponseOptions) (bodyToWrite []byte, openaiResp *types.ImageGenerationResponse, err error)
	GetHeaders(model *types.Model, req *types.ImageGenerationRequest) map[string]string
}

type Registry struct {
	adapters []T2IAdapter
}

func NewRegistry() *Registry {
	return &Registry{
		adapters: []T2IAdapter{
			NewHFInferenceToolkitAdapter(),
			NewOpenAICompatibleAdapter(),
		},
	}
}

func (r *Registry) GetAdapter(model *types.Model) T2IAdapter {
	if model == nil {
		return nil
	}
	for _, adapter := range r.adapters {
		if adapter.CanHandle(model) {
			return adapter
		}
	}
	return nil
}

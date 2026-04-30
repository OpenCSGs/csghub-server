package text2video

import (
	"context"
	"mime/multipart"
	"net/url"

	"opencsg.com/csghub-server/aigateway/types"
)

type Capabilities struct {
	SupportsCreate                  bool
	SupportsImageReference          bool
	SupportsMultipartInputReference bool
	SupportsJSONFileID              bool
	SupportsJSONImageURL            bool
	SupportsDirectContentStreaming  bool
}

type CreateRequestInput struct {
	Request     types.VideoGenerationRequest
	Multipart   *multipart.Form
	IsMultipart bool
}

type T2VAdapter interface {
	Name() string
	CanHandle(model *types.Model) bool
	Capabilities(model *types.Model) Capabilities
	BuildCreateRequest(ctx context.Context, model *types.Model, input CreateRequestInput) (*ProviderRequest, error)
	ParseCreateResponse(ctx context.Context, body []byte) (*ProviderResponse, error)
	BuildRetrieveRequest(ctx context.Context, model *types.Model, providerResourceID string, providerMetadata map[string]any) (*ProviderRequest, error)
	ParseRetrieveResponse(ctx context.Context, body []byte) (*ProviderResponse, error)
	BuildContentRequest(ctx context.Context, model *types.Model, providerResourceID string, providerMetadata map[string]any) (*ProviderRequest, error)
	ParseContentResponse(ctx context.Context, body []byte) (*ContentResponse, error)
}

type ProviderRequest struct {
	Method      string
	Path        string
	Query       url.Values
	Body        []byte
	ContentType string
}

type ProviderResponse struct {
	Video            *types.VideoObject
	ProviderMetadata map[string]any
}

type ContentResponse struct {
	DownloadURL      string
	ProviderMetadata map[string]any
}

type Registry struct {
	adapters []T2VAdapter
}

func NewRegistry() *Registry {
	return &Registry{
		adapters: []T2VAdapter{
			NewMiniMaxAdapter(),
			NewSeedanceAdapter(),
			NewLightX2VAdapter(),
			NewOpenAICompatibleAdapter(),
		},
	}
}

func (r *Registry) GetAdapter(model *types.Model) T2VAdapter {
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

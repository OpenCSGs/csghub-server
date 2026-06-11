package text2video

import (
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"

	"opencsg.com/csghub-server/aigateway/types"
	commonTypes "opencsg.com/csghub-server/common/types"
	commonutils "opencsg.com/csghub-server/common/utils/common"
)

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
	if HasVideoAPIConfig(model) {
		return false
	}
	return model.Task == string(commonTypes.Text2Video) || model.Task == string(commonTypes.Image2Video)
}

func (a *OpenAICompatibleAdapter) Capabilities(model *types.Model) Capabilities {
	if !a.CanHandle(model) {
		return Capabilities{}
	}
	return Capabilities{
		SupportsCreate:                  true,
		SupportsImageReference:          true,
		SupportsMultipartInputReference: true,
		SupportsJSONFileID:              true,
		SupportsJSONImageURL:            true,
		SupportsDirectContentStreaming:  true,
	}
}

func (a *OpenAICompatibleAdapter) TransformRequest(ctx context.Context, req types.VideoGenerationRequest) ([]byte, error) {
	return json.Marshal(req)
}

func (a *OpenAICompatibleAdapter) ParseVideoResponse(ctx context.Context, body []byte) (*types.VideoObject, error) {
	var resp types.VideoObject
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (a *OpenAICompatibleAdapter) BuildCreateRequest(ctx context.Context, model *types.Model, input CreateRequestInput) (*ProviderRequest, error) {
	if input.IsMultipart {
		body, contentType, err := buildMultipartBody(func(writer *multipart.Writer) error {
			if err := writer.WriteField("model", input.Request.Model); err != nil {
				return err
			}
			return writeMultipartValues(writer, input.Multipart, map[string]struct{}{"model": {}})
		})
		if err != nil {
			return nil, err
		}
		return &ProviderRequest{
			Method:      http.MethodPost,
			Path:        commonutils.ExtractURLPath(model.Endpoint),
			Body:        body,
			ContentType: contentType,
		}, nil
	}
	body, err := a.TransformRequest(ctx, input.Request)
	if err != nil {
		return nil, err
	}
	return &ProviderRequest{
		Method:      http.MethodPost,
		Path:        commonutils.ExtractURLPath(model.Endpoint),
		Body:        body,
		ContentType: "application/json",
	}, nil
}

func (a *OpenAICompatibleAdapter) ParseCreateResponse(ctx context.Context, body []byte) (*ProviderResponse, error) {
	video, err := a.ParseVideoResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	return &ProviderResponse{Video: video, ProviderMetadata: WithProviderStatus(nil, video.Status)}, nil
}

func (a *OpenAICompatibleAdapter) BuildRetrieveRequest(ctx context.Context, model *types.Model, providerResourceID string, providerMetadata map[string]any) (*ProviderRequest, error) {
	return &ProviderRequest{
		Method: http.MethodGet,
		Path:   commonutils.JoinURLPath(commonutils.ExtractURLPath(model.Endpoint), providerResourceID),
	}, nil
}

func (a *OpenAICompatibleAdapter) ParseRetrieveResponse(ctx context.Context, body []byte) (*ProviderResponse, error) {
	return a.ParseCreateResponse(ctx, body)
}

func (a *OpenAICompatibleAdapter) BuildContentRequest(ctx context.Context, model *types.Model, providerResourceID string, providerMetadata map[string]any) (*ProviderRequest, error) {
	return &ProviderRequest{
		Method: http.MethodGet,
		Path:   commonutils.JoinURLPath(commonutils.ExtractURLPath(model.Endpoint), providerResourceID, "content"),
	}, nil
}

func (a *OpenAICompatibleAdapter) ParseContentResponse(ctx context.Context, body []byte) (*ContentResponse, error) {
	return &ContentResponse{}, nil
}

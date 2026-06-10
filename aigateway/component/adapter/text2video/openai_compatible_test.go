package text2video

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commonTypes "opencsg.com/csghub-server/common/types"
)

func TestOpenAICompatibleAdapter_CanHandle(t *testing.T) {
	adapter := NewOpenAICompatibleAdapter()

	require.True(t, adapter.CanHandle(&types.Model{
		BaseModel:         types.BaseModel{Task: string(commonTypes.Text2Video)},
		ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"},
	}))
	require.True(t, adapter.CanHandle(&types.Model{
		BaseModel:         types.BaseModel{Task: string(commonTypes.Text2Video)},
		ExternalModelInfo: types.ExternalModelInfo{Provider: "custom-openai-compatible"},
	}))
	require.True(t, adapter.CanHandle(&types.Model{
		BaseModel: types.BaseModel{Task: string(commonTypes.Text2Video)},
	}))
	require.True(t, adapter.CanHandle(&types.Model{
		BaseModel: types.BaseModel{Task: string(commonTypes.Image2Video)},
	}))
	require.False(t, adapter.CanHandle(&types.Model{
		BaseModel: types.BaseModel{Task: string(commonTypes.Text2Image)},
	}))

	caps := adapter.Capabilities(&types.Model{
		BaseModel:         types.BaseModel{Task: string(commonTypes.Image2Video)},
		ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"},
	})
	require.True(t, caps.SupportsCreate)
	require.True(t, caps.SupportsImageReference)
	require.True(t, caps.SupportsMultipartInputReference)
	require.True(t, caps.SupportsJSONFileID)
	require.True(t, caps.SupportsJSONImageURL)
	require.True(t, caps.SupportsDirectContentStreaming)
}

func TestOpenAICompatibleAdapter_TransformAndParse(t *testing.T) {
	adapter := NewOpenAICompatibleAdapter()
	req := types.VideoGenerationRequest{
		Model:   "gpt-video",
		Prompt:  "a small robot walking on the moon",
		Size:    "1280x720",
		Seconds: 5,
		InputReference: &types.ImageInputReferenceParam{
			ImageURL: "https://example.com/frame.png",
		},
	}

	body, err := adapter.TransformRequest(context.Background(), req)
	require.NoError(t, err)
	require.Contains(t, string(body), `"model":"gpt-video"`)
	require.Contains(t, string(body), `"input_reference":{"image_url":"https://example.com/frame.png"}`)

	resp, err := adapter.ParseVideoResponse(context.Background(), []byte(`{"id":"vid_123","object":"video","status":"queued"}`))
	require.NoError(t, err)
	require.Equal(t, "vid_123", resp.ID)
	require.Equal(t, string(commonTypes.AIGatewayAsyncGenerationStatusQueued), resp.Status)
}

func TestOpenAICompatibleAdapter_BuildCreateRequest_Multipart(t *testing.T) {
	adapter := NewOpenAICompatibleAdapter()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "original-model"))
	require.NoError(t, writer.WriteField("prompt", "animate this image"))
	part, err := writer.CreateFormFile("input_reference", "frame.png")
	require.NoError(t, err)
	_, err = io.WriteString(part, "fake-image")
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(body.Bytes()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, req.ParseMultipartForm(int64(body.Len()+1024)))

	providerReq, err := adapter.BuildCreateRequest(context.Background(), &types.Model{Endpoint: "https://example.com/v1/videos"}, CreateRequestInput{
		Request: types.VideoGenerationRequest{
			Model: "rewritten-model",
		},
		Multipart:   req.MultipartForm,
		IsMultipart: true,
	})
	require.NoError(t, err)
	require.Equal(t, "/v1/videos", providerReq.Path)
	require.Contains(t, providerReq.ContentType, "multipart/form-data")
	require.Contains(t, string(providerReq.Body), `name="model"`)
	require.Contains(t, string(providerReq.Body), "rewritten-model")
	require.NotContains(t, string(providerReq.Body), "original-model")
	require.Contains(t, string(providerReq.Body), `name="input_reference"`)
}

func TestOpenAICompatibleAdapter_ParseRetrieveResponseSetsProviderStatus(t *testing.T) {
	adapter := NewOpenAICompatibleAdapter()

	resp, err := adapter.ParseRetrieveResponse(context.Background(), []byte(`{"id":"vid_1","object":"video","status":"in_progress"}`))
	require.NoError(t, err)
	require.Equal(t, "vid_1", resp.Video.ID)
	require.Equal(t, string(commonTypes.AIGatewayAsyncGenerationStatusInProgress), resp.Video.Status)
	require.Equal(t, "in_progress", resp.ProviderMetadata[ProviderStatusMetadataKey])
}

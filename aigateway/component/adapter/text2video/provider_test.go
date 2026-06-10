package text2video

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commonTypes "opencsg.com/csghub-server/common/types"
)

func TestRegistry_SelectsProviderAdapterByVideoAPIType(t *testing.T) {
	registry := NewRegistry()

	adapter := registry.GetAdapter(&types.Model{
		BaseModel: types.BaseModel{
			Task: string(commonTypes.Text2Video),
			Metadata: map[string]any{
				"video_api": map[string]any{"type": "minimax"},
			},
		},
		ExternalModelInfo: types.ExternalModelInfo{Provider: "minimax"},
	})
	require.Equal(t, "minimax", adapter.Name())

	adapter = registry.GetAdapter(&types.Model{
		BaseModel: types.BaseModel{
			Task: string(commonTypes.Text2Video),
			Metadata: map[string]any{
				"video_api": map[string]any{"type": "unknown"},
			},
		},
		ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"},
	})
	require.Nil(t, adapter)
}

func TestRegistry_SelectsLightX2VAdapter(t *testing.T) {
	registry := NewRegistry()

	adapter := registry.GetAdapter(&types.Model{
		BaseModel: types.BaseModel{
			Task: string(commonTypes.Text2Video),
		},
		InternalModelInfo: types.InternalModelInfo{
			CSGHubModelID:    "Wan-AI/Wan2.2-T2V-A14B",
			RuntimeFramework: "lightx2v",
		},
	})
	require.Equal(t, "lightx2v", adapter.Name())
}

func TestMiniMaxAdapter_NormalizesVideoAPI(t *testing.T) {
	adapter := NewMiniMaxAdapter()
	model := &types.Model{
		BaseModel: types.BaseModel{
			Metadata: map[string]any{"video_api": map[string]any{"type": "minimax"}},
		},
		Endpoint: "https://api.minimaxi.com/v1/video_generation",
	}

	providerReq, err := adapter.BuildCreateRequest(context.Background(), model, CreateRequestInput{
		Request: types.VideoGenerationRequest{
			Model:   "MiniMax-Hailuo-02",
			Prompt:  "a calm sea",
			Size:    "1080P",
			Seconds: 6,
			InputReference: &types.ImageInputReferenceParam{
				ImageURL: "https://example.com/frame.png",
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "/v1/video_generation", providerReq.Path)
	require.JSONEq(t, `{"model":"MiniMax-Hailuo-02","prompt":"a calm sea","resolution":"1080P","duration":6,"first_frame_image":"https://example.com/frame.png"}`, string(providerReq.Body))

	retrieveReq, err := adapter.BuildRetrieveRequest(context.Background(), model, "task_123", nil)
	require.NoError(t, err)
	require.Equal(t, "/v1/query/video_generation", retrieveReq.Path)

	contentReq, err := adapter.BuildContentRequest(context.Background(), model, "task_123", map[string]any{"file_id": "file_123"})
	require.NoError(t, err)
	require.Equal(t, "/v1/files/retrieve", contentReq.Path)

	resp, err := adapter.ParseCreateResponse(context.Background(), []byte(`{"task_id":"task_123"}`))
	require.NoError(t, err)
	require.Equal(t, "task_123", resp.Video.ID)
	require.Equal(t, string(commonTypes.AIGatewayAsyncGenerationStatusQueued), resp.Video.Status)

	resp, err = adapter.ParseRetrieveResponse(context.Background(), []byte(`{"task_id":"task_123","status":"Success","file_id":"file_123"}`))
	require.NoError(t, err)
	require.Equal(t, string(commonTypes.AIGatewayAsyncGenerationStatusCompleted), resp.Video.Status)
	require.Equal(t, "file_123", resp.ProviderMetadata["file_id"])
	require.Equal(t, "Success", resp.ProviderMetadata[ProviderStatusMetadataKey])

	content, err := adapter.ParseContentResponse(context.Background(), []byte(`{"file":{"download_url":"https://files.example.com/video.mp4"}}`))
	require.NoError(t, err)
	require.Equal(t, "https://files.example.com/video.mp4", content.DownloadURL)
}

func TestMiniMaxAdapter_NormalizesOpenAICompatibleSize(t *testing.T) {
	adapter := NewMiniMaxAdapter()
	model := &types.Model{
		BaseModel: types.BaseModel{
			Metadata: map[string]any{"video_api": map[string]any{"type": "minimax"}},
		},
		Endpoint: "https://api.minimaxi.com/v1/video_generation",
	}

	providerReq, err := adapter.BuildCreateRequest(context.Background(), model, CreateRequestInput{
		Request: types.VideoGenerationRequest{
			Model:   "MiniMax-Hailuo-02",
			Prompt:  "a calm sea",
			Size:    "1280x720",
			Seconds: 6,
		},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"MiniMax-Hailuo-02","prompt":"a calm sea","resolution":"768P","duration":6}`, string(providerReq.Body))
}

func TestMiniMaxAdapter_RejectsUnsupportedOpenAICompatibleSize(t *testing.T) {
	adapter := NewMiniMaxAdapter()
	model := &types.Model{
		BaseModel: types.BaseModel{
			Metadata: map[string]any{"video_api": map[string]any{"type": "minimax"}},
		},
		Endpoint: "https://api.minimaxi.com/v1/video_generation",
	}

	_, err := adapter.BuildCreateRequest(context.Background(), model, CreateRequestInput{
		Request: types.VideoGenerationRequest{
			Model:  "MiniMax-Hailuo-02",
			Prompt: "a calm sea",
			Size:   "1024x1792",
		},
	})
	require.Error(t, err)
	var requestValidationErr *RequestValidationError
	require.ErrorAs(t, err, &requestValidationErr)
	require.Contains(t, requestValidationErr.Error(), "MiniMax video backend does not support size")
}

func TestMiniMaxAdapter_SurfacesProviderErrorEnvelope(t *testing.T) {
	adapter := NewMiniMaxAdapter()

	_, err := adapter.ParseCreateResponse(context.Background(), []byte(`{
		"base_resp": {
			"status_code": 1004,
			"status_msg": "unsupported resolution for model"
		}
	}`))
	require.Error(t, err)
	var requestValidationErr *RequestValidationError
	require.ErrorAs(t, err, &requestValidationErr)
	require.Equal(t, "unsupported resolution for model", requestValidationErr.Error())
}

func TestMiniMaxAdapter_DerivesPrefixedRoutesFromEndpoint(t *testing.T) {
	adapter := NewMiniMaxAdapter()
	model := &types.Model{
		BaseModel: types.BaseModel{
			Metadata: map[string]any{"video_api": map[string]any{"type": "minimax"}},
		},
		Endpoint: "https://cloud.infini-ai.com/maas/router/minimax/v1/video_generation",
	}

	createReq, err := adapter.BuildCreateRequest(context.Background(), model, CreateRequestInput{
		Request: types.VideoGenerationRequest{
			Model:  "MiniMax-Hailuo-02",
			Prompt: "a calm sea",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "/maas/router/minimax/v1/video_generation", createReq.Path)

	retrieveReq, err := adapter.BuildRetrieveRequest(context.Background(), model, "task_123", nil)
	require.NoError(t, err)
	require.Equal(t, "/maas/router/minimax/v1/query/video_generation", retrieveReq.Path)

	contentReq, err := adapter.BuildContentRequest(context.Background(), model, "task_123", map[string]any{"file_id": "file_123"})
	require.NoError(t, err)
	require.Equal(t, "/maas/router/minimax/v1/files/retrieve", contentReq.Path)
}

func TestMiniMaxAdapter_RetrieveRawStatusUnmapped(t *testing.T) {
	adapter := NewMiniMaxAdapter()

	resp, err := adapter.ParseRetrieveResponse(context.Background(), []byte(`{"task_id":"task_123","status":"weird"}`))
	require.NoError(t, err)
	require.Equal(t, "weird", resp.Video.Status)
	require.Equal(t, "weird", resp.ProviderMetadata[ProviderStatusMetadataKey])
}

func TestMiniMaxAdapter_CreateResponseHasNoStatus(t *testing.T) {
	adapter := NewMiniMaxAdapter()

	resp, err := adapter.ParseCreateResponse(context.Background(), []byte(`{"task_id":"task_123"}`))
	require.NoError(t, err)
	require.Equal(t, "task_123", resp.Video.ID)
	require.Equal(t, string(commonTypes.AIGatewayAsyncGenerationStatusQueued), resp.Video.Status)
	require.Nil(t, resp.ProviderMetadata)
}

func TestSeedanceAdapter_NormalizesVideoAPI(t *testing.T) {
	adapter := NewSeedanceAdapter()
	model := &types.Model{
		BaseModel: types.BaseModel{Metadata: map[string]any{"video_api": map[string]any{"type": "seedance"}}},
		Endpoint:  "https://ark.ap-southeast.bytepluses.com/api/v3/contents/generations/tasks",
	}

	providerReq, err := adapter.BuildCreateRequest(context.Background(), model, CreateRequestInput{
		Request: types.VideoGenerationRequest{
			Model:   "seedance-v1",
			Prompt:  "a mountain lake",
			Size:    "1280x720",
			Seconds: 5,
			InputReference: &types.ImageInputReferenceParam{
				ImageURL: "https://example.com/frame.png",
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "/api/v3/contents/generations/tasks", providerReq.Path)
	require.JSONEq(t, `{
		"model":"seedance-v1",
		"content":[
			{"type":"text","text":"a mountain lake"},
			{"type":"image_url","image_url":{"url":"https://example.com/frame.png"}}
		],
		"duration":5,
		"resolution":"720p",
		"ratio":"16:9"
	}`, string(providerReq.Body))

	resp, err := adapter.ParseRetrieveResponse(context.Background(), []byte(`{"id":"task_456","status":"succeeded","content":{"video_url":"https://files.example.com/video.mp4"}}`))
	require.NoError(t, err)
	require.Equal(t, "task_456", resp.Video.ID)
	require.Equal(t, string(commonTypes.AIGatewayAsyncGenerationStatusCompleted), resp.Video.Status)
	require.Equal(t, "https://files.example.com/video.mp4", resp.ProviderMetadata["download_url"])
	require.Equal(t, "succeeded", resp.ProviderMetadata[ProviderStatusMetadataKey])

	retrieveReq, err := adapter.BuildRetrieveRequest(context.Background(), model, "task_456", nil)
	require.NoError(t, err)
	require.Equal(t, "/api/v3/contents/generations/tasks/task_456", retrieveReq.Path)

	content, err := adapter.ParseContentResponse(context.Background(), []byte(`{"id":"task_456","status":"succeeded","content":{"video_url":"https://files.example.com/video.mp4"}}`))
	require.NoError(t, err)
	require.Equal(t, "https://files.example.com/video.mp4", content.DownloadURL)
}

func TestSeedanceAdapter_AcceptsNativeResolutionSize(t *testing.T) {
	adapter := NewSeedanceAdapter()
	model := &types.Model{
		BaseModel: types.BaseModel{Metadata: map[string]any{"video_api": map[string]any{"type": "seedance"}}},
		Endpoint:  "https://ark.ap-southeast.bytepluses.com/api/v3/contents/generations/tasks",
	}

	providerReq, err := adapter.BuildCreateRequest(context.Background(), model, CreateRequestInput{
		Request: types.VideoGenerationRequest{
			Model:  "seedance-v1",
			Prompt: "a mountain lake",
			Size:   "1080p",
		},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{
		"model":"seedance-v1",
		"content":[{"type":"text","text":"a mountain lake"}],
		"resolution":"1080p",
		"ratio":"16:9"
	}`, string(providerReq.Body))
}

func TestSeedanceAdapter_RejectsUnsupportedOpenAICompatibleSize(t *testing.T) {
	adapter := NewSeedanceAdapter()
	model := &types.Model{
		BaseModel: types.BaseModel{Metadata: map[string]any{"video_api": map[string]any{"type": "seedance"}}},
		Endpoint:  "https://ark.ap-southeast.bytepluses.com/api/v3/contents/generations/tasks",
	}

	_, err := adapter.BuildCreateRequest(context.Background(), model, CreateRequestInput{
		Request: types.VideoGenerationRequest{
			Model:  "seedance-v1",
			Prompt: "a mountain lake",
			Size:   "1024x1792",
		},
	})
	require.Error(t, err)
	var requestValidationErr *RequestValidationError
	require.ErrorAs(t, err, &requestValidationErr)
	require.Contains(t, requestValidationErr.Error(), "Seedance video backend does not support size")
}

func TestSeedanceAdapter_DerivesPrefixedRoutesFromEndpoint(t *testing.T) {
	adapter := NewSeedanceAdapter()
	model := &types.Model{
		BaseModel: types.BaseModel{Metadata: map[string]any{"video_api": map[string]any{"type": "seedance"}}},
		Endpoint:  "https://proxy.example.com/byteplus/router/api/v3/contents/generations/tasks",
	}

	createReq, err := adapter.BuildCreateRequest(context.Background(), model, CreateRequestInput{
		Request: types.VideoGenerationRequest{
			Model:  "seedance-v1",
			Prompt: "a mountain lake",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "/byteplus/router/api/v3/contents/generations/tasks", createReq.Path)

	retrieveReq, err := adapter.BuildRetrieveRequest(context.Background(), model, "task_456", nil)
	require.NoError(t, err)
	require.Equal(t, "/byteplus/router/api/v3/contents/generations/tasks/task_456", retrieveReq.Path)
}

func TestSeedanceAdapter_PropagatesFailureMessage(t *testing.T) {
	adapter := NewSeedanceAdapter()

	for _, status := range []string{"failed", "cancelled", "canceled"} {
		resp, err := adapter.ParseRetrieveResponse(context.Background(), []byte(`{
			"id":"task_456",
			"status":"`+status+`",
			"message":"unsafe prompt"
		}`))
		require.NoError(t, err)
		require.Equal(t, string(commonTypes.AIGatewayAsyncGenerationStatusFailed), resp.Video.Status)
		require.NotNil(t, resp.Video.Error)
		require.Equal(t, "generation_failed", resp.Video.Error.Code)
		require.Equal(t, "unsafe prompt", resp.Video.Error.Message)
		require.Equal(t, status, resp.ProviderMetadata[ProviderStatusMetadataKey])
	}
}

func TestSeedanceAdapter_RawStatusUnmapped(t *testing.T) {
	adapter := NewSeedanceAdapter()

	resp, err := adapter.ParseRetrieveResponse(context.Background(), []byte(`{"id":"task_456","status":"WeirdValue"}`))
	require.NoError(t, err)
	require.Equal(t, "WeirdValue", resp.Video.Status)
	require.Equal(t, "WeirdValue", resp.ProviderMetadata[ProviderStatusMetadataKey])
}

func TestSeedanceAdapter_RawStatusAbsentOnEmpty(t *testing.T) {
	adapter := NewSeedanceAdapter()

	resp, err := adapter.ParseCreateResponse(context.Background(), []byte(`{"id":"task_456"}`))
	require.NoError(t, err)
	require.Equal(t, string(commonTypes.AIGatewayAsyncGenerationStatusQueued), resp.Video.Status)
	_, hasKey := resp.ProviderMetadata[ProviderStatusMetadataKey]
	require.False(t, hasKey)
}

func TestWithProviderStatus_SetsKeyOnNonEmpty(t *testing.T) {
	fromNil := WithProviderStatus(nil, "running")
	require.NotNil(t, fromNil)
	require.Equal(t, "running", fromNil[ProviderStatusMetadataKey])
	require.Len(t, fromNil, 1)

	existing := map[string]any{"file_id": "file_123"}
	merged := WithProviderStatus(existing, "succeeded")
	require.True(t, mapsSameInstance(existing, merged), "should mutate the same map reference")
	require.Equal(t, "file_123", merged["file_id"])
	require.Equal(t, "succeeded", merged[ProviderStatusMetadataKey])
}

func TestWithProviderStatus_DropsOnEmpty(t *testing.T) {
	fromEmpty := WithProviderStatus(nil, "")
	require.Nil(t, fromEmpty)

	fromWhitespace := WithProviderStatus(nil, "   \t\n")
	require.Nil(t, fromWhitespace)

	existing := map[string]any{"file_id": "file_123"}
	unchanged := WithProviderStatus(existing, " ")
	require.True(t, mapsSameInstance(existing, unchanged))
	require.Equal(t, map[string]any{"file_id": "file_123"}, unchanged)
	_, hasKey := unchanged[ProviderStatusMetadataKey]
	require.False(t, hasKey)
}

func mapsSameInstance(a, b map[string]any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

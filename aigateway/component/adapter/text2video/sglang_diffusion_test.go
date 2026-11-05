package text2video

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commonTypes "opencsg.com/csghub-server/common/types"
)

func newSGLangDiffusionModel() *types.Model {
	return &types.Model{
		BaseModel:         types.BaseModel{Task: string(commonTypes.Text2Video)},
		InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "sglang", CSGHubModelID: "MiniMaxAI/MiniMax-H3"},
	}
}

func TestSGLangDiffusionAdapter_CanHandle(t *testing.T) {
	adapter := NewSGLangDiffusionAdapter()
	require.True(t, adapter.CanHandle(newSGLangDiffusionModel()))
	require.False(t, adapter.CanHandle(nil))
	require.False(t, adapter.CanHandle(&types.Model{
		BaseModel:         types.BaseModel{Task: string(commonTypes.Text2Video)},
		InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "sglang"},
	}))
	require.True(t, adapter.CanHandle(&types.Model{
		BaseModel:         types.BaseModel{Task: string(commonTypes.Image2Video)},
		InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "sglang", CSGHubModelID: "MiniMaxAI/MiniMax-H3"},
	}))
	require.False(t, adapter.CanHandle(&types.Model{
		BaseModel:         types.BaseModel{Task: string(commonTypes.Text2Image)},
		InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "sglang", CSGHubModelID: "MiniMaxAI/MiniMax-H3"},
	}))
	require.False(t, adapter.CanHandle(&types.Model{
		BaseModel:         types.BaseModel{Task: string(commonTypes.Text2Video)},
		InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "vllm", CSGHubModelID: "MiniMaxAI/MiniMax-H3"},
	}))
}

func TestRegistry_SelectsSGLangDiffusionAdapter(t *testing.T) {
	adapter := NewRegistry().GetAdapter(newSGLangDiffusionModel())
	require.NotNil(t, adapter)
	require.Equal(t, sglangDiffusionAdapterName, adapter.Name())
}

func TestSGLangDiffusionAdapter_BuildCreateRequest(t *testing.T) {
	adapter := NewSGLangDiffusionAdapter()
	var req types.VideoGenerationRequest
	require.NoError(t, json.Unmarshal([]byte(`{"model":"MiniMax-H3","prompt":"a calm sea","size":"1344x768","seconds":5,"seed":7,"num_inference_steps":50}`), &req))

	providerReq, err := adapter.BuildCreateRequest(context.Background(), newSGLangDiffusionModel(), CreateRequestInput{Request: req})
	require.NoError(t, err)
	require.Equal(t, "POST", providerReq.Method)
	require.Equal(t, "/v1/videos", providerReq.Path)
	require.Equal(t, "application/json", providerReq.ContentType)
	require.JSONEq(t, `{"prompt":"a calm sea","task":"t2va","seconds":5,"width":1344,"height":768,"seed":7,"num_inference_steps":50,"target":{"short_edge":768,"aspect_ratio":"16:9","duration_seconds":5}}`, string(providerReq.Body))
}

func TestSGLangDiffusionAdapter_BuildCreateRequest_Target(t *testing.T) {
	adapter := NewSGLangDiffusionAdapter()
	cases := []struct {
		name   string
		req    types.VideoGenerationRequest
		target string
	}{
		{"defaults", types.VideoGenerationRequest{Prompt: "a fox"}, `{"short_edge":768,"aspect_ratio":"16:9","duration_seconds":5}`},
		{"portrait", types.VideoGenerationRequest{Prompt: "a fox", Size: "720x1280", Seconds: 10}, `{"short_edge":720,"aspect_ratio":"9:16","duration_seconds":10}`},
		{"square", types.VideoGenerationRequest{Prompt: "a fox", Size: "768x768"}, `{"short_edge":768,"aspect_ratio":"1:1","duration_seconds":5}`},
		{"ultrawide", types.VideoGenerationRequest{Prompt: "a fox", Size: "1792x768"}, `{"short_edge":768,"aspect_ratio":"21:9","duration_seconds":5}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			providerReq, err := adapter.BuildCreateRequest(context.Background(), newSGLangDiffusionModel(), CreateRequestInput{Request: tc.req})
			require.NoError(t, err)
			var body map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(providerReq.Body, &body))
			require.JSONEq(t, tc.target, string(body["target"]))
		})
	}
}

func TestSGLangDiffusionAdapter_BuildCreateRequest_KeepsCallerTask(t *testing.T) {
	adapter := NewSGLangDiffusionAdapter()
	var req types.VideoGenerationRequest
	require.NoError(t, json.Unmarshal([]byte(`{"prompt":"a fox","task":"fl2va"}`), &req))

	providerReq, err := adapter.BuildCreateRequest(context.Background(), newSGLangDiffusionModel(), CreateRequestInput{Request: req})
	require.NoError(t, err)
	require.JSONEq(t, `{"prompt":"a fox","task":"fl2va","target":{"short_edge":768,"aspect_ratio":"16:9","duration_seconds":5}}`, string(providerReq.Body))
}

func TestSGLangDiffusionAdapter_BuildCreateRequest_Rejects(t *testing.T) {
	adapter := NewSGLangDiffusionAdapter()

	_, err := adapter.BuildCreateRequest(context.Background(), newSGLangDiffusionModel(), CreateRequestInput{
		Request: types.VideoGenerationRequest{Prompt: "a calm sea", Size: "large"},
	})
	var validationErr *RequestValidationError
	require.ErrorAs(t, err, &validationErr)

	_, err = adapter.BuildCreateRequest(context.Background(), newSGLangDiffusionModel(), CreateRequestInput{
		Request: types.VideoGenerationRequest{Prompt: "a calm sea", InputReference: &types.ImageInputReferenceParam{ImageURL: "https://example.com/a.png"}},
	})
	require.ErrorAs(t, err, &validationErr)

	// A caller target would make generation diverge from the size and seconds
	// that pricing and metering read.
	var req types.VideoGenerationRequest
	require.NoError(t, json.Unmarshal([]byte(`{"prompt":"a calm sea","size":"768x432","seconds":4,"target":{"short_edge":1080,"aspect_ratio":"16:9","duration_seconds":15}}`), &req))
	_, err = adapter.BuildCreateRequest(context.Background(), newSGLangDiffusionModel(), CreateRequestInput{Request: req})
	require.ErrorAs(t, err, &validationErr)
}

func TestSGLangDiffusionAdapter_ParseResponse(t *testing.T) {
	adapter := NewSGLangDiffusionAdapter()

	resp, err := adapter.ParseCreateResponse(context.Background(), []byte(`{"id":"video_1","object":"video","status":"in_progress","progress":40}`))
	require.NoError(t, err)
	require.Equal(t, "video_1", resp.Video.ID)
	require.Equal(t, "in_progress", resp.Video.Status)
	require.NotNil(t, resp.Video.Progress)
	require.Equal(t, float64(40), *resp.Video.Progress)
	require.Nil(t, resp.Video.Error)
	require.Equal(t, "in_progress", resp.ProviderMetadata[ProviderStatusMetadataKey])

	resp, err = adapter.ParseRetrieveResponse(context.Background(), []byte(`{"id":"video_1","status":"failed","error":{"code":500,"message":"CUDA out of memory"}}`))
	require.NoError(t, err)
	require.Equal(t, "failed", resp.Video.Status)
	require.Equal(t, &types.VideoError{Code: "generation_failed", Message: "CUDA out of memory"}, resp.Video.Error)

	_, err = adapter.ParseCreateResponse(context.Background(), []byte(`{"status":"queued"}`))
	require.Error(t, err)
}

func TestSGLangDiffusionAdapter_RetrieveAndContentPaths(t *testing.T) {
	adapter := NewSGLangDiffusionAdapter()
	model := newSGLangDiffusionModel()

	retrieveReq, err := adapter.BuildRetrieveRequest(context.Background(), model, "video_1", nil)
	require.NoError(t, err)
	require.Equal(t, "GET", retrieveReq.Method)
	require.Equal(t, "/v1/videos/video_1", retrieveReq.Path)

	contentReq, err := adapter.BuildContentRequest(context.Background(), model, "video_1", nil)
	require.NoError(t, err)
	require.Equal(t, "GET", contentReq.Method)
	require.Equal(t, "/v1/videos/video_1/content", contentReq.Path)
}

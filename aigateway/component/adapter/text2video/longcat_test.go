package text2video

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commonTypes "opencsg.com/csghub-server/common/types"
)

func TestLongCatAdapter_CanHandle(t *testing.T) {
	adapter := NewLongCatAdapter()
	require.True(t, adapter.CanHandle(longCatTestModel(string(commonTypes.Text2Video))))
	require.True(t, adapter.CanHandle(longCatTestModel(string(commonTypes.Image2Video))))
	require.True(t, adapter.CanHandle(longCatTestModel(string(commonTypes.AudioText2Video))))
	require.True(t, adapter.CanHandle(longCatTestModel(string(commonTypes.AudioImageText2Video))))
	require.True(t, adapter.CanHandle(longCatTestModel(string(commonTypes.AudioDrivenVideoContinuation))))
	v1Model := longCatTestModel(string(commonTypes.Image2Video))
	v1Model.CSGHubModelID = "meituan-longcat/LongCat-Video-Avatar"
	require.True(t, adapter.CanHandle(v1Model))
	amdModel := longCatTestModel(string(commonTypes.Image2Video))
	amdModel.RuntimeFramework = frameworkAMDLongCat
	require.True(t, adapter.CanHandle(amdModel))

	model := longCatTestModel(string(commonTypes.Text2Video))
	model.RuntimeFramework = "lightx2v"
	require.False(t, adapter.CanHandle(model))

	model = longCatTestModel(string(commonTypes.Text2Video))
	model.CSGHubModelID = "other/model"
	require.False(t, adapter.CanHandle(model))
}

func TestLongCatAdapter_BuildCreateRequestSingleSpeaker(t *testing.T) {
	adapter := NewLongCatAdapter()
	form := buildLongCatMultipartForm(t, 1, false, map[string]string{
		"num_segments":     "3",
		"ref_img_index":    "10",
		"mask_frame_range": "3",
	})
	req, err := adapter.BuildCreateRequest(context.Background(), longCatTestModel(string(commonTypes.Text2Video)), CreateRequestInput{
		Request: types.VideoGenerationRequest{
			Prompt: "a presenter speaking",
			Size:   "832x480",
		},
		Multipart:   form,
		IsMultipart: true,
	})
	require.NoError(t, err)
	require.Equal(t, "/v1/tasks/video/form", req.Path)

	proxied := parseLongCatProviderRequest(t, req)
	require.Equal(t, "a presenter speaking", proxied.FormValue("prompt"))
	require.Equal(t, "480p", proxied.FormValue("resolution"))
	require.Equal(t, "3", proxied.FormValue("num_segments"))
	require.Len(t, proxied.MultipartForm.File["audio"], 1)
	require.Empty(t, proxied.MultipartForm.File["image_file"])
}

func TestLongCatAdapter_BuildCreateRequestMultiSpeaker(t *testing.T) {
	adapter := NewLongCatAdapter()
	form := buildLongCatMultipartForm(t, 2, true, map[string]string{
		"audio_type": "para",
		"bbox":       `{"person1":[0,0,480,400],"person2":[0,432,480,832],"others":[100,400,300,432]}`,
	})
	req, err := adapter.BuildCreateRequest(context.Background(), longCatTestModel(string(commonTypes.Image2Video)), CreateRequestInput{
		Request: types.VideoGenerationRequest{
			Prompt: "two people talking",
			Size:   "1280x768",
		},
		Multipart:   form,
		IsMultipart: true,
	})
	require.NoError(t, err)

	proxied := parseLongCatProviderRequest(t, req)
	require.Equal(t, "720p", proxied.FormValue("resolution"))
	require.Equal(t, "para", proxied.FormValue("audio_type"))
	require.Len(t, proxied.MultipartForm.File["audio"], 2)
	require.Len(t, proxied.MultipartForm.File["image_file"], 1)
	require.Empty(t, proxied.MultipartForm.File["input_reference"])
}

func TestLongCatAdapter_ValidatesRequests(t *testing.T) {
	adapter := NewLongCatAdapter()
	model := longCatTestModel(string(commonTypes.Image2Video))

	_, err := adapter.BuildCreateRequest(context.Background(), model, CreateRequestInput{})
	require.ErrorContains(t, err, "multipart/form-data")

	_, err = adapter.BuildCreateRequest(context.Background(), model, CreateRequestInput{
		Request:     types.VideoGenerationRequest{Prompt: "test"},
		Multipart:   buildLongCatMultipartForm(t, 2, false, nil),
		IsMultipart: true,
	})
	require.ErrorContains(t, err, "requires input_reference")

	_, err = adapter.BuildCreateRequest(context.Background(), model, CreateRequestInput{
		Request:     types.VideoGenerationRequest{Prompt: "test", Size: "1280x720"},
		Multipart:   buildLongCatMultipartForm(t, 1, false, nil),
		IsMultipart: true,
	})
	require.ErrorContains(t, err, "832x480 or 1280x768")

	require.NoError(t, validateLongCatFormValues(
		buildLongCatMultipartForm(t, 1, false, map[string]string{"ref_img_index": "-10"}),
		1,
	))
	for field, value := range map[string]string{
		"num_segments":     "0",
		"ref_img_index":    "-1001",
		"mask_frame_range": "-1",
	} {
		t.Run(field+"_out_of_range", func(t *testing.T) {
			err := validateLongCatFormValues(
				buildLongCatMultipartForm(t, 1, false, map[string]string{field: value}),
				1,
			)
			require.ErrorContains(t, err, field+" must be an integer between")
		})
	}
}

func TestLongCatAdapter_TaskLifecycle(t *testing.T) {
	adapter := NewLongCatAdapter()
	resp, err := adapter.ParseCreateResponse(context.Background(), []byte(`{"task_id":"task_1","status":"submitted"}`))
	require.NoError(t, err)
	require.Equal(t, string(commonTypes.AIGatewayAsyncGenerationStatusQueued), resp.Video.Status)

	resp, err = adapter.ParseRetrieveResponse(context.Background(), []byte(`{"task_id":"task_1","status":"running","progress":0.25}`))
	require.NoError(t, err)
	require.Equal(t, string(commonTypes.AIGatewayAsyncGenerationStatusInProgress), resp.Video.Status)
	require.Equal(t, 0.25, *resp.Video.Progress)

	resp, err = adapter.ParseRetrieveResponse(context.Background(), []byte(`{"task_id":"task_1","status":"succeed","download_url":"https://video-bucket.oss.example.com/aigateway/generated/videos/task_1.mp4"}`))
	require.NoError(t, err)
	require.Equal(t, string(commonTypes.AIGatewayAsyncGenerationStatusCompleted), resp.Video.Status)
	require.Equal(t, "https://video-bucket.oss.example.com/aigateway/generated/videos/task_1.mp4", resp.ProviderMetadata["download_url"])

	retrieve, err := adapter.BuildRetrieveRequest(context.Background(), nil, "task_1", nil)
	require.NoError(t, err)
	require.Equal(t, "/v1/tasks/task_1/status", retrieve.Path)

	content, err := adapter.BuildContentRequest(context.Background(), nil, "task_1", nil)
	require.NoError(t, err)
	require.Equal(t, "/v1/files/download/outputs/videos/task_1.mp4", content.Path)
}

func longCatTestModel(task string) *types.Model {
	return &types.Model{
		BaseModel: types.BaseModel{Task: task},
		InternalModelInfo: types.InternalModelInfo{
			RuntimeFramework: frameworkLongCat,
			CSGHubModelID:    "meituan-longcat/LongCat-Video-Avatar-1.5",
		},
	}
}

func buildLongCatMultipartForm(t *testing.T, audioCount int, withImage bool, values map[string]string) *multipart.Form {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", longCatModelNameV15))
	require.NoError(t, writer.WriteField("prompt", "ignored"))
	for key, value := range values {
		require.NoError(t, writer.WriteField(key, value))
	}
	for index := 0; index < audioCount; index++ {
		part, err := writer.CreateFormFile("audio", "speaker.wav")
		require.NoError(t, err)
		_, err = part.Write([]byte("RIFF-audio"))
		require.NoError(t, err)
	}
	if withImage {
		part, err := writer.CreateFormFile("input_reference", "frame.png")
		require.NoError(t, err)
		_, err = part.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(body.Bytes()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, req.ParseMultipartForm(int64(body.Len()+1024)))
	return req.MultipartForm
}

func parseLongCatProviderRequest(t *testing.T, providerReq *ProviderRequest) *http.Request {
	t.Helper()
	req, err := http.NewRequest(providerReq.Method, providerReq.Path, bytes.NewReader(providerReq.Body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", providerReq.ContentType)
	require.NoError(t, req.ParseMultipartForm(int64(len(providerReq.Body)+1024)))
	return req
}

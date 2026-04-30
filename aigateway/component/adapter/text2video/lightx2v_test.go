package text2video

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commonTypes "opencsg.com/csghub-server/common/types"
)

func TestLightX2VAdapter_CanHandle(t *testing.T) {
	adapter := NewLightX2VAdapter()
	require.True(t, adapter.CanHandle(&types.Model{
		BaseModel:         types.BaseModel{Task: string(commonTypes.Text2Video)},
		InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "lightx2v", CSGHubModelID: "Wan-AI/Wan2.2-T2V-A14B"},
	}))
	require.True(t, adapter.CanHandle(&types.Model{
		BaseModel:         types.BaseModel{Task: string(commonTypes.Image2Video)},
		InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "lightx2v", CSGHubModelID: "Wan-AI/Wan2.2-I2V-A14B"},
	}))
	require.False(t, adapter.CanHandle(&types.Model{
		BaseModel:         types.BaseModel{Task: string(commonTypes.Text2Video)},
		InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "other"},
	}))
	require.False(t, adapter.CanHandle(&types.Model{
		BaseModel:         types.BaseModel{Task: string(commonTypes.Text2Image)},
		InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "lightx2v", CSGHubModelID: "Wan-AI/Wan2.2-T2V-A14B"},
	}))
	require.False(t, adapter.CanHandle(&types.Model{
		BaseModel:         types.BaseModel{Task: string(commonTypes.Text2Video)},
		InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "lightx2v"},
	}))
}

func TestLightX2VAdapter_BuildCreateRequest_T2V(t *testing.T) {
	adapter := NewLightX2VAdapter()
	providerReq, err := adapter.BuildCreateRequest(context.Background(), &types.Model{InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "lightx2v", CSGHubModelID: "Wan-AI/Wan2.2-T2V-A14B"}}, CreateRequestInput{
		Request: types.VideoGenerationRequest{
			Prompt:  "a calm sea",
			Size:    "1280x720",
			Seconds: 5,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "/v1/tasks/video", providerReq.Path)
	require.Equal(t, "application/json", providerReq.ContentType)
	require.JSONEq(t, `{"prompt":"a calm sea","video_duration":5,"width":1280,"height":720}`, string(providerReq.Body))
}

func TestLightX2VAdapter_BuildCreateRequest_Multipart(t *testing.T) {
	adapter := NewLightX2VAdapter()
	form := buildLightX2VMultipartForm(t)
	providerReq, err := adapter.BuildCreateRequest(context.Background(), &types.Model{InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "lightx2v", CSGHubModelID: "Wan-AI/Wan2.2-I2V-A14B"}}, CreateRequestInput{
		Request: types.VideoGenerationRequest{
			Prompt:  "animate this image",
			Size:    "1280x720",
			Seconds: 5,
		},
		Multipart:   form,
		IsMultipart: true,
	})
	require.NoError(t, err)
	require.Equal(t, "/v1/tasks/video/form", providerReq.Path)
	require.Contains(t, providerReq.ContentType, "multipart/form-data")
	require.Contains(t, string(providerReq.Body), `name="image_file"`)
	require.Contains(t, string(providerReq.Body), `name="prompt"`)
	require.Contains(t, string(providerReq.Body), "animate this image")
	require.Contains(t, string(providerReq.Body), `name="width"`)
	require.Contains(t, string(providerReq.Body), "1280")
}

func TestLightX2VAdapter_BuildCreateRequest_MultipartWithoutImageFallsBackToJSON(t *testing.T) {
	adapter := NewLightX2VAdapter()
	form := buildLightX2VTextOnlyMultipartForm(t)
	providerReq, err := adapter.BuildCreateRequest(context.Background(), &types.Model{InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "lightx2v", CSGHubModelID: "Wan-AI/Wan2.2-T2V-A14B"}}, CreateRequestInput{
		Request: types.VideoGenerationRequest{
			Prompt:  "text only multipart",
			Size:    "1280x720",
			Seconds: 5,
		},
		Multipart:   form,
		IsMultipart: true,
	})
	require.NoError(t, err)
	require.Equal(t, "/v1/tasks/video", providerReq.Path)
	require.Equal(t, "application/json", providerReq.ContentType)
	require.JSONEq(t, `{"prompt":"text only multipart","video_duration":5,"width":1280,"height":720}`, string(providerReq.Body))
}

func TestLightX2VAdapter_BuildCreateRequest_ImageURL(t *testing.T) {
	adapter := NewLightX2VAdapter()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	}))
	defer server.Close()

	providerReq, err := adapter.BuildCreateRequest(context.Background(), &types.Model{InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "lightx2v", CSGHubModelID: "Wan-AI/Wan2.2-I2V-A14B"}}, CreateRequestInput{
		Request: types.VideoGenerationRequest{
			Prompt: "animate this image",
			InputReference: &types.ImageInputReferenceParam{
				ImageURL: server.URL,
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "/v1/tasks/video/form", providerReq.Path)
	require.Contains(t, string(providerReq.Body), `name="image_file"`)
}

func TestLightX2VAdapter_RejectsFileID(t *testing.T) {
	adapter := NewLightX2VAdapter()
	_, err := adapter.BuildCreateRequest(context.Background(), &types.Model{InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "lightx2v", CSGHubModelID: "Wan-AI/Wan2.2-I2V-A14B"}}, CreateRequestInput{
		Request: types.VideoGenerationRequest{
			Prompt: "animate this image",
			InputReference: &types.ImageInputReferenceParam{
				FileID: "file_123",
			},
		},
	})
	require.Error(t, err)
	var requestValidationErr *RequestValidationError
	require.ErrorAs(t, err, &requestValidationErr)
	require.Contains(t, requestValidationErr.Error(), "input_reference.file_id")
}

func TestLightX2VAdapter_ParseResponses(t *testing.T) {
	adapter := NewLightX2VAdapter()

	resp, err := adapter.ParseCreateResponse(context.Background(), []byte(`{"task_id":"abc123","status":"submitted"}`))
	require.NoError(t, err)
	require.Equal(t, "abc123", resp.Video.ID)
	require.Equal(t, "queued", resp.Video.Status)

	resp, err = adapter.ParseRetrieveResponse(context.Background(), []byte(`{"task_id":"abc123","status":"running","progress":0.5}`))
	require.NoError(t, err)
	require.Equal(t, "in_progress", resp.Video.Status)
	require.NotNil(t, resp.Video.Progress)
	require.Equal(t, 0.5, *resp.Video.Progress)

	resp, err = adapter.ParseRetrieveResponse(context.Background(), []byte(`{"task_id":"abc123","status":"failed","message":"boom"}`))
	require.NoError(t, err)
	require.Equal(t, "failed", resp.Video.Status)
	require.NotNil(t, resp.Video.Error)
	require.Equal(t, "boom", resp.Video.Error.Message)
}

func TestLightX2VAdapter_BuildContentRequest(t *testing.T) {
	adapter := NewLightX2VAdapter()
	providerReq, err := adapter.BuildContentRequest(context.Background(), nil, "abc123", nil)
	require.NoError(t, err)
	require.Equal(t, "/v1/files/download/outputs/videos/abc123.mp4", providerReq.Path)
}

func buildLightX2VMultipartForm(t *testing.T) *multipart.Form {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("prompt", "ignored"))
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": []string{`form-data; name="input_reference"; filename="frame.png"`},
		"Content-Type":        []string{"image/png"},
	})
	require.NoError(t, err)
	_, err = part.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(body.Bytes()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, req.ParseMultipartForm(int64(body.Len()+1024)))
	return req.MultipartForm
}

func buildLightX2VTextOnlyMultipartForm(t *testing.T) *multipart.Form {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("prompt", "ignored"))
	require.NoError(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(body.Bytes()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, req.ParseMultipartForm(int64(body.Len()+1024)))
	return req.MultipartForm
}

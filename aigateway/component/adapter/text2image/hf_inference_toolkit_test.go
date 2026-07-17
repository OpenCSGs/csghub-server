package text2image

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commonTypes "opencsg.com/csghub-server/common/types"
)

// mockStorage implements types.Storage for tests
type mockStorage struct {
	putAndPresignGet func(ctx context.Context, bucket, key string, data []byte, contentType string) (string, error)
}

func (m *mockStorage) PutAndPresignGet(ctx context.Context, bucket, key string, data []byte, contentType string) (string, error) {
	if m.putAndPresignGet != nil {
		return m.putAndPresignGet(ctx, bucket, key, data, contentType)
	}
	return "https://example.com/presigned/" + key, nil
}

func TestHFInferenceToolkitAdapter_CanHandle(t *testing.T) {
	adapter := NewHFInferenceToolkitAdapter()

	tests := []struct {
		name  string
		model *types.Model
		want  bool
	}{
		{
			name: "hf inference toolkit self-hosted text to image",
			model: &types.Model{
				BaseModel:         types.BaseModel{Task: string(commonTypes.Text2Image)},
				InternalModelInfo: types.InternalModelInfo{CSGHubModelID: "namespace/model", RuntimeFramework: frameworkHFInferenceToolkit},
			},
			want: true,
		},
		{
			name: "amd hf inference toolkit self-hosted text to image",
			model: &types.Model{
				BaseModel:         types.BaseModel{Task: string(commonTypes.Text2Image)},
				InternalModelInfo: types.InternalModelInfo{CSGHubModelID: "namespace/model", RuntimeFramework: frameworkAMDHfInferenceToolkit},
			},
			want: true,
		},
		{
			name: "runtime framework matching is normalized",
			model: &types.Model{
				BaseModel:         types.BaseModel{Task: string(commonTypes.Text2Image)},
				InternalModelInfo: types.InternalModelInfo{CSGHubModelID: "namespace/model", RuntimeFramework: " AMD-HF-Inference-Toolkit "},
			},
			want: true,
		},
		{
			name: "reject non text to image task",
			model: &types.Model{
				BaseModel:         types.BaseModel{Task: string(commonTypes.TextGeneration)},
				InternalModelInfo: types.InternalModelInfo{CSGHubModelID: "namespace/model", RuntimeFramework: frameworkAMDHfInferenceToolkit},
			},
			want: false,
		},
		{
			name: "reject missing csghub model id",
			model: &types.Model{
				BaseModel:         types.BaseModel{Task: string(commonTypes.Text2Image)},
				InternalModelInfo: types.InternalModelInfo{RuntimeFramework: frameworkAMDHfInferenceToolkit},
			},
			want: false,
		},
		{
			name:  "reject nil model",
			model: nil,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, adapter.CanHandle(tt.model))
		})
	}
}

func TestHFInferenceToolkitAdapter_TransformResponse_responseFormatURL(t *testing.T) {
	adapter := NewHFInferenceToolkitAdapter()
	ctx := context.Background()
	// PNG magic bytes + minimal payload
	pngBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	contentType := "image/png"

	t.Run("response_format=url and storage set returns URL and no B64JSON", func(t *testing.T) {
		presignedURL := "https://storage.example.com/aigateway/generated/images/abc.png?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature=xyz"
		storage := &mockStorage{
			putAndPresignGet: func(_ context.Context, bucket, key string, data []byte, ct string) (string, error) {
				assert.Equal(t, "my-bucket", bucket)
				assert.Contains(t, key, "aigateway/generated/images/")
				assert.Contains(t, key, ".png")
				assert.Equal(t, pngBytes, data)
				assert.Equal(t, "image/png", ct)
				return presignedURL, nil
			},
		}
		opts := &types.TransformResponseOptions{
			ResponseFormat: "url",
			Size:           "1024x1024",
			OutputFormat:   "png",
			Storage:        storage,
			Bucket:         "my-bucket",
		}
		body, resp, err := adapter.TransformResponse(ctx, pngBytes, contentType, "", opts)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Len(t, resp.Data, 1)
		assert.Equal(t, openai.ImagesResponseSize("1024x1024"), resp.Size)
		assert.Equal(t, openai.ImagesResponseOutputFormatPNG, resp.OutputFormat)
		assert.Equal(t, presignedURL, resp.Data[0].URL)
		assert.Empty(t, resp.Data[0].B64JSON)
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(body, &decoded))
		dataArr, _ := decoded["data"].([]any)
		require.Len(t, dataArr, 1)
		data0, _ := dataArr[0].(map[string]any)
		assert.Equal(t, presignedURL, data0["url"])
		b64Val, hasB64 := data0["b64_json"]
		assert.True(t, !hasB64 || b64Val == "" || b64Val == nil)
		require.Contains(t, string(body), "&X-Amz-Signature")
		require.NotContains(t, string(body), `\u0026`)
	})

	t.Run("response_format=b64_json returns B64JSON", func(t *testing.T) {
		opts := &types.TransformResponseOptions{
			ResponseFormat: "b64_json",
			Storage:        &mockStorage{},
			Bucket:         "my-bucket",
		}
		body, resp, err := adapter.TransformResponse(ctx, pngBytes, contentType, "", opts)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Len(t, resp.Data, 1)
		assert.Empty(t, resp.Data[0].URL)
		assert.NotEmpty(t, resp.Data[0].B64JSON)
		assert.Equal(t, base64.StdEncoding.EncodeToString(pngBytes), resp.Data[0].B64JSON)
		_ = body
	})

	t.Run("opts nil returns B64JSON", func(t *testing.T) {
		body, resp, err := adapter.TransformResponse(ctx, pngBytes, contentType, "", nil)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Len(t, resp.Data, 1)
		assert.Empty(t, resp.Data[0].URL)
		assert.Equal(t, base64.StdEncoding.EncodeToString(pngBytes), resp.Data[0].B64JSON)
		_ = body
	})

	t.Run("infers output_format from png bytes", func(t *testing.T) {
		opts := &types.TransformResponseOptions{
			Size: "1024x1024",
		}
		_, resp, err := adapter.TransformResponse(ctx, pngBytes, "", "", opts)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, openai.ImagesResponseSize("1024x1024"), resp.Size)
		assert.Equal(t, openai.ImagesResponseOutputFormatPNG, resp.OutputFormat)
	})

	t.Run("uses explicit output_format", func(t *testing.T) {
		opts := &types.TransformResponseOptions{
			OutputFormat: "webp",
		}
		_, resp, err := adapter.TransformResponse(ctx, pngBytes, contentType, "", opts)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, openai.ImagesResponseOutputFormatWebP, resp.OutputFormat)
	})

	t.Run("response_format=url but storage nil returns B64JSON", func(t *testing.T) {
		opts := &types.TransformResponseOptions{
			ResponseFormat: "url",
			Storage:        nil,
			Bucket:         "my-bucket",
		}
		_, resp, err := adapter.TransformResponse(ctx, pngBytes, contentType, "", opts)
		require.NoError(t, err)
		require.Len(t, resp.Data, 1)
		assert.Empty(t, resp.Data[0].URL)
		assert.NotEmpty(t, resp.Data[0].B64JSON)
	})

	t.Run("response_format=url but bucket empty returns B64JSON", func(t *testing.T) {
		opts := &types.TransformResponseOptions{
			ResponseFormat: "url",
			Storage:        &mockStorage{},
			Bucket:         "",
		}
		_, resp, err := adapter.TransformResponse(ctx, pngBytes, contentType, "", opts)
		require.NoError(t, err)
		require.Len(t, resp.Data, 1)
		assert.Empty(t, resp.Data[0].URL)
		assert.NotEmpty(t, resp.Data[0].B64JSON)
	})
}

func TestHFInferenceToolkitAdapter_GetHeaders(t *testing.T) {
	adapter := NewHFInferenceToolkitAdapter()
	model := &types.Model{}

	t.Run("nil req defaults to image/png", func(t *testing.T) {
		headers := adapter.GetHeaders(model, nil)
		assert.Equal(t, map[string]string{"Accept": "image/png"}, headers)
	})

	t.Run("empty output_format defaults to image/png", func(t *testing.T) {
		req := &types.ImageGenerationRequest{}
		headers := adapter.GetHeaders(model, req)
		assert.Equal(t, map[string]string{"Accept": "image/png"}, headers)
	})

	t.Run("output_format png returns image/png", func(t *testing.T) {
		req := &types.ImageGenerationRequest{}
		req.OutputFormat = "png"
		headers := adapter.GetHeaders(model, req)
		assert.Equal(t, map[string]string{"Accept": "image/png"}, headers)
	})

	t.Run("output_format jpeg returns image/jpeg", func(t *testing.T) {
		req := &types.ImageGenerationRequest{}
		req.OutputFormat = "jpeg"
		headers := adapter.GetHeaders(model, req)
		assert.Equal(t, map[string]string{"Accept": "image/jpeg"}, headers)
	})

	t.Run("output_format webp returns image/webp", func(t *testing.T) {
		req := &types.ImageGenerationRequest{}
		req.OutputFormat = "webp"
		headers := adapter.GetHeaders(model, req)
		assert.Equal(t, map[string]string{"Accept": "image/webp"}, headers)
	})

	t.Run("unknown output_format defaults to image/png", func(t *testing.T) {
		req := &types.ImageGenerationRequest{}
		req.OutputFormat = "bmp"
		headers := adapter.GetHeaders(model, req)
		assert.Equal(t, map[string]string{"Accept": "image/png"}, headers)
	})
}

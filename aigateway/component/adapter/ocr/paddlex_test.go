package ocr

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func boolPtr(b bool) *bool { return &b }

func TestPaddleXAdapter_CanHandle(t *testing.T) {
	a := NewPaddleXAdapter()

	tests := []struct {
		name  string
		model *types.Model
		want  bool
	}{
		{
			name: "by runtime framework",
			model: &types.Model{
				InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "paddleocr"},
			},
			want: true,
		},
		{
			name: "by task",
			model: &types.Model{
				BaseModel: types.BaseModel{Task: string(commontypes.OpticalCharacterRecognition)},
			},
			want: true,
		},
		{
			name: "by comma separated task",
			model: &types.Model{
				BaseModel: types.BaseModel{Task: "text-generation, optical-character-recognition"},
			},
			want: true,
		},
		{
			name: "by opencsg provider",
			model: &types.Model{
				ExternalModelInfo: types.ExternalModelInfo{Provider: " OpenCSG "},
			},
			want: true,
		},
		{
			name: "other framework and task",
			model: &types.Model{
				BaseModel:         types.BaseModel{Task: "text-generation"},
				InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "vllm"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, a.CanHandle(tt.model))
		})
	}
}

func TestPaddleXAdapter_BuildUpstreamRequest(t *testing.T) {
	a := NewPaddleXAdapter()

	body, err := a.BuildUpstreamRequest(&UpstreamInput{
		FileBytes:                 []byte("fake-image-bytes"),
		FileType:                  FileTypeImage,
		UseDocOrientationClassify: boolPtr(true),
		UseTextlineOrientation:    boolPtr(false),
		Visualize:                 true,
	})
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(body, &got))

	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("fake-image-bytes")), got["file"])
	assert.EqualValues(t, FileTypeImage, got["fileType"])
	assert.Equal(t, true, got["useDocOrientationClassify"])
	assert.Equal(t, false, got["useTextlineOrientation"])
	assert.Equal(t, true, got["visualize"])
	// unset optional flags must be omitted
	assert.NotContains(t, got, "useDocUnwarping")
}

func TestPaddleXAdapter_BuildUpstreamRequest_EmptyFile(t *testing.T) {
	a := NewPaddleXAdapter()
	_, err := a.BuildUpstreamRequest(&UpstreamInput{FileType: FileTypeImage})
	require.Error(t, err)
}

const paddleXSuccessBody = `{
  "logId": "log-1",
  "errorCode": 0,
  "errorMsg": "Success",
  "result": {
    "ocrResults": [
      {
        "prunedResult": {
          "rec_texts": ["hello", "world"],
          "rec_scores": [0.99, 0.87],
          "rec_polys": [
            [[1, 2], [50, 2], [50, 20], [1, 20]],
            [[1, 30], [60, 30], [60, 48], [1, 48]]
          ],
          "rec_boxes": [[1, 2, 50, 20], [1, 30, 60, 48]]
        },
        "inputImage": "aW1hZ2U="
      }
    ],
    "dataInfo": {"width": 100, "height": 50}
  }
}`

func TestPaddleXAdapter_TransformResponse_Success(t *testing.T) {
	a := NewPaddleXAdapter()

	resp, err := a.TransformResponse([]byte(paddleXSuccessBody), &ResponseOptions{
		ModelID: "owner/paddleocr-demo:abc123",
	})
	require.NoError(t, err)

	assert.Equal(t, types.OCRResponseObject, resp.Object)
	assert.Equal(t, "owner/paddleocr-demo:abc123", resp.Model)
	assert.NotEmpty(t, resp.ID)
	assert.NotZero(t, resp.Created)
	assert.Equal(t, "hello\nworld", resp.Text)

	require.Len(t, resp.Pages, 1)
	page := resp.Pages[0]
	assert.Equal(t, 0, page.Index)
	assert.Equal(t, "hello\nworld", page.Text)
	require.Len(t, page.Lines, 2)
	assert.Equal(t, "hello", page.Lines[0].Text)
	require.NotNil(t, page.Lines[0].Score)
	assert.InDelta(t, 0.99, *page.Lines[0].Score, 1e-9)
	assert.NotNil(t, page.Lines[1].BBox)

	assert.Equal(t, 1, resp.Usage.Pages)
	assert.Equal(t, 1, resp.Usage.Images)
	assert.Nil(t, resp.RawResult)
}

func TestPaddleXAdapter_TransformResponse_RawResultGating(t *testing.T) {
	a := NewPaddleXAdapter()

	resp, err := a.TransformResponse([]byte(paddleXSuccessBody), &ResponseOptions{RawResponse: true})
	require.NoError(t, err)

	raw, ok := resp.RawResult.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "log-1", raw["logId"])
}

func TestPaddleXAdapter_TransformResponse_UpstreamError(t *testing.T) {
	a := NewPaddleXAdapter()

	body := `{"logId": "log-2", "errorCode": 101, "errorMsg": "invalid image"}`
	_, err := a.TransformResponse([]byte(body), &ResponseOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image")
}

func TestPaddleXAdapter_TransformResponse_Malformed(t *testing.T) {
	a := NewPaddleXAdapter()
	_, err := a.TransformResponse([]byte(`not-json`), &ResponseOptions{})
	require.Error(t, err)
}

func TestPaddleXAdapter_TransformResponse_MissingScoresAndPolys(t *testing.T) {
	a := NewPaddleXAdapter()

	body := `{
      "errorCode": 0,
      "result": {"ocrResults": [{"prunedResult": {"rec_texts": ["only text"]}}]}
    }`
	resp, err := a.TransformResponse([]byte(body), &ResponseOptions{})
	require.NoError(t, err)

	require.Len(t, resp.Pages, 1)
	require.Len(t, resp.Pages[0].Lines, 1)
	assert.Equal(t, "only text", resp.Pages[0].Lines[0].Text)
	assert.Nil(t, resp.Pages[0].Lines[0].Score)
	assert.Nil(t, resp.Pages[0].Lines[0].BBox)
}

func TestPaddleXAdapter_TransformResponse_MultiPage(t *testing.T) {
	a := NewPaddleXAdapter()

	body := `{
      "errorCode": 0,
      "result": {"ocrResults": [
        {"prunedResult": {"rec_texts": ["page one"]}},
        {"prunedResult": {"rec_texts": ["page two"]}}
      ]}
    }`
	resp, err := a.TransformResponse([]byte(body), &ResponseOptions{})
	require.NoError(t, err)

	require.Len(t, resp.Pages, 2)
	assert.Equal(t, 1, resp.Pages[1].Index)
	assert.Equal(t, "page one\npage two", resp.Text)
	assert.Equal(t, 2, resp.Usage.Pages)
}

func TestRegistry_GetAdapter(t *testing.T) {
	r := NewRegistry()

	model := &types.Model{
		InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "paddleocr"},
	}
	assert.Equal(t, paddleXAdapterName, r.GetAdapter(model).Name())

	other := &types.Model{
		BaseModel:         types.BaseModel{Task: "text-generation"},
		InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "vllm"},
	}
	assert.Nil(t, r.GetAdapter(other))
	assert.Nil(t, r.GetAdapter(nil))
}

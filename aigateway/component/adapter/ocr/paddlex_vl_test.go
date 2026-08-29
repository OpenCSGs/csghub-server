package ocr

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/aigateway/types"
)

func TestPaddleXVLAdapter_CanHandleAndFileTypes(t *testing.T) {
	a := NewPaddleXVLAdapter()

	assert.True(t, a.CanHandle(&types.Model{InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "paddleocr-vl"}}))
	assert.False(t, a.CanHandle(&types.Model{InternalModelInfo: types.InternalModelInfo{RuntimeFramework: "paddleocr"}}))
	assert.False(t, a.CanHandle(nil))
	assert.True(t, a.SupportsFileType(FileTypeImage))
	assert.True(t, a.SupportsFileType(FileTypePDF))
	assert.Equal(t, paddleXVLEndpointPath, a.EndpointPath(nil))
}

func TestPaddleXVLAdapter_BuildUpstreamRequest(t *testing.T) {
	a := NewPaddleXVLAdapter()
	body, err := a.BuildUpstreamRequest(&UpstreamInput{
		FileBytes:                 []byte("pdf-bytes"),
		FileType:                  FileTypePDF,
		UseDocOrientationClassify: boolPtr(true),
		Visualize:                 true,
		ReturnMarkdownImages:      true,
	})
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(body, &got))
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("pdf-bytes")), got["file"])
	assert.EqualValues(t, FileTypePDF, got["fileType"])
	assert.Equal(t, true, got["useDocOrientationClassify"])
	assert.Equal(t, true, got["visualize"])
	assert.Equal(t, true, got["returnMarkdownImages"])
	assert.NotContains(t, got, "useDocUnwarping")
}

func TestPaddleXVLAdapter_RejectsClassicTextlineOption(t *testing.T) {
	_, err := NewPaddleXVLAdapter().BuildUpstreamRequest(&UpstreamInput{
		FileBytes:              []byte("image"),
		FileType:               FileTypeImage,
		UseTextlineOrientation: boolPtr(true),
	})
	require.ErrorIs(t, err, ErrUnsupportedOption)
}

const paddleXVLSuccessBody = `{
  "logId": "vl-log-1",
  "errorCode": 0,
  "errorMsg": "Success",
  "result": {
    "layoutParsingResults": [
      {
        "prunedResult": {
          "parsing_res_list": [
            {"block_label": "title", "block_content": "Title"},
            {"block_label": "text", "block_content": "First paragraph"}
          ]
        },
        "markdown": {
          "text": "# Title\n\nFirst paragraph",
          "images": {"imgs/figure_1.jpg": "aW1hZ2U="}
        },
        "outputImages": {"layout_det_res": "aW1hZ2U="}
      },
      {
        "prunedResult": {"parsing_res_list": []},
        "markdown": {"text": "| A | B |\n|---|---|", "images": null}
      }
    ]
  }
}`

func TestPaddleXVLAdapter_TransformResponse(t *testing.T) {
	a := NewPaddleXVLAdapter()
	resp, err := a.TransformResponse([]byte(paddleXVLSuccessBody), &ResponseOptions{ModelID: "owner/vl:1"})
	require.NoError(t, err)

	assert.Equal(t, "owner/vl:1", resp.Model)
	assert.Equal(t, "Title\nFirst paragraph\n", resp.Text)
	require.Len(t, resp.Pages, 2)
	assert.Equal(t, "Title\nFirst paragraph", resp.Pages[0].Text)
	assert.Equal(t, "# Title\n\nFirst paragraph", resp.Pages[0].Markdown)
	assert.Empty(t, resp.Pages[0].Lines)
	assert.Equal(t, "| A | B |\n|---|---|", resp.Pages[1].Markdown)
	assert.Equal(t, 2, resp.Usage.Pages)
	assert.Equal(t, 1, resp.Usage.Images)
	assert.Nil(t, resp.RawResult)
}

func TestPaddleXVLAdapter_TransformResponseRawResult(t *testing.T) {
	resp, err := NewPaddleXVLAdapter().TransformResponse([]byte(paddleXVLSuccessBody), &ResponseOptions{RawResponse: true})
	require.NoError(t, err)

	raw, ok := resp.RawResult.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "vl-log-1", raw["logId"])
}

func TestPaddleXVLAdapter_TransformResponseAllowsMissingMarkdownImages(t *testing.T) {
	body := `{
      "errorCode": 0,
      "result": {"layoutParsingResults": [{
        "prunedResult": {"parsing_res_list": []},
        "markdown": {"text": "plain markdown"}
      }]}
    }`

	resp, err := NewPaddleXVLAdapter().TransformResponse([]byte(body), nil)
	require.NoError(t, err)
	require.Len(t, resp.Pages, 1)
	assert.Equal(t, "plain markdown", resp.Pages[0].Markdown)
}

func TestPaddleXVLAdapter_TransformResponseErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "malformed", body: `not-json`, want: "decode paddlex vl response"},
		{name: "upstream error", body: `{"errorCode":42,"errorMsg":"bad document"}`, want: "bad document"},
		{name: "no pages", body: `{"errorCode":0,"result":{"layoutParsingResults":[]}}`, want: "no layoutParsingResults"},
		{name: "missing markdown text", body: `{"errorCode":0,"result":{"layoutParsingResults":[{"prunedResult":{"parsing_res_list":[]},"markdown":{}}]}}`, want: "markdown.text"},
		{name: "missing parsing results", body: `{"errorCode":0,"result":{"layoutParsingResults":[{"prunedResult":{},"markdown":{"text":"ok"}}]}}`, want: "parsing_res_list"},
		{name: "wrong markdown text type", body: `{"errorCode":0,"result":{"layoutParsingResults":[{"prunedResult":{"parsing_res_list":[]},"markdown":{"text":{}}}]}}`, want: "decode paddlex vl response"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPaddleXVLAdapter().TransformResponse([]byte(tt.body), nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

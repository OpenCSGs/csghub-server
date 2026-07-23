package types

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOCRResponse_JSONShape(t *testing.T) {
	score := 0.98
	resp := OCRResponse{
		ID:      "ocr_abc123",
		Object:  OCRResponseObject,
		Created: 1760000000,
		Model:   "owner/paddleocr-demo:abc123",
		Text:    "recognized full text",
		Pages: []OCRPage{
			{
				Index: 0,
				Text:  "recognized page text",
				Lines: []OCRLine{
					{
						Text:  "line text",
						Score: &score,
						BBox:  [][]int{{10, 20}, {200, 20}, {200, 40}, {10, 40}},
					},
				},
			},
		},
		Usage: OCRUsage{Pages: 1, Images: 1},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))

	assert.Equal(t, "ocr_abc123", got["id"])
	assert.Equal(t, "ocr.result", got["object"])
	assert.Equal(t, "owner/paddleocr-demo:abc123", got["model"])
	assert.Equal(t, "recognized full text", got["text"])

	pages, ok := got["pages"].([]any)
	require.True(t, ok)
	require.Len(t, pages, 1)
	page := pages[0].(map[string]any)
	assert.Equal(t, "recognized page text", page["text"])

	lines, ok := page["lines"].([]any)
	require.True(t, ok)
	require.Len(t, lines, 1)
	line := lines[0].(map[string]any)
	assert.Equal(t, "line text", line["text"])
	assert.InDelta(t, 0.98, line["score"], 1e-9)

	usage := got["usage"].(map[string]any)
	assert.EqualValues(t, 1, usage["pages"])
	assert.EqualValues(t, 1, usage["images"])
}

func TestOCRResponse_OmitsOptionalFields(t *testing.T) {
	resp := OCRResponse{
		ID:     "ocr_x",
		Object: OCRResponseObject,
		Pages: []OCRPage{
			{Index: 0, Text: "t", Lines: []OCRLine{{Text: "l"}}},
		},
		Usage: OCRUsage{Pages: 1, Images: 1},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)
	s := string(data)

	assert.NotContains(t, s, "raw_result")
	assert.NotContains(t, s, "markdown")
	assert.NotContains(t, s, "image_url")
	assert.NotContains(t, s, "score")
	assert.NotContains(t, s, "bbox")
}

func TestOCRResponse_RawResultIncludedWhenSet(t *testing.T) {
	resp := OCRResponse{
		ID:        "ocr_x",
		Object:    OCRResponseObject,
		RawResult: map[string]any{"logId": "123"},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"raw_result"`)
	assert.Contains(t, string(data), `"logId"`)
}

func TestNewOCRResponseID(t *testing.T) {
	id := NewOCRResponseID()
	assert.True(t, strings.HasPrefix(id, "ocr_"))
	assert.NotContains(t, id, "-")
	assert.Len(t, id, len("ocr_")+32)
}

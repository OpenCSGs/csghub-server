package types

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// OCR object type returned in OCRResponse.Object.
const OCRResponseObject = "ocr.result"

// NewOCRResponseID generates the public id for an OCR result, mirroring the
// id style used by other AIGateway response objects.
func NewOCRResponseID() string {
	return fmt.Sprintf("ocr_%s", strings.ReplaceAll(uuid.NewString(), "-", ""))
}

// OCRRequest carries the normalized multipart form values of a /v1/ocr call.
type OCRRequest struct {
	Model                     string
	PageRanges                string
	UseDocOrientationClassify *bool
	UseDocUnwarping           *bool
	UseTextlineOrientation    *bool
	ReturnImage               bool
	RawResponse               bool
}

// OCRResponse is the AIGateway-owned normalized OCR result.
type OCRResponse struct {
	ID        string    `json:"id"`
	Object    string    `json:"object"`
	Created   int64     `json:"created"`
	Model     string    `json:"model"`
	Text      string    `json:"text"`
	Pages     []OCRPage `json:"pages"`
	Usage     OCRUsage  `json:"usage"`
	RawResult any       `json:"raw_result,omitempty"`
}

// OCRPage is the recognized content of a single page or image.
type OCRPage struct {
	Index    int       `json:"index"`
	Text     string    `json:"text"`
	Markdown string    `json:"markdown,omitempty"`
	Lines    []OCRLine `json:"lines"`
	ImageURL string    `json:"image_url,omitempty"`
}

// OCRLine is one recognized text line with its confidence and bounding box.
type OCRLine struct {
	Text  string   `json:"text"`
	Score *float64 `json:"score,omitempty"`
	BBox  any      `json:"bbox,omitempty"`
}

// OCRUsage reports the billable dimensions of an OCR request.
type OCRUsage struct {
	Pages  int `json:"pages"`
	Images int `json:"images"`
}

package ocr

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

const (
	paddleXAdapterName  = "paddlex"
	paddleXEndpointPath = "/ocr"
	// RuntimeFrameworkPaddleOCR is the engine_name of the PaddleOCR runtime
	// framework registered from configs/inference/paddleocr.json.
	RuntimeFrameworkPaddleOCR = "paddleocr"
)

// PaddleXAdapter implements the PaddleX serving (`paddlex --serve --pipeline OCR`)
// protocol: JSON body with a base64 file, camelCase option flags, and an
// ocrResults response envelope.
type PaddleXAdapter struct{}

func NewPaddleXAdapter() *PaddleXAdapter {
	return &PaddleXAdapter{}
}

func (a *PaddleXAdapter) Name() string {
	return paddleXAdapterName
}

func (a *PaddleXAdapter) CanHandle(model *types.Model) bool {
	if model == nil {
		return false
	}
	if isValue(model.RuntimeFramework, RuntimeFrameworkPaddleOCR) {
		return true
	}
	if isValue(model.Provider, "opencsg") {
		return true
	}
	for _, task := range strings.Split(model.Task, ",") {
		if strings.TrimSpace(task) == string(commontypes.OpticalCharacterRecognition) {
			return true
		}
	}
	return false
}

func isValue(value, expected string) bool {
	return strings.EqualFold(strings.TrimSpace(value), expected)
}

func (a *PaddleXAdapter) EndpointPath(_ *types.Model) string {
	return paddleXEndpointPath
}

// paddleXRequest is the upstream request contract. Optional booleans use
// pointers so they are only sent when the client explicitly set them.
type paddleXRequest struct {
	File                      string `json:"file"`
	FileType                  int    `json:"fileType"`
	UseDocOrientationClassify *bool  `json:"useDocOrientationClassify,omitempty"`
	UseDocUnwarping           *bool  `json:"useDocUnwarping,omitempty"`
	UseTextlineOrientation    *bool  `json:"useTextlineOrientation,omitempty"`
	Visualize                 bool   `json:"visualize"`
}

func (a *PaddleXAdapter) BuildUpstreamRequest(in *UpstreamInput) ([]byte, error) {
	if in == nil || len(in.FileBytes) == 0 {
		return nil, fmt.Errorf("ocr upstream input file is empty")
	}
	body := paddleXRequest{
		File:                      base64.StdEncoding.EncodeToString(in.FileBytes),
		FileType:                  in.FileType,
		UseDocOrientationClassify: in.UseDocOrientationClassify,
		UseDocUnwarping:           in.UseDocUnwarping,
		UseTextlineOrientation:    in.UseTextlineOrientation,
		Visualize:                 in.Visualize,
	}
	return json.Marshal(body)
}

// paddleXResponse mirrors the PaddleX serving response envelope. Unknown
// fields are ignored on purpose so newer PaddleX versions keep parsing.
type paddleXResponse struct {
	LogID     string        `json:"logId"`
	ErrorCode int           `json:"errorCode"`
	ErrorMsg  string        `json:"errorMsg"`
	Result    paddleXResult `json:"result"`
}

type paddleXResult struct {
	OCRResults []paddleXOCRResult `json:"ocrResults"`
}

type paddleXOCRResult struct {
	PrunedResult paddleXPrunedResult `json:"prunedResult"`
	OCRImage     string              `json:"ocrImage"`
	InputImage   string              `json:"inputImage"`
}

type paddleXPrunedResult struct {
	RecTexts  []string  `json:"rec_texts"`
	RecScores []float64 `json:"rec_scores"`
	RecPolys  []any     `json:"rec_polys"`
}

func (a *PaddleXAdapter) TransformResponse(respBody []byte, opts *ResponseOptions) (*types.OCRResponse, error) {
	var upstream paddleXResponse
	if err := json.Unmarshal(respBody, &upstream); err != nil {
		return nil, fmt.Errorf("decode paddlex ocr response: %w", err)
	}
	if upstream.ErrorCode != 0 {
		return nil, fmt.Errorf("paddlex ocr error %d: %s", upstream.ErrorCode, upstream.ErrorMsg)
	}

	resp := &types.OCRResponse{
		ID:      types.NewOCRResponseID(),
		Object:  types.OCRResponseObject,
		Created: time.Now().Unix(),
	}
	if opts != nil {
		resp.Model = opts.ModelID
	}

	var pageTexts []string
	for i, result := range upstream.Result.OCRResults {
		page := types.OCRPage{Index: i}
		pruned := result.PrunedResult
		for j, text := range pruned.RecTexts {
			line := types.OCRLine{Text: text}
			if j < len(pruned.RecScores) {
				score := pruned.RecScores[j]
				line.Score = &score
			}
			if j < len(pruned.RecPolys) {
				line.BBox = pruned.RecPolys[j]
			}
			page.Lines = append(page.Lines, line)
		}
		page.Text = joinOCRLineTexts(pruned.RecTexts)
		pageTexts = append(pageTexts, page.Text)
		resp.Pages = append(resp.Pages, page)
	}
	if len(resp.Pages) == 0 {
		// Upstream succeeded but returned no ocrResults: keep one empty page so
		// the response shape stays stable for image inputs.
		resp.Pages = []types.OCRPage{{Index: 0}}
		pageTexts = []string{""}
	}
	resp.Text = strings.Join(pageTexts, "\n")
	resp.Usage = types.OCRUsage{
		Pages:  len(resp.Pages),
		Images: 1,
	}

	if opts != nil && opts.RawResponse {
		var raw any
		if err := json.Unmarshal(respBody, &raw); err == nil {
			resp.RawResult = raw
		}
	}
	return resp, nil
}

func joinOCRLineTexts(texts []string) string {
	filtered := make([]string, 0, len(texts))
	for _, t := range texts {
		if t != "" {
			filtered = append(filtered, t)
		}
	}
	return strings.Join(filtered, "\n")
}

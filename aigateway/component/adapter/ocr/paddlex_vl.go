package ocr

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"opencsg.com/csghub-server/aigateway/types"
)

const (
	paddleXVLAdapterName  = "paddlex-vl"
	paddleXVLEndpointPath = "/layout-parsing"

	// RuntimeFrameworkPaddleOCRVL is the engine_name of the full PaddleOCR-VL runtime.
	RuntimeFrameworkPaddleOCRVL = "paddleocr-vl"
)

// PaddleXVLAdapter implements the full PaddleOCR-VL layout-parsing protocol.
type PaddleXVLAdapter struct{}

func NewPaddleXVLAdapter() *PaddleXVLAdapter {
	return &PaddleXVLAdapter{}
}

func (a *PaddleXVLAdapter) Name() string {
	return paddleXVLAdapterName
}

func (a *PaddleXVLAdapter) CanHandle(model *types.Model) bool {
	return model != nil && isValue(model.RuntimeFramework, RuntimeFrameworkPaddleOCRVL)
}

func (a *PaddleXVLAdapter) SupportsFileType(fileType int) bool {
	return fileType == FileTypeImage || fileType == FileTypePDF
}

func (a *PaddleXVLAdapter) EndpointPath(_ *types.Model) string {
	return paddleXVLEndpointPath
}

type paddleXVLRequest struct {
	File                      string `json:"file"`
	FileType                  int    `json:"fileType"`
	UseDocOrientationClassify bool   `json:"useDocOrientationClassify"`
	UseDocUnwarping           bool   `json:"useDocUnwarping"`
	Visualize                 bool   `json:"visualize"`
	ReturnMarkdownImages      bool   `json:"returnMarkdownImages"`
}

func (a *PaddleXVLAdapter) BuildUpstreamRequest(in *UpstreamInput) ([]byte, error) {
	if in == nil || len(in.FileBytes) == 0 {
		return nil, fmt.Errorf("ocr upstream input file is empty")
	}
	if !a.SupportsFileType(in.FileType) {
		return nil, fmt.Errorf("unsupported PaddleOCR-VL file type %d", in.FileType)
	}
	if in.UseTextlineOrientation != nil {
		return nil, fmt.Errorf("%w: use_textline_orientation is not supported by paddleocr-vl", ErrUnsupportedOption)
	}
	return json.Marshal(paddleXVLRequest{
		File:                      base64.StdEncoding.EncodeToString(in.FileBytes),
		FileType:                  in.FileType,
		UseDocOrientationClassify: in.UseDocOrientationClassify != nil && *in.UseDocOrientationClassify,
		UseDocUnwarping:           in.UseDocUnwarping != nil && *in.UseDocUnwarping,
		Visualize:                 in.Visualize,
		ReturnMarkdownImages:      in.ReturnMarkdownImages,
	})
}

type paddleXVLResponse struct {
	LogID     string          `json:"logId"`
	ErrorCode int             `json:"errorCode"`
	ErrorMsg  string          `json:"errorMsg"`
	Result    paddleXVLResult `json:"result"`
}

type paddleXVLResult struct {
	LayoutParsingResults []paddleXVLLayoutResult `json:"layoutParsingResults"`
}

type paddleXVLLayoutResult struct {
	PrunedResult paddleXVLPrunedResult `json:"prunedResult"`
	Markdown     paddleXVLMarkdown     `json:"markdown"`
}

type paddleXVLPrunedResult struct {
	ParsingResults []paddleXVLParsingResult `json:"parsing_res_list"`
}

type paddleXVLParsingResult struct {
	BlockContent string `json:"block_content"`
}

type paddleXVLMarkdown struct {
	Text *string `json:"text"`
	// Images is the upstream filename -> base64 mapping for embedded figures.
	// The upstream contract is a JSON object or null; raw JSON is kept so an
	// unexpected shape degrades to absent images instead of failing the page.
	Images json.RawMessage `json:"images"`
}

func (a *PaddleXVLAdapter) TransformResponse(respBody []byte, opts *ResponseOptions) (*types.OCRResponse, error) {
	var upstream paddleXVLResponse
	if err := json.Unmarshal(respBody, &upstream); err != nil {
		return nil, fmt.Errorf("decode paddlex vl response: %w", err)
	}
	if upstream.ErrorCode != 0 {
		return nil, fmt.Errorf("paddlex vl error %d: %s", upstream.ErrorCode, upstream.ErrorMsg)
	}
	if len(upstream.Result.LayoutParsingResults) == 0 {
		return nil, fmt.Errorf("paddlex vl response has no layoutParsingResults")
	}

	resp := &types.OCRResponse{
		ID:      types.NewOCRResponseID(),
		Object:  types.OCRResponseObject,
		Created: time.Now().Unix(),
		Pages:   make([]types.OCRPage, 0, len(upstream.Result.LayoutParsingResults)),
	}
	if opts != nil {
		resp.Model = opts.ModelID
	}

	pageTexts := make([]string, 0, len(upstream.Result.LayoutParsingResults))
	for i, result := range upstream.Result.LayoutParsingResults {
		if result.Markdown.Text == nil {
			return nil, fmt.Errorf("paddlex vl response page %d has no markdown.text", i)
		}
		if result.PrunedResult.ParsingResults == nil {
			return nil, fmt.Errorf("paddlex vl response page %d has no prunedResult.parsing_res_list", i)
		}

		blocks := make([]string, 0, len(result.PrunedResult.ParsingResults))
		for _, block := range result.PrunedResult.ParsingResults {
			if block.BlockContent != "" {
				blocks = append(blocks, block.BlockContent)
			}
		}
		pageText := strings.Join(blocks, "\n")
		pageTexts = append(pageTexts, pageText)
		page := types.OCRPage{
			Index:    i,
			Text:     pageText,
			Markdown: *result.Markdown.Text,
			Lines:    []types.OCRLine{},
		}
		if opts != nil && opts.ReturnImage {
			page.Images = ocrImagesFromMarkdown(markdownImages(result.Markdown.Images))
		}
		resp.Pages = append(resp.Pages, page)
	}

	resp.Text = strings.Join(pageTexts, "\n")
	resp.Usage = types.OCRUsage{Pages: len(resp.Pages), Images: 1}
	if opts != nil && opts.RawResponse {
		var raw any
		if err := json.Unmarshal(respBody, &raw); err == nil {
			resp.RawResult = raw
		}
	}
	return resp, nil
}

// markdownImages decodes the upstream markdown.images mapping. Non-object
// values (arrays, strings, numbers) are treated as absent so a changed
// upstream contract never fails the whole OCR response.
func markdownImages(raw json.RawMessage) map[string]string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var images map[string]string
	if err := json.Unmarshal(raw, &images); err != nil {
		return nil
	}
	return images
}

// ocrImagesFromMarkdown converts the upstream markdown.images map
// (filename -> base64) into the normalized OCRImage list with a stable order.
func ocrImagesFromMarkdown(images map[string]string) []types.OCRImage {
	if len(images) == 0 {
		return nil
	}
	names := make([]string, 0, len(images))
	for name := range images {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]types.OCRImage, 0, len(names))
	for _, name := range names {
		out = append(out, types.OCRImage{Name: name, Content: images[name]})
	}
	return out
}

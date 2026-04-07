package text2image

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/openai/openai-go/v3"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/compress"
	commonTypes "opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

const frameworkHFInferenceToolkit = "hf-inference-toolkit"

var hfProviders = []string{"opencsg"}

type HFInferenceToolkitAdapter struct{}

// stable diffusion framework
func NewHFInferenceToolkitAdapter() *HFInferenceToolkitAdapter {
	return &HFInferenceToolkitAdapter{}
}

func (a *HFInferenceToolkitAdapter) Name() string {
	return "hf-inference-toolkit"
}

func (a *HFInferenceToolkitAdapter) CanHandle(model *types.Model) bool {
	if model == nil {
		return false
	}
	if model.Provider != "" {
		return slices.Contains(hfProviders, strings.ToLower(model.Provider))
	}
	if model.CSGHubModelID == "" {
		return false
	}
	return model.Task == string(commonTypes.Text2Image) && model.RuntimeFramework == frameworkHFInferenceToolkit
}

func (a *HFInferenceToolkitAdapter) TransformRequest(ctx context.Context, openaiReq types.ImageGenerationRequest) ([]byte, error) {
	params := map[string]any{}
	if len(openaiReq.RawJSON) > 0 {
		if err := json.Unmarshal(openaiReq.RawJSON, &params); err != nil {
			return nil, err
		}
	}
	if width, height := parseSizeToWidthHeight(string(openaiReq.Size)); width > 0 && height > 0 {
		params["width"] = width
		params["height"] = height
	}
	hfReq := map[string]any{
		"inputs":     openaiReq.Prompt,
		"parameters": params,
	}
	return json.Marshal(hfReq)
}

func parseSizeToWidthHeight(size string) (width, height int) {
	if size == "" || size == "auto" {
		return 0, 0
	}
	parts := strings.SplitN(size, "x", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return 0, 0
	}
	return w, h
}

func (a *HFInferenceToolkitAdapter) NeedsResponseTransform() bool {
	return true
}

func (a *HFInferenceToolkitAdapter) TransformResponse(ctx context.Context, respBody []byte, contentType string, encodingHeader string, opts *types.TransformResponseOptions) ([]byte, *types.ImageGenerationResponse, error) {
	decoded, err := compress.Decode(encodingHeader, respBody)
	if err != nil {
		decoded = respBody
	}
	isPNG := strings.HasPrefix(contentType, "image/") ||
		(len(decoded) >= 4 && decoded[0] == 0x89 && string(decoded[1:4]) == "PNG")
	if isPNG {
		openaiResp := &types.ImageGenerationResponse{
			ImagesResponse: openai.ImagesResponse{
				Created: time.Now().Unix(),
				Data:    []openai.Image{},
			},
		}
		if opts != nil && opts.ResponseFormat == "url" && opts.Storage != nil && opts.Bucket != "" {
			mediaType, _, _ := strings.Cut(contentType, ";")
			mediaType = strings.TrimSpace(mediaType)
			ext := "png"
			if strings.HasPrefix(mediaType, "image/") {
				ext = strings.TrimSpace(mediaType[len("image/"):])
			}
			if mediaType == "" {
				mediaType = "image/" + ext
			}
			key := fmt.Sprintf("aigateway/generated/images/%s.%s", uuid.New().String(), ext)
			presignedURL, err := opts.Storage.PutAndPresignGet(ctx, opts.Bucket, key, decoded, mediaType)
			if err != nil {
				return nil, nil, fmt.Errorf("upload image to storage: %w", err)
			}
			openaiResp.Data = []openai.Image{{URL: presignedURL}}
		} else {
			b64Data := base64.StdEncoding.EncodeToString(decoded)
			openaiResp.Data = []openai.Image{{B64JSON: b64Data}}
		}
		body, err := common.MarshalJSONWithoutHTMLEscape(openaiResp)
		if err != nil {
			return nil, nil, err
		}
		return body, openaiResp, nil
	}
	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(decoded, &errResp); err == nil && errResp.Error != "" {
		return nil, nil, fmt.Errorf("hf-inference-toolkit error: %s", errResp.Error)
	}
	return nil, nil, fmt.Errorf("unexpected response format, content-type: %s", contentType)
}

func (a *HFInferenceToolkitAdapter) GetHeaders(model *types.Model, req *types.ImageGenerationRequest) map[string]string {
	accept := "image/png"
	if req != nil {
		switch string(req.OutputFormat) {
		case "jpeg":
			accept = "image/jpeg"
		case "webp":
			accept = "image/webp"
		case "png", "":
			accept = "image/png"
		default:
			accept = "image/png"
		}
	}
	return map[string]string{
		"Accept": accept,
	}
}

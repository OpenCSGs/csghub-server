package text2video

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
	commonTypes "opencsg.com/csghub-server/common/types"
)

const (
	sglangDiffusionAdapterName = "sglang-diffusion"
	// Video models share the sglang runtime; serve.sh starts SGLang Diffusion
	// for text-to-video and image-to-video deploys.
	frameworkSGLang            = "sglang"
	sglangDiffusionVideosPath  = "/v1/videos"
	sglangDiffusionDefaultTask = "t2va"
	// MiniMax-H3 shapes the output from target only; these are its
	// recommended short edge and default clip length.
	sglangDiffusionDefaultShortEdge = 768
	sglangDiffusionDefaultSeconds   = 5
)

// sglangDiffusionAspectRatios are the finite target.aspect_ratio values
// MiniMax-H3 accepts.
var sglangDiffusionAspectRatios = []struct {
	name  string
	ratio float64
}{
	{"21:9", 21.0 / 9}, {"16:9", 16.0 / 9}, {"4:3", 4.0 / 3},
	{"1:1", 1}, {"3:4", 3.0 / 4}, {"9:16", 9.0 / 16},
}

// SGLangDiffusionAdapter normalizes the SGLang Diffusion async video API served
// by CSGHub deployments, such as MiniMax-H3.
// Docs:
//   - https://docs.sglang.io/cookbook/diffusion/MiniMax/MiniMax-H3
type SGLangDiffusionAdapter struct{}

func NewSGLangDiffusionAdapter() *SGLangDiffusionAdapter {
	return &SGLangDiffusionAdapter{}
}

func (a *SGLangDiffusionAdapter) Name() string {
	return sglangDiffusionAdapterName
}

func (a *SGLangDiffusionAdapter) CanHandle(model *types.Model) bool {
	if model == nil {
		return false
	}
	if strings.TrimSpace(model.CSGHubModelID) == "" {
		return false
	}
	// serve.sh starts SGLang Diffusion for both video tasks, since a repo tagged
	// with both may deploy as either one.
	if model.Task != string(commonTypes.Text2Video) && model.Task != string(commonTypes.Image2Video) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(model.RuntimeFramework), frameworkSGLang)
}

func (a *SGLangDiffusionAdapter) Capabilities(model *types.Model) Capabilities {
	if !a.CanHandle(model) {
		return Capabilities{}
	}
	return Capabilities{
		SupportsCreate:                 true,
		SupportsDirectContentStreaming: true,
	}
}

func (a *SGLangDiffusionAdapter) BuildCreateRequest(ctx context.Context, model *types.Model, input CreateRequestInput) (*ProviderRequest, error) {
	if input.IsMultipart || input.Request.InputReference != nil {
		return nil, &RequestValidationError{Message: "selected model does not support input_reference"}
	}
	body, err := a.buildCreateBody(input.Request)
	if err != nil {
		return nil, err
	}
	return &ProviderRequest{
		Method:      http.MethodPost,
		Path:        sglangDiffusionVideosPath,
		Body:        body,
		ContentType: "application/json",
	}, nil
}

// buildCreateBody forwards caller extension fields (task, seed,
// num_inference_steps, ...) and maps the OpenAI size and seconds to width,
// height and the target MiniMax-H3 requires. A caller target is rejected
// because MiniMax-H3 generates from target while pricing and metering read
// size and seconds. The gateway model ID is dropped because the engine serves
// a single local model.
func (a *SGLangDiffusionAdapter) buildCreateBody(req types.VideoGenerationRequest) ([]byte, error) {
	payload := map[string]any{}
	if len(req.RawJSON) > 0 {
		if err := json.Unmarshal(req.RawJSON, &payload); err != nil {
			return nil, err
		}
	}
	if _, ok := payload["target"]; ok {
		return nil, &RequestValidationError{Message: "SGLang Diffusion video backend derives target from size and seconds; set those instead"}
	}
	payload["prompt"] = req.Prompt
	if _, ok := payload["task"]; !ok {
		payload["task"] = sglangDiffusionDefaultTask
	}
	target := map[string]any{
		"short_edge":       sglangDiffusionDefaultShortEdge,
		"aspect_ratio":     "16:9",
		"duration_seconds": sglangDiffusionDefaultSeconds,
	}
	if req.Seconds > 0 {
		payload["seconds"] = req.Seconds
		target["duration_seconds"] = req.Seconds
	}
	if req.Size != "" {
		width, height, ok := parseOpenAIVideoSize(req.Size)
		if !ok {
			return nil, &RequestValidationError{Message: fmt.Sprintf("SGLang Diffusion video backend requires size in {width}x{height} format, got %q", req.Size)}
		}
		payload["width"] = width
		payload["height"] = height
		target["short_edge"] = min(width, height)
		target["aspect_ratio"] = nearestSGLangDiffusionAspectRatio(width, height)
	}
	payload["target"] = target
	return json.Marshal(payload)
}

func nearestSGLangDiffusionAspectRatio(width, height int) string {
	ratio := float64(width) / float64(height)
	best := sglangDiffusionAspectRatios[0]
	for _, candidate := range sglangDiffusionAspectRatios[1:] {
		if math.Abs(candidate.ratio-ratio) < math.Abs(best.ratio-ratio) {
			best = candidate
		}
	}
	return best.name
}

func (a *SGLangDiffusionAdapter) ParseCreateResponse(ctx context.Context, body []byte) (*ProviderResponse, error) {
	payload, err := decodeJSON(body)
	if err != nil {
		return nil, err
	}
	id := stringAt(payload, "id")
	if id == "" {
		return nil, fmt.Errorf("sglang diffusion response missing id")
	}
	rawStatus := stringAt(payload, "status")
	video := &types.VideoObject{
		ID:     id,
		Object: "video",
		Status: rawStatus,
	}
	if progress, ok := float64At(payload, "progress"); ok {
		video.Progress = &progress
	}
	if rawStatus == string(commonTypes.AIGatewayAsyncGenerationStatusFailed) {
		// SGLang reports error as a free-form object; keep only its message.
		video.Error = &types.VideoError{
			Code:    "generation_failed",
			Message: stringAt(payload, "error.message"),
		}
	}
	return &ProviderResponse{Video: video, ProviderMetadata: WithProviderStatus(nil, rawStatus)}, nil
}

func (a *SGLangDiffusionAdapter) BuildRetrieveRequest(ctx context.Context, model *types.Model, providerResourceID string, providerMetadata map[string]any) (*ProviderRequest, error) {
	return &ProviderRequest{
		Method: http.MethodGet,
		Path:   sglangDiffusionVideosPath + "/" + providerResourceID,
	}, nil
}

func (a *SGLangDiffusionAdapter) ParseRetrieveResponse(ctx context.Context, body []byte) (*ProviderResponse, error) {
	return a.ParseCreateResponse(ctx, body)
}

func (a *SGLangDiffusionAdapter) BuildContentRequest(ctx context.Context, model *types.Model, providerResourceID string, providerMetadata map[string]any) (*ProviderRequest, error) {
	return &ProviderRequest{
		Method: http.MethodGet,
		Path:   sglangDiffusionVideosPath + "/" + providerResourceID + "/content",
	}, nil
}

func (a *SGLangDiffusionAdapter) ParseContentResponse(ctx context.Context, body []byte) (*ContentResponse, error) {
	return &ContentResponse{}, nil
}

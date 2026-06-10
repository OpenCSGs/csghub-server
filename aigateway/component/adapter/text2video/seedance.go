package text2video

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
	commonutils "opencsg.com/csghub-server/common/utils/common"
)

// SeedanceAdapter normalizes BytePlus ModelArk Seedance video generation APIs.
// Docs:
//   - https://docs.byteplus.com/en/docs/ModelArk/1520757
type SeedanceAdapter struct{}

const seedanceCreatePathSuffix = "/api/v3/contents/generations/tasks"

func NewSeedanceAdapter() *SeedanceAdapter {
	return &SeedanceAdapter{}
}

func (a *SeedanceAdapter) Name() string {
	return videoAPITypeSeedance
}

func (a *SeedanceAdapter) CanHandle(model *types.Model) bool {
	return isProviderType(model, videoAPITypeSeedance)
}

func (a *SeedanceAdapter) Capabilities(model *types.Model) Capabilities {
	if !a.CanHandle(model) {
		return Capabilities{}
	}
	return Capabilities{
		SupportsCreate:         true,
		SupportsImageReference: true,
		SupportsJSONImageURL:   true,
	}
}

func (a *SeedanceAdapter) TransformRequest(ctx context.Context, req types.VideoGenerationRequest) ([]byte, error) {
	content := []map[string]any{{"type": "text", "text": req.Prompt}}
	if req.InputReference != nil && req.InputReference.ImageURL != "" {
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": req.InputReference.ImageURL},
		})
	}
	payload := map[string]any{
		"model":   req.Model,
		"content": content,
	}
	if req.Seconds > 0 {
		payload["duration"] = req.Seconds
	}
	if req.Size != "" {
		resolution, ratio, err := normalizeSeedanceVideoSize(req.Size)
		if err != nil {
			return nil, err
		}
		if resolution != "" {
			payload["resolution"] = resolution
		}
		if ratio != "" {
			payload["ratio"] = ratio
		}
	}
	return json.Marshal(payload)
}

func (a *SeedanceAdapter) BuildCreateRequest(ctx context.Context, model *types.Model, input CreateRequestInput) (*ProviderRequest, error) {
	body, err := a.TransformRequest(ctx, input.Request)
	if err != nil {
		return nil, err
	}
	path, err := buildSeedanceCreatePath(model)
	if err != nil {
		return nil, err
	}
	return &ProviderRequest{
		Method:      http.MethodPost,
		Path:        path,
		Body:        body,
		ContentType: "application/json",
	}, nil
}

func (a *SeedanceAdapter) ParseCreateResponse(ctx context.Context, body []byte) (*ProviderResponse, error) {
	video, metadata, err := a.parseResponse(body)
	if err != nil {
		return nil, err
	}
	if video.Status == "" {
		video.Status = string(commontypes.AIGatewayAsyncGenerationStatusQueued)
	}
	return &ProviderResponse{Video: video, ProviderMetadata: metadata}, nil
}

func (a *SeedanceAdapter) BuildRetrieveRequest(ctx context.Context, model *types.Model, providerResourceID string, providerMetadata map[string]any) (*ProviderRequest, error) {
	path, err := buildSeedanceRetrievePath(model, providerResourceID)
	if err != nil {
		return nil, err
	}
	return &ProviderRequest{
		Method: http.MethodGet,
		Path:   path,
	}, nil
}

func (a *SeedanceAdapter) ParseRetrieveResponse(ctx context.Context, body []byte) (*ProviderResponse, error) {
	video, metadata, err := a.parseResponse(body)
	if err != nil {
		return nil, err
	}
	return &ProviderResponse{Video: video, ProviderMetadata: metadata}, nil
}

func (a *SeedanceAdapter) BuildContentRequest(ctx context.Context, model *types.Model, providerResourceID string, providerMetadata map[string]any) (*ProviderRequest, error) {
	return a.BuildRetrieveRequest(ctx, model, providerResourceID, providerMetadata)
}

func (a *SeedanceAdapter) ParseContentResponse(ctx context.Context, body []byte) (*ContentResponse, error) {
	_, metadata, err := a.parseResponse(body)
	if err != nil {
		return nil, err
	}
	downloadURL, _ := metadata["download_url"].(string)
	if downloadURL == "" {
		// Seedance exposes the final video URL on the task response; unlike
		// MiniMax there is no separate file-resolution step for this adapter.
		return nil, fmt.Errorf("seedance response missing video download url")
	}
	return &ContentResponse{DownloadURL: downloadURL, ProviderMetadata: metadata}, nil
}

func (a *SeedanceAdapter) ParseVideoResponse(ctx context.Context, body []byte) (*types.VideoObject, error) {
	video, _, err := a.parseResponse(body)
	return video, err
}

func (a *SeedanceAdapter) parseResponse(body []byte) (*types.VideoObject, map[string]any, error) {
	payload, err := decodeJSON(body)
	if err != nil {
		return nil, nil, err
	}
	id := stringAt(payload, "id")
	if id == "" {
		id = stringAt(payload, "task_id")
	}
	if id == "" {
		return nil, nil, fmt.Errorf("seedance response missing id")
	}
	rawStatus := stringAt(payload, "status")
	status := mapSeedanceStatus(rawStatus)
	downloadURL := stringAt(payload, "content.video_url")
	if downloadURL == "" {
		downloadURL = stringAt(payload, "video_url")
	}
	video := &types.VideoObject{ID: id, Object: "video", Status: status}
	if status == string(commontypes.AIGatewayAsyncGenerationStatusFailed) ||
		status == string(commontypes.AIGatewayAsyncGenerationStatusCancelled) {
		if message := seedanceFailureMessage(payload); message != "" {
			video.Error = &types.VideoError{
				Code:    "generation_failed",
				Message: message,
			}
		}
	}
	metadata := WithProviderStatus(map[string]any{"download_url": downloadURL}, rawStatus)
	return video, metadata, nil
}

func mapSeedanceStatus(status string) string {
	switch status {
	case "queued", "pending":
		return string(commontypes.AIGatewayAsyncGenerationStatusQueued)
	case "running", "processing":
		return string(commontypes.AIGatewayAsyncGenerationStatusInProgress)
	case "succeeded", "success", "completed":
		return string(commontypes.AIGatewayAsyncGenerationStatusCompleted)
	case "failed", "expired", "cancelled", "canceled":
		return string(commontypes.AIGatewayAsyncGenerationStatusFailed)
	default:
		return status
	}
}

func normalizeSeedanceVideoSize(size string) (string, string, error) {
	normalized := strings.ToLower(strings.TrimSpace(size))
	switch normalized {
	case "":
		return "", "", nil
	case "480p", "720p", "1080p":
		return normalized, "16:9", nil
	}

	width, height, ok := parseVideoSize(normalized)
	if !ok {
		return "", "", &RequestValidationError{
			Message: fmt.Sprintf("Seedance video backend does not support size %q; use an OpenAI-compatible size that maps to Seedance resolution and ratio or native Seedance resolution (480p, 720p, 1080p)", size),
		}
	}

	resolution := seedanceResolutionFromDimensions(width, height)
	ratio := seedanceAspectRatio(width, height)
	if resolution == "" || ratio == "" {
		return "", "", &RequestValidationError{
			Message: fmt.Sprintf("Seedance video backend does not support size %q; use an OpenAI-compatible size that maps to Seedance resolution and ratio or native Seedance resolution (480p, 720p, 1080p)", size),
		}
	}

	return resolution, ratio, nil
}

func parseVideoSize(size string) (int, int, bool) {
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || width <= 0 {
		return 0, 0, false
	}
	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func seedanceResolutionFromDimensions(width, height int) string {
	switch min(width, height) {
	case 480:
		return "480p"
	case 720:
		return "720p"
	case 1080:
		return "1080p"
	default:
		return ""
	}
}

func seedanceAspectRatio(width, height int) string {
	divisor := gcd(width, height)
	if divisor == 0 {
		return ""
	}
	ratio := fmt.Sprintf("%d:%d", width/divisor, height/divisor)
	switch ratio {
	case "21:9", "16:9", "4:3", "1:1", "3:4", "9:16":
		return ratio
	default:
		return ""
	}
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func buildSeedanceCreatePath(model *types.Model) (string, error) {
	if model == nil {
		return "", fmt.Errorf("seedance model is required")
	}
	path := commonutils.ExtractURLPath(model.Endpoint)
	if path == "" {
		return "", fmt.Errorf("seedance endpoint path is empty")
	}
	if !strings.HasSuffix(path, seedanceCreatePathSuffix) {
		return "", &RequestValidationError{
			Message: fmt.Sprintf("Seedance endpoint path must end with %s", seedanceCreatePathSuffix),
		}
	}
	return path, nil
}

func buildSeedanceRetrievePath(model *types.Model, providerResourceID string) (string, error) {
	createPath, err := buildSeedanceCreatePath(model)
	if err != nil {
		return "", err
	}
	return commonutils.JoinURLPath(createPath, providerResourceID), nil
}

func seedanceFailureMessage(payload map[string]any) string {
	for _, path := range []string{
		"error.message",
		"content.error.message",
		"message",
		"reason",
		"error",
	} {
		if message := strings.TrimSpace(stringAt(payload, path)); message != "" {
			return message
		}
	}
	return ""
}

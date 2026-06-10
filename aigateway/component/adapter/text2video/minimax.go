package text2video

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
	commonutils "opencsg.com/csghub-server/common/utils/common"
)

// MiniMaxAdapter normalizes MiniMax video generation APIs.
// Docs:
//   - https://platform.minimaxi.com/docs/api-reference/video-generation-t2v
//   - https://platform.minimaxi.com/docs/api-reference/video-generation-i2v
//   - https://platform.minimaxi.com/docs/api-reference/video-generation-query
//   - https://platform.minimaxi.com/docs/api-reference/video-generation-download
type MiniMaxAdapter struct{}

type miniMaxBaseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

type miniMaxCreateResponse struct {
	BaseResp miniMaxBaseResp `json:"base_resp"`
	TaskID   string          `json:"task_id"`
}

type miniMaxStatusResponse struct {
	BaseResp miniMaxBaseResp `json:"base_resp"`
	TaskID   string          `json:"task_id"`
	Status   string          `json:"status"`
	FileID   json.RawMessage `json:"file_id"`
}

const miniMaxCreatePathSuffix = "/v1/video_generation"

func NewMiniMaxAdapter() *MiniMaxAdapter {
	return &MiniMaxAdapter{}
}

func (a *MiniMaxAdapter) Name() string {
	return videoAPITypeMiniMax
}

func (a *MiniMaxAdapter) CanHandle(model *types.Model) bool {
	return isProviderType(model, videoAPITypeMiniMax)
}

func (a *MiniMaxAdapter) Capabilities(model *types.Model) Capabilities {
	if !a.CanHandle(model) {
		return Capabilities{}
	}
	return Capabilities{
		SupportsCreate:         true,
		SupportsImageReference: true,
		SupportsJSONImageURL:   true,
	}
}

func (a *MiniMaxAdapter) TransformRequest(ctx context.Context, req types.VideoGenerationRequest) ([]byte, error) {
	payload := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
	}
	if req.Seconds > 0 {
		payload["duration"] = req.Seconds
	}
	if req.Size != "" {
		resolution, err := normalizeMiniMaxResolution(req.Size)
		if err != nil {
			return nil, err
		}
		payload["resolution"] = resolution
	}
	if req.InputReference != nil && req.InputReference.ImageURL != "" {
		payload["first_frame_image"] = req.InputReference.ImageURL
	}
	return json.Marshal(payload)
}

func (a *MiniMaxAdapter) BuildCreateRequest(ctx context.Context, model *types.Model, input CreateRequestInput) (*ProviderRequest, error) {
	body, err := a.TransformRequest(ctx, input.Request)
	if err != nil {
		return nil, err
	}
	path, err := buildMiniMaxCreatePath(model)
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

func (a *MiniMaxAdapter) ParseCreateResponse(ctx context.Context, body []byte) (*ProviderResponse, error) {
	var payload miniMaxCreateResponse
	err := json.Unmarshal(body, &payload)
	if err != nil {
		return nil, err
	}
	if err := parseMiniMaxError(payload.BaseResp); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(payload.TaskID)
	if id == "" {
		return nil, fmt.Errorf("minimax response missing task_id")
	}
	return &ProviderResponse{Video: &types.VideoObject{ID: id, Object: "video", Status: string(commontypes.AIGatewayAsyncGenerationStatusQueued)}}, nil
}

func (a *MiniMaxAdapter) BuildRetrieveRequest(ctx context.Context, model *types.Model, providerResourceID string, providerMetadata map[string]any) (*ProviderRequest, error) {
	path, err := buildMiniMaxRetrievePath(model)
	if err != nil {
		return nil, err
	}
	return &ProviderRequest{
		Method: http.MethodGet,
		Path:   path,
		Query:  url.Values{"task_id": []string{providerResourceID}},
	}, nil
}

func (a *MiniMaxAdapter) ParseRetrieveResponse(ctx context.Context, body []byte) (*ProviderResponse, error) {
	video, metadata, err := a.parseStatusResponse(body)
	if err != nil {
		return nil, err
	}
	return &ProviderResponse{Video: video, ProviderMetadata: metadata}, nil
}

func (a *MiniMaxAdapter) BuildContentRequest(ctx context.Context, model *types.Model, providerResourceID string, providerMetadata map[string]any) (*ProviderRequest, error) {
	fileID, _ := providerMetadata["file_id"].(string)
	if fileID == "" {
		return a.BuildRetrieveRequest(ctx, model, providerResourceID, providerMetadata)
	}
	path, err := buildMiniMaxFileRetrievePath(model)
	if err != nil {
		return nil, err
	}
	return &ProviderRequest{
		Method: http.MethodGet,
		Path:   path,
		Query:  url.Values{"file_id": []string{fileID}},
	}, nil
}

func (a *MiniMaxAdapter) ParseContentResponse(ctx context.Context, body []byte) (*ContentResponse, error) {
	payload, err := decodeJSON(body)
	if err != nil {
		return nil, err
	}
	downloadURL := stringAt(payload, "file.download_url")
	if downloadURL != "" {
		return &ContentResponse{DownloadURL: downloadURL}, nil
	}
	_, metadata, err := a.parseStatusResponse(body)
	if err != nil {
		return nil, err
	}
	fileID, _ := metadata["file_id"].(string)
	if fileID == "" {
		return nil, fmt.Errorf("minimax content response missing download_url or file_id")
	}
	return &ContentResponse{ProviderMetadata: metadata}, nil
}

func (a *MiniMaxAdapter) ParseVideoResponse(ctx context.Context, body []byte) (*types.VideoObject, error) {
	resp, err := a.ParseCreateResponse(ctx, body)
	if err != nil {
		video, _, statusErr := a.parseStatusResponse(body)
		if statusErr != nil {
			return nil, err
		}
		return video, nil
	}
	return resp.Video, nil
}

func (a *MiniMaxAdapter) parseStatusResponse(body []byte) (*types.VideoObject, map[string]any, error) {
	var payload miniMaxStatusResponse
	err := json.Unmarshal(body, &payload)
	if err != nil {
		return nil, nil, err
	}
	if err := parseMiniMaxError(payload.BaseResp); err != nil {
		return nil, nil, err
	}
	id := strings.TrimSpace(payload.TaskID)
	rawStatus := strings.TrimSpace(payload.Status)
	status := mapMiniMaxStatus(rawStatus)
	fileID := parseMiniMaxFileID(payload.FileID)
	if id == "" {
		return nil, nil, fmt.Errorf("minimax response missing task_id")
	}
	metadata := WithProviderStatus(map[string]any{"file_id": fileID}, rawStatus)
	return &types.VideoObject{ID: id, Object: "video", Status: status}, metadata, nil
}

func parseMiniMaxError(baseResp miniMaxBaseResp) error {
	if baseResp.StatusCode == 0 {
		return nil
	}
	statusMsg := strings.TrimSpace(baseResp.StatusMsg)
	if statusMsg == "" {
		statusMsg = fmt.Sprintf("MiniMax provider error (status_code=%d)", baseResp.StatusCode)
	}
	return &RequestValidationError{Message: statusMsg}
}

func parseMiniMaxFileID(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var fileID string
	if err := json.Unmarshal(raw, &fileID); err == nil {
		return strings.TrimSpace(fileID)
	}

	var nested struct {
		FileID string `json:"file_id"`
	}
	if err := json.Unmarshal(raw, &nested); err == nil {
		return strings.TrimSpace(nested.FileID)
	}

	return ""
}

func mapMiniMaxStatus(status string) string {
	switch status {
	case "Queueing", "Preparing":
		return string(commontypes.AIGatewayAsyncGenerationStatusQueued)
	case "Processing":
		return string(commontypes.AIGatewayAsyncGenerationStatusInProgress)
	case "Success":
		return string(commontypes.AIGatewayAsyncGenerationStatusCompleted)
	case "Fail":
		return string(commontypes.AIGatewayAsyncGenerationStatusFailed)
	default:
		return status
	}
}

func buildMiniMaxCreatePath(model *types.Model) (string, error) {
	return miniMaxPathFromEndpoint(model, miniMaxCreatePathSuffix)
}

func buildMiniMaxRetrievePath(model *types.Model) (string, error) {
	return miniMaxDerivedPath(model, "/v1/query/video_generation")
}

func buildMiniMaxFileRetrievePath(model *types.Model) (string, error) {
	return miniMaxDerivedPath(model, "/v1/files/retrieve")
}

func miniMaxDerivedPath(model *types.Model, replacement string) (string, error) {
	createPath, err := buildMiniMaxCreatePath(model)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(createPath, miniMaxCreatePathSuffix) + replacement, nil
}

func miniMaxPathFromEndpoint(model *types.Model, expectedSuffix string) (string, error) {
	if model == nil {
		return "", fmt.Errorf("minimax model is required")
	}
	path := commonutils.ExtractURLPath(model.Endpoint)
	if path == "" {
		return "", fmt.Errorf("minimax endpoint path is empty")
	}
	if !strings.HasSuffix(path, expectedSuffix) {
		return "", &RequestValidationError{
			Message: fmt.Sprintf("MiniMax endpoint path must end with %s", expectedSuffix),
		}
	}
	return path, nil
}

func normalizeMiniMaxResolution(size string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(size))
	switch normalized {
	case "":
		return "", nil
	case "768P", "1080P":
		return normalized, nil
	case "1280X720", "720X1280":
		return "768P", nil
	case "1920X1080", "1080X1920":
		return "1080P", nil
	default:
		return "", &RequestValidationError{
			Message: fmt.Sprintf("MiniMax video backend does not support size %q; supported sizes are 1280x720 (768P) and 1920x1080 (1080P)", size),
		}
	}
}

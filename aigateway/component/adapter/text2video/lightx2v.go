package text2video

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
	commonTypes "opencsg.com/csghub-server/common/types"
)

const (
	frameworkLightX2V     = "lightx2v"
	lightX2VMaxImageBytes = 20 << 20
)

// LightX2VAdapter normalizes the internal CSGHub LightX2V video generation API.
// Docs:
//   - docker/inference/video-generation/Readme.md
type LightX2VAdapter struct{}

func NewLightX2VAdapter() *LightX2VAdapter {
	return &LightX2VAdapter{}
}

func (a *LightX2VAdapter) Name() string {
	return frameworkLightX2V
}

func (a *LightX2VAdapter) CanHandle(model *types.Model) bool {
	if model == nil {
		return false
	}
	if strings.TrimSpace(model.CSGHubModelID) == "" {
		return false
	}
	if model.Task != string(commonTypes.Text2Video) && model.Task != string(commonTypes.Image2Video) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(model.RuntimeFramework), frameworkLightX2V)
}

func (a *LightX2VAdapter) Capabilities(model *types.Model) Capabilities {
	if !a.CanHandle(model) {
		return Capabilities{}
	}
	return Capabilities{
		SupportsCreate:                  true,
		SupportsImageReference:          true,
		SupportsMultipartInputReference: true,
		SupportsJSONImageURL:            true,
		SupportsDirectContentStreaming:  true,
	}
}

func (a *LightX2VAdapter) BuildCreateRequest(ctx context.Context, model *types.Model, input CreateRequestInput) (*ProviderRequest, error) {
	if input.Request.InputReference != nil && input.Request.InputReference.FileID != "" {
		return nil, &RequestValidationError{Message: "selected model does not support input_reference.file_id"}
	}
	if input.IsMultipart && hasMultipartInputReference(input.Multipart) {
		body, contentType, err := a.buildMultipartCreateBody(input)
		if err != nil {
			return nil, err
		}
		return &ProviderRequest{
			Method:      http.MethodPost,
			Path:        "/v1/tasks/video/form",
			Body:        body,
			ContentType: contentType,
		}, nil
	}

	if input.Request.InputReference != nil && input.Request.InputReference.ImageURL != "" {
		body, contentType, err := a.buildImageURLCreateBody(ctx, input.Request)
		if err != nil {
			return nil, err
		}
		return &ProviderRequest{
			Method:      http.MethodPost,
			Path:        "/v1/tasks/video/form",
			Body:        body,
			ContentType: contentType,
		}, nil
	}

	body, err := a.buildJSONCreateBody(input.Request)
	if err != nil {
		return nil, err
	}
	return &ProviderRequest{
		Method:      http.MethodPost,
		Path:        "/v1/tasks/video",
		Body:        body,
		ContentType: "application/json",
	}, nil
}

func (a *LightX2VAdapter) ParseCreateResponse(ctx context.Context, body []byte) (*ProviderResponse, error) {
	return a.parseProviderResponse(body)
}

func (a *LightX2VAdapter) BuildRetrieveRequest(ctx context.Context, model *types.Model, providerResourceID string, providerMetadata map[string]any) (*ProviderRequest, error) {
	return &ProviderRequest{
		Method: http.MethodGet,
		Path:   "/v1/tasks/" + providerResourceID + "/status",
	}, nil
}

func (a *LightX2VAdapter) ParseRetrieveResponse(ctx context.Context, body []byte) (*ProviderResponse, error) {
	return a.parseProviderResponse(body)
}

func (a *LightX2VAdapter) BuildContentRequest(ctx context.Context, model *types.Model, providerResourceID string, providerMetadata map[string]any) (*ProviderRequest, error) {
	return &ProviderRequest{
		Method: http.MethodGet,
		Path:   "/v1/files/download/outputs/videos/" + providerResourceID + ".mp4",
	}, nil
}

func (a *LightX2VAdapter) ParseContentResponse(ctx context.Context, body []byte) (*ContentResponse, error) {
	return &ContentResponse{}, nil
}

func (a *LightX2VAdapter) buildJSONCreateBody(req types.VideoGenerationRequest) ([]byte, error) {
	payload := map[string]any{
		"prompt": req.Prompt,
	}
	if req.Seconds > 0 {
		payload["video_duration"] = req.Seconds
	}
	if req.Size != "" {
		width, height, ok := parseOpenAIVideoSize(req.Size)
		if !ok {
			return nil, &RequestValidationError{Message: fmt.Sprintf("LightX2V video backend requires size in {width}x{height} format, got %q", req.Size)}
		}
		payload["width"] = width
		payload["height"] = height
	}
	return json.Marshal(payload)
}

func (a *LightX2VAdapter) buildMultipartCreateBody(input CreateRequestInput) ([]byte, string, error) {
	if input.Multipart == nil {
		return nil, "", fmt.Errorf("multipart form is required")
	}
	files := input.Multipart.File["input_reference"]
	if len(files) == 0 {
		return nil, "", &RequestValidationError{Message: "selected model requires multipart input_reference for image-guided video generation"}
	}
	fileHeader := files[0]
	return buildMultipartBody(func(writer *multipart.Writer) error {
		if err := writeLightX2VMultipartFields(writer, input.Request); err != nil {
			return err
		}
		return copyMultipartFileAsImage(writer, fileHeader)
	})
}

func hasMultipartInputReference(form *multipart.Form) bool {
	if form == nil {
		return false
	}
	return len(form.File["input_reference"]) > 0
}

func (a *LightX2VAdapter) buildImageURLCreateBody(ctx context.Context, req types.VideoGenerationRequest) ([]byte, string, error) {
	imageURL := strings.TrimSpace(req.InputReference.ImageURL)
	if imageURL == "" {
		return nil, "", &RequestValidationError{Message: "input_reference.image_url cannot be empty"}
	}
	imageBytes, contentType, err := fetchRemoteImage(ctx, imageURL)
	if err != nil {
		return nil, "", err
	}
	return buildMultipartBody(func(writer *multipart.Writer) error {
		if err := writeLightX2VMultipartFields(writer, req); err != nil {
			return err
		}
		return writeImageFilePart(writer, "input_reference"+contentTypeExtension(contentType), contentType, imageBytes)
	})
}

func writeLightX2VMultipartFields(writer *multipart.Writer, req types.VideoGenerationRequest) error {
	if err := writer.WriteField("prompt", req.Prompt); err != nil {
		return err
	}
	if req.Seconds > 0 {
		if err := writer.WriteField("video_duration", strconv.FormatInt(req.Seconds, 10)); err != nil {
			return err
		}
	}
	if req.Size != "" {
		width, height, ok := parseOpenAIVideoSize(req.Size)
		if !ok {
			return &RequestValidationError{Message: fmt.Sprintf("LightX2V video backend requires size in {width}x{height} format, got %q", req.Size)}
		}
		if err := writer.WriteField("width", strconv.Itoa(width)); err != nil {
			return err
		}
		if err := writer.WriteField("height", strconv.Itoa(height)); err != nil {
			return err
		}
	}
	return nil
}

func copyMultipartFileAsImage(writer *multipart.Writer, fileHeader *multipart.FileHeader) error {
	if fileHeader == nil {
		return fmt.Errorf("multipart file header is nil")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	imageBytes, contentType, err := readAndValidateImage(file, fileHeader.Size)
	if err != nil {
		return err
	}
	filename := fileHeader.Filename
	if filename == "" {
		filename = "input_reference" + contentTypeExtension(contentType)
	}
	return writeImageFilePart(writer, filename, contentType, imageBytes)
}

func writeImageFilePart(writer *multipart.Writer, filename, contentType string, imageBytes []byte) error {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "image_file", escapeMultipartQuotes(filename)))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, bytes.NewReader(imageBytes))
	return err
}

func fetchRemoteImage(ctx context.Context, imageURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, "", &RequestValidationError{Message: "invalid input_reference.image_url: " + err.Error()}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", &RequestValidationError{Message: "failed to fetch input_reference.image_url: " + err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return nil, "", &RequestValidationError{Message: fmt.Sprintf("failed to fetch input_reference.image_url: unexpected status %d", resp.StatusCode)}
	}
	return readAndValidateImage(resp.Body, resp.ContentLength)
}

func readAndValidateImage(reader io.Reader, contentLength int64) ([]byte, string, error) {
	if contentLength > lightX2VMaxImageBytes {
		return nil, "", &RequestValidationError{Message: fmt.Sprintf("input_reference image exceeds %d bytes", lightX2VMaxImageBytes)}
	}
	limited := io.LimitReader(reader, lightX2VMaxImageBytes+1)
	imageBytes, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", err
	}
	if len(imageBytes) == 0 {
		return nil, "", &RequestValidationError{Message: "input_reference image is empty"}
	}
	if len(imageBytes) > lightX2VMaxImageBytes {
		return nil, "", &RequestValidationError{Message: fmt.Sprintf("input_reference image exceeds %d bytes", lightX2VMaxImageBytes)}
	}
	contentType := strings.ToLower(strings.TrimSpace(http.DetectContentType(imageBytes)))
	contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	switch contentType {
	case "image/jpeg", "image/png", "image/webp":
		return imageBytes, contentType, nil
	default:
		return nil, "", &RequestValidationError{Message: fmt.Sprintf("unsupported input_reference image content type %q", contentType)}
	}
}

func contentTypeExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ".bin"
	}
}

func (a *LightX2VAdapter) parseProviderResponse(body []byte) (*ProviderResponse, error) {
	payload, err := decodeJSON(body)
	if err != nil {
		return nil, err
	}
	taskID := stringAt(payload, "task_id")
	if taskID == "" {
		return nil, fmt.Errorf("lightx2v response missing task_id")
	}
	rawStatus := stringAt(payload, "status")
	status := mapLightX2VStatus(rawStatus)
	video := &types.VideoObject{
		ID:     taskID,
		Object: "video",
		Status: status,
	}
	if progress, ok := float64At(payload, "progress"); ok {
		video.Progress = &progress
	}
	if status == string(commonTypes.AIGatewayAsyncGenerationStatusFailed) {
		if message := stringAt(payload, "message"); message != "" {
			video.Error = &types.VideoError{
				Code:    "generation_failed",
				Message: message,
			}
		}
	}
	return &ProviderResponse{Video: video, ProviderMetadata: WithProviderStatus(nil, rawStatus)}, nil
}

func mapLightX2VStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "submitted":
		return string(commonTypes.AIGatewayAsyncGenerationStatusQueued)
	case "running":
		return string(commonTypes.AIGatewayAsyncGenerationStatusInProgress)
	case "succeed":
		return string(commonTypes.AIGatewayAsyncGenerationStatusCompleted)
	case "failed":
		return string(commonTypes.AIGatewayAsyncGenerationStatusFailed)
	default:
		return status
	}
}

func float64At(payload map[string]any, path string) (float64, bool) {
	if payload == nil || path == "" {
		return 0, false
	}
	var cur any = payload
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return 0, false
		}
		cur = m[part]
	}
	switch value := cur.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	default:
		return 0, false
	}
}

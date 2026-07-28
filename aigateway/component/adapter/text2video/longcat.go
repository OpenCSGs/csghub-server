package text2video

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
	commonTypes "opencsg.com/csghub-server/common/types"
)

const (
	frameworkLongCat    = "longcat-video"
	frameworkAMDLongCat = "amd-longcat-video"
	longCatModelNameV1  = "LongCat-Video-Avatar"
	longCatModelNameV15 = "LongCat-Video-Avatar-1.5"
)

// LongCatAdapter maps audio-driven avatar requests onto the task lifecycle
// shared with the internal LightX2V runtime.
type LongCatAdapter struct{}

func NewLongCatAdapter() *LongCatAdapter {
	return &LongCatAdapter{}
}

func (a *LongCatAdapter) Name() string {
	return frameworkLongCat
}

func (a *LongCatAdapter) CanHandle(model *types.Model) bool {
	if model == nil || strings.TrimSpace(model.CSGHubModelID) == "" {
		return false
	}
	switch commonTypes.PipelineTask(model.Task) {
	case commonTypes.Text2Video,
		commonTypes.Image2Video,
		commonTypes.AudioText2Video,
		commonTypes.AudioImageText2Video,
		commonTypes.AudioDrivenVideoContinuation:
	default:
		return false
	}
	framework := strings.ToLower(strings.TrimSpace(model.RuntimeFramework))
	if framework != frameworkLongCat && framework != frameworkAMDLongCat {
		return false
	}
	parts := strings.Split(strings.TrimSpace(model.CSGHubModelID), "/")
	modelName := parts[len(parts)-1]
	return strings.EqualFold(modelName, longCatModelNameV1) ||
		strings.EqualFold(modelName, longCatModelNameV15)
}

func (a *LongCatAdapter) Capabilities(model *types.Model) Capabilities {
	if !a.CanHandle(model) {
		return Capabilities{}
	}
	return Capabilities{
		SupportsCreate:                  true,
		SupportsImageReference:          true,
		SupportsMultipartInputReference: true,
		SupportsAudioReference:          true,
		RequiresAudioReference:          true,
		MaxAudioReferences:              2,
		SupportsDirectContentStreaming:  true,
	}
}

func (a *LongCatAdapter) BuildCreateRequest(_ context.Context, _ *types.Model, input CreateRequestInput) (*ProviderRequest, error) {
	if !input.IsMultipart || input.Multipart == nil {
		return nil, &RequestValidationError{Message: "LongCat video backend requires multipart/form-data"}
	}
	audioFiles := input.Multipart.File["audio"]
	if len(audioFiles) < 1 || len(audioFiles) > 2 {
		return nil, &RequestValidationError{Message: "LongCat video backend requires one or two audio files"}
	}
	imageFiles := input.Multipart.File["input_reference"]
	if len(audioFiles) == 2 && len(imageFiles) == 0 {
		return nil, &RequestValidationError{Message: "LongCat multi-speaker generation requires input_reference"}
	}
	if len(imageFiles) > 1 {
		return nil, &RequestValidationError{Message: "input_reference only supports one uploaded file"}
	}
	if input.Request.Seconds > 0 {
		return nil, &RequestValidationError{Message: "LongCat video backend does not support arbitrary seconds; use num_segments"}
	}

	resolution, err := longCatResolution(input.Request.Size)
	if err != nil {
		return nil, err
	}
	if err := validateLongCatFormValues(input.Multipart, len(audioFiles)); err != nil {
		return nil, err
	}

	body, contentType, err := buildMultipartBody(func(writer *multipart.Writer) error {
		if err := writer.WriteField("prompt", input.Request.Prompt); err != nil {
			return err
		}
		if err := writer.WriteField("resolution", resolution); err != nil {
			return err
		}
		for _, key := range []string{"audio_type", "num_segments", "ref_img_index", "mask_frame_range", "bbox"} {
			for _, value := range input.Multipart.Value[key] {
				if err := writer.WriteField(key, value); err != nil {
					return err
				}
			}
		}
		for _, audioFile := range audioFiles {
			if err := copyMultipartFileField(writer, "audio", audioFile); err != nil {
				return err
			}
		}
		if len(imageFiles) == 1 {
			if err := copyMultipartFileField(writer, "image_file", imageFiles[0]); err != nil {
				return err
			}
		}
		return nil
	})
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

func longCatResolution(size string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(size)) {
	case "", "832x480", "480p":
		return "480p", nil
	case "1280x768", "720p":
		return "720p", nil
	default:
		return "", &RequestValidationError{Message: fmt.Sprintf("LongCat video backend supports size 832x480 or 1280x768, got %q", size)}
	}
}

func validateLongCatFormValues(form *multipart.Form, audioCount int) error {
	audioType := strings.ToLower(strings.TrimSpace(firstFormValue(form, "audio_type")))
	if audioType != "" && audioType != "para" && audioType != "add" {
		return &RequestValidationError{Message: "audio_type must be para or add"}
	}
	if audioCount == 1 && audioType != "" {
		return &RequestValidationError{Message: "audio_type is only valid with two audio files"}
	}
	for _, field := range []struct {
		name    string
		minimum int
		maximum int
	}{
		{name: "num_segments", minimum: 1, maximum: 100},
		{name: "ref_img_index", minimum: -1000, maximum: 1000},
		{name: "mask_frame_range", minimum: 0, maximum: 1000},
	} {
		if err := validateLongCatIntegerField(form, field.name, field.minimum, field.maximum); err != nil {
			return err
		}
	}
	if rawBBox := strings.TrimSpace(firstFormValue(form, "bbox")); rawBBox != "" {
		var bbox map[string][]int
		if err := json.Unmarshal([]byte(rawBBox), &bbox); err != nil {
			return &RequestValidationError{Message: "bbox must be a JSON object containing [ymin,xmin,ymax,xmax] arrays"}
		}
		for speaker, box := range bbox {
			if speaker != "person1" && speaker != "person2" && speaker != "others" {
				return &RequestValidationError{Message: fmt.Sprintf("bbox contains unsupported key %q", speaker)}
			}
			if (speaker != "others" && len(box) != 4) || (speaker == "others" && (len(box) == 0 || len(box)%4 != 0)) {
				return &RequestValidationError{Message: fmt.Sprintf("bbox for %s must contain four coordinates", speaker)}
			}
			for offset := 0; offset < len(box); offset += 4 {
				if box[offset] < 0 || box[offset+1] < 0 || box[offset+2] < 0 || box[offset+3] < 0 ||
					box[offset] > box[offset+2] || box[offset+1] > box[offset+3] {
					return &RequestValidationError{Message: fmt.Sprintf("bbox for %s has invalid coordinate order", speaker)}
				}
			}
		}
	}
	return nil
}

func validateLongCatIntegerField(form *multipart.Form, field string, minimum, maximum int) error {
	value := strings.TrimSpace(firstFormValue(form, field))
	if value == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minimum || parsed > maximum {
		return &RequestValidationError{
			Message: fmt.Sprintf("%s must be an integer between %d and %d", field, minimum, maximum),
		}
	}
	return nil
}

func firstFormValue(form *multipart.Form, key string) string {
	if form == nil || len(form.Value[key]) == 0 {
		return ""
	}
	return form.Value[key][0]
}

func (a *LongCatAdapter) ParseCreateResponse(_ context.Context, body []byte) (*ProviderResponse, error) {
	return a.parseProviderResponse(body)
}

func (a *LongCatAdapter) BuildRetrieveRequest(_ context.Context, _ *types.Model, providerResourceID string, _ map[string]any) (*ProviderRequest, error) {
	return &ProviderRequest{Method: http.MethodGet, Path: "/v1/tasks/" + providerResourceID + "/status"}, nil
}

func (a *LongCatAdapter) ParseRetrieveResponse(_ context.Context, body []byte) (*ProviderResponse, error) {
	return a.parseProviderResponse(body)
}

func (a *LongCatAdapter) BuildContentRequest(_ context.Context, _ *types.Model, providerResourceID string, _ map[string]any) (*ProviderRequest, error) {
	return &ProviderRequest{Method: http.MethodGet, Path: "/v1/files/download/outputs/videos/" + providerResourceID + ".mp4"}, nil
}

func (a *LongCatAdapter) ParseContentResponse(_ context.Context, _ []byte) (*ContentResponse, error) {
	return &ContentResponse{}, nil
}

func (a *LongCatAdapter) parseProviderResponse(body []byte) (*ProviderResponse, error) {
	payload, err := decodeJSON(body)
	if err != nil {
		return nil, err
	}
	taskID := stringAt(payload, "task_id")
	if taskID == "" {
		return nil, fmt.Errorf("longcat response missing task_id")
	}
	rawStatus := stringAt(payload, "status")
	status := mapLightX2VStatus(rawStatus)
	video := &types.VideoObject{ID: taskID, Object: "video", Status: status}
	if progress, ok := float64At(payload, "progress"); ok {
		video.Progress = &progress
	}
	if status == string(commonTypes.AIGatewayAsyncGenerationStatusFailed) {
		video.Error = &types.VideoError{Code: "generation_failed", Message: stringAt(payload, "message")}
	}
	metadata := WithProviderStatus(nil, rawStatus)
	if downloadURL := stringAt(payload, "download_url"); downloadURL != "" {
		if metadata == nil {
			metadata = make(map[string]any)
		}
		metadata["download_url"] = downloadURL
	}
	return &ProviderResponse{Video: video, ProviderMetadata: metadata}, nil
}

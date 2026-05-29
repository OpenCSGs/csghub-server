package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"opencsg.com/csghub-server/aigateway/component/adapter/text2video"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/builder/store/database"
)

// CreateVideo godoc
// @Security     ApiKey
// @Summary      Create a video generation
// @Description  Creates an OpenAI-compatible text-to-video or image-to-video generation request. Image input can be supplied by JSON input_reference or multipart input_reference.
// @Tags         AIGateway
// @Accept       json
// @Accept       multipart/form-data
// @Produce      json
// @Param        request body types.VideoGenerationRequest true "Video generation request"
// @Param        model formData string false "Model ID for multipart requests"
// @Param        prompt formData string false "Video prompt for multipart requests"
// @Param        size formData string false "Video size for multipart requests"
// @Param        seconds formData int false "Video duration in seconds for multipart requests"
// @Param        input_reference formData file false "Image input reference for multipart image-to-video requests"
// @Success      200 {object} types.VideoObject "OK"
// @Failure      400 {object} types.Error "Bad request"
// @Failure      404 {object} types.Error "Model not found"
// @Failure      500 {object} types.Error "Internal server error"
// @Router       /v1/videos [post]
func (h *OpenAIHandlerImpl) CreateVideo(c *gin.Context) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	ctx := c.Request.Context()

	input, ok := parseCreateVideoInput(c)
	if !ok {
		return
	}

	modelTarget, err := h.resolveModelTarget(ctx, username, input.modelID, c.Request.Header)
	if err != nil {
		handleModelTargetError(c, ctx, input.modelID, "failed to get video target address", err)
		return
	}

	adapter := h.t2vRegistry.GetAdapter(modelTarget.Model)
	if !validateCreateVideoAdapter(c, input, adapter, modelTarget.Model) {
		return
	}

	if !h.authorizeCreateVideo(c, ctx, nsUUID, input, modelTarget) {
		return
	}

	providerReq, ok := buildCreateVideoProviderRequest(c, ctx, input, adapter, modelTarget)
	if !ok {
		return
	}
	capture, ok := proxyCreateVideoRequest(c, ctx, providerReq, modelTarget)
	if !ok {
		return
	}
	body := capture.Body()
	if capture.StatusCode() < http.StatusBadRequest {
		body, ok = h.normalizeCreateVideoResponse(c, ctx, nsUUID, input.modelID, adapter, body)
		if !ok {
			return
		}
	}

	copyProxyResponse(c, capture.Header(), capture.StatusCode(), body)
}

type createVideoInput struct {
	adapterReq                 types.VideoGenerationRequest
	form                       *multipart.Form
	modelID                    string
	isMultipart                bool
	hasMultipartInputReference bool
}

func parseCreateVideoInput(c *gin.Context) (*createVideoInput, bool) {
	input := &createVideoInput{
		isMultipart: strings.HasPrefix(c.ContentType(), "multipart/form-data"),
	}
	if input.isMultipart {
		return parseCreateVideoMultipartInput(c, input)
	}
	return parseCreateVideoJSONInput(c, input)
}

func parseCreateVideoMultipartInput(c *gin.Context, input *createVideoInput) (*createVideoInput, bool) {
	form, err := c.MultipartForm()
	if err != nil {
		writeVideoAPIError(c, http.StatusBadRequest, "invalid_request_error", "invalid multipart form: "+err.Error(), "invalid_request_error")
		return nil, false
	}
	modelID := strings.TrimSpace(firstMultipartValue(form, "model"))
	prompt := firstMultipartValue(form, "prompt")
	if modelID == "" || strings.TrimSpace(prompt) == "" {
		writeVideoAPIError(c, http.StatusBadRequest, "invalid_request_error", "Model and prompt cannot be empty", "invalid_request_error")
		return nil, false
	}
	if err := validateVideoMultipartInputReference(form); err != nil {
		writeVideoAPIError(c, http.StatusBadRequest, "invalid_request_error", err.Error(), "invalid_request_error")
		return nil, false
	}

	input.form = form
	input.modelID = modelID
	input.hasMultipartInputReference = len(form.File["input_reference"]) > 0
	input.adapterReq = types.VideoGenerationRequest{
		Model:  modelID,
		Prompt: prompt,
		Size:   firstMultipartValue(form, "size"),
	}
	if seconds := strings.TrimSpace(firstMultipartValue(form, "seconds")); seconds != "" {
		var parsed int64
		if _, err := fmt.Sscanf(seconds, "%d", &parsed); err == nil {
			input.adapterReq.Seconds = parsed
		}
	}
	return input, true
}

func parseCreateVideoJSONInput(c *gin.Context, input *createVideoInput) (*createVideoInput, bool) {
	if err := c.ShouldBindJSON(&input.adapterReq); err != nil {
		writeVideoAPIError(c, http.StatusBadRequest, "invalid_request_error", err.Error(), "invalid_request_error")
		return nil, false
	}
	input.modelID = input.adapterReq.Model
	if strings.TrimSpace(input.modelID) == "" || strings.TrimSpace(input.adapterReq.Prompt) == "" {
		writeVideoAPIError(c, http.StatusBadRequest, "invalid_request_error", "Model and prompt cannot be empty", "invalid_request_error")
		return nil, false
	}
	if err := validateVideoJSONInputReference(input.adapterReq); err != nil {
		writeVideoAPIError(c, http.StatusBadRequest, "invalid_request_error", err.Error(), "invalid_request_error")
		return nil, false
	}
	return input, true
}

func validateCreateVideoAdapter(c *gin.Context, input *createVideoInput, adapter text2video.T2VAdapter, model *types.Model) bool {
	if adapter == nil {
		writeVideoAPIError(c, http.StatusBadRequest, "unsupported_model", fmt.Sprintf("no video adapter for model '%s'", input.modelID), "invalid_request_error")
		return false
	}
	caps := adapter.Capabilities(model)
	if !caps.SupportsCreate {
		writeVideoAPIError(c, http.StatusBadRequest, "unsupported_model", fmt.Sprintf("model '%s' does not support video generation", input.modelID), "invalid_request_error")
		return false
	}
	if err := validateVideoAdapterCompatibility(input.adapterReq, input.isMultipart, input.hasMultipartInputReference, caps); err != nil {
		writeVideoAPIError(c, http.StatusBadRequest, "invalid_request_error", err.Error(), "invalid_request_error")
		return false
	}
	return true
}

func (h *OpenAIHandlerImpl) authorizeCreateVideo(c *gin.Context, ctx context.Context, nsUUID string, input *createVideoInput, modelTarget *resolvedModelTarget) bool {
	if !modelTarget.Model.SkipBalance() {
		if err := h.openaiComponent.CheckBalance(ctx, nsUUID); err != nil {
			h.handleInsufficientBalance(c, false, nsUUID, input.modelID, err)
			return false
		}
	}

	result, err := h.modComponent.CheckImagePrompts(ctx, input.adapterReq.Prompt, nsUUID)
	if err != nil {
		writeVideoAPIError(c, http.StatusInternalServerError, "moderation_error", "failed to check video prompts: "+err.Error(), "internal_error")
		return false
	}
	if result != nil && result.IsSensitive {
		writeVideoAPIError(c, http.StatusBadRequest, "content_policy_violation", "Input data may contain inappropriate content.", "invalid_request_error")
		return false
	}
	return true
}

func buildCreateVideoProviderRequest(c *gin.Context, ctx context.Context, input *createVideoInput, adapter text2video.T2VAdapter, modelTarget *resolvedModelTarget) (*text2video.ProviderRequest, bool) {
	req := input.adapterReq
	req.Model = modelTarget.ModelName
	providerReq, err := adapter.BuildCreateRequest(ctx, modelTarget.Model, text2video.CreateRequestInput{
		Request:     req,
		Multipart:   input.form,
		IsMultipart: input.isMultipart,
	})
	if err != nil {
		var requestValidationErr *text2video.RequestValidationError
		if errors.As(err, &requestValidationErr) {
			writeVideoAPIError(c, http.StatusBadRequest, "invalid_request_error", requestValidationErr.Error(), "invalid_request_error")
			return nil, false
		}
		writeVideoAPIError(c, http.StatusInternalServerError, "internal_error", err.Error(), "internal_error")
		return nil, false
	}
	return providerReq, true
}

func proxyCreateVideoRequest(c *gin.Context, ctx context.Context, providerReq *text2video.ProviderRequest, modelTarget *resolvedModelTarget) (*videoProxyCapture, bool) {
	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}

	rp, err := proxy.NewReverseProxy(modelTarget.Target, proxy.WithoutAcceptEncoding())
	if err != nil {
		slog.ErrorContext(ctx, "failed to create reverse proxy", slog.Any("error", err))
		writeVideoAPIError(c, http.StatusInternalServerError, "internal_error", fmt.Errorf("failed to create reverse proxy:%w", err).Error(), "internal_error")
		return nil, false
	}

	capture := newVideoProxyCapture()
	rp.ServeHTTP(capture, applyVideoProviderRequest(c.Request, providerReq), providerReq.Path, modelTarget.Host)
	return capture, true
}

func (h *OpenAIHandlerImpl) normalizeCreateVideoResponse(c *gin.Context, ctx context.Context, nsUUID, modelID string, adapter text2video.T2VAdapter, body []byte) ([]byte, bool) {
	videoProviderResp, err := adapter.ParseCreateResponse(ctx, body)
	if err != nil {
		var requestValidationErr *text2video.RequestValidationError
		if errors.As(err, &requestValidationErr) {
			writeVideoAPIError(c, http.StatusBadRequest, "invalid_request_error", requestValidationErr.Error(), "invalid_request_error")
			return nil, false
		}
		message := "failed to parse downstream video response: " + err.Error()
		if downstreamPayload := downstreamErrorPayload(body); downstreamPayload != "" {
			message += "; downstream payload: " + downstreamPayload
		}
		writeVideoAPIError(c, http.StatusInternalServerError, "internal_error", message, "internal_error")
		return nil, false
	}
	videoResp := videoProviderResp.Video
	if videoResp.ID == "" || h.aiGenerationStore == nil {
		return body, true
	}

	providerResourceID := videoResp.ID
	videoID := newGatewayResourceID()
	if _, err := h.aiGenerationStore.Create(ctx, database.AIGeneration{
		ResourceType:       database.AIGenerationResourceTypeVideo,
		ResourceID:         videoID,
		ProviderResourceID: providerResourceID,
		ProviderMetadata:   videoProviderResp.ProviderMetadata,
		OwnerUUID:          nsUUID,
		ModelID:            modelID,
		Status:             videoResp.Status,
	}); err != nil {
		slog.ErrorContext(ctx, "failed to persist ai video generation", slog.Any("error", err), slog.String("video_id", videoID), slog.String("provider_resource_id", providerResourceID))
		writeVideoAPIError(c, http.StatusInternalServerError, "internal_error", "failed to persist ai generation", "internal_error")
		return nil, false
	}
	return normalizeVideoResponseBody(adapter, body, videoResp, videoID), true
}

func writeVideoAPIError(c *gin.Context, status int, code, message, errorType string) {
	c.JSON(status, gin.H{"error": types.Error{
		Code:    code,
		Message: message,
		Type:    errorType,
	}})
}

func downstreamErrorPayload(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, body); err == nil {
		return strings.TrimSpace(compact.String())
	}
	return strings.TrimSpace(string(body))
}

// GetVideo godoc
// @Security     ApiKey
// @Summary      Get a video generation
// @Description  Retrieves an OpenAI-compatible video generation object by gateway video ID.
// @Tags         AIGateway
// @Produce      json
// @Param        video_id path string true "Gateway video ID"
// @Success      200 {object} types.VideoObject "OK"
// @Failure      400 {object} types.Error "Bad request"
// @Failure      404 {object} types.Error "Video not found"
// @Failure      500 {object} types.Error "Internal server error"
// @Router       /v1/videos/{video_id} [get]
func (h *OpenAIHandlerImpl) GetVideo(c *gin.Context) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	videoID := c.Param("video_id")
	ctx := c.Request.Context()

	target, ok := h.resolveVideoGenerationRequest(c, ctx, username, nsUUID, videoID)
	if !ok {
		return
	}
	adapter, ok := h.videoAdapterForGeneration(c, target)
	if !ok {
		return
	}
	rp, ok := newVideoReverseProxy(c, ctx, target.modelTarget, proxy.WithoutAcceptEncoding())
	if !ok {
		return
	}
	capture, videoProviderResp, ok := h.fetchVideoRetrieveResponse(c, ctx, target, adapter, rp)
	if !ok {
		return
	}
	body := capture.Body()
	var videoResp *types.VideoObject
	if videoProviderResp != nil {
		videoResp = videoProviderResp.Video
	}
	copyProxyResponse(c, capture.Header(), capture.StatusCode(), normalizeVideoResponseBody(adapter, body, videoResp, target.generation.ResourceID))
}

// GetVideoContent godoc
// @Security     ApiKey
// @Summary      Download generated video content
// @Description  Streams generated video bytes for an OpenAI-compatible video generation by gateway video ID.
// @Tags         AIGateway
// @Produce      application/octet-stream
// @Produce      video/mp4
// @Param        video_id path string true "Gateway video ID"
// @Success      200 {file} binary "Generated video content"
// @Failure      400 {object} types.Error "Bad request"
// @Failure      404 {object} types.Error "Video not found"
// @Failure      500 {object} types.Error "Internal server error"
// @Router       /v1/videos/{video_id}/content [get]
func (h *OpenAIHandlerImpl) GetVideoContent(c *gin.Context) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	videoID := c.Param("video_id")
	ctx := c.Request.Context()

	target, ok := h.resolveVideoGenerationRequest(c, ctx, username, nsUUID, videoID)
	if !ok {
		return
	}
	adapter, ok := h.videoAdapterForGeneration(c, target)
	if !ok {
		return
	}
	rp, ok := newVideoReverseProxy(c, ctx, target.modelTarget, proxy.WithoutAcceptEncoding())
	if !ok {
		return
	}
	providerReq, ok := buildVideoContentRequest(c, ctx, adapter, target)
	if !ok {
		return
	}

	if adapter.Capabilities(target.modelTarget.Model).SupportsDirectContentStreaming {
		rp.ServeHTTP(videoStreamingWriter{w: c.Writer}, applyVideoProviderRequest(c.Request, providerReq), providerReq.Path, target.modelTarget.Host)
		return
	}

	contentResp, ok := h.resolveVideoContentDownload(c, ctx, target, adapter, rp, providerReq)
	if !ok {
		return
	}
	streamVideoDownloadURL(c, contentResp.DownloadURL)
}

type videoGenerationTarget struct {
	generation  *database.AIGeneration
	modelTarget *resolvedModelTarget
}

func (h *OpenAIHandlerImpl) resolveVideoGenerationRequest(c *gin.Context, ctx context.Context, username, nsUUID, videoID string) (*videoGenerationTarget, bool) {
	generation, modelTarget, err := h.resolveAIGenerationTarget(ctx, username, nsUUID, videoID, c.Request.Header)
	if err != nil {
		handleAIGenerationError(c, err)
		return nil, false
	}
	return &videoGenerationTarget{
		generation:  generation,
		modelTarget: modelTarget,
	}, true
}

func (h *OpenAIHandlerImpl) videoAdapterForGeneration(c *gin.Context, target *videoGenerationTarget) (text2video.T2VAdapter, bool) {
	adapter := h.t2vRegistry.GetAdapter(target.modelTarget.Model)
	if adapter == nil {
		writeVideoAPIError(c, http.StatusBadRequest, "unsupported_model", fmt.Sprintf("no video adapter for model '%s'", target.generation.ModelID), "invalid_request_error")
		return nil, false
	}
	return adapter, true
}

func newVideoReverseProxy(c *gin.Context, ctx context.Context, modelTarget *resolvedModelTarget, opts ...proxy.ReverseProxyOption) (proxy.ReverseProxy, bool) {
	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}

	rp, err := proxy.NewReverseProxy(modelTarget.Target, opts...)
	if err != nil {
		writeVideoAPIError(c, http.StatusInternalServerError, "internal_error", fmt.Errorf("failed to create reverse proxy:%w", err).Error(), "internal_error")
		return nil, false
	}
	return rp, true
}

func providerResourceID(generation *database.AIGeneration) string {
	if generation.ProviderResourceID != "" {
		return generation.ProviderResourceID
	}
	return generation.ResourceID
}

func (h *OpenAIHandlerImpl) fetchVideoRetrieveResponse(c *gin.Context, ctx context.Context, target *videoGenerationTarget, adapter text2video.T2VAdapter, rp proxy.ReverseProxy) (*videoProxyCapture, *text2video.ProviderResponse, bool) {
	providerReq, ok := buildVideoRetrieveRequest(c, ctx, adapter, target)
	if !ok {
		return nil, nil, false
	}

	capture := newVideoProxyCapture()
	rp.ServeHTTP(capture, applyVideoProviderRequest(c.Request, providerReq), providerReq.Path, target.modelTarget.Host)
	if capture.StatusCode() >= http.StatusBadRequest {
		return capture, nil, true
	}

	videoProviderResp, err := adapter.ParseRetrieveResponse(ctx, capture.Body())
	if err != nil {
		writeVideoAPIError(c, http.StatusInternalServerError, "internal_error", "failed to parse downstream video response: "+err.Error(), "internal_error")
		return nil, nil, false
	}
	h.updateVideoGenerationFromRetrieve(ctx, target.generation, videoProviderResp)
	return capture, videoProviderResp, true
}

func buildVideoRetrieveRequest(c *gin.Context, ctx context.Context, adapter text2video.T2VAdapter, target *videoGenerationTarget) (*text2video.ProviderRequest, bool) {
	providerReq, err := adapter.BuildRetrieveRequest(ctx, target.modelTarget.Model, providerResourceID(target.generation), target.generation.ProviderMetadata)
	if err != nil {
		writeVideoAPIError(c, http.StatusInternalServerError, "internal_error", err.Error(), "internal_error")
		return nil, false
	}
	return providerReq, true
}

func (h *OpenAIHandlerImpl) updateVideoGenerationFromRetrieve(ctx context.Context, generation *database.AIGeneration, videoProviderResp *text2video.ProviderResponse) {
	if videoProviderResp == nil || videoProviderResp.Video == nil || h.aiGenerationStore == nil {
		return
	}
	generation.Status = videoProviderResp.Video.Status
	generation.ProviderMetadata = text2video.MergeProviderMetadata(generation.ProviderMetadata, videoProviderResp.ProviderMetadata)
	if _, updateErr := h.aiGenerationStore.Update(ctx, *generation); updateErr != nil {
		slog.WarnContext(ctx, "failed to update video generation status", slog.Any("error", updateErr), slog.String("video_id", generation.ResourceID))
	}
}

func buildVideoContentRequest(c *gin.Context, ctx context.Context, adapter text2video.T2VAdapter, target *videoGenerationTarget) (*text2video.ProviderRequest, bool) {
	providerReq, err := adapter.BuildContentRequest(ctx, target.modelTarget.Model, providerResourceID(target.generation), target.generation.ProviderMetadata)
	if err != nil {
		writeVideoAPIError(c, http.StatusInternalServerError, "internal_error", err.Error(), "internal_error")
		return nil, false
	}
	return providerReq, true
}

func (h *OpenAIHandlerImpl) resolveVideoContentDownload(c *gin.Context, ctx context.Context, target *videoGenerationTarget, adapter text2video.T2VAdapter, rp proxy.ReverseProxy, providerReq *text2video.ProviderRequest) (*text2video.ContentResponse, bool) {
	contentResp, ok := h.fetchAndPersistVideoContentResponse(c, ctx, target, adapter, rp, providerReq)
	if !ok {
		return nil, false
	}
	if contentResp.DownloadURL == "" && len(contentResp.ProviderMetadata) > 0 {
		nextProviderReq, ok := buildVideoContentRequest(c, ctx, adapter, target)
		if !ok {
			return nil, false
		}
		if nextProviderReq.Path != providerReq.Path {
			contentResp, ok = h.fetchAndPersistVideoContentResponse(c, ctx, target, adapter, rp, nextProviderReq)
			if !ok {
				return nil, false
			}
		}
	}
	if contentResp.DownloadURL == "" {
		writeVideoAPIError(c, http.StatusInternalServerError, "internal_error", "provider response did not include a video download url", "internal_error")
		return nil, false
	}
	return contentResp, true
}

func (h *OpenAIHandlerImpl) fetchAndPersistVideoContentResponse(c *gin.Context, ctx context.Context, target *videoGenerationTarget, adapter text2video.T2VAdapter, rp proxy.ReverseProxy, providerReq *text2video.ProviderRequest) (*text2video.ContentResponse, bool) {
	contentResp, upstreamErr, err := fetchVideoContentResponse(ctx, adapter, rp, c.Request, providerReq, target.modelTarget.Host)
	if upstreamErr != nil {
		copyProxyResponse(c, upstreamErr.Header(), upstreamErr.StatusCode(), upstreamErr.Body())
		return nil, false
	}
	if err != nil {
		writeVideoAPIError(c, http.StatusInternalServerError, "internal_error", err.Error(), "internal_error")
		return nil, false
	}
	h.updateVideoGenerationMetadata(ctx, target.generation, contentResp.ProviderMetadata)
	return contentResp, true
}

func (h *OpenAIHandlerImpl) updateVideoGenerationMetadata(ctx context.Context, generation *database.AIGeneration, providerMetadata map[string]any) {
	if len(providerMetadata) == 0 || h.aiGenerationStore == nil {
		return
	}
	generation.ProviderMetadata = text2video.MergeProviderMetadata(generation.ProviderMetadata, providerMetadata)
	if _, updateErr := h.aiGenerationStore.Update(ctx, *generation); updateErr != nil {
		slog.WarnContext(ctx, "failed to update video generation metadata", slog.Any("error", updateErr), slog.String("video_id", generation.ResourceID))
	}
}

func fetchVideoContentResponse(ctx context.Context, adapter text2video.T2VAdapter, rp proxy.ReverseProxy, req *http.Request, providerReq *text2video.ProviderRequest, host string) (*text2video.ContentResponse, *videoProxyCapture, error) {
	capture := newVideoProxyCapture()
	rp.ServeHTTP(capture, applyVideoProviderRequest(req, providerReq), providerReq.Path, host)
	if capture.StatusCode() >= http.StatusBadRequest {
		return nil, capture, nil
	}
	contentResp, err := adapter.ParseContentResponse(ctx, capture.Body())
	if err != nil {
		return nil, nil, err
	}
	return contentResp, nil, nil
}

func validateVideoJSONInputReference(req types.VideoGenerationRequest) error {
	if req.InputReference == nil {
		return nil
	}
	if req.InputReference.IsZero() {
		return fmt.Errorf("input_reference must include file_id or image_url")
	}
	return nil
}

func validateVideoMultipartInputReference(form *multipart.Form) error {
	if form == nil {
		return nil
	}
	files := form.File["input_reference"]
	if len(files) == 0 {
		return nil
	}
	if len(files) > 1 {
		return fmt.Errorf("input_reference only supports one uploaded file")
	}
	contentType, err := sniffMultipartFileContentType(files[0])
	if err != nil {
		return fmt.Errorf("read input_reference: %w", err)
	}
	switch contentType {
	case "image/jpeg", "image/png", "image/webp":
		return nil
	default:
		return fmt.Errorf("unsupported input_reference content type %q", contentType)
	}
}

func sniffMultipartFileContentType(fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader == nil {
		return "", fmt.Errorf("file header is nil")
	}
	if contentType := strings.TrimSpace(fileHeader.Header.Get("Content-Type")); contentType != "" {
		contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
		if contentType != "application/octet-stream" {
			return contentType, nil
		}
	}
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.ToLower(http.DetectContentType(buf[:n])), nil
}

func validateVideoAdapterCompatibility(req types.VideoGenerationRequest, isMultipart bool, hasMultipartInputReference bool, caps text2video.Capabilities) error {
	if req.InputReference == nil && !isMultipart {
		return nil
	}
	if isMultipart {
		if !hasMultipartInputReference {
			return nil
		}
		if !caps.SupportsImageReference || !caps.SupportsMultipartInputReference {
			return fmt.Errorf("selected model does not support multipart input_reference")
		}
		return nil
	}
	if req.InputReference == nil {
		return nil
	}
	if !caps.SupportsImageReference {
		return fmt.Errorf("selected model does not support image-guided video generation")
	}
	switch {
	case req.InputReference.FileID != "":
		if !caps.SupportsJSONFileID {
			return fmt.Errorf("selected model does not support input_reference.file_id")
		}
	case req.InputReference.ImageURL != "":
		if !caps.SupportsJSONImageURL {
			return fmt.Errorf("selected model does not support input_reference.image_url")
		}
	default:
		return fmt.Errorf("input_reference must include file_id or image_url")
	}
	return nil
}

type aiGenerationError struct {
	Status   int
	APIError types.Error
	Cause    error
}

func (e *aiGenerationError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.APIError.Message
}

func (h *OpenAIHandlerImpl) resolveAIGenerationTarget(ctx context.Context, username, nsUUID, videoID string, headers http.Header) (*database.AIGeneration, *resolvedModelTarget, error) {
	if strings.TrimSpace(videoID) == "" {
		return nil, nil, &aiGenerationError{
			Status: http.StatusBadRequest,
			APIError: types.Error{
				Code:    "invalid_request_error",
				Message: "video id cannot be empty",
				Type:    "invalid_request_error",
			},
		}
	}
	if h.aiGenerationStore == nil {
		return nil, nil, &aiGenerationError{
			Status: http.StatusInternalServerError,
			APIError: types.Error{
				Code:    "internal_error",
				Message: "ai generation store not configured",
				Type:    "internal_error",
			},
		}
	}
	generation, err := h.aiGenerationStore.FindByResourceID(ctx, database.AIGenerationResourceTypeVideo, videoID)
	if err != nil {
		return nil, nil, &aiGenerationError{
			Status: http.StatusNotFound,
			APIError: types.Error{
				Code:    "not_found",
				Message: "video not found",
				Type:    "invalid_request_error",
			},
			Cause: err,
		}
	}
	if generation.OwnerUUID != nsUUID {
		return nil, nil, &aiGenerationError{
			Status: http.StatusNotFound,
			APIError: types.Error{
				Code:    "not_found",
				Message: "video not found",
				Type:    "invalid_request_error",
			},
		}
	}
	target, err := h.resolveModelTarget(ctx, username, generation.ModelID, headers)
	if err != nil {
		return nil, nil, err
	}
	return generation, target, nil
}

func handleAIGenerationError(c *gin.Context, err error) {
	var generationErr *aiGenerationError
	if errors.As(err, &generationErr) {
		c.JSON(generationErr.Status, gin.H{"error": generationErr.APIError})
		return
	}
	handleModelTargetError(c, c.Request.Context(), "", "failed to resolve ai video generation target", err)
}

func applyVideoProviderRequest(req *http.Request, providerReq *text2video.ProviderRequest) *http.Request {
	if providerReq == nil {
		return req
	}
	cloned := req.Clone(req.Context())
	if providerReq.Method != "" {
		cloned.Method = providerReq.Method
	}
	if providerReq.Query != nil {
		cloned.URL.RawQuery = providerReq.Query.Encode()
	}
	if providerReq.Body != nil {
		cloned.Body = io.NopCloser(bytes.NewReader(providerReq.Body))
		cloned.ContentLength = int64(len(providerReq.Body))
	}
	if providerReq.ContentType != "" {
		cloned.Header.Set("Content-Type", providerReq.ContentType)
	}
	return cloned
}

func streamVideoDownloadURL(c *gin.Context, downloadURL string) {
	parsed, err := url.Parse(downloadURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code:    "internal_error",
			Message: err.Error(),
			Type:    "internal_error",
		}})
		return
	}
	target := parsed.Scheme + "://" + parsed.Host
	rp, err := proxy.NewReverseProxy(target)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code:    "internal_error",
			Message: err.Error(),
			Type:    "internal_error",
		}})
		return
	}

	req := c.Request.Clone(c.Request.Context())
	req.Method = http.MethodGet
	req.URL.RawQuery = parsed.RawQuery
	req.Body = nil
	req.ContentLength = 0

	rp.ServeHTTP(videoStreamingWriter{w: c.Writer}, req, parsed.Path, parsed.Host)
}

func newGatewayResourceID() string {
	return "video_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func replaceVideoResponseID(body []byte, videoID string) []byte {
	if videoID == "" || len(body) == 0 {
		return body
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	if _, ok := payload["id"]; !ok {
		return body
	}
	payload["id"] = videoID
	rewritten, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return rewritten
}

func normalizeVideoResponseBody(adapter text2video.T2VAdapter, body []byte, videoResp *types.VideoObject, videoID string) []byte {
	if adapter == nil || adapter.Name() == "openai-compatible" {
		return replaceVideoResponseID(body, videoID)
	}
	if videoResp == nil {
		return body
	}
	normalized := *videoResp
	normalized.ID = videoID
	if normalized.Object == "" {
		normalized.Object = "video"
	}
	rewritten, err := json.Marshal(normalized)
	if err != nil {
		return body
	}
	return rewritten
}

type videoProxyCapture struct {
	header     http.Header
	body       bytes.Buffer
	statusCode int
}

func newVideoProxyCapture() *videoProxyCapture {
	return &videoProxyCapture{
		header:     http.Header{},
		statusCode: http.StatusOK,
	}
}

func (w *videoProxyCapture) Header() http.Header {
	return w.header
}

func (w *videoProxyCapture) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *videoProxyCapture) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func (w *videoProxyCapture) StatusCode() int {
	return w.statusCode
}

func (w *videoProxyCapture) Body() []byte {
	return w.body.Bytes()
}

type videoStreamingWriter struct {
	w gin.ResponseWriter
}

func (w videoStreamingWriter) Header() http.Header {
	return w.w.Header()
}

func (w videoStreamingWriter) WriteHeader(statusCode int) {
	w.w.WriteHeader(statusCode)
}

func (w videoStreamingWriter) Write(data []byte) (int, error) {
	return w.w.Write(data)
}

func (w videoStreamingWriter) Flush() {
	w.w.Flush()
}

func copyProxyResponse(c *gin.Context, header http.Header, statusCode int, body []byte) {
	for key := range c.Writer.Header() {
		c.Writer.Header().Del(key)
	}
	for key, values := range header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}
	c.Writer.Header().Del("Content-Length")
	c.Status(statusCode)
	_, _ = c.Writer.Write(body)
}

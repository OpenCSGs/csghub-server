package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"opencsg.com/csghub-server/aigateway/component/adapter/text2image"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	"opencsg.com/csghub-server/aigateway/http/response/wrapper"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	commontrace "opencsg.com/csghub-server/common/utils/trace"
)

// ImageHandlerImpl extends OpenAIHandlerImpl to serve the
// /v1/images/generations and /v1/images/edits endpoints through the
// three-stage pipeline.
type ImageHandlerImpl struct {
	*OpenAIHandlerImpl
	imagePipeline *imagePipelineHandler
	orchestrator  *plan.Orchestrator
}

func NewImageHandler(openai *OpenAIHandlerImpl) *ImageHandlerImpl {
	h := &ImageHandlerImpl{
		OpenAIHandlerImpl: openai,
		imagePipeline:     &imagePipelineHandler{handler: openai},
	}

	h.orchestrator = newOrchestrator(openai)

	return h
}

// GenerateImage handles POST /v1/images/generations.
// @Summary      Generate image from text prompt
// @Description  Generates images based on a text prompt
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        request body  types.ImageGenerationRequest true "Image generation request"
// @Success      200  {object}  types.ImageGenerationResponse "OK"
// @Failure      400  {object}  error "Bad request or sensitive input"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/images/generations [post]
func (h *ImageHandlerImpl) GenerateImage(c *gin.Context) {
	h.orchestrator.Dispatch(c, h.imagePipeline, h.imagePipeline)
}

// EditImage handles POST /v1/images/edits.
// @Summary      Edit image from prompt and input image
// @Description  Edits images with an OpenAI-compatible multipart/form-data request
// @Tags         AIGateway
// @Accept       multipart/form-data
// @Produce      json
// @Param        model formData string true "Model ID"
// @Param        prompt formData string true "Edit prompt"
// @Param        image formData file true "Input image"
// @Success      200  {object}  types.ImageGenerationResponse "OK"
// @Failure      400  {object}  error "Bad request or sensitive input"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/images/edits [post]
func (h *ImageHandlerImpl) EditImage(c *gin.Context) {
	h.orchestrator.Dispatch(c, h.imagePipeline, h.imagePipeline)
}

// imagePipelineHandler implements plan.MetadataExtractor and
// plan.ProtocolHandler for the /v1/images/generations and /v1/images/edits endpoints.
type imagePipelineHandler struct {
	handler *OpenAIHandlerImpl
}

var (
	_ plan.MetadataExtractor = (*imagePipelineHandler)(nil)
	_ plan.ProtocolHandler   = (*imagePipelineHandler)(nil)
)

// imageParsedBody carries the parsed image request through the pipeline.
type imageParsedBody struct {
	IsEdit      bool
	GenerateReq *types.ImageGenerationRequest
	// For edit (multipart)
	Form   *multipart.Form
	Model  string
	Prompt string
}

// --- Phase 1: Extract ---

func (h *imagePipelineHandler) Extract(c *gin.Context) (*types.RequestMetadata, error) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)

	isEdit := c.FullPath() == "/v1/images/edits"

	if isEdit {
		if err := c.Request.ParseMultipartForm(maxImageEditMultipartMemory); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
				Code: "invalid_request_error", Message: err.Error(), Type: "invalid_request_error",
			}})
			return nil, err
		}
		modelID := c.PostForm("model")
		prompt := c.PostForm("prompt")
		if modelID == "" || prompt == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
				Code: "invalid_request_error", Message: "Model and prompt cannot be empty", Type: "invalid_request_error",
			}})
			return nil, fmt.Errorf("model and prompt cannot be empty")
		}
		if !hasMultipartFile(c.Request.MultipartForm, "image") {
			c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
				Code: "invalid_request_error", Message: "Image is required", Type: "invalid_request_error",
			}})
			return nil, fmt.Errorf("image is required")
		}

		return &types.RequestMetadata{
			Protocol:   string(types.ProtocolChat),
			Task:       "text-to-image",
			Model:      modelID,
			TenantID:   nsUUID,
			UserID:     username,
			APIKeyID:   httpbase.GetAccessToken(c),
			Streaming:  false,
			Headers:    c.Request.Header,
			ParsedBody: &imageParsedBody{IsEdit: true, Form: c.Request.MultipartForm, Model: modelID, Prompt: prompt},
		}, nil
	}

	var req types.ImageGenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: err.Error(), Type: "invalid_request_error",
		}})
		return nil, err
	}
	if req.Prompt == "" || req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "Model and prompt cannot be empty", Type: "invalid_request_error",
		}})
		return nil, fmt.Errorf("model and prompt cannot be empty")
	}

	return &types.RequestMetadata{
		Protocol:   string(types.ProtocolChat),
		Task:       "text-to-image",
		Model:      req.Model,
		TenantID:   nsUUID,
		UserID:     username,
		APIKeyID:   httpbase.GetAccessToken(c),
		Streaming:  false,
		Headers:    c.Request.Header,
		ParsedBody: &imageParsedBody{IsEdit: false, GenerateReq: &req},
	}, nil
}

// --- Phase 3: Execute ---

func (h *imagePipelineHandler) Execute(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan) error {
	ctx := c.Request.Context()
	nsUUID := meta.TenantID
	apikey := meta.APIKeyID
	requestID := commontrace.GetTraceIDInGinContext(c)

	if pt := plan.GetPreflightTracer(c); pt != nil {
		pt.SetTargetModel(meta.Model, p.ModelTarget)
		pt.End()
		plan.SetPreflightTracer(c, nil)
	}

	mt := p.ModelTarget
	parsed := meta.ParsedBody.(*imageParsedBody)

	// Modal price guard — stays in Execute (admission check based on model + size).
	var sizeStr string
	if parsed.IsEdit {
		sizeStr = c.PostForm("size")
	} else {
		sizeStr = string(parsed.GenerateReq.Size)
	}
	if guardErr := checkModalRequestAllowed(mt.Model, sizeStr); guardErr != nil {
		handleModelTargetError(c, ctx, meta.Model, "modal price guard rejected request", guardErr)
		return nil
	}

	// Start modal generation trace.
	var traceMetadata map[string]any
	var promptText string

	if parsed.IsEdit {
		promptText = parsed.Prompt
		traceMetadata = map[string]any{
			llmtrace.TraceMetadataKeyImageSize:           c.PostForm("size"),
			llmtrace.TraceMetadataKeyImageResponseFormat: imageEditResponseFormat(parsed.Form),
			llmtrace.TraceMetadataKeyImageOutputFormat:   c.PostForm("output_format"),
		}
	} else {
		promptText = parsed.GenerateReq.Prompt
		traceMetadata = map[string]any{
			llmtrace.TraceMetadataKeyImageSize:           string(parsed.GenerateReq.Size),
			llmtrace.TraceMetadataKeyImageQuality:        string(parsed.GenerateReq.Quality),
			llmtrace.TraceMetadataKeyImageResponseFormat: string(parsed.GenerateReq.ResponseFormat),
			llmtrace.TraceMetadataKeyImageOutputFormat:   string(parsed.GenerateReq.OutputFormat),
			llmtrace.TraceMetadataKeyImageN:              parsed.GenerateReq.N,
		}
	}

	traceCtx, generationRecorder := h.handler.startModalGenerationTrace(ctx, modalTraceStartInput{
		API:           c.FullPath(),
		OperationName: modalTraceOperationGenerateContent,
		OutputType:    modalTraceOutputImage,
		RequestID:     requestID,
		NSUUID:        nsUUID,
		ModelID:       meta.Model,
		ModelTarget:   &resolvedModelTarget{
			Model: mt.Model, Upstream: mt.Upstream, Target: mt.Target, Host: mt.Host, ModelName: mt.ModelName,
		},
		Metadata: traceMetadata,
	})
	ctx = traceCtx
	c.Request = c.Request.WithContext(traceCtx)

	// Content-safety check via CheckImagePrompts (stays in Execute).
	result, err := h.handler.modComponent.CheckImagePrompts(ctx, promptText, nsUUID)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "moderation_error", Message: "failed to check image prompts: " + err.Error(), Type: "internal_error",
		}})
		return nil
	}
	if result != nil && result.IsSensitive {
		finishModalGenerationTraceWithError(generationRecorder, ErrSensitiveContent, types.TraceErrSensitivePrompt)
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "content_policy_violation", Message: "Input data may contain inappropriate content.", Type: "invalid_request_error",
		}})
		return nil
	}

	if parsed.IsEdit {
		return h.executeEditImage(c, ctx, nsUUID, meta.Model, meta.UserID, mt, parsed, generationRecorder, apikey)
	}
	return h.executeGenerateImage(c, ctx, nsUUID, meta.Model, meta.UserID, mt, parsed.GenerateReq, generationRecorder, apikey)
}

func (h *imagePipelineHandler) executeGenerateImage(c *gin.Context, ctx context.Context, nsUUID, modelID, userID string, mt *types.ModelTarget, req *types.ImageGenerationRequest, generationRecorder llmtrace.GenerationRecorder, apikey string) error {
	adapter := h.handler.t2iRegistry.GetAdapter(mt.Model)
	if adapter == nil {
		finishModalGenerationTraceWithError(generationRecorder, fmt.Errorf("no adapter for model '%s'", modelID), types.TraceErrUpstreamUnavailable)
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "unsupported_model", Message: fmt.Sprintf("no adapter for model '%s'", modelID), Type: "invalid_request_error",
		}})
		return nil
	}

	typesReq := types.ImageGenerationRequest{
		ImageGenerateParams: req.ImageGenerateParams,
		RawJSON:             req.RawJSON,
	}
	typesReq.Model = mt.ModelName
	bodyBytes, err := adapter.TransformRequest(ctx, typesReq)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		slog.ErrorContext(ctx, "failed to transform image request", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "internal_error", Message: err.Error(), Type: "internal_error",
		}})
		return nil
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	c.Request.ContentLength = int64(len(bodyBytes))
	for k, v := range adapter.GetHeaders(mt.Model, &typesReq) {
		c.Request.Header.Set(k, v)
	}

	rp, err := proxy.NewReverseProxy(mt.Target)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		httpbase.ServerError(c, err)
		return nil
	}
	slog.InfoContext(ctx, "proxy image generation request to model target",
		slog.Any("target", mt.Target), slog.Any("host", mt.Host),
		slog.Any("user", userID), slog.Any("model_name", mt.ModelName))

	imageCounter := token.NewImageUsageCounter()
	imageCounter.SetRequestDetails(string(req.Size), req.N.Or(1))
	responseFormat := string(req.ResponseFormat)
	if responseFormat == "" {
		responseFormat = "url"
	}
	imageWrapper := wrapper.NewImageGeneration(c.Writer, adapter, h.handler.modComponent, h.handler.config.AIGateway.SensitiveDefaultImg, imageCounter, responseFormat, string(req.Size), string(req.OutputFormat), h.handler.storage, h.handler.config.S3.Bucket)
	var w http.ResponseWriter = imageWrapper

	proxyToApi := ""
	if mt.Model.Endpoint != "" {
		uri, err := url.ParseRequestURI(mt.Model.Endpoint)
		if err != nil {
			slog.WarnContext(ctx, "endpoint has wrong struct", slog.String("model", mt.ModelName))
		} else {
			proxyToApi = uri.Path
			if proxyToApi == "" {
				proxyToApi = "/"
			}
		}
	}

	if err := applyModelAuthHeaders(c.Request.Header, mt.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head", slog.String("model", mt.ModelName), slog.Any("error", err))
	}

	rp.ServeHTTP(w, c.Request, proxyToApi, mt.Host)
	h.finishImageTrace(c, ctx, nsUUID, apikey, mt, imageWrapper, imageCounter, generationRecorder)
	return nil
}

func (h *imagePipelineHandler) executeEditImage(c *gin.Context, ctx context.Context, nsUUID, modelID, userID string, mt *types.ModelTarget, parsed *imageParsedBody, generationRecorder llmtrace.GenerationRecorder, apikey string) error {
	body, contentType, err := rebuildImageEditMultipartBody(parsed.Form, mt.ModelName)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		slog.ErrorContext(ctx, "failed to rebuild image edit multipart request", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "internal_error", Message: err.Error(), Type: "internal_error",
		}})
		return nil
	}

	c.Request.Body = body
	c.Request.ContentLength = -1
	c.Request.Header.Set("Content-Type", contentType)
	c.Request.Header.Del("Content-Length")

	rp, err := proxy.NewReverseProxy(mt.Target)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		httpbase.ServerError(c, err)
		return nil
	}
	slog.InfoContext(ctx, "proxy image edit request to model target",
		slog.Any("target", mt.Target), slog.Any("host", mt.Host),
		slog.Any("user", userID), slog.Any("model_name", mt.ModelName))

	adapter := text2image.NewOpenAICompatibleAdapter()
	imageCounter := token.NewImageUsageCounter()
	imageWrapper := wrapper.NewImageGeneration(
		c.Writer, adapter, h.handler.modComponent, h.handler.config.AIGateway.SensitiveDefaultImg,
		imageCounter, imageEditResponseFormat(parsed.Form), c.PostForm("size"), c.PostForm("output_format"),
		h.handler.storage, h.handler.config.S3.Bucket,
	)
	var w http.ResponseWriter = imageWrapper

	proxyToApi := imageEditProxyPath(mt.Model.Endpoint)
	if err := applyModelAuthHeaders(c.Request.Header, mt.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head", slog.String("model", mt.ModelName), slog.Any("error", err))
	}

	rp.ServeHTTP(w, c.Request, proxyToApi, mt.Host)
	h.finishImageTrace(c, ctx, nsUUID, apikey, mt, imageWrapper, imageCounter, generationRecorder)
	return nil
}

// finishImageTrace handles Finalize + async usage recording shared by both
// generate and edit image.
func (h *imagePipelineHandler) finishImageTrace(c *gin.Context, ctx context.Context, nsUUID, apikey string, mt *types.ModelTarget, imageWrapper *wrapper.ImageGeneration, imageCounter *token.ImageUsageCounter, generationRecorder llmtrace.GenerationRecorder) {
	if err := imageWrapper.Finalize(); err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamError)
		slog.ErrorContext(ctx, "failed to finalize image response", slog.Any("error", err))
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(ctx, "panic in image usage recording", slog.Any("panic", r))
			}
		}()
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()

		var usage *token.Usage
		if isSuccessfulStatus(imageWrapper.StatusCode()) && imageCounter != nil {
			var usageErr error
			usage, usageErr = imageCounter.Usage(usageCtx)
			if usageErr != nil {
				slog.ErrorContext(usageCtx, "failed to get image token usage", slog.Any("error", usageErr))
			}
		}
		if generationRecorder != nil {
			metadata := map[string]any{}
			if imageResp := imageWrapper.Response(); imageResp != nil {
				metadata[llmtrace.TraceMetadataKeyImageOutputCount] = len(imageResp.Data)
			}
			recordModalGenerationTraceCompletion(modalTraceCompletionInput{
				Recorder:   generationRecorder,
				Provider:   mt.Model.Provider,
				Model:      mt.ModelName,
				Usage:      usage,
				StatusCode: imageWrapper.StatusCode(),
				Metadata:   metadata,
			})
			generationRecorder.End()
		}
		if isSuccessfulStatus(imageWrapper.StatusCode()) && usage != nil {
			if err := h.handler.openaiComponent.RecordUsageFromTokenUsage(usageCtx, nsUUID, mt.Model, mt.ModelName, usage, apikey); err != nil {
				slog.ErrorContext(usageCtx, "failed to record image usage", slog.Any("error", err))
			}
		}
	}()
}

// --- Error handling ---

func (h *imagePipelineHandler) HandlePlanError(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan, err error) {
	if pt := plan.GetPreflightTracer(c); pt != nil {
		pt.RecordError(err, "plan_error")
		plan.SetPreflightTracer(c, nil)
	}

	frontendURL := ""
	if h.handler.config != nil {
		frontendURL = h.handler.config.Frontend.URL
	}
	handleOpenAIPlanError(c, meta, p, err, frontendURL)
}


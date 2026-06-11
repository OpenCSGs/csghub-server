package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/http/response/wrapper"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/common/utils/trace"
)

// GenerateImage godoc
// @Security     ApiKey
// @Summary      Generate image from text prompt
// @Description  Generates images based on a text prompt
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        request body  ImageGenerationRequest true "Image generation request"
// @Success      200  {object}  types.ImageGenerationResponse "OK"
// @Failure      400  {object}  error "Bad request or sensitive input"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/images/generations [post]
func (h *OpenAIHandlerImpl) GenerateImage(c *gin.Context) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	apikey := httpbase.GetAccessToken(c)
	ctx := c.Request.Context()
	requestID := trace.GetTraceIDInGinContext(c)
	ctx, preflight := startPreflightTrace(ctx, preflightTraceStart{
		API:       c.FullPath(),
		RequestID: requestID,
		UserID:    nsUUID,
	})
	c.Request = c.Request.WithContext(ctx)

	var req ImageGenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		preflight.RecordError(err, "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: err.Error(), Type: "invalid_request_error",
		}})
		return
	}
	if req.Prompt == "" || req.Model == "" {
		preflight.RecordError(fmt.Errorf("model and prompt cannot be empty"), "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "Model and prompt cannot be empty", Type: "invalid_request_error",
		}})
		return
	}

	modelID := req.Model
	modelTarget, err := h.resolveModelTarget(ctx, username, modelID, c.Request.Header)
	if err != nil {
		preflight.RecordError(err, "model_resolve")
		handleModelTargetError(c, ctx, modelID, "failed to get model target address", err)
		return
	}
	preflight.SetTargetModel(modelID, modelTarget)
	preflight.End()

	traceCtx, generationRecorder := h.startModalGenerationTrace(ctx, modalTraceStartInput{
		API:           c.FullPath(),
		OperationName: modalTraceOperationGenerateContent,
		OutputType:    modalTraceOutputImage,
		RequestID:     requestID,
		NSUUID:        nsUUID,
		ModelID:       modelID,
		ModelTarget:   modelTarget,
		Metadata: map[string]any{
			llmtrace.TraceMetadataKeyImageSize:           string(req.Size),
			llmtrace.TraceMetadataKeyImageQuality:        string(req.Quality),
			llmtrace.TraceMetadataKeyImageResponseFormat: string(req.ResponseFormat),
			llmtrace.TraceMetadataKeyImageOutputFormat:   string(req.OutputFormat),
			llmtrace.TraceMetadataKeyImageN:              req.N,
		},
	})
	ctx = traceCtx
	c.Request = c.Request.WithContext(traceCtx)

	adapter := h.t2iRegistry.GetAdapter(modelTarget.Model)
	if adapter == nil {
		finishModalGenerationTraceWithError(generationRecorder, fmt.Errorf("no adapter for model '%s'", modelID), types.TraceErrUpstreamUnavailable)
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "unsupported_model", Message: fmt.Sprintf("no adapter for model '%s'", modelID), Type: "invalid_request_error",
		}})
		return
	}

	if err := h.openaiComponent.CheckBalance(ctx, nsUUID); err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrInsufficientBalance)
		h.handleInsufficientBalance(c, false, nsUUID, modelID, err)
		return
	}

	typesReq := types.ImageGenerationRequest{
		ImageGenerateParams: req.ImageGenerateParams,
		RawJSON:             req.RawJSON,
	}
	typesReq.Model = modelTarget.ModelName
	bodyBytes, err := adapter.TransformRequest(ctx, typesReq)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		slog.ErrorContext(ctx, "failed to transform image request", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "internal_error", Message: err.Error(), Type: "internal_error",
		}})
		return
	}

	result, err := h.modComponent.CheckImagePrompts(ctx, req.Prompt, nsUUID)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "moderation_error", Message: "failed to check image prompts: " + err.Error(), Type: "internal_error",
		}})
		return
	}
	if result != nil && result.IsSensitive {
		finishModalGenerationTraceWithError(generationRecorder, ErrSensitiveContent, types.TraceErrSensitivePrompt)
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "content_policy_violation", Message: "Input data may contain inappropriate content.", Type: "invalid_request_error",
		}})
		return
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	c.Request.ContentLength = int64(len(bodyBytes))
	for k, v := range adapter.GetHeaders(modelTarget.Model, &typesReq) {
		c.Request.Header.Set(k, v)
	}

	rp, err := proxy.NewReverseProxy(modelTarget.Target)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		httpbase.ServerError(c, err)
		return
	}
	slog.InfoContext(ctx, "proxy image generation request to model target", slog.Any("target", modelTarget.Target), slog.Any("host", modelTarget.Host),
		slog.Any("user", username), slog.Any("model_name", modelTarget.ModelName))

	imageCounter := token.NewImageUsageCounter()
	imageCounter.SetRequestDetails(string(req.Size), req.N.Or(1))
	responseFormat := string(req.ResponseFormat)
	if responseFormat == "" {
		responseFormat = "url"
	}
	imageWrapper := wrapper.NewImageGeneration(c.Writer, adapter, h.modComponent, h.config.AIGateway.SensitiveDefaultImg, imageCounter, responseFormat, string(req.Size), string(req.OutputFormat), h.storage, h.config.S3.Bucket)
	var w http.ResponseWriter = imageWrapper

	proxyToApi := ""
	if modelTarget.Model.Endpoint != "" {
		uri, err := url.ParseRequestURI(modelTarget.Model.Endpoint)
		if err != nil {
			slog.WarnContext(ctx, "endpoint has wrong struct", slog.String("model", modelTarget.ModelName))
		} else {
			proxyToApi = uri.Path
			if proxyToApi == "" {
				proxyToApi = "/"
			}
		}
	}

	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}

	rp.ServeHTTP(w, c.Request, proxyToApi, modelTarget.Host)

	if err := imageWrapper.Finalize(); err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamError)
		slog.ErrorContext(ctx, "failed to finalize image response", slog.Any("error", err))
		return
	}
	go func() {
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
		defer cancel()

		var usage *token.Usage
		if imageWrapper.StatusCode() < http.StatusBadRequest && imageCounter != nil {
			var usageErr error
			usage, usageErr = imageCounter.Usage(usageCtx)
			if usageErr != nil {
				slog.ErrorContext(usageCtx, "failed to get image generation token usage", slog.Any("error", usageErr))
			}
		}
		if generationRecorder != nil {
			metadata := map[string]any{}
			if imageResp := imageWrapper.Response(); imageResp != nil {
				metadata[llmtrace.TraceMetadataKeyImageOutputCount] = len(imageResp.Data)
			}
			recordModalGenerationTraceCompletion(modalTraceCompletionInput{
				Recorder:   generationRecorder,
				Provider:   modelTarget.Model.Provider,
				Model:      modelTarget.ModelName,
				Usage:      usage,
				StatusCode: imageWrapper.StatusCode(),
				Metadata:   metadata,
			})
			generationRecorder.End()
		}
		if imageWrapper.StatusCode() < http.StatusBadRequest && usage != nil {
			if err := h.openaiComponent.RecordUsageFromTokenUsage(usageCtx, nsUUID, modelTarget.Model, modelTarget.ModelName, usage, apikey); err != nil {
				slog.ErrorContext(usageCtx, "failed to record image usage", slog.Any("error", err))
			}
		}
	}()
}

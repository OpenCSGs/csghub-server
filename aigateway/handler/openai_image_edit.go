package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/component/adapter/text2image"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/http/response/wrapper"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/common/utils/trace"
)

const maxImageEditMultipartMemory = 128 << 20

// EditImage godoc
// @Security     ApiKey
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
func (h *OpenAIHandlerImpl) EditImage(c *gin.Context) {
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

	if err := c.Request.ParseMultipartForm(maxImageEditMultipartMemory); err != nil {
		preflight.RecordError(err, "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: err.Error(), Type: "invalid_request_error",
		}})
		return
	}

	modelID := c.PostForm("model")
	prompt := c.PostForm("prompt")
	if modelID == "" || prompt == "" {
		preflight.RecordError(fmt.Errorf("model and prompt cannot be empty"), "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "Model and prompt cannot be empty", Type: "invalid_request_error",
		}})
		return
	}
	if !hasMultipartFile(c.Request.MultipartForm, "image") {
		preflight.RecordError(fmt.Errorf("image is required"), "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "Image is required", Type: "invalid_request_error",
		}})
		return
	}

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
			llmtrace.TraceMetadataKeyImageSize:           c.PostForm("size"),
			llmtrace.TraceMetadataKeyImageResponseFormat: imageEditResponseFormat(c.Request.MultipartForm),
			llmtrace.TraceMetadataKeyImageOutputFormat:   c.PostForm("output_format"),
		},
	})
	ctx = traceCtx
	c.Request = c.Request.WithContext(traceCtx)

	if !modelTarget.Model.SkipBalance() {
		if err := h.openaiComponent.CheckBalance(ctx, nsUUID); err != nil {
			finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrInsufficientBalance)
			h.handleInsufficientBalance(c, false, nsUUID, modelID, err)
			return
		}
	}

	result, err := h.modComponent.CheckImagePrompts(ctx, prompt, nsUUID)
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

	body, contentType, err := rebuildImageEditMultipartBody(c.Request.MultipartForm, modelTarget.ModelName)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		slog.ErrorContext(ctx, "failed to rebuild image edit multipart request", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "internal_error", Message: err.Error(), Type: "internal_error",
		}})
		return
	}

	c.Request.Body = body
	c.Request.ContentLength = -1
	c.Request.Header.Set("Content-Type", contentType)
	c.Request.Header.Del("Content-Length")

	rp, err := proxy.NewReverseProxy(modelTarget.Target)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		httpbase.ServerError(c, err)
		return
	}
	slog.InfoContext(ctx, "proxy image edit request to model target", slog.Any("target", modelTarget.Target), slog.Any("host", modelTarget.Host),
		slog.Any("user", username), slog.Any("model_name", modelTarget.ModelName))

	adapter := text2image.NewOpenAICompatibleAdapter()
	imageCounter := token.NewImageUsageCounter()
	imageWrapper := wrapper.NewImageGeneration(
		c.Writer,
		adapter,
		h.modComponent,
		h.config.AIGateway.SensitiveDefaultImg,
		imageCounter,
		imageEditResponseFormat(c.Request.MultipartForm),
		c.PostForm("size"),
		c.PostForm("output_format"),
		h.storage,
		h.config.S3.Bucket,
	)
	var w http.ResponseWriter = imageWrapper

	proxyToApi := imageEditProxyPath(modelTarget.Model.Endpoint)
	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}

	rp.ServeHTTP(w, c.Request, proxyToApi, modelTarget.Host)

	if err := imageWrapper.Finalize(); err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamError)
		slog.ErrorContext(ctx, "failed to finalize image edit response", slog.Any("error", err))
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
				slog.ErrorContext(usageCtx, "failed to get image edit token usage", slog.Any("error", usageErr))
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
				slog.ErrorContext(usageCtx, "failed to record image edit usage", slog.Any("error", err))
			}
		}
	}()
}

func hasMultipartFile(form *multipart.Form, key string) bool {
	if form == nil {
		return false
	}
	return len(form.File[key]) > 0
}

func imageEditResponseFormat(form *multipart.Form) string {
	if form != nil {
		if values := form.Value["response_format"]; len(values) > 0 && strings.TrimSpace(values[0]) != "" {
			return strings.TrimSpace(values[0])
		}
	}
	return "b64_json"
}

func rebuildImageEditMultipartBody(form *multipart.Form, modelName string) (io.ReadCloser, string, error) {
	return rewriteMultipartModelStreamWithOptions(form, modelName, multipartRewriteOptions{
		defaultFields: map[string]string{
			"response_format": "b64_json",
		},
		normalizeFields: map[string]func(string) string{
			"response_format": func(value string) string {
				if strings.TrimSpace(value) == "" {
					return "b64_json"
				}
				return value
			},
		},
	})
}

func imageEditProxyPath(endpoint string) string {
	if endpoint == "" {
		return ""
	}
	uri, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return ""
	}
	if uri.Path == "" {
		return "/"
	}
	return uri.Path
}

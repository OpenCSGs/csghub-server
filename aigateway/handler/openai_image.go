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
	"opencsg.com/csghub-server/aigateway/http/response/wrapper"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
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

	var req ImageGenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: err.Error(), Type: "invalid_request_error",
		}})
		return
	}
	if req.Prompt == "" || req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "Model and prompt cannot be empty", Type: "invalid_request_error",
		}})
		return
	}

	modelID := req.Model
	modelTarget, err := h.resolveModelTarget(ctx, username, modelID, c.Request.Header)
	if err != nil {
		handleModelTargetError(c, ctx, modelID, "failed to get model target address", err)
		return
	}

	adapter := h.t2iRegistry.GetAdapter(modelTarget.Model)
	if adapter == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "unsupported_model", Message: fmt.Sprintf("no adapter for model '%s'", modelID), Type: "invalid_request_error",
		}})
		return
	}

	if err := h.openaiComponent.CheckBalance(ctx, nsUUID); err != nil {
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
		slog.ErrorContext(ctx, "failed to transform image request", slog.Any("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "internal_error", Message: err.Error(), Type: "internal_error",
		}})
		return
	}

	result, err := h.modComponent.CheckImagePrompts(ctx, req.Prompt, nsUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "moderation_error", Message: "failed to check image prompts: " + err.Error(), Type: "internal_error",
		}})
		return
	}
	if result != nil && result.IsSensitive {
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

	rp, _ := proxy.NewReverseProxy(modelTarget.Target)
	slog.InfoContext(ctx, "proxy image generation request to model target", slog.Any("target", modelTarget.Target), slog.Any("host", modelTarget.Host),
		slog.Any("user", username), slog.Any("model_name", modelTarget.ModelName))

	imageCounter := token.NewImageUsageCounter()
	responseFormat := string(req.ResponseFormat)
	if responseFormat == "" {
		responseFormat = "url"
	}
	imageWrapper := wrapper.NewImageGeneration(c.Writer, adapter, h.modComponent, h.config.AIGateway.SensitiveDefaultImg, imageCounter, responseFormat, h.storage, h.config.S3.Bucket)
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
		slog.ErrorContext(ctx, "failed to finalize image response", slog.Any("error", err))
	}
	go func() {
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
		defer cancel()
		if err := h.openaiComponent.RecordUsage(usageCtx, nsUUID, modelTarget.Model, imageCounter, apikey); err != nil {
			slog.ErrorContext(usageCtx, "failed to record image usage", slog.Any("error", err))
		}
	}()
}

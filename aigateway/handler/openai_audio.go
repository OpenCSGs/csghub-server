package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
)

// Transcription godoc
// @Security     ApiKey
// @Summary      Transcribe audio to text
// @Description  Sends an OpenAI-compatible multipart audio transcription request to the backend model
// @Tags         AIGateway
// @Accept       multipart/form-data
// @Produce      json
// @Param        model formData string true "Model ID"
// @Param        file formData file true "Audio file"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  error "Bad request"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/audio/transcriptions [post]
func (h *OpenAIHandlerImpl) Transcription(c *gin.Context) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	apikey := httpbase.GetAccessToken(c)
	ctx := c.Request.Context()

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code:    "invalid_request_error",
			Message: "invalid multipart form: " + err.Error(),
			Type:    "invalid_request_error",
		}})
		return
	}
	if form == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code:    "invalid_request_error",
			Message: "request must be multipart/form-data",
			Type:    "invalid_request_error",
		}})
		return
	}

	modelID := strings.TrimSpace(firstMultipartValue(form, "model"))
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code:    "invalid_request_error",
			Message: "Model cannot be empty",
			Type:    "invalid_request_error",
		}})
		return
	}
	if len(form.File["file"]) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code:    "invalid_request_error",
			Message: "File cannot be empty",
			Type:    "invalid_request_error",
		}})
		return
	}

	modelTarget, err := h.resolveModelTarget(ctx, username, modelID, c.Request.Header)
	if err != nil {
		handleModelTargetError(c, ctx, modelID, "failed to get transcription target address", err)
		return
	}

	if !modelTarget.Model.SkipBalance() {
		if err := h.openaiComponent.CheckBalance(ctx, nsUUID); err != nil {
			h.handleInsufficientBalance(c, false, nsUUID, modelID, err)
			return
		}
	}

	body, contentType := rewriteMultipartModelStream(form, modelTarget.ModelName)
	c.Request.Body = body
	c.Request.ContentLength = -1
	c.Request.Header.Set("Content-Type", contentType)
	c.Request.Header.Del("Content-Length")

	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}

	rp, err := proxy.NewReverseProxy(modelTarget.Target, proxy.WithoutAcceptEncoding())
	if err != nil {
		slog.ErrorContext(ctx, "failed to create reverse proxy", slog.Any("error", err))
		c.String(http.StatusInternalServerError, fmt.Errorf("failed to create reverse proxy:%w", err).Error())
		return
	}

	proxyToApi := ""
	if modelTarget.Model.Endpoint != "" {
		uri, err := url.ParseRequestURI(modelTarget.Model.Endpoint)
		if err != nil {
			slog.WarnContext(ctx, "endpoint has wrong struct", slog.String("model", modelTarget.ModelName))
		} else {
			proxyToApi = uri.Path
		}
	}

	slog.InfoContext(ctx, "proxy audio transcription request to model endpoint", slog.Any("target", modelTarget.Target), slog.Any("host", modelTarget.Host), slog.Any("user", username), slog.Any("model_id", modelID), slog.Any("model_name", modelTarget.ModelName))

	audioCounter := token.NewAudioUsageCounter(token.NewTokenizerImpl(modelTarget.Target, modelTarget.Host, modelTarget.ModelName, modelTarget.Model.ImageID, modelTarget.Model.Provider))
	w := NewResponseWriterWrapperAudio(c.Writer, audioCounter)
	rp.ServeHTTP(w, c.Request, proxyToApi, modelTarget.Host)

	go func() {
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
		defer cancel()

		if err := h.openaiComponent.RecordUsage(usageCtx, nsUUID, modelTarget.Model, modelTarget.ModelName, audioCounter, apikey); err != nil {
			slog.ErrorContext(usageCtx, "failed to record audio transcription usage", slog.Any("error", err))
		}
	}()
}

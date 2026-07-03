package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/common/utils/trace"
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
	requestID := trace.GetTraceIDInGinContext(c)
	ctx, preflight := startPreflightTrace(ctx, preflightTraceStart{
		API:       c.FullPath(),
		RequestID: requestID,
		UserID:    nsUUID,
	})
	c.Request = c.Request.WithContext(ctx)

	form, err := c.MultipartForm()
	if err != nil {
		preflight.RecordError(err, "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code:    "invalid_request_error",
			Message: "invalid multipart form: " + err.Error(),
			Type:    "invalid_request_error",
		}})
		return
	}
	if form == nil {
		preflight.RecordError(fmt.Errorf("request must be multipart/form-data"), "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code:    "invalid_request_error",
			Message: "request must be multipart/form-data",
			Type:    "invalid_request_error",
		}})
		return
	}

	modelID := strings.TrimSpace(firstMultipartValue(form, "model"))
	if modelID == "" {
		preflight.RecordError(fmt.Errorf("model cannot be empty"), "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code:    "invalid_request_error",
			Message: "Model cannot be empty",
			Type:    "invalid_request_error",
		}})
		return
	}
	if len(form.File["file"]) == 0 {
		preflight.RecordError(fmt.Errorf("file cannot be empty"), "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code:    "invalid_request_error",
			Message: "File cannot be empty",
			Type:    "invalid_request_error",
		}})
		return
	}

	isStream := strings.EqualFold(firstMultipartValue(form, "stream"), "true")

	modelTarget, err := h.resolveModelTarget(ctx, username, modelID, c.Request.Header)
	if err != nil {
		preflight.RecordError(err, "model_resolve")
		handleModelTargetError(c, ctx, modelID, "failed to get transcription target address", err)
		return
	}
	preflight.SetTargetModel(modelID, modelTarget)
	preflight.End()

	traceCtx, generationRecorder := h.startModalGenerationTrace(ctx, modalTraceStartInput{
		API:           c.FullPath(),
		OperationName: modalTraceOperationGenerateContent,
		OutputType:    modalTraceOutputText,
		RequestID:     requestID,
		NSUUID:        nsUUID,
		ModelID:       modelID,
		ModelTarget:   modelTarget,
		Metadata: map[string]any{
			"aigateway.audio.response_format": firstMultipartValue(form, "response_format"),
			"aigateway.audio.language":        firstMultipartValue(form, "language"),
		},
	})
	ctx = traceCtx
	c.Request = c.Request.WithContext(traceCtx)

	if !modelTarget.Model.SkipBalance() {
		if err := h.openaiComponent.CheckBalance(ctx, nsUUID); err != nil {
			finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrInsufficientBalance)
			if isStream {
				c.Writer.Header().Set("Content-Type", "text/event-stream")
			}
			h.handleInsufficientBalance(c, isStream, nsUUID, modelID, err)
			return
		}
	}

	options := multipartRewriteOptions{
		defaultFields: map[string]string{
			"stream": strconv.FormatBool(isStream),
		},
	}
	body, contentType, _ := rewriteMultipartModelStreamWithOptions(form, modelTarget.ModelName, options)
	c.Request.Body = body
	c.Request.ContentLength = -1
	c.Request.Header.Set("Content-Type", contentType)
	c.Request.Header.Del("Content-Length")

	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}

	rp, err := proxy.NewReverseProxy(modelTarget.Target, proxy.WithoutAcceptEncoding())
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
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
	w := NewResponseWriterWrapperAudio(c.Writer, audioCounter, isStream)
	rp.ServeHTTP(w, c.Request, proxyToApi, modelTarget.Host)

	go func() {
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
		defer cancel()

		var usage *token.Usage
		if w.StatusCode() < http.StatusBadRequest {
			var usageErr error
			usage, usageErr = audioCounter.Usage(usageCtx)
			if usageErr != nil {
				slog.ErrorContext(usageCtx, "failed to get audio transcription token usage", slog.Any("error", usageErr))
			}
		}
		if generationRecorder != nil {
			metadata := map[string]any{}
			if durationSeconds, ok := w.DurationSeconds(); ok {
				metadata[llmtrace.TraceMetadataKeyAudioDurationSeconds] = durationSeconds
			}
			recordModalGenerationTraceCompletion(modalTraceCompletionInput{
				Recorder:   generationRecorder,
				Provider:   modelTarget.Model.Provider,
				Model:      modelTarget.ModelName,
				Usage:      usage,
				StatusCode: w.StatusCode(),
				Metadata:   metadata,
			})
			generationRecorder.End()
		}

		if w.StatusCode() < http.StatusBadRequest && usage != nil {
			if err := h.openaiComponent.RecordUsageFromTokenUsage(usageCtx, nsUUID, modelTarget.Model, modelTarget.ModelName, usage, apikey); err != nil {
				slog.ErrorContext(usageCtx, "failed to record audio transcription usage", slog.Any("error", err))
			}
		}
	}()
}

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
	audioadapter "opencsg.com/csghub-server/aigateway/component/adapter/audio"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/common/utils/trace"
)

const (
	audioSTTKindTranscription = "transcription"
	audioSTTKindTranslation   = "translation"

	audioTranscriptionsPathSuffix = "/audio/transcriptions"
	audioTranslationsPathSuffix   = "/audio/translations"
	audioTranslationsDefaultPath  = "/v1/audio/translations"
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
	h.handleAudioSTT(c, audioSTTKindTranscription)
}

// Translation godoc
// @Security     ApiKey
// @Summary      Translate audio to English text
// @Description  Sends an OpenAI-compatible multipart audio translation request to the backend model. Translates speech in any language into English.
// @Tags         AIGateway
// @Accept       multipart/form-data
// @Produce      json
// @Param        model formData string true "Model ID"
// @Param        file formData file true "Audio file"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  error "Bad request"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/audio/translations [post]
func (h *OpenAIHandlerImpl) Translation(c *gin.Context) {
	h.handleAudioSTT(c, audioSTTKindTranslation)
}

func (h *OpenAIHandlerImpl) handleAudioSTT(c *gin.Context, kind string) {
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
	SetMetricsModelTarget(SetMetricsModelParams{
		C:           c,
		ModelID:     modelID,
		ModelTarget: modelTarget,
		IsStream:    isStream,
	})
	if err != nil {
		preflight.RecordError(err, "model_resolve")
		handleModelTargetError(c, ctx, modelID, fmt.Sprintf("failed to get %s target address", kind), err)
		return
	}
	preflight.SetTargetModel(modelID, modelTarget)
	preflight.End()

	traceMetadata := map[string]any{
		"aigateway.audio.response_format": firstMultipartValue(form, "response_format"),
		"aigateway.audio.kind":            kind,
	}
	if kind == audioSTTKindTranscription {
		traceMetadata["aigateway.audio.language"] = firstMultipartValue(form, "language")
	} else {
		traceMetadata["aigateway.audio.language"] = "english"
	}

	traceCtx, generationRecorder := h.startModalGenerationTrace(ctx, modalTraceStartInput{
		API:           c.FullPath(),
		OperationName: modalTraceOperationGenerateContent,
		OutputType:    modalTraceOutputText,
		RequestID:     requestID,
		NSUUID:        nsUUID,
		ModelID:       modelID,
		ModelTarget:   modelTarget,
		Metadata:      traceMetadata,
	})
	ctx = traceCtx
	c.Request = c.Request.WithContext(traceCtx)

	if !modelTarget.Model.SkipBalance() {
		if err := h.openaiComponent.CheckBalance(ctx, nsUUID); err != nil {
			finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrInsufficientBalance)
			h.handleInsufficientBalance(c, isStream, nsUUID, modelID, err)
			return
		}
	}

	options := multipartRewriteOptions{
		defaultFields: map[string]string{
			"stream": strconv.FormatBool(isStream),
		},
	}
	adapter := h.audioAdapter(modelTarget.Model)
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

	proxyToApi, pathErr := audioSTTProxyPath(kind, modelTarget.Model.Endpoint)
	if pathErr != nil {
		slog.WarnContext(ctx, "endpoint has wrong struct", slog.String("model", modelTarget.ModelName), slog.Any("error", pathErr))
	}
	slog.InfoContext(ctx, fmt.Sprintf("proxy audio %s request to model endpoint", kind),
		slog.Any("target", modelTarget.Target),
		slog.Any("host", modelTarget.Host),
		slog.Any("user", username),
		slog.Any("model_id", modelID),
		slog.Any("model_name", modelTarget.ModelName),
		slog.String("proxy_path", proxyToApi),
	)

	audioCounter := token.NewAudioUsageCounter(token.NewTokenizerImpl(modelTarget.Target, modelTarget.Host, modelTarget.ModelName, modelTarget.Model.ImageID, modelTarget.Model.Provider))
	w := NewResponseWriterWrapperAudio(c.Writer, audioCounter, isStream, adapter)
	rp.ServeHTTP(w, c.Request, proxyToApi, modelTarget.Host)

	go func() {
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
		defer cancel()

		var usage *token.Usage
		if isSuccessfulStatus(w.StatusCode()) {
			var usageErr error
			usage, usageErr = audioCounter.Usage(usageCtx)
			if usageErr != nil {
				slog.ErrorContext(usageCtx, fmt.Sprintf("failed to get audio %s token usage", kind), slog.Any("error", usageErr))
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

		if isSuccessfulStatus(w.StatusCode()) && usage != nil {
			if err := h.openaiComponent.RecordUsageFromTokenUsage(usageCtx, nsUUID, modelTarget.Model, modelTarget.ModelName, usage, apikey); err != nil {
				slog.ErrorContext(usageCtx, fmt.Sprintf("failed to record audio %s usage", kind), slog.Any("error", err))
			}
		}
	}()
}

func audioSTTProxyPath(kind, endpoint string) (string, error) {
	if kind == audioSTTKindTranslation {
		return audioTranslationProxyPath(endpoint)
	}
	return audioTranscriptionProxyPath(endpoint)
}

func audioTranscriptionProxyPath(endpoint string) (string, error) {
	if endpoint == "" {
		return "", nil
	}
	uri, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return "", err
	}
	return uri.Path, nil
}

// audioTranslationProxyPath rewrites a model endpoint that points at
// /audio/transcriptions to /audio/translations so the same Whisper-style
// deployment can serve OpenAI-compatible translations without a separate
// endpoint configuration.
func audioTranslationProxyPath(endpoint string) (string, error) {
	if endpoint == "" {
		return audioTranslationsDefaultPath, nil
	}
	uri, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return audioTranslationsDefaultPath, err
	}
	path := uri.Path
	if path == "" || path == "/" {
		return audioTranslationsDefaultPath, nil
	}
	if strings.HasSuffix(path, audioTranslationsPathSuffix) {
		return path, nil
	}
	if strings.HasSuffix(path, audioTranscriptionsPathSuffix) {
		return strings.TrimSuffix(path, audioTranscriptionsPathSuffix) + audioTranslationsPathSuffix, nil
	}
	return path, nil
}

func (h *OpenAIHandlerImpl) audioAdapter(model *types.Model) audioadapter.Adapter {
	if h != nil && h.audioRegistry != nil {
		if adapter := h.audioRegistry.GetAdapter(model); adapter != nil {
			return adapter
		}
	}
	return audioadapter.NewOpenAICompatibleAdapter()
}

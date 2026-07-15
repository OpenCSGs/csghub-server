package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	commontypes "opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/trace"
)

// Speech godoc
// @Security     ApiKey
// @Summary      Generate speech from text
// @Description  Sends an OpenAI-compatible text-to-speech request to the backend model and returns audio
// @Tags         AIGateway
// @Accept       json
// @Produce      audio/wav
// @Param        request body  SpeechRequest true "Speech generation request"
// @Success      200  {file}  binary "Audio data"
// @Failure      400  {object}  error "Bad request or sensitive input"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/audio/speech [post]
func (h *OpenAIHandlerImpl) Speech(c *gin.Context) {
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

	var req SpeechRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		preflight.RecordError(err, "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: err.Error(), Type: "invalid_request_error",
		}})
		return
	}
	if strings.TrimSpace(req.Model) == "" || strings.TrimSpace(req.Input) == "" {
		preflight.RecordError(fmt.Errorf("model and input cannot be empty"), "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "Model and input cannot be empty", Type: "invalid_request_error",
		}})
		return
	}

	modelID := req.Model
	// SSE is selected by stream=true or stream_format="sse";
	// stream_format="audio" streams raw audio bytes instead.
	isSSE := req.Stream || strings.EqualFold(req.StreamFormat, "sse")

	modelTarget, err := h.resolveModelTarget(ctx, username, modelID, c.Request.Header)
	if err != nil {
		preflight.RecordError(err, "model_resolve")
		handleModelTargetError(c, ctx, modelID, "failed to get speech target address", err)
		return
	}
	if !supportsSpeechTask(modelTarget.Model.Task) {
		err := fmt.Errorf("model '%s' does not support audio generation", modelID)
		preflight.RecordError(err, "model_task_mismatch")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "model_task_mismatch", Message: err.Error(), Type: "invalid_request_error",
		}})
		return
	}
	preflight.SetTargetModel(modelID, modelTarget)
	preflight.End()

	traceCtx, generationRecorder := h.startModalGenerationTrace(ctx, modalTraceStartInput{
		API:           c.FullPath(),
		OperationName: modalTraceOperationGenerateContent,
		OutputType:    modalTraceOutputAudio,
		RequestID:     requestID,
		NSUUID:        nsUUID,
		ModelID:       modelID,
		ModelTarget:   modelTarget,
		Metadata: map[string]any{
			"aigateway.audio.voice":           req.Voice,
			"aigateway.audio.response_format": req.ResponseFormat,
			"aigateway.audio.stream_format":   req.StreamFormat,
		},
	})
	ctx = traceCtx
	c.Request = c.Request.WithContext(traceCtx)

	if !modelTarget.Model.SkipBalance() {
		if err := h.openaiComponent.CheckBalance(ctx, nsUUID); err != nil {
			finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrInsufficientBalance)
			if isSSE {
				c.Writer.Header().Set("Content-Type", "text/event-stream")
			}
			h.handleInsufficientBalance(c, isSSE, nsUUID, modelID, err)
			return
		}
	}

	result, err := h.modComponent.CheckImagePrompts(ctx, req.Input, nsUUID)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "moderation_error", Message: "failed to check speech input: " + err.Error(), Type: "internal_error",
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

	req.Model = modelTarget.ModelName
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "internal_error", Message: err.Error(), Type: "internal_error",
		}})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	c.Request.ContentLength = int64(len(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

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

	slog.InfoContext(ctx, "proxy audio speech request to model endpoint", slog.Any("target", modelTarget.Target), slog.Any("host", modelTarget.Host), slog.Any("user", username), slog.Any("model_id", modelID), slog.Any("model_name", modelTarget.ModelName))

	// Non-streaming TTS responses are binary audio without usage data, so bill
	// by input characters as fallback; SSE responses carry token usage in the
	// terminal speech.audio.done event which overrides the fallback.
	speechCounter := token.NewAudioUsageCounter(nil)
	speechCounter.Text(req.Input)
	w := NewResponseWriterWrapperSpeech(c.Writer, speechCounter)
	rp.ServeHTTP(w, c.Request, proxyToApi, modelTarget.Host)

	go func() {
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
		defer cancel()

		var usage *token.Usage
		if w.StatusCode() < http.StatusBadRequest {
			var usageErr error
			usage, usageErr = speechCounter.Usage(usageCtx)
			if usageErr != nil {
				slog.ErrorContext(usageCtx, "failed to get audio speech token usage", slog.Any("error", usageErr))
			}
		}
		if generationRecorder != nil {
			recordModalGenerationTraceCompletion(modalTraceCompletionInput{
				Recorder:   generationRecorder,
				Provider:   modelTarget.Model.Provider,
				Model:      modelTarget.ModelName,
				Usage:      usage,
				StatusCode: w.StatusCode(),
			})
			generationRecorder.End()
		}

		if w.StatusCode() < http.StatusBadRequest && usage != nil {
			if err := h.openaiComponent.RecordUsageFromTokenUsage(usageCtx, nsUUID, modelTarget.Model, modelTarget.ModelName, usage, apikey); err != nil {
				slog.ErrorContext(usageCtx, "failed to record audio speech usage", slog.Any("error", err))
			}
		}
	}()
}

const (
	// maxSpeechBatchItems bounds the number of items in one batch speech
	// request to protect the moderation service and the backend model.
	maxSpeechBatchItems = 100
	// maxSpeechBatchInputChars bounds the total input text length across all
	// items in one batch speech request.
	maxSpeechBatchInputChars = 100000
)

// SpeechBatch godoc
// @Security     ApiKey
// @Summary      Generate speech for multiple texts in a single request
// @Description  Sends an OpenAI-compatible batch text-to-speech request to the backend model and returns base64-encoded audio results
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        request body  BatchSpeechRequest true "Batch speech generation request"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  error "Bad request or sensitive input"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/audio/speech/batch [post]
func (h *OpenAIHandlerImpl) SpeechBatch(c *gin.Context) {
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

	var req BatchSpeechRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		preflight.RecordError(err, "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: err.Error(), Type: "invalid_request_error",
		}})
		return
	}
	if strings.TrimSpace(req.Model) == "" || len(req.Items) == 0 {
		preflight.RecordError(fmt.Errorf("model and items cannot be empty"), "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "Model and items cannot be empty", Type: "invalid_request_error",
		}})
		return
	}
	if len(req.Items) > maxSpeechBatchItems {
		preflight.RecordError(fmt.Errorf("too many batch items"), "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: fmt.Sprintf("too many items, at most %d items are allowed per batch", maxSpeechBatchItems), Type: "invalid_request_error",
		}})
		return
	}
	modelID := req.Model
	inputTexts := req.InputTexts()
	totalChars := 0
	for _, text := range inputTexts {
		totalChars += len([]rune(text))
	}
	if totalChars > maxSpeechBatchInputChars {
		preflight.RecordError(fmt.Errorf("batch input too long"), "bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: fmt.Sprintf("total input text too long, at most %d characters are allowed per batch", maxSpeechBatchInputChars), Type: "invalid_request_error",
		}})
		return
	}

	modelTarget, err := h.resolveModelTarget(ctx, username, modelID, c.Request.Header)
	if err != nil {
		preflight.RecordError(err, "model_resolve")
		handleModelTargetError(c, ctx, modelID, "failed to get speech batch target address", err)
		return
	}
	if !supportsSpeechTask(modelTarget.Model.Task) {
		err := fmt.Errorf("model '%s' does not support audio generation", modelID)
		preflight.RecordError(err, "model_task_mismatch")
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "model_task_mismatch", Message: err.Error(), Type: "invalid_request_error",
		}})
		return
	}
	preflight.SetTargetModel(modelID, modelTarget)
	preflight.End()

	traceCtx, generationRecorder := h.startModalGenerationTrace(ctx, modalTraceStartInput{
		API:           c.FullPath(),
		OperationName: modalTraceOperationGenerateContent,
		OutputType:    modalTraceOutputAudio,
		RequestID:     requestID,
		NSUUID:        nsUUID,
		ModelID:       modelID,
		ModelTarget:   modelTarget,
		Metadata: map[string]any{
			"aigateway.audio.batch_items": len(req.Items),
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

	result, err := h.modComponent.CheckImagePrompts(ctx, strings.Join(inputTexts, "\n"), nsUUID)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "moderation_error", Message: "failed to check speech input: " + err.Error(), Type: "internal_error",
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

	req.Model = modelTarget.ModelName
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "internal_error", Message: err.Error(), Type: "internal_error",
		}})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	c.Request.ContentLength = int64(len(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

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

	slog.InfoContext(ctx, "proxy audio speech batch request to model endpoint", slog.Any("target", modelTarget.Target), slog.Any("host", modelTarget.Host), slog.Any("user", username), slog.Any("model_id", modelID), slog.Any("model_name", modelTarget.ModelName), slog.Any("items", len(req.Items)))

	// The batch response carries per-item usage which the writer sums; if the
	// response has no usage, bill by input characters of all items.
	speechCounter := token.NewAudioUsageCounter(nil)
	speechCounter.Text(strings.Join(inputTexts, ""))
	w := NewResponseWriterWrapperSpeechBatch(c.Writer, speechCounter)
	rp.ServeHTTP(w, c.Request, speechBatchProxyPath(ctx, modelTarget.Model.Endpoint), modelTarget.Host)

	go func() {
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 3*time.Second)
		defer cancel()

		var usage *token.Usage
		if w.StatusCode() < http.StatusBadRequest {
			var usageErr error
			usage, usageErr = speechCounter.Usage(usageCtx)
			if usageErr != nil {
				slog.ErrorContext(usageCtx, "failed to get audio speech batch token usage", slog.Any("error", usageErr))
			}
		}
		if generationRecorder != nil {
			recordModalGenerationTraceCompletion(modalTraceCompletionInput{
				Recorder:   generationRecorder,
				Provider:   modelTarget.Model.Provider,
				Model:      modelTarget.ModelName,
				Usage:      usage,
				StatusCode: w.StatusCode(),
			})
			generationRecorder.End()
		}

		if w.StatusCode() < http.StatusBadRequest && usage != nil {
			if err := h.openaiComponent.RecordUsageFromTokenUsage(usageCtx, nsUUID, modelTarget.Model, modelTarget.ModelName, usage, apikey); err != nil {
				slog.ErrorContext(usageCtx, "failed to record audio speech batch usage", slog.Any("error", err))
			}
		}
	}()
}

func supportsSpeechTask(task string) bool {
	for _, candidate := range strings.Split(task, ",") {
		switch strings.TrimSpace(candidate) {
		case string(commontypes.TextToSpeech), string(commontypes.TextToAudio):
			return true
		}
	}
	return false
}

// speechBatchProxyPath derives the upstream path for batch speech requests.
// External model endpoints are configured with the single speech API path
// (e.g. https://host/v1/audio/speech), so "/batch" is appended. CSGHub
// serverless endpoints have no path and pass the incoming request path
// (/v1/audio/speech/batch) through unchanged.
func speechBatchProxyPath(ctx context.Context, endpoint string) string {
	if endpoint == "" {
		return ""
	}
	uri, err := url.ParseRequestURI(endpoint)
	if err != nil {
		slog.WarnContext(ctx, "endpoint has wrong struct", slog.String("endpoint", endpoint))
		return ""
	}
	path := uri.Path
	if path == "" || path == "/" {
		return ""
	}
	if strings.HasSuffix(path, "/batch") {
		return path
	}
	return strings.TrimSuffix(path, "/") + "/batch"
}

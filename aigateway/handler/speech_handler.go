package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"encoding/json"
	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
	"net/url"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	commontrace "opencsg.com/csghub-server/common/utils/trace"
)

// SpeechHandlerImpl extends OpenAIHandlerImpl to serve the
// /v1/audio/speech and /v1/audio/speech/batch endpoints through the
// three-stage pipeline.
type SpeechHandlerImpl struct {
	*OpenAIHandlerImpl
	speechPipeline *speechPipelineHandler
	orchestrator   *plan.Orchestrator
}

func NewSpeechHandler(openai *OpenAIHandlerImpl) *SpeechHandlerImpl {
	h := &SpeechHandlerImpl{
		OpenAIHandlerImpl: openai,
		speechPipeline:    &speechPipelineHandler{handler: openai},
	}

	h.orchestrator = newOrchestrator(openai)

	return h
}

// Speech handles POST /v1/audio/speech.
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
func (h *SpeechHandlerImpl) Speech(c *gin.Context) {
	h.orchestrator.Dispatch(c, h.speechPipeline, h.speechPipeline)
}

// SpeechBatch handles POST /v1/audio/speech/batch.
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
func (h *SpeechHandlerImpl) SpeechBatch(c *gin.Context) {
	h.orchestrator.Dispatch(c, h.speechPipeline, h.speechPipeline)
}

// speechPipelineHandler implements plan.MetadataExtractor and
// plan.ProtocolHandler for the /v1/audio/speech and /v1/audio/speech/batch endpoints.
type speechPipelineHandler struct {
	handler *OpenAIHandlerImpl
}

var (
	_ plan.MetadataExtractor = (*speechPipelineHandler)(nil)
	_ plan.ProtocolHandler   = (*speechPipelineHandler)(nil)
)

// speechParsedBody carries the parsed speech request through the pipeline.
// For batch requests, isBatch is true and batchReq is set.
type speechParsedBody struct {
	IsBatch  bool
	Req      *types.SpeechRequest
	BatchReq *types.BatchSpeechRequest
}

// --- Phase 1: Extract ---

func (h *speechPipelineHandler) Extract(c *gin.Context) (*types.RequestMetadata, error) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)

	// Determine if this is a batch request based on the route.
	isBatch := c.FullPath() == "/v1/audio/speech/batch"

	if isBatch {
		var req types.BatchSpeechRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
				Code: "invalid_request_error", Message: err.Error(), Type: "invalid_request_error",
			}})
			return nil, err
		}
		if strings.TrimSpace(req.Model) == "" || len(req.Items) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
				Code: "invalid_request_error", Message: "Model and items cannot be empty", Type: "invalid_request_error",
			}})
			return nil, fmt.Errorf("model and items cannot be empty")
		}
		if len(req.Items) > maxSpeechBatchItems {
			c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
				Code: "invalid_request_error", Message: fmt.Sprintf("too many items, at most %d items are allowed per batch", maxSpeechBatchItems), Type: "invalid_request_error",
			}})
			return nil, fmt.Errorf("too many batch items")
		}
		inputTexts := req.InputTexts()
		totalChars := 0
		for _, text := range inputTexts {
			totalChars += len([]rune(text))
		}
		if totalChars > maxSpeechBatchInputChars {
			c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
				Code: "invalid_request_error", Message: fmt.Sprintf("total input text too long, at most %d characters are allowed per batch", maxSpeechBatchInputChars), Type: "invalid_request_error",
			}})
			return nil, fmt.Errorf("batch input too long")
		}

		return &types.RequestMetadata{
			Protocol:   string(types.ProtocolChat),
			Task:       "speech",
			Model:      req.Model,
			TenantID:   nsUUID,
			UserID:     username,
			APIKeyID:   httpbase.GetAccessToken(c),
			Streaming:  false,
			Headers:    c.Request.Header,
			ParsedBody: &speechParsedBody{IsBatch: true, BatchReq: &req},
		}, nil
	}

	var req types.SpeechRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: err.Error(), Type: "invalid_request_error",
		}})
		return nil, err
	}
	if strings.TrimSpace(req.Model) == "" || strings.TrimSpace(req.Input) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "Model and input cannot be empty", Type: "invalid_request_error",
		}})
		return nil, fmt.Errorf("model and input cannot be empty")
	}

	isSSE := req.Stream || strings.EqualFold(req.StreamFormat, "sse")

	return &types.RequestMetadata{
		Protocol:   string(types.ProtocolChat),
		Task:       "speech",
		Model:      req.Model,
		TenantID:   nsUUID,
		UserID:     username,
		APIKeyID:   httpbase.GetAccessToken(c),
		Streaming:  isSSE,
		Headers:    c.Request.Header,
		ParsedBody: &speechParsedBody{IsBatch: false, Req: &req},
	}, nil
}

// --- Phase 3: Execute ---

func (h *speechPipelineHandler) Execute(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan) error {
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
	parsed := meta.ParsedBody.(*speechParsedBody)

	// Check model task supports speech.
	if !supportsSpeechTask(mt.Model.Task) {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "model_task_mismatch", Message: fmt.Sprintf("model '%s' does not support audio generation", meta.Model), Type: "invalid_request_error",
		}})
		return nil
	}

	var traceMetadata map[string]any
	var inputTextForSafety string
	var marshalReq any
	isSSE := meta.Streaming

	if parsed.IsBatch {
		inputTexts := parsed.BatchReq.InputTexts()
		inputTextForSafety = strings.Join(inputTexts, "\n")
		traceMetadata = map[string]any{
			"aigateway.audio.batch_items": len(parsed.BatchReq.Items),
		}
		marshalReq = parsed.BatchReq
	} else {
		inputTextForSafety = parsed.Req.Input
		traceMetadata = map[string]any{
			"aigateway.audio.voice":           parsed.Req.Voice,
			"aigateway.audio.response_format": parsed.Req.ResponseFormat,
			"aigateway.audio.stream_format":   parsed.Req.StreamFormat,
		}
		marshalReq = parsed.Req
	}

	traceCtx, generationRecorder := h.handler.startModalGenerationTrace(ctx, modalTraceStartInput{
		API:           c.FullPath(),
		OperationName: modalTraceOperationGenerateContent,
		OutputType:    modalTraceOutputAudio,
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

	// Content-safety check via CheckImagePrompts (stays in Execute — bypasses SensitivePolicy gate).
	result, err := h.handler.modComponent.CheckImagePrompts(ctx, inputTextForSafety, nsUUID)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "moderation_error", Message: "failed to check speech input: " + err.Error(), Type: "internal_error",
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

	// Marshal request with resolved model name.
	if parsed.IsBatch {
		parsed.BatchReq.Model = mt.ModelName
	} else {
		parsed.Req.Model = mt.ModelName
	}
	bodyBytes, err := json.Marshal(marshalReq)
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		c.JSON(http.StatusInternalServerError, gin.H{"error": types.Error{
			Code: "internal_error", Message: err.Error(), Type: "internal_error",
		}})
		return nil
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	c.Request.ContentLength = int64(len(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	if err := applyModelAuthHeaders(c.Request.Header, mt.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head", slog.String("model", mt.ModelName), slog.Any("error", err))
	}

	rp, err := proxy.NewReverseProxy(mt.Target, proxy.WithoutAcceptEncoding())
	if err != nil {
		finishModalGenerationTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		slog.ErrorContext(ctx, "failed to create reverse proxy", slog.Any("error", err))
		c.String(http.StatusInternalServerError, fmt.Errorf("failed to create reverse proxy:%w", err).Error())
		return nil
	}

	var proxyToApi string
	if parsed.IsBatch {
		proxyToApi = speechBatchProxyPath(ctx, mt.Model.Endpoint)
		slog.InfoContext(ctx, "proxy audio speech batch request to model endpoint",
			slog.Any("target", mt.Target), slog.Any("host", mt.Host),
			slog.Any("user", meta.UserID), slog.Any("model_id", meta.Model),
			slog.Any("items", len(parsed.BatchReq.Items)))
	} else {
		proxyToApi = ""
		if mt.Model.Endpoint != "" {
			uri, err := url.ParseRequestURI(mt.Model.Endpoint)
			if err != nil {
				slog.WarnContext(ctx, "endpoint has wrong struct", slog.String("model", mt.ModelName))
			} else {
				proxyToApi = uri.Path
			}
		}
		slog.InfoContext(ctx, "proxy audio speech request to model endpoint",
			slog.Any("target", mt.Target), slog.Any("host", mt.Host),
			slog.Any("user", meta.UserID), slog.Any("model_id", meta.Model))
	}

	speechCounter := token.NewAudioUsageCounter(nil)
	speechCounter.Text(inputTextForSafety)

	var w http.ResponseWriter
	if parsed.IsBatch {
		w = NewResponseWriterWrapperSpeechBatch(c.Writer, speechCounter)
	} else {
		w = NewResponseWriterWrapperSpeech(c.Writer, speechCounter)
	}
	rp.ServeHTTP(w, c.Request, proxyToApi, mt.Host)

	// Record status code for non-streaming.
	var statusCode int
	if rw, ok := w.(interface{ StatusCode() int }); ok {
		statusCode = rw.StatusCode()
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(ctx, "panic in speech usage recording", slog.Any("panic", r))
			}
		}()
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()

		var usage *token.Usage
		if isSuccessfulStatus(statusCode) {
			var usageErr error
			usage, usageErr = speechCounter.Usage(usageCtx)
			if usageErr != nil {
				slog.ErrorContext(usageCtx, "failed to get audio speech token usage", slog.Any("error", usageErr))
			}
		}
		if generationRecorder != nil {
			recordModalGenerationTraceCompletion(modalTraceCompletionInput{
				Recorder:   generationRecorder,
				Provider:   mt.Model.Provider,
				Model:      mt.ModelName,
				Usage:      usage,
				StatusCode: statusCode,
			})
			generationRecorder.End()
		}

		if isSuccessfulStatus(statusCode) && usage != nil {
			if err := h.handler.openaiComponent.RecordUsageFromTokenUsage(usageCtx, nsUUID, mt.Model, mt.ModelName, usage, apikey); err != nil {
				slog.ErrorContext(usageCtx, "failed to record audio speech usage", slog.Any("error", err))
			}
		}
	}()

	_ = isSSE // isSSE is used for metrics in the old handler but not critical here
	return nil
}

// --- Error handling ---

func (h *speechPipelineHandler) HandlePlanError(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan, err error) {
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


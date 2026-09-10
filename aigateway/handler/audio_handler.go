package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	commontrace "opencsg.com/csghub-server/common/utils/trace"
)

// AudioHandlerImpl extends OpenAIHandlerImpl to serve the
// /v1/audio/transcriptions endpoint through the three-stage pipeline.
type AudioHandlerImpl struct {
	*OpenAIHandlerImpl
	audioPipeline *audioPipelineHandler
	orchestrator  *plan.Orchestrator
}

func NewAudioHandler(openai *OpenAIHandlerImpl) *AudioHandlerImpl {
	h := &AudioHandlerImpl{
		OpenAIHandlerImpl: openai,
		audioPipeline:     &audioPipelineHandler{handler: openai},
	}

	h.orchestrator = newOrchestrator(openai)

	return h
}

// Transcription handles POST /v1/audio/transcriptions.
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
func (h *AudioHandlerImpl) Transcription(c *gin.Context) {
	h.orchestrator.Dispatch(c, h.audioPipeline, h.audioPipeline)
}

// audioPipelineHandler implements plan.MetadataExtractor and
// plan.ProtocolHandler for the /v1/audio/transcriptions endpoint.
type audioPipelineHandler struct {
	handler *OpenAIHandlerImpl
}

var (
	_ plan.MetadataExtractor = (*audioPipelineHandler)(nil)
	_ plan.ProtocolHandler   = (*audioPipelineHandler)(nil)
)

// --- Phase 1: Extract ---

func (h *audioPipelineHandler) Extract(c *gin.Context) (*types.RequestMetadata, error) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "invalid multipart form: " + err.Error(), Type: "invalid_request_error",
		}})
		return nil, err
	}
	if form == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "request must be multipart/form-data", Type: "invalid_request_error",
		}})
		return nil, fmt.Errorf("request must be multipart/form-data")
	}

	modelID := strings.TrimSpace(firstMultipartValue(form, "model"))
	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "Model cannot be empty", Type: "invalid_request_error",
		}})
		return nil, fmt.Errorf("model cannot be empty")
	}
	if len(form.File["file"]) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": types.Error{
			Code: "invalid_request_error", Message: "File cannot be empty", Type: "invalid_request_error",
		}})
		return nil, fmt.Errorf("file cannot be empty")
	}

	isStream := strings.EqualFold(firstMultipartValue(form, "stream"), "true")

	return &types.RequestMetadata{
		Protocol:   string(types.ProtocolChat),
		Task:       "audio",
		Model:      modelID,
		TenantID:   nsUUID,
		UserID:     username,
		APIKeyID:   httpbase.GetAccessToken(c),
		Streaming:  isStream,
		Headers:    c.Request.Header,
		ParsedBody: form,
	}, nil
}

// --- Phase 3: Execute ---

func (h *audioPipelineHandler) Execute(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan) error {
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
	form := meta.ParsedBody.(*multipart.Form)
	isStream := meta.Streaming

	traceCtx, generationRecorder := h.handler.startModalGenerationTrace(ctx, modalTraceStartInput{
		API:           c.FullPath(),
		OperationName: modalTraceOperationGenerateContent,
		OutputType:    modalTraceOutputText,
		RequestID:     requestID,
		NSUUID:        nsUUID,
		ModelID:       meta.Model,
		ModelTarget:   &resolvedModelTarget{
			Model: mt.Model, Upstream: mt.Upstream, Target: mt.Target, Host: mt.Host, ModelName: mt.ModelName,
		},
		Metadata: map[string]any{
			"aigateway.audio.response_format": firstMultipartValue(form, "response_format"),
			"aigateway.audio.language":        firstMultipartValue(form, "language"),
		},
	})
	ctx = traceCtx
	c.Request = c.Request.WithContext(traceCtx)

	options := multipartRewriteOptions{
		defaultFields: map[string]string{
			"stream": strconv.FormatBool(isStream),
		},
	}
	adapter := h.handler.audioAdapter(mt.Model)
	body, contentType, _ := rewriteMultipartModelStreamWithOptions(form, mt.ModelName, options)
	c.Request.Body = body
	c.Request.ContentLength = -1
	c.Request.Header.Set("Content-Type", contentType)
	c.Request.Header.Del("Content-Length")

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

	proxyToApi := ""
	if mt.Model.Endpoint != "" {
		uri, err := url.ParseRequestURI(mt.Model.Endpoint)
		if err != nil {
			slog.WarnContext(ctx, "endpoint has wrong struct", slog.String("model", mt.ModelName))
		} else {
			proxyToApi = uri.Path
		}
	}

	slog.InfoContext(ctx, "proxy audio transcription request to model endpoint",
		slog.Any("target", mt.Target), slog.Any("host", mt.Host),
		slog.Any("user", meta.UserID), slog.Any("model_id", meta.Model))

	audioCounter := token.NewAudioUsageCounter(token.NewTokenizerImpl(mt.Target, mt.Host, mt.ModelName, mt.Model.ImageID, mt.Model.Provider))
	w := NewResponseWriterWrapperAudio(c.Writer, audioCounter, isStream, adapter)
	rp.ServeHTTP(w, c.Request, proxyToApi, mt.Host)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(ctx, "panic in audio usage recording", slog.Any("panic", r))
			}
		}()
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()

		var usage *token.Usage
		if isSuccessfulStatus(w.StatusCode()) {
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
				Provider:   mt.Model.Provider,
				Model:      mt.ModelName,
				Usage:      usage,
				StatusCode: w.StatusCode(),
				Metadata:   metadata,
			})
			generationRecorder.End()
		}

		if isSuccessfulStatus(w.StatusCode()) && usage != nil {
			if err := h.handler.openaiComponent.RecordUsageFromTokenUsage(usageCtx, nsUUID, mt.Model, mt.ModelName, usage, apikey); err != nil {
				slog.ErrorContext(usageCtx, "failed to record audio transcription usage", slog.Any("error", err))
			}
		}
	}()

	return nil
}

// --- Error handling ---

func (h *audioPipelineHandler) HandlePlanError(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan, err error) {
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


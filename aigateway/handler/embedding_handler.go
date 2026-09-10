package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"encoding/json"
	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
	commontrace "opencsg.com/csghub-server/common/utils/trace"
)

// EmbeddingHandlerImpl extends OpenAIHandlerImpl to serve the
// /v1/embeddings endpoint through the three-stage pipeline.
type EmbeddingHandlerImpl struct {
	*OpenAIHandlerImpl
	embeddingPipeline *embeddingPipelineHandler
	orchestrator      *plan.Orchestrator
}

func NewEmbeddingHandler(openai *OpenAIHandlerImpl) *EmbeddingHandlerImpl {
	h := &EmbeddingHandlerImpl{
		OpenAIHandlerImpl: openai,
		embeddingPipeline: &embeddingPipelineHandler{handler: openai},
	}

	h.orchestrator = newOrchestrator(openai)

	return h
}

// Embedding handles POST /v1/embeddings.
// @Summary      Create embeddings
// @Description  Proxies embedding requests to model endpoints.
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        request body EmbeddingRequest true "Embedding request"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  error "Bad request"
// @Failure      402  {object}  error "Insufficient balance"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/embeddings [post]
func (h *EmbeddingHandlerImpl) Embedding(c *gin.Context) {
	h.orchestrator.Dispatch(c, h.embeddingPipeline, h.embeddingPipeline)
}

// embeddingPipelineHandler implements plan.MetadataExtractor and
// plan.ProtocolHandler for the /v1/embeddings endpoint.
type embeddingPipelineHandler struct {
	handler *OpenAIHandlerImpl
}

var (
	_ plan.MetadataExtractor = (*embeddingPipelineHandler)(nil)
	_ plan.ProtocolHandler   = (*embeddingPipelineHandler)(nil)
)

// --- Phase 1: Extract ---

func (h *embeddingPipelineHandler) Extract(c *gin.Context) (*types.RequestMetadata, error) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)

	var req types.EmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return nil, err
	}
	if req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model cannot be empty"})
		return nil, fmt.Errorf("model cannot be empty")
	}
	if req.Input.OfString.String() == "" &&
		len(req.Input.OfArrayOfStrings) == 0 &&
		len(req.Input.OfArrayOfTokenArrays) == 0 &&
		len(req.Input.OfArrayOfTokens) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input cannot be empty"})
		return nil, fmt.Errorf("input cannot be empty")
	}

	return &types.RequestMetadata{
		Protocol:   string(types.ProtocolChat),
		Task:       "embedding",
		Model:      req.Model,
		TenantID:   nsUUID,
		UserID:     username,
		APIKeyID:   httpbase.GetAccessToken(c),
		Streaming:  false,
		Headers:    c.Request.Header,
		ParsedBody: &req,
	}, nil
}

// --- Phase 3: Execute ---

func (h *embeddingPipelineHandler) Execute(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan) error {
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
	req := meta.ParsedBody.(*types.EmbeddingRequest)

	traceCtx, embeddingRecorder := h.handler.startEmbeddingTrace(
		ctx,
		meta.Model,
		&resolvedModelTarget{
			Model:     mt.Model,
			Upstream:  mt.Upstream,
			Target:    mt.Target,
			Host:      mt.Host,
			ModelName: mt.ModelName,
		},
		req,
		requestID,
		nsUUID,
	)
	ctx = traceCtx
	c.Request = c.Request.WithContext(traceCtx)

	req.Model = mt.ModelName
	data, err := json.Marshal(req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal embedding request", slog.Any("error", err))
		httpbase.ServerError(c, err)
		return nil
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	c.Request.ContentLength = int64(len(data))

	if err := applyModelAuthHeaders(c.Request.Header, mt.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head", slog.String("model", mt.ModelName), slog.Any("error", err))
	}

	slog.InfoContext(ctx, "proxy embedding request to model endpoint",
		slog.Any("target", mt.Target), slog.Any("host", mt.Host),
		slog.Any("user", meta.UserID), slog.Any("model_id", meta.Model))

	proxyToAPI := resolveProxyPathFromModelEndpoint(mt.Model.Endpoint, mt.ModelName)
	rp, err := proxy.NewReverseProxy(mt.Target, proxy.WithoutAcceptEncoding())
	if err != nil {
		finishEmbeddingTraceWithError(embeddingRecorder, err, types.TraceErrUpstreamUnavailable)
		httpbase.ServerError(c, err)
		return nil
	}

	tokenCounter := h.handler.tokenCounterFactory.NewEmbedding(token.CreateParam{
		Endpoint: mt.Target,
		Host:     mt.Host,
		Model:    mt.ModelName,
		ImageID:  mt.Model.ImageID,
		Provider: mt.Model.Provider,
	})
	w := NewResponseWriterWrapperEmbedding(c.Writer, tokenCounter)
	if req.Input.OfString.String() != "" {
		tokenCounter.Input(req.Input.OfString.Value)
	}

	proxyStartTime := time.Now()
	rp.ServeHTTP(w, c.Request, proxyToAPI, mt.Host)

	w.CaptureEmbeddingUsage()
	embeddingUsage := preComputeUsage(ctx, tokenCounter)
	RecordMetrics(RecordMetricsParams{
		C:              c,
		Ctx:            ctx,
		FinalWrite:     nil,
		Counter:        tokenCounter,
		ProxyStartTime: proxyStartTime,
		Usage:          embeddingUsage,
	})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(ctx, "panic in embedding usage recording", slog.Any("panic", r))
			}
		}()
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()

		usage := embeddingUsage
		if usage == nil && tokenCounter != nil {
			var usageErr error
			usage, usageErr = tokenCounter.Usage(usageCtx)
			if usageErr != nil {
				slog.ErrorContext(usageCtx, "failed to get embedding token usage", slog.Any("error", usageErr))
			}
		}

		if embeddingRecorder != nil {
			recordEmbeddingTraceCompletion(embeddingRecorder, req, mt.ModelName, usage, w.StatusCode())
			embeddingRecorder.End()
		}

		if usage != nil && isSuccessfulStatus(w.StatusCode()) {
			if err := h.handler.openaiComponent.RecordUsageFromTokenUsage(usageCtx, nsUUID, mt.Model, mt.ModelName, usage, apikey); err != nil {
				slog.ErrorContext(usageCtx, "failed to record embedding token usage", "error", err)
			}
		}
	}()

	return nil
}

// --- Error handling ---

func (h *embeddingPipelineHandler) HandlePlanError(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan, err error) {
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


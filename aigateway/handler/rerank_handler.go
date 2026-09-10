package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/proxy"
)

// RerankHandlerImpl extends OpenAIHandlerImpl to serve the /v1/rerank
// endpoint through the three-stage pipeline (Extract → Plan → Execute).
// It embeds *OpenAIHandlerImpl so all shared infrastructure is reused,
// and adds the rerank-specific pipeline handler + Orchestrator wiring.
type RerankHandlerImpl struct {
	*OpenAIHandlerImpl
	rerankPipeline *rerankPipelineHandler
	orchestrator   *plan.Orchestrator
}

// NewRerankHandler creates a RerankHandlerImpl from an existing
// OpenAIHandlerImpl.
func NewRerankHandler(openai *OpenAIHandlerImpl) *RerankHandlerImpl {
	h := &RerankHandlerImpl{
		OpenAIHandlerImpl: openai,
		rerankPipeline:    &rerankPipelineHandler{handler: openai},
	}

	h.orchestrator = newOrchestrator(openai)

	return h
}

// Rerank handles POST /v1/rerank.
// @Summary      Rerank
// @Description  Proxies rerank requests to text-ranking model endpoints (vllm / TEI / llama.cpp).
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        request body RerankRequest true "Rerank request"
// @Success      200  {object}  types.Response{} "OK"
// @Failure      400  {object}  error "Bad request"
// @Failure      402  {object}  error "Insufficient balance"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/rerank [post]
func (h *RerankHandlerImpl) Rerank(c *gin.Context) {
	h.orchestrator.Dispatch(c, h.rerankPipeline, h.rerankPipeline)
}

// rerankPipelineHandler implements plan.MetadataExtractor and
// plan.ProtocolHandler for the /v1/rerank endpoint.  It moves the
// shared admission logic (model resolution, balance, usage limit,
// content safety) into the Planner, keeping only the
// protocol-specific Execute logic (proxy + token counting + usage
// recording) in this handler.
type rerankPipelineHandler struct {
	handler *OpenAIHandlerImpl
}

// Ensure rerankPipelineHandler implements both interfaces.
var (
	_ plan.MetadataExtractor = (*rerankPipelineHandler)(nil)
	_ plan.ProtocolHandler   = (*rerankPipelineHandler)(nil)
)

// --- Phase 1: Extract ---

func (h *rerankPipelineHandler) Extract(c *gin.Context) (*types.RequestMetadata, error) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)

	var req types.RerankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return nil, err
	}
	if req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model cannot be empty"})
		return nil, fmt.Errorf("model cannot be empty")
	}
	if req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query cannot be empty"})
		return nil, fmt.Errorf("query cannot be empty")
	}
	if len(req.Documents) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Documents cannot be empty"})
		return nil, fmt.Errorf("documents cannot be empty")
	}

	return &types.RequestMetadata{
		Protocol:   string(types.ProtocolChat),
		Task:       "rerank",
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

func (h *rerankPipelineHandler) Execute(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan) error {
	ctx := c.Request.Context()
	nsUUID := meta.TenantID
	apikey := meta.APIKeyID

	// End the preflight span that was started by the entry handler.
	if pt := plan.GetPreflightTracer(c); pt != nil {
		pt.SetTargetModel(meta.Model, p.ModelTarget)
		pt.End()
		plan.SetPreflightTracer(c, nil)
	}

	mt := p.ModelTarget
	req := meta.ParsedBody.(*types.RerankRequest)

	req.Model = mt.ModelName
	data, err := json.Marshal(req)
	if err != nil {
		slog.ErrorContext(ctx, "failed to marshal rerank request", slog.Any("error", err))
		httpbase.ServerError(c, err)
		return nil
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	c.Request.ContentLength = int64(len(data))

	if err := applyModelAuthHeaders(c.Request.Header, mt.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head", slog.String("model", mt.ModelName), slog.Any("error", err))
	}

	slog.InfoContext(ctx, "proxy rerank request to model endpoint",
		slog.Any("target", mt.Target), slog.Any("host", mt.Host),
		slog.Any("user", meta.UserID), slog.Any("model_id", meta.Model))

	proxyToAPI := resolveProxyPathFromModelEndpoint(mt.Model.Endpoint, mt.ModelName)
	if proxyToAPI == "" {
		proxyToAPI = "/rerank"
	}

	rp, err := proxy.NewReverseProxy(mt.Target, proxy.WithoutAcceptEncoding())
	if err != nil {
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
	w := NewResponseWriterWrapperRerank(c.Writer, tokenCounter)
	tokenCounter.Input(req.Query + "\n" + strings.Join(req.Documents, "\n"))

	proxyStartTime := time.Now()
	rp.ServeHTTP(w, c.Request, proxyToAPI, mt.Host)

	w.CaptureRerankUsage()
	rerankUsage := preComputeUsage(ctx, tokenCounter)
	RecordMetrics(RecordMetricsParams{
		C:              c,
		Ctx:            ctx,
		FinalWrite:     nil,
		Counter:        tokenCounter,
		ProxyStartTime: proxyStartTime,
		Usage:          rerankUsage,
	})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(ctx, "panic in rerank usage recording", slog.Any("panic", r))
			}
		}()
		usageCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()

		usage := rerankUsage
		if usage == nil && tokenCounter != nil {
			var usageErr error
			usage, usageErr = tokenCounter.Usage(usageCtx)
			if usageErr != nil {
				slog.ErrorContext(usageCtx, "failed to get rerank token usage", slog.Any("error", usageErr))
			}
		}

		if usage != nil && isSuccessfulStatus(w.StatusCode()) {
			if err := h.handler.openaiComponent.RecordUsageFromTokenUsage(usageCtx, nsUUID, mt.Model, mt.ModelName, usage, apikey); err != nil {
				slog.ErrorContext(usageCtx, "failed to record rerank token usage", "error", err)
			}
		}
	}()

	return nil
}

// --- Error handling ---

func (h *rerankPipelineHandler) HandlePlanError(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan, err error) {
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


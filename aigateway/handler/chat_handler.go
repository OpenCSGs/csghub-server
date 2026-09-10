package handler

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/rpc"
	commontrace "opencsg.com/csghub-server/common/utils/trace"
)

// ChatHandlerImpl extends OpenAIHandlerImpl to serve the
// /v1/chat/completions endpoint through the three-stage pipeline
// (Extract → Plan → Execute).  It embeds *OpenAIHandlerImpl so all
// shared infrastructure is reused, and adds the chat-specific pipeline
// handler + Orchestrator wiring.
type ChatHandlerImpl struct {
	*OpenAIHandlerImpl
	chatPipeline *chatPipelineHandler
	orchestrator *plan.Orchestrator
}

// NewChatHandler creates a ChatHandlerImpl from an existing
// OpenAIHandlerImpl.
func NewChatHandler(openai *OpenAIHandlerImpl) *ChatHandlerImpl {
	h := &ChatHandlerImpl{
		OpenAIHandlerImpl: openai,
		chatPipeline:      &chatPipelineHandler{handler: openai},
	}

	h.orchestrator = newOrchestrator(openai)

	return h
}

// Chat handles POST /v1/chat/completions.
// @Security     ApiKey
// @Summary      Create chat completion
// @Description  Sends an OpenAI-compatible chat completion request to the backend model and returns the response. Streams Server-Sent Events when `stream: true`.
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        request body ChatCompletionRequest true "Chat completion request"
// @Success      200  {object}  map[string]interface{} "OK"
// @Success      200  {object}  string "Server-Sent Events stream when stream=true"
// @Failure      400  {object}  error "Bad request"
// @Failure      402  {object}  error "Insufficient balance"
// @Failure      404  {object}  error "Model not found"
// @Failure      429  {object}  error "Usage limit exceeded"
// @Failure      500  {object}  error "Internal server error"
// @Router       /v1/chat/completions [post]
func (h *ChatHandlerImpl) Chat(c *gin.Context) {
	h.orchestrator.Dispatch(c, h.chatPipeline, h.chatPipeline)
}

// chatPipelineHandler implements plan.MetadataExtractor and
// plan.ProtocolHandler for the /v1/chat/completions endpoint.  It moves
// the shared admission logic (model resolution, balance, usage limit,
// content safety) into the Planner, keeping only the protocol-specific
// Execute logic (endpoint compatibility, trace setup, proxy with
// fallback/retry, metrics, async post-process) in this handler.
type chatPipelineHandler struct {
	handler *OpenAIHandlerImpl
}

// Ensure chatPipelineHandler implements both interfaces.
var (
	_ plan.MetadataExtractor = (*chatPipelineHandler)(nil)
	_ plan.ProtocolHandler   = (*chatPipelineHandler)(nil)
)

// chatParsedBody carries the parsed request plus fields the Execute
// phase needs but the Planner does not.
type chatParsedBody struct {
	Req *types.ChatCompletionRequest
}

// PromptText satisfies types.PromptTextProvider so the Planner can extract
// prompt text for input content-safety checking.
func (b *chatParsedBody) PromptText() string {
	if b == nil || b.Req == nil {
		return ""
	}
	return b.Req.PromptText()
}

// --- Phase 1: Extract ---

func (h *chatPipelineHandler) Extract(c *gin.Context) (*types.RequestMetadata, error) {
	ctx := c.Request.Context()
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	apikey := httpbase.GetAccessToken(c)

	chatReq := &types.ChatCompletionRequest{}
	if err := c.BindJSON(chatReq); err != nil {
		slog.ErrorContext(ctx, "invalid chat completion request body", slog.Any("error", err))
		c.String(http.StatusBadRequest, fmt.Errorf("invalid chat completion request body:%w", err).Error())
		return nil, err
	}

	return &types.RequestMetadata{
		Protocol:   string(types.ProtocolChat),
		Task:       "chat",
		Model:      chatReq.Model,
		TenantID:   nsUUID,
		UserID:     username,
		APIKeyID:   apikey,
		Streaming:  chatReq.Stream,
		Headers:    c.Request.Header,
		ParsedBody: &chatParsedBody{Req: chatReq},
	}, nil
}

// --- Phase 3: Execute ---

func (h *chatPipelineHandler) Execute(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan) error {
	ctx := c.Request.Context()
	pb := meta.ParsedBody.(*chatParsedBody)
	chatReq := pb.Req
	username := meta.UserID
	nsUUID := meta.TenantID
	apikey := meta.APIKeyID
	modelID := meta.Model

	mt := modelTargetToResolved(p.ModelTarget)

	// End the preflight span that was started by the entry handler.
	if pt := plan.GetPreflightTracer(c); pt != nil {
		pt.SetTargetModel(modelID, p.ModelTarget)
		pt.End()
		plan.SetPreflightTracer(c, nil)
	}

	applyChatCompletionsEndpointCompatibility(ctx, mt)
	chatReq.Model = mt.ModelName

	if chatReq.Stream {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		if !strings.Contains(mt.Model.ImageID, "vllm-cpu") {
			chatReq.StreamOptions = &types.StreamOptions{
				IncludeUsage: true,
			}
		}
	}

	requestID := commontrace.GetTraceIDInGinContext(c)
	traceCtx, generationRecorder := h.handler.startChatTrace(
		ctx,
		c.Request.Header,
		modelID,
		mt,
		chatReq,
		requestID,
		nsUUID,
	)
	ctx = traceCtx
	c.Request = c.Request.WithContext(traceCtx)

	// The Planner already checked prompt-level content safety; if sensitive
	// content was detected, HandlePlanError rendered the response.  Here we
	// only need to determine whether to enable the moderation component for
	// stream-time output moderation.  We call CheckResponsesSensitive with
	// an empty prompt — the policy returns (true, nil, nil) when the model
	// is enrolled in sensitive checking (NeedSensitiveCheck && not
	// whitelisted), without making a moderation service call for the empty
	// text.  This mirrors responses_handler.go and preserves the namespace
	// whitelist gate that a bare NeedSensitiveCheck check would lose.
	var modComponent component.Moderation
	shouldCheck, _, checkErr := h.handler.sensitivePolicy.CheckResponsesSensitive(
		ctx, mt.Model, "", meta.TenantID, chatReq.Stream, mt.Upstream.Provider,
	)
	if checkErr != nil {
		slog.WarnContext(ctx, "chat sensitive policy check error", slog.Any("error", checkErr))
	} else if shouldCheck {
		modComponent = h.handler.modComponent
	}

	chatCtx := h.handler.setupChatContext(
		ctx,
		mt,
		chatReq,
		modComponent,
		c.Writer,
		commontrace.GetTraceIDInGinContext(c),
		nsUUID,
	)
	defer chatCtx.responseWriter.ClearBuffer()

	chatCtx.tokenCounter.AppendPrompts(chatReq.Messages)

	if err := applyModelAuthHeaders(c.Request.Header, mt.Model); err != nil {
		slog.WarnContext(ctx, "invalid auth head",
			slog.String("model", mt.ModelName),
			slog.Any("error", err))
	}

	proxyStartTime := time.Now()
	log := slog.With(
		slog.String("proxy_start_time", proxyStartTime.Format(time.RFC3339)),
		slog.Any("model_name", mt.ModelName),
		slog.Any("current_user", username),
		slog.Any("target", mt.Target),
		slog.Any("host", mt.Host),
	)
	primaryWriter, proxyErr := h.handler.executeChatProxyAttempt(c, chatCtx.responseWriter, mt, nsUUID, chatReq)
	if proxyErr != nil {
		finishLLMTraceWithError(generationRecorder, proxyErr, types.TraceErrUpstreamUnavailable)
		h.handler.handleProxyError(c, chatReq.Stream, username, modelID, proxyErr)
		log.ErrorContext(ctx, "failed to execute chat proxy", slog.Int("status", retryWriterStatusCode(primaryWriter)), slog.Any("error", proxyErr))
		return nil
	}
	log.InfoContext(ctx, "proxy chat request to model target", slog.Int("status", primaryWriter.statusCode), slog.Int64("proxy_latency(ms)", time.Since(proxyStartTime).Milliseconds()), slog.Int64("ttft(ms)", retryWriterTTFTMs(primaryWriter, proxyStartTime)))

	finalWriter, err := h.handler.executeChatWithFallback(c, chatCtx, mt, nsUUID, chatReq, primaryWriter, username, modelID)
	if err != nil {
		finishLLMTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		h.handler.handleProxyError(c, chatReq.Stream, username, modelID, err)
		log.ErrorContext(ctx, "failed to execute chat fallback", slog.Int("status", retryWriterStatusCode(finalWriter)), slog.Any("error", err))
		return nil
	}
	log.InfoContext(ctx, "fallback chat request to model target", slog.Int("status", retryWriterStatusCode(finalWriter)), slog.Int64("proxy_latency(ms)", time.Since(proxyStartTime).Milliseconds()), slog.Int64("ttft(ms)", retryWriterTTFTMs(finalWriter, proxyStartTime)))

	// Synchronously record proxy-level metrics before c.Next() returns.
	chatUsage := preComputeUsage(ctx, chatCtx.tokenCounter)
	RecordMetrics(RecordMetricsParams{
		C:              c,
		Ctx:            ctx,
		FinalWrite:     finalWriter,
		Counter:        chatCtx.tokenCounter,
		ProxyStartTime: proxyStartTime,
		Usage:          chatUsage,
	})

	h.handler.runChatPostProcessAsync(ctx, chatPostProcessInput{
		NSUUID:          nsUUID,
		ApiKey:          apikey,
		Model:           mt.Model,
		TargetModelName: mt.ModelName,
		TokenCounter:    chatCtx.tokenCounter,
		Usage:           chatUsage,
		LogCapture:      chatCtx.logCapture,
		Trace:           newChatTracePostProcessInput(generationRecorder, chatReq, finalWriter),
		StatusCode:      retryWriterStatusCode(finalWriter),
	})

	return nil
}

// --- Error handling ---

func (h *chatPipelineHandler) HandlePlanError(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan, err error) {
	if pt := plan.GetPreflightTracer(c); pt != nil {
		pt.RecordError(err, "plan_error")
		plan.SetPreflightTracer(c, nil)
	}

	// Chat has a special sensitive-content response: it returns a 200 OK
	// with a canned blocked message (stream or JSON) instead of an error.
	if p != nil && p.ErrorCode == types.PlanErrSensitive {
		blockedResult := &rpc.CheckResult{
			IsSensitive: true,
			Reason:      "",
		}
		if p.Safety != nil && p.Safety.Message != "" {
			blockedResult.Reason = p.Safety.Message
		}
		handleSensitiveResponse(c, meta.Streaming, blockedResult)
		return
	}

	// For all other error categories, use the standard OpenAI error format.
	frontendURL := ""
	if h.handler.config != nil {
		frontendURL = h.handler.config.Frontend.URL
	}
	handleOpenAIPlanError(c, meta, p, err, frontendURL)
}


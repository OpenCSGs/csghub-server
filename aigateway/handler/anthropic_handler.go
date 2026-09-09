package handler

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/handler/anthropic"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	"opencsg.com/csghub-server/api/httpbase"
	commontrace "opencsg.com/csghub-server/common/utils/trace"
)

// AnthropicHandlerImpl extends OpenAIHandlerImpl to serve the Anthropic
// Messages API (/v1/messages).  It embeds *OpenAIHandlerImpl so it inherits
// all OpenAI protocol infrastructure (model resolution, balance, usage limit,
// sensitive policy, metrics, usage recording, LLM tracing, LLM log publishing)
// without duplicating it, and adds the Anthropic-specific handler + Orchestrator
// wiring.
//
// The embedded Orchestrator runs the shared three-phase flow
// (Extract → Plan → Execute).  Future protocols (e.g. Gemini) can follow
// the same pattern: embed *OpenAIHandlerImpl, create a protocol-specific
// handler, and dispatch through the Orchestrator.
type AnthropicHandlerImpl struct {
	*OpenAIHandlerImpl
	messagesHandler *anthropic.Handler
	orchestrator    *plan.Orchestrator
}

// NewAnthropicHandler creates an AnthropicHandlerImpl from an existing
// OpenAIHandlerImpl.  The OpenAI handler is embedded, so all shared
// infrastructure (model resolution, balance, sensitive policy, metrics,
// usage recording, LLM tracing, LLM log publishing) is reused.
func NewAnthropicHandler(openai *OpenAIHandlerImpl) *AnthropicHandlerImpl {
	h := &AnthropicHandlerImpl{OpenAIHandlerImpl: openai}

	bridge := newMessagesHandlerBridge(openai)
	h.messagesHandler = anthropic.New(bridge.toMessagesDeps())

	mr, bc, ulc, cs := newPlannerDeps(openai)
	h.orchestrator = plan.NewOrchestrator(plan.NewPlanner(mr, bc, ulc, cs))

	return h
}

// Messages handles POST /v1/messages (Anthropic Messages API).
// @Summary      Anthropic Messages API
// @Description  Create a message using the Anthropic Messages API format. Supports streaming and non-streaming responses, multi-turn conversations, tool use, vision, and thinking/reasoning. Requests are routed through the shared three-phase pipeline (Extract → Plan → Execute) with preflight tracing, LLM generation tracing, and LLM training log capture.
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        request  body      types.AnthropicMessagesRequest  true  "Anthropic Messages request"
// @Success      200      {object}  types.AnthropicMessagesResponse
// @Failure      400      {object}  types.Error
// @Failure      402      {object}  types.Error
// @Failure      404      {object}  types.Error
// @Failure      429      {object}  types.Error
// @Failure      500      {object}  types.Error
// @Failure      503      {object}  types.Error
// @Router       /v1/messages [post]
func (h *AnthropicHandlerImpl) Messages(c *gin.Context) {
	ctx := c.Request.Context()
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	requestID := commontrace.GetTraceIDInGinContext(c)

	// Start preflight trace span — covers Extract → Plan → Execute.
	ctx, preflight := startPreflightTrace(ctx, preflightTraceStart{
		API:       c.FullPath(),
		RequestID: requestID,
		UserID:    nsUUID,
	})
	c.Request = c.Request.WithContext(ctx)

	// Store the preflight tracer in the gin context so the anthropic Handler
	// can record model-resolution attributes (Execute) and errors
	// (HandlePlanError) on the same span.
	anthropic.SetPreflightTracer(c, &preflightTracerAdapter{trace: preflight})

	// Ensure the preflight span is always ended, even if Extract fails
	// (Orchestrator.Dispatch returns early on Extract error without calling
	// Execute or HandlePlanError).  End() uses sync.Once so it's safe to call
	// again if Execute or HandlePlanError already ended it.
	defer preflight.End()

	h.orchestrator.Dispatch(c, h.messagesHandler, h.messagesHandler)
}

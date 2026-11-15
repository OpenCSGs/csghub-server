package handler

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/handler/decisions"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	_ "opencsg.com/csghub-server/aigateway/types"
)

// DecisionsHandlerImpl extends OpenAIHandlerImpl to serve OpenAI Decisions
// API (/v1/decisions).  It embeds *OpenAIHandlerImpl so it inherits all
// shared infrastructure (model resolution, balance, metrics, usage recording,
// and LLM tracing) without duplicating it, and adds Decisions-specific
// handling and Orchestrator wiring.
//
// The embedded Orchestrator runs the shared three-phase flow
// (Extract → Plan → Execute), following the same pattern as the Anthropic
// Messages handler.
type DecisionsHandlerImpl struct {
	*OpenAIHandlerImpl
	decisionsHandler *decisions.Handler
	orchestrator     *plan.Orchestrator
}

// NewDecisionsHandler creates a DecisionsHandlerImpl from an existing OpenAIHandlerImpl.
// The OpenAI handler is embedded, so all shared infrastructure (model
// resolution, balance, metrics, usage recording, and LLM tracing) is reused.
func NewDecisionsHandler(openai *OpenAIHandlerImpl) *DecisionsHandlerImpl {
	h := &DecisionsHandlerImpl{OpenAIHandlerImpl: openai}

	bridge := &jevHandlerBridge{handler: openai}
	deps := decisions.Deps{
		ProxyExecutor:      bridge,
		UsageRecorder:      bridge,
		AdmissionFinalizer: bridge,
		MetricsRecorder:    bridge,
		ErrorRecorder:      errorRecorderFunc(SetMetricsError),
	}
	if openai.llmTracer != nil {
		deps.LLMTracer = &decisionsTracerAdapter{tracer: openai.llmTracer}
	}
	h.decisionsHandler = decisions.New(deps)

	h.orchestrator = newOrchestrator(openai)

	return h
}

// Decisions handles POST /v1/decisions.
// @Summary      OpenAI Decisions API
// @Description  Evaluate text or image input against typed questions. Requests use the shared planning pipeline with usage recording and LLM generation tracing.
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        request  body      types.DecisionRequest  true  "Decisions request"
// @Success      200      {object}  types.DecisionResponse
// @Failure      400      {object}  types.Error
// @Failure      402      {object}  types.Error
// @Failure      404      {object}  types.Error
// @Failure      429      {object}  types.Error
// @Failure      500      {object}  types.Error
// @Failure      504      {object}  types.Error
// @Router       /v1/decisions [post]
func (h *DecisionsHandlerImpl) Decisions(c *gin.Context) {
	h.orchestrator.Dispatch(c, h.decisionsHandler, h.decisionsHandler)
}

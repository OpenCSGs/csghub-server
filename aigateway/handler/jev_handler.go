package handler

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/handler/jev"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	_ "opencsg.com/csghub-server/aigateway/types"
)

// JevHandlerImpl extends OpenAIHandlerImpl to serve the Jev (System One)
// API (/v1/systemone).  It embeds *OpenAIHandlerImpl so it inherits all
// shared infrastructure (model resolution, balance, sensitive policy,
// metrics, usage recording, LLM tracing, LLM log publishing) without
// duplicating it, and adds the Jev-specific handler + Orchestrator wiring.
//
// The embedded Orchestrator runs the shared three-phase flow
// (Extract → Plan → Execute), following the same pattern as the Anthropic
// Messages handler.
type JevHandlerImpl struct {
	*OpenAIHandlerImpl
	systemOneHandler *jev.Handler
	orchestrator     *plan.Orchestrator
}

// NewJevHandler creates a JevHandlerImpl from an existing OpenAIHandlerImpl.
// The OpenAI handler is embedded, so all shared infrastructure (model
// resolution, balance, sensitive policy, metrics, usage recording, LLM
// tracing, LLM log publishing) is reused.
func NewJevHandler(openai *OpenAIHandlerImpl) *JevHandlerImpl {
	h := &JevHandlerImpl{OpenAIHandlerImpl: openai}

	bridge := newJevHandlerBridge(openai)
	h.systemOneHandler = jev.New(bridge.toJevDeps())

	h.orchestrator = newOrchestrator(openai)

	return h
}

// SystemOne handles POST /v1/systemone (Jev System One API).
// @Summary      Jev System One API
// @Description  Evaluate a state against a set of typed questions using a Jev (System One) model, following the TypeSafe SDK protocol. Requests are routed through the shared three-phase pipeline (Extract → Plan → Execute) with upstream authentication, timeout handling, LLM generation tracing, and LLM training log capture.
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        request  body      types.JevRequest  true  "Jev System One request"
// @Success      200      {object}  types.JevResponse
// @Failure      400      {object}  types.Error
// @Failure      402      {object}  types.Error
// @Failure      404      {object}  types.Error
// @Failure      429      {object}  types.Error
// @Failure      500      {object}  types.Error
// @Failure      504      {object}  types.Error
// @Router       /v1/systemone [post]
func (h *JevHandlerImpl) SystemOne(c *gin.Context) {
	h.orchestrator.Dispatch(c, h.systemOneHandler, h.systemOneHandler)
}

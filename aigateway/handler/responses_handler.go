package handler

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
	"opencsg.com/csghub-server/aigateway/component"
	"opencsg.com/csghub-server/aigateway/handler/plan"
	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/rpc"
	commontrace "opencsg.com/csghub-server/common/utils/trace"
)

// ResponsesHandlerImpl extends OpenAIHandlerImpl to serve the /v1/responses
// endpoint through the three-stage pipeline (Extract → Plan → Execute).
// It embeds *OpenAIHandlerImpl so all shared infrastructure is reused,
// and adds the responses-specific pipeline handler + Orchestrator wiring.
type ResponsesHandlerImpl struct {
	*OpenAIHandlerImpl
	responsesPipeline *responsesPipelineHandler
	orchestrator      *plan.Orchestrator
}

// NewResponsesHandler creates a ResponsesHandlerImpl from an existing
// OpenAIHandlerImpl.
func NewResponsesHandler(openai *OpenAIHandlerImpl) *ResponsesHandlerImpl {
	h := &ResponsesHandlerImpl{
		OpenAIHandlerImpl: openai,
		responsesPipeline: &responsesPipelineHandler{handler: openai},
	}

	h.orchestrator = newOrchestrator(openai)

	return h
}

// Responses handles POST /v1/responses.
// @Security     ApiKey
// @Summary      Create a model response
// @Description  Sends an OpenAI-compatible Responses API request to the backend model and returns the response. Streams Server-Sent Events when `stream: true`.
// @Tags         AIGateway
// @Accept       json
// @Produce      json
// @Param        request body types.ResponsesRequest true "Responses request"
// @Success      200  {object}  types.ResponsesResponse "OK"
// @Success      200  {object}  string "Server-Sent Events stream when stream=true"
// @Failure      400  {object}  error "Bad request or unsupported feature"
// @Failure      402  {object}  error "Insufficient balance or usage limit exceeded"
// @Failure      404  {object}  error "Model not found"
// @Failure      500  {object}  error "Internal server error"
// @Failure      502  {object}  error "Upstream returned an invalid response"
// @Router       /v1/responses [post]
func (h *ResponsesHandlerImpl) Responses(c *gin.Context) {
	h.orchestrator.Dispatch(c, h.responsesPipeline, h.responsesPipeline)
}

// responsesPipelineHandler implements plan.MetadataExtractor and
// plan.ProtocolHandler for the /v1/responses endpoint.  It moves the
// shared admission logic (model resolution with RequiredUpstreamID,
// balance, usage limit, content safety) into the Planner, keeping only
// the protocol-specific Execute logic (Responses-specific routing,
// native/adapter dispatch, trace + capture setup) in this handler.
type responsesPipelineHandler struct {
	handler *OpenAIHandlerImpl
}

// Ensure responsesPipelineHandler implements both interfaces.
var (
	_ plan.MetadataExtractor = (*responsesPipelineHandler)(nil)
	_ plan.ProtocolHandler   = (*responsesPipelineHandler)(nil)
)

// responsesParsedBody carries the parsed request plus protocol-specific
// fields that the Execute phase needs but the Planner does not.
type responsesParsedBody struct {
	Req                   *types.ResponsesRequest
	PublicModelID         string
	PublicPreviousRespID  string
	UpstreamResponseID    string
	Owner                 string
}

// PromptText satisfies types.PromptTextProvider so the Planner can extract
// prompt text for input content-safety checking.
func (b *responsesParsedBody) PromptText() string {
	if b == nil || b.Req == nil {
		return ""
	}
	return b.Req.PromptText()
}

// --- Phase 1: Extract ---

func (h *responsesPipelineHandler) Extract(c *gin.Context) (*types.RequestMetadata, error) {
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	apikey := httpbase.GetAccessToken(c)
	owner := responsesOwnerBinding(c)

	req := &types.ResponsesRequest{}
	if err := c.BindJSON(req); err != nil {
		writeResponsesError(c, http.StatusBadRequest, "invalid_request_error", "invalid_request_error", err.Error())
		return nil, err
	}
	if err := req.Validate(); err != nil {
		writeResponsesError(c, http.StatusBadRequest, "invalid_request_error", "invalid_request_error", err.Error())
		return nil, err
	}

	publicModelID := req.Model
	publicPreviousResponseID := req.PreviousResponseID

	// Resolve previous_response_id → RequiredUpstreamID + upstream ID.
	// This must happen before model resolution because RequiredUpstreamID
	// pins the upstream selection.
	var requiredUpstreamID int64
	var upstreamResponseID string
	if publicPreviousResponseID != "" {
		route, ok := h.handler.resolvePreviousResponseRoute(c, publicPreviousResponseID, owner)
		if !ok {
			return nil, fmt.Errorf("failed to resolve previous_response_id")
		}
		requiredUpstreamID = route.RequiredUpstreamID
		upstreamResponseID = route.UpstreamResponseID
	}
	if upstreamResponseID != "" {
		req.PreviousResponseID = upstreamResponseID
	}

	return &types.RequestMetadata{
		Protocol:           string(types.ProtocolResponses),
		Task:               "responses",
		Model:              publicModelID,
		TenantID:           nsUUID,
		UserID:             username,
		APIKeyID:           apikey,
		Streaming:          req.Stream,
		Headers:            c.Request.Header,
		ParsedBody: &responsesParsedBody{
			Req:                  req,
			PublicModelID:        publicModelID,
			PublicPreviousRespID: publicPreviousResponseID,
			UpstreamResponseID:   upstreamResponseID,
			Owner:                owner,
		},
		RequiredUpstreamID: requiredUpstreamID,
	}, nil
}

// --- Phase 3: Execute ---

func (h *responsesPipelineHandler) Execute(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan) error {
	ctx := c.Request.Context()
	pb := meta.ParsedBody.(*responsesParsedBody)
	req := pb.Req

	// Record resolved model on the preflight span and end it.
	if pt := plan.GetPreflightTracer(c); pt != nil {
		pt.SetTargetModel(pb.PublicModelID, p.ModelTarget)
		pt.End()
		plan.SetPreflightTracer(c, nil)
	}

	mt := modelTargetToResolved(p.ModelTarget)

	// Responses-specific routing (native vs chat-adapter).  This is separate
	// from the Planner's protocol routing — the Planner handles the general
	// protocol routing matrix and disabled check, while responsespkg.ResolveRouting
	// decides whether to use the native Responses endpoint or adapt to Chat.
	decision, err := responsespkg.ResolveRouting(responsespkg.RoutingTarget{
		ModelID:          mt.Model.ID,
		Target:           mt.Target,
		CSGHubHosted:     isCSGHubHostedModel(mt.Model),
		RuntimeFramework: mt.Model.RuntimeFramework,
		ImageID:          mt.Model.ImageID,
	})
	if err != nil {
		writeResponsesError(c, http.StatusBadRequest, "unsupported_feature", "invalid_request_error", err.Error())
		return nil
	}
	if decision.Mode == responsespkg.ResponsesModeDisabled {
		writeResponsesError(c, http.StatusBadRequest, "unsupported_feature", "invalid_request_error", "/v1/responses is disabled for this model")
		return nil
	}

	// Apply backend URL from routing decision.
	resolvedTarget := withResponsesBackendURL(mt, decision.BackendURL)

	requestID := commontrace.GetTraceIDInGinContext(c)
	traceCtx, generationRecorder := h.handler.startResponsesTrace(
		ctx,
		c.Request.Header,
		pb.PublicModelID,
		resolvedTarget,
		req,
		decision,
		requestID,
		meta.TenantID,
	)
	ctx = traceCtx
	c.Request = c.Request.WithContext(traceCtx)

	responseCapture := h.handler.setupResponsesCapture(c, req, resolvedTarget, decision, meta.TenantID)

	// Content safety was checked by the Planner.  If the Planner flagged
	// sensitive content, HandlePlanError already rendered the response.
	// Here we only need to determine whether to enable the moderation
	// component for stream-time output moderation.  We call
	// CheckResponsesSensitive with an empty prompt — the policy returns
	// (true, nil, nil) when the model is enrolled in sensitive checking
	// (NeedSensitiveCheck && not whitelisted), without making a moderation
	// service call for the empty text.
	var responsesModeration component.Moderation
	shouldCheck, _, checkErr := h.handler.sensitivePolicy.CheckResponsesSensitive(
		ctx, resolvedTarget.Model, "", meta.TenantID, req.Stream, resolvedTarget.Upstream.Provider,
	)
	if checkErr != nil {
		slog.WarnContext(ctx, "responses sensitive policy check error", slog.Any("error", checkErr))
	} else if shouldCheck {
		responsesModeration = h.handler.modComponent
	}

	switch decision.Mode {
	case responsespkg.ResponsesModeNative:
		h.handler.executeNativeResponses(c, req, resolvedTarget, decision, pb.Owner, meta.TenantID, meta.APIKeyID, pb.PublicModelID, pb.PublicPreviousRespID, responsesModeration, responseCapture, generationRecorder)
	case responsespkg.ResponsesModeChatAdapter:
		h.handler.executeAdapterResponses(c, req, resolvedTarget, meta.TenantID, meta.APIKeyID, pb.PublicModelID, responsesModeration, responseCapture, generationRecorder)
	default:
		writeResponsesError(c, http.StatusBadRequest, "unsupported_feature", "invalid_request_error", "unsupported responses execution mode")
	}

	return nil
}

// --- Error handling ---

func (h *responsesPipelineHandler) HandlePlanError(c *gin.Context, meta *types.RequestMetadata, p *types.RequestPlan, err error) {
	// Record preflight error if the span is still open.
	if pt := plan.GetPreflightTracer(c); pt != nil {
		pt.RecordError(err, "plan_error")
		plan.SetPreflightTracer(c, nil)
	}

	// Responses has a special sensitive-content response: it returns a
	// 200 OK with a canned blocked message instead of an error JSON.
	if p != nil && p.ErrorCode == types.PlanErrSensitive {
		// Build a synthetic CheckResult for the Responses sensitive handler.
		blockedResult := &rpc.CheckResult{
			IsSensitive: true,
			Reason:      "",
		}
		if p.Safety != nil && p.Safety.Message != "" {
			blockedResult.Reason = p.Safety.Message
		}
		responsespkg.HandleSensitiveResponse(c, meta.Streaming, blockedResult)
		return
	}

	// Preserve the old error contract for previous_response_id route
	// failures: when the required upstream is no longer available, the
	// legacy code returned 400 response_route_unavailable (not the generic
	// 503 model_unavailable that handleOpenAIPlanError would render).
	if p != nil && p.ErrorCode == types.PlanErrModelUnavailable {
		var ce plan.CodedError
		if errors.As(err, &ce) && ce.ModelErrorCode() == "required_upstream_unavailable" {
			writeResponsesError(c, http.StatusBadRequest, "response_route_unavailable", "invalid_request_error",
				"previous_response_id was created by an upstream that is no longer available")
			return
		}
	}

	// For all other error categories, use the standard OpenAI error format.
	frontendURL := ""
	if h.handler.config != nil {
		frontendURL = h.handler.config.Frontend.URL
	}
	handleOpenAIPlanError(c, meta, p, err, frontendURL)
}

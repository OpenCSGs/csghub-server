package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/component"
	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/utils/trace"
)

// Responses godoc
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
func (h *OpenAIHandlerImpl) Responses(c *gin.Context) {
	ctx := c.Request.Context()
	username := httpbase.GetCurrentUser(c)
	nsUUID := httpbase.GetCurrentNamespaceUUID(c)
	apikey := httpbase.GetAccessToken(c)
	owner := responsesOwnerBinding(c)

	req := &types.ResponsesRequest{}
	if err := c.BindJSON(req); err != nil {
		writeResponsesError(c, http.StatusBadRequest, "invalid_request_error", "invalid_request_error", fmt.Sprintf("invalid responses request body:%v", err))
		return
	}
	if err := req.Validate(); err != nil {
		writeResponsesError(c, http.StatusBadRequest, "invalid_request_error", "invalid_request_error", err.Error())
		return
	}

	publicModelID := req.Model
	publicPreviousResponseID := req.PreviousResponseID
	previousResponse, ok := h.resolvePreviousResponseRoute(c, publicPreviousResponseID, owner)
	if !ok {
		return
	}
	if previousResponse.UpstreamResponseID != "" {
		req.PreviousResponseID = previousResponse.UpstreamResponseID
	}

	modelTarget, ok := h.resolveResponsesModelTarget(c, username, publicModelID, previousResponse.RequiredUpstreamID)
	if !ok {
		return
	}

	decision, err := responsespkg.ResolveRouting(responsespkg.RoutingTarget{
		ModelID: modelTarget.Model.ID,
		Target:  modelTarget.Target,
	})
	if err != nil {
		writeResponsesError(c, http.StatusBadRequest, "unsupported_feature", "invalid_request_error", err.Error())
		return
	}
	if decision.Mode == responsespkg.ResponsesModeDisabled {
		writeResponsesError(c, http.StatusBadRequest, "unsupported_feature", "invalid_request_error", "/v1/responses is disabled for this model")
		return
	}
	requestID := trace.GetTraceIDInGinContext(c)
	traceCtx, generationRecorder := h.startResponsesTrace(
		ctx,
		c.Request.Header,
		publicModelID,
		modelTarget,
		req,
		decision,
		requestID,
		nsUUID,
	)
	ctx = traceCtx
	c.Request = c.Request.WithContext(traceCtx)
	responseCapture := h.setupResponsesCapture(c, req, modelTarget, decision, nsUUID)

	if !modelTarget.Model.SkipBalance() {
		if err := h.openaiComponent.CheckBalance(ctx, nsUUID); err != nil {
			finishLLMTraceWithError(generationRecorder, err, types.TraceErrInsufficientBalance)
			h.handleInsufficientBalance(c, req.Stream, nsUUID, publicModelID, err)
			return
		}
	}

	var responsesModeration component.Moderation
	if isCheck, result, err := h.sensitivePolicy.CheckResponsesSensitive(ctx, modelTarget.Model, types.ResponsesPromptText(req), nsUUID, req.Stream, modelTarget.Upstream.Provider); err != nil {
		slog.WarnContext(ctx, "responses sensitive policy check error", slog.Any("error", err))
	} else if isCheck && result != nil && result.IsSensitive {
		finishLLMTraceWithError(generationRecorder, ErrSensitiveContent, types.TraceErrSensitivePrompt)
		responsespkg.HandleSensitiveResponse(c, req.Stream, result)
		return
	} else if isCheck {
		responsesModeration = h.modComponent
	}

	switch decision.Mode {
	case responsespkg.ResponsesModeNative:
		h.executeNativeResponses(c, req, modelTarget, decision, owner, nsUUID, apikey, publicModelID, publicPreviousResponseID, responsesModeration, responseCapture, generationRecorder)
	case responsespkg.ResponsesModeChatAdapter:
		h.executeAdapterResponses(c, req, modelTarget, nsUUID, apikey, publicModelID, responsesModeration, responseCapture, generationRecorder)
	default:
		writeResponsesError(c, http.StatusBadRequest, "unsupported_feature", "invalid_request_error", "unsupported responses execution mode")
	}
}

func (h *OpenAIHandlerImpl) setupResponsesCapture(c *gin.Context, req *types.ResponsesRequest, modelTarget *resolvedModelTarget, decision responsespkg.RoutingDecision, nsUUID string) *responsespkg.LLMLogRecorder {
	needsLogCapture := h.config != nil && h.config.AIGateway.EnableLLMLog && h.llmLogPublisher != nil
	if !needsLogCapture && h.llmTracer == nil {
		return nil
	}
	if modelTarget == nil || modelTarget.Model == nil {
		return nil
	}
	metadata := map[string]any{
		"source":                   "aigateway",
		"api":                      "/v1/responses",
		"stream":                   req.Stream,
		"provider":                 modelTarget.Model.Provider,
		"svc_name":                 modelTarget.Model.SvcName,
		"responses_execution_mode": string(decision.Mode),
	}
	capture, err := responsespkg.NewLLMLogRecorder(trace.GetTraceIDInGinContext(c), modelTarget.ModelName, nsUUID, req, metadata)
	if err != nil {
		slog.WarnContext(c.Request.Context(), "failed to initialize responses capture", slog.Any("error", err))
		return nil
	}
	return capture
}

type previousResponseRoute struct {
	RequiredUpstreamID int64
	UpstreamResponseID string
}

func (h *OpenAIHandlerImpl) resolvePreviousResponseRoute(c *gin.Context, previousResponseID, owner string) (previousResponseRoute, bool) {
	var route previousResponseRoute
	if previousResponseID == "" {
		return route, true
	}
	if responsespkg.IsAdapterResponseID(previousResponseID) {
		writeResponsesError(c, http.StatusBadRequest, "unsupported_feature", "invalid_request_error", "adapter response ids cannot be used as previous_response_id")
		return route, false
	}
	mapper, err := h.getResponsesIDMapper()
	if err != nil {
		writeResponsesError(c, http.StatusInternalServerError, "internal_error", "internal_error", err.Error())
		return route, false
	}
	claims, err := mapper.Unwrap(previousResponseID, owner)
	if err != nil {
		code := "invalid_response_id"
		if errors.Is(err, responsespkg.ErrResponseIDOwner) {
			code = "response_id_forbidden"
		}
		writeResponsesError(c, http.StatusBadRequest, code, "invalid_request_error", err.Error())
		return route, false
	}
	return previousResponseRoute{
		RequiredUpstreamID: claims.UpstreamID,
		UpstreamResponseID: claims.UpstreamResponseID,
	}, true
}

func (h *OpenAIHandlerImpl) resolveResponsesModelTarget(c *gin.Context, username, publicModelID string, requiredUpstreamID int64) (*resolvedModelTarget, bool) {
	ctx := c.Request.Context()
	modelTarget, err := h.resolveModelTargetWithOptions(ctx, username, publicModelID, c.Request.Header, modelTargetResolveOptions{
		RequiredUpstreamID: requiredUpstreamID,
	})
	if err == nil {
		return modelTarget, true
	}

	var targetErr *modelTargetError
	if requiredUpstreamID != 0 && errors.As(err, &targetErr) && targetErr.APIError.Code == "required_upstream_unavailable" {
		writeResponsesError(c, http.StatusBadRequest, "response_route_unavailable", "invalid_request_error", "previous_response_id was created by an upstream that is no longer available")
		return nil, false
	}
	handleModelTargetError(c, ctx, publicModelID, "failed to get responses target address", err)
	return nil, false
}

func (h *OpenAIHandlerImpl) getResponsesIDMapper() (*responsespkg.IDMapper, error) {
	h.responsesIDMapperOnce.Do(func() {
		h.responsesIDMapper, h.responsesIDMapperErr = responsespkg.NewIDMapperFromConfig(h.config)
	})
	return h.responsesIDMapper, h.responsesIDMapperErr
}

func adapterErrorCode(err error) string {
	msg := err.Error()
	if strings.HasPrefix(msg, "unsupported_feature:") {
		return "unsupported_feature"
	}
	return "invalid_request_error"
}

func writeResponsesError(c *gin.Context, status int, code, typ, message string) {
	c.JSON(status, gin.H{"error": types.Error{Code: code, Type: typ, Message: message}})
}

func responsesOwnerBinding(c *gin.Context) string {
	return httpbase.GetCurrentNamespaceUUID(c)
}

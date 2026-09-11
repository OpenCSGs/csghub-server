package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/utils/trace"
)

func isCSGHubHostedModel(model *types.Model) bool {
	return model != nil && model.SvcName != ""
}

func withResponsesBackendURL(modelTarget *resolvedModelTarget, backendURL string) *resolvedModelTarget {
	return withBackendURL(modelTarget, backendURL)
}

// withBackendURL returns a copy of modelTarget whose Target, Upstream.URL, and
// Model.Endpoint fields are replaced by backendURL.  This is used by protocol
// handlers to apply the Planner's resolved BackendURL (which may have a
// rewritten path) before proxying the request upstream.
func withBackendURL(modelTarget *resolvedModelTarget, backendURL string) *resolvedModelTarget {
	backendURL = strings.TrimSpace(backendURL)
	if modelTarget == nil || backendURL == "" {
		return modelTarget
	}
	resolved := *modelTarget
	resolved.Target = backendURL
	resolved.Upstream.URL = backendURL
	if modelTarget.Model != nil {
		modelCopy := *modelTarget.Model
		modelCopy.Endpoint = backendURL
		resolved.Model = &modelCopy
	}
	return &resolved
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

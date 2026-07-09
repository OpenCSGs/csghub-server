package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/component"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/proxy"
)

func (h *OpenAIHandlerImpl) executeNativeResponses(c *gin.Context, req *types.ResponsesRequest, modelTarget *resolvedModelTarget, decision responsespkg.RoutingDecision, owner, nsUUID, apikey, publicModelID, publicPreviousResponseID string, moderation component.Moderation, logCapture *responsespkg.LLMLogRecorder, generationRecorder llmtrace.GenerationRecorder) {
	if err := h.openaiComponent.CheckUsageLimit(c.Request.Context(), nsUUID, modelTarget.Model, decision.NativeURL); err != nil {
		finishLLMTraceWithError(generationRecorder, err, types.TraceErrInsufficientBalance)
		h.handleUsageLimitExceeded(c, req.Stream, nsUUID, publicModelID, err)
		return
	}
	responsesCounter := h.newResponsesTokenCounter(modelTarget)
	responsesCounter.Request(req)
	mapper, err := h.getResponsesIDMapper()
	if err != nil {
		finishLLMTraceWithError(generationRecorder, err, types.TraceErrUpstreamError)
		writeResponsesError(c, http.StatusInternalServerError, "internal_error", "internal_error", err.Error())
		return
	}
	reqCopy := *req
	reqCopy.Model = modelTarget.ModelName
	body, err := json.Marshal(&reqCopy)
	if err != nil {
		finishLLMTraceWithError(generationRecorder, err, types.TraceErrUpstreamError)
		writeResponsesError(c, http.StatusBadRequest, "invalid_request_error", "invalid_request_error", err.Error())
		return
	}
	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(c.Request.Context(), "invalid auth header", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}
	rp, err := proxy.NewReverseProxy(decision.NativeURL, proxy.WithoutAcceptEncoding())
	if err != nil {
		finishLLMTraceWithError(generationRecorder, err, types.TraceErrUpstreamUnavailable)
		h.handleProxyError(c, req.Stream, nsUUID, publicModelID, err)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	transformer := newResponsesNativePayloadTransformer(
		mapper,
		responsespkg.IDClaims{
			NamespaceUUID: owner,
			UpstreamID:    modelTarget.Upstream.ID,
		},
		publicPreviousResponseID,
		responsesCounter,
		logCapture,
	)
	slog.InfoContext(c.Request.Context(), "proxy responses request to native upstream",
		slog.String("api", "/v1/responses"),
		slog.String("backend_api", "/v1/responses"),
		slog.String("responses_execution_mode", string(responsespkg.ResponsesModeNative)),
		slog.Int64("responses_route_upstream_id", modelTarget.Upstream.ID),
		slog.String("responses_route_provider", modelTarget.Model.Provider),
		slog.String("responses_route_model", modelTarget.ModelName),
	)
	proxyPath := resolveProxyPathFromModelEndpoint(decision.NativeURL, modelTarget.ModelName)
	writer := newResponsesNativeResponseWriter(c.Writer, req.Stream, transformer, moderation, newResponsesModerationSessionID())
	rp.ServeHTTP(writer, c.Request, proxyPath, modelTarget.Host)
	if err := writer.Finalize(); err != nil {
		finishLLMTraceWithError(generationRecorder, err, types.TraceErrUpstreamError)
		writeResponsesError(c, http.StatusBadGateway, "upstream_response_invalid", "api_error", err.Error())
		return
	}
	traceInput := newResponsesTracePostProcessInput(generationRecorder, req, writer.StatusCode(), writer.FirstWriteAt())
	h.recordResponsesUsageWithTrace(c, responsesCounter, nsUUID, modelTarget, apikey, logCapture, traceInput)
}

package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/component"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/compress"
)

func (h *OpenAIHandlerImpl) executeAdapterResponses(c *gin.Context, req *types.ResponsesRequest, modelTarget *resolvedModelTarget, nsUUID, apikey, publicModelID string, moderation component.Moderation, logCapture *responsespkg.LLMLogRecorder, generationRecorder llmtrace.GenerationRecorder) {
	if err := validateResponsesAdapterRequest(req); err != nil {
		finishLLMTraceWithError(generationRecorder, err, types.TraceErrUpstreamError)
		writeResponsesError(c, http.StatusBadRequest, adapterErrorCode(err), "invalid_request_error", err.Error())
		return
	}
	chatReq, err := responsesToChatRequest(c.Request.Context(), req, modelTarget.ModelName, modelTarget.Upstream.Metadata)
	if err != nil {
		finishLLMTraceWithError(generationRecorder, err, types.TraceErrUpstreamError)
		writeResponsesError(c, http.StatusBadRequest, adapterErrorCode(err), "invalid_request_error", err.Error())
		return
	}
	responsesCounter := h.newResponsesTokenCounter(modelTarget)
	responsesCounter.Request(req)
	if req.Stream {
		if !strings.Contains(modelTarget.Model.ImageID, "vllm-cpu") {
			chatReq.StreamOptions = &StreamOptions{IncludeUsage: true}
		}
	}
	if err := applyModelAuthHeaders(c.Request.Header, modelTarget.Model); err != nil {
		slog.WarnContext(c.Request.Context(), "invalid auth header", slog.String("model", modelTarget.ModelName), slog.Any("error", err))
	}
	writer := newResponsesAdapterResponseWriter(c.Writer, req.Stream, publicModelID, responsesCounter, moderation, newResponsesModerationSessionID(), logCapture)
	primaryWriter, proxyErr := h.executeChatProxyAttempt(c, writer, modelTarget, nsUUID, chatReq)
	if proxyErr != nil {
		finishLLMTraceWithError(generationRecorder, proxyErr, types.TraceErrUpstreamUnavailable)
		h.handleProxyError(c, req.Stream, nsUUID, publicModelID, proxyErr)
		return
	}
	finalWriter, fallbackErr := h.executeChatWithFallback(c, &chatContext{responseWriter: writer}, modelTarget, nsUUID, chatReq, primaryWriter, nsUUID, publicModelID)
	if fallbackErr != nil {
		finishLLMTraceWithError(generationRecorder, fallbackErr, types.TraceErrUpstreamUnavailable)
		h.handleProxyError(c, req.Stream, nsUUID, publicModelID, fallbackErr)
		return
	}
	if err := writer.Finalize(retryWriterStatusCode(finalWriter)); err != nil {
		finishLLMTraceWithError(generationRecorder, err, types.TraceErrUpstreamError)
		writeResponsesError(c, http.StatusBadGateway, "upstream_response_invalid", "api_error", err.Error())
		return
	}
	traceInput := newResponsesTracePostProcessInput(generationRecorder, req, retryWriterStatusCode(finalWriter), finalWriter.FirstWriteAt())
	h.recordResponsesUsageWithTrace(c, responsesCounter, nsUUID, modelTarget, apikey, logCapture, traceInput)
}

func decodeResponsesAdapterChatBody(bufferWriter *bufferCommonResponseWriter) ([]byte, error) {
	if bufferWriter == nil {
		return nil, fmt.Errorf("upstream chat response is empty")
	}
	return compress.Decode(bufferWriter.Header().Get("Content-Encoding"), bufferWriter.body.Bytes())
}

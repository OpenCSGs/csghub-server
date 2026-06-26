package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/compress"
)

func (h *OpenAIHandlerImpl) executeAdapterResponses(c *gin.Context, req *types.ResponsesRequest, modelTarget *resolvedModelTarget, nsUUID, apikey, publicModelID string) {
	if err := validateResponsesAdapterRequest(req); err != nil {
		writeResponsesError(c, http.StatusBadRequest, adapterErrorCode(err), "invalid_request_error", err.Error())
		return
	}
	chatReq, err := responsesToChatRequest(c.Request.Context(), req, modelTarget.ModelName)
	if err != nil {
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
	writer := newResponsesAdapterResponseWriter(c.Writer, req.Stream, publicModelID, responsesCounter)
	primaryWriter, proxyErr := h.executeChatProxyAttempt(c, writer, modelTarget, nsUUID, chatReq)
	if proxyErr != nil {
		h.handleProxyError(c, req.Stream, nsUUID, publicModelID, proxyErr)
		return
	}
	finalWriter, fallbackErr := h.executeChatWithFallback(c, &chatContext{responseWriter: writer}, modelTarget, nsUUID, chatReq, primaryWriter, nsUUID, publicModelID)
	if fallbackErr != nil {
		h.handleProxyError(c, req.Stream, nsUUID, publicModelID, fallbackErr)
		return
	}
	if err := writer.Finalize(retryWriterStatusCode(finalWriter)); err != nil {
		writeResponsesError(c, http.StatusBadGateway, "upstream_response_invalid", "api_error", err.Error())
		return
	}
	h.recordResponsesUsage(c, responsesCounter, nsUUID, modelTarget, apikey)
}

func decodeResponsesAdapterChatBody(bufferWriter *bufferCommonResponseWriter) ([]byte, error) {
	if bufferWriter == nil {
		return nil, fmt.Errorf("upstream chat response is empty")
	}
	return compress.Decode(bufferWriter.Header().Get("Content-Encoding"), bufferWriter.body.Bytes())
}

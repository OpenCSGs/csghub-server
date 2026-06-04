package handler

import (
	"context"
	"net/http"

	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

func (h *OpenAIHandlerImpl) startEmbeddingTrace(ctx context.Context, modelID string, modelTarget *resolvedModelTarget, req *EmbeddingRequest, requestID string, userID string) (context.Context, llmtrace.EmbeddingRecorder) {
	if h == nil || h.llmTracer == nil || modelTarget == nil || modelTarget.Model == nil {
		return ctx, nil
	}
	traceCtx, recorder := h.llmTracer.StartEmbedding(ctx, types.EmbeddingStart{
		Provider:       modelTarget.Model.Provider,
		RequestModel:   modelID,
		ResolvedModel:  modelTarget.ModelName,
		Dimensions:     embeddingTraceDimensions(req),
		EncodingFormat: embeddingTraceEncodingFormat(req),
		Metadata: map[string]any{
			llmtrace.TraceMetadataKeyAIGatewayAPI:     "/v1/embeddings",
			llmtrace.TraceMetadataKeyAIGatewayModelID: modelTarget.Model.ID,
			"request_id": requestID,
			"user_id":    userID,
		},
	})
	if traceCtx == nil {
		traceCtx = ctx
	}
	return traceCtx, recorder
}

func recordEmbeddingTraceCompletion(recorder llmtrace.EmbeddingRecorder, req *EmbeddingRequest, model string, usage *token.Usage, statusCode int) {
	if recorder == nil {
		return
	}
	// The caller owns End so it can finish post-processing work in one place.
	if statusCode >= http.StatusBadRequest {
		recorder.SetError(httpStatusTraceError(statusCode), types.TraceErrUpstreamError)
	}
	var inputTokens int64
	if usage != nil {
		inputTokens = usage.PromptTokens
	}
	recorder.SetResult(types.EmbeddingResult{
		InputCount:    embeddingTraceInputCount(req),
		InputTokens:   inputTokens,
		ResponseModel: model,
		Dimensions:    embeddingTraceDimensions(req),
	})
}

func finishEmbeddingTraceWithError(recorder llmtrace.EmbeddingRecorder, err error, code string) {
	if recorder == nil || err == nil {
		return
	}
	recorder.SetError(err, code)
	recorder.End()
}

func embeddingTraceDimensions(req *EmbeddingRequest) *int64 {
	if req == nil || !req.Dimensions.Valid() {
		return nil
	}
	value := req.Dimensions.Value
	return &value
}

func embeddingTraceEncodingFormat(req *EmbeddingRequest) string {
	if req == nil {
		return ""
	}
	return string(req.EncodingFormat)
}

func embeddingTraceInputCount(req *EmbeddingRequest) int {
	if req == nil {
		return 0
	}
	switch {
	case req.Input.OfString.String() != "":
		return 1
	case len(req.Input.OfArrayOfStrings) > 0:
		return len(req.Input.OfArrayOfStrings)
	case len(req.Input.OfArrayOfTokenArrays) > 0:
		return len(req.Input.OfArrayOfTokenArrays)
	case len(req.Input.OfArrayOfTokens) > 0:
		return 1
	default:
		return 0
	}
}

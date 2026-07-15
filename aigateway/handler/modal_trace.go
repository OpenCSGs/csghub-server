package handler

import (
	"context"
	"net/http"

	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
)

const (
	modalTraceOperationGenerateContent = "generate_content"
	modalTraceOutputImage              = "image"
	modalTraceOutputText               = "text"
	modalTraceOutputVideo              = "video"
	modalTraceOutputAudio              = "audio"
)

type modalTraceStartInput struct {
	API           string
	OperationName string
	OutputType    string
	RequestID     string
	NSUUID        string
	ModelID       string
	ModelTarget   *resolvedModelTarget
	Metadata      map[string]any
}

func (h *OpenAIHandlerImpl) startModalGenerationTrace(ctx context.Context, input modalTraceStartInput) (context.Context, llmtrace.GenerationRecorder) {
	if h == nil || h.llmTracer == nil || input.ModelTarget == nil || input.ModelTarget.Model == nil {
		return ctx, nil
	}
	metadata := map[string]any{
		llmtrace.TraceMetadataKeyAIGatewayAPI:     input.API,
		llmtrace.TraceMetadataKeyAIGatewayModelID: input.ModelTarget.Model.ID,
		llmtrace.TraceMetadataKeyGenAIOutputType:  input.OutputType,
	}
	for k, v := range input.Metadata {
		if includeModalTraceMetadata(v) {
			metadata[k] = v
		}
	}
	return h.startLLMTrace(ctx, types.GenerationStart{
		RequestID:     input.RequestID,
		UserID:        input.NSUUID,
		Provider:      input.ModelTarget.Model.Provider,
		RequestModel:  input.ModelID,
		ResolvedModel: input.ModelTarget.ModelName,
		Mode:          types.GenerationModeSync,
		OperationName: input.OperationName,
		Metadata:      metadata,
	}, false)
}

func includeModalTraceMetadata(v any) bool {
	switch value := v.(type) {
	case nil:
		return false
	case string:
		return value != ""
	default:
		return true
	}
}

type modalTraceCompletionInput struct {
	Recorder   llmtrace.GenerationRecorder
	Provider   string
	Model      string
	Usage      *token.Usage
	StatusCode int
	Metadata   map[string]any
	Artifacts  []types.GenerationArtifact
}

func recordModalGenerationTraceCompletion(input modalTraceCompletionInput) {
	if input.Recorder == nil {
		return
	}
	input.Recorder.SetResponse(types.GenerationResponse{
		Provider:      input.Provider,
		Model:         input.Model,
		ResponseModel: input.Model,
		Metadata:      input.Metadata,
		Artifacts:     input.Artifacts,
	})
	if input.StatusCode >= http.StatusBadRequest {
		input.Recorder.SetError(httpStatusTraceError(input.StatusCode), types.TraceErrUpstreamError)
	}
	recordLLMTraceUsage(input.Recorder, input.Usage)
}

func finishModalGenerationTraceWithError(recorder llmtrace.GenerationRecorder, err error, code string) {
	finishLLMTraceWithError(recorder, err, code)
}

func videoTraceCompletionMetadata(videoResp *types.VideoObject, input *createVideoInput) map[string]any {
	if videoResp == nil {
		return nil
	}
	metadata := map[string]any{
		llmtrace.TraceMetadataKeyVideoID:      videoResp.ID,
		llmtrace.TraceMetadataKeyVideoStatus:  videoResp.Status,
		llmtrace.TraceMetadataKeyVideoSize:    videoResp.Size,
		llmtrace.TraceMetadataKeyVideoSeconds: videoResp.Seconds,
	}
	if input != nil {
		if metadata[llmtrace.TraceMetadataKeyVideoSize] == "" {
			metadata[llmtrace.TraceMetadataKeyVideoSize] = input.adapterReq.Size
		}
		if metadata[llmtrace.TraceMetadataKeyVideoSeconds] == int64(0) {
			metadata[llmtrace.TraceMetadataKeyVideoSeconds] = input.adapterReq.Seconds
		}
	}
	return metadata
}

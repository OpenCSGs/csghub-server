package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"
	"strings"
	"time"

	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func (h *OpenAIHandlerImpl) startResponsesTrace(ctx context.Context, headers http.Header, modelID string, modelTarget *resolvedModelTarget, req *types.ResponsesRequest, decision responsespkg.RoutingDecision, requestID string, userID string) (context.Context, llmtrace.GenerationRecorder) {
	if h == nil || h.llmTracer == nil || modelTarget == nil || modelTarget.Model == nil || req == nil {
		return ctx, nil
	}
	mode := types.GenerationModeSync
	if req.Stream {
		mode = types.GenerationModeStream
	}
	return h.startLLMTrace(ctx, types.GenerationStart{
		RequestID:      requestID,
		ConversationID: extractChatSessionID(headers),
		UserID:         userID,
		Provider:       modelTarget.Model.Provider,
		RequestModel:   modelID,
		ResolvedModel:  modelTarget.ModelName,
		Mode:           mode,
		Tools:          responsesTraceTools(req),
		ToolCount:      responsesTraceToolCount(req),
		MaxTokens:      responsesTraceMaxOutputTokens(req),
		Temperature:    req.Temperature,
		TopP:           req.TopP,
		ToolChoice:     responsesTraceToolChoice(req),
		Metadata: map[string]any{
			llmtrace.TraceMetadataKeyAIGatewayAPI:           "/v1/responses",
			llmtrace.TraceMetadataKeyAIGatewayModelID:       modelTarget.Model.ID,
			llmtrace.TraceMetadataKeyResponsesExecutionMode: string(decision.Mode),
		},
	}, req.Stream)
}

type responsesTraceToolDefinition struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
	Function    struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

func responsesTraceTools(req *types.ResponsesRequest) []types.GenerationToolDefinition {
	if req == nil || len(req.Tools) == 0 {
		return nil
	}
	var rawTools []json.RawMessage
	if err := json.Unmarshal(req.Tools, &rawTools); err != nil {
		return nil
	}
	tools := make([]types.GenerationToolDefinition, 0, len(rawTools))
	for _, rawTool := range rawTools {
		var parsed responsesTraceToolDefinition
		if err := json.Unmarshal(rawTool, &parsed); err != nil {
			continue
		}
		name := parsed.Function.Name
		if name == "" {
			name = parsed.Name
		}
		if name == "" {
			continue
		}
		desc := parsed.Function.Description
		if desc == "" {
			desc = parsed.Description
		}
		params := parsed.Function.Parameters
		if len(params) == 0 {
			params = parsed.Parameters
		}
		toolType := parsed.Type
		if toolType == "" {
			toolType = "function"
		}
		tools = append(tools, types.GenerationToolDefinition{
			Name:        name,
			Description: desc,
			Type:        toolType,
			InputSchema: params,
		})
	}
	return tools
}

func responsesTraceToolCount(req *types.ResponsesRequest) int {
	if req == nil || len(req.Tools) == 0 {
		return 0
	}
	var tools []json.RawMessage
	if err := json.Unmarshal(req.Tools, &tools); err != nil {
		return 0
	}
	return len(tools)
}

func responsesTraceToolChoice(req *types.ResponsesRequest) *string {
	if req == nil || len(req.ToolChoice) == 0 {
		return nil
	}
	value := strings.TrimSpace(string(req.ToolChoice))
	if value == "" || value == "null" || value == "{}" {
		return nil
	}
	var stringValue string
	if err := json.Unmarshal(req.ToolChoice, &stringValue); err == nil {
		if stringValue = strings.TrimSpace(stringValue); stringValue != "" {
			return &stringValue
		}
		return nil
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, req.ToolChoice); err == nil {
		compactValue := compacted.String()
		return &compactValue
	}
	return &value
}

func responsesTraceMaxOutputTokens(req *types.ResponsesRequest) *int64 {
	if req == nil || req.MaxOutputTokens == nil || *req.MaxOutputTokens == 0 {
		return nil
	}
	value := int64(*req.MaxOutputTokens)
	return &value
}

type responsesTracePostProcessInput struct {
	Recorder     llmtrace.GenerationRecorder
	Completion   bool
	Stream       bool
	FirstWriteAt time.Time
	StatusCode   int
}

func newResponsesTracePostProcessInput(recorder llmtrace.GenerationRecorder, req *types.ResponsesRequest, statusCode int, firstWriteAt time.Time) responsesTracePostProcessInput {
	input := responsesTracePostProcessInput{
		Recorder:     recorder,
		Completion:   recorder != nil,
		StatusCode:   statusCode,
		FirstWriteAt: firstWriteAt,
	}
	if req != nil {
		input.Stream = req.Stream
	}
	return input
}

func recordResponsesTraceCompletion(input responsesTracePostProcessInput, provider string, model string, usage *token.Usage, inputMsgs []types.GenerationMessage, outputMsgs []types.GenerationMessage, traceInfo commontypes.LLMLogTraceInfo) {
	if input.Recorder == nil {
		return
	}
	if input.Completion {
		firstChunkAt := time.Time{}
		if input.Stream {
			firstChunkAt = input.FirstWriteAt
		}
		recordLLMTraceCompletion(llmTraceCompletionInput{
			Recorder:      input.Recorder,
			Provider:      provider,
			Model:         model,
			Usage:         usage,
			Input:         inputMsgs,
			Output:        outputMsgs,
			ResponseID:    traceInfo.ResponseID,
			FinishReasons: traceInfo.FinishReasons,
			FirstChunkAt:  firstChunkAt,
			StatusCode:    input.StatusCode,
		})
		return
	}
	recordLLMTraceUsage(input.Recorder, usage)
}

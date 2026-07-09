package handler

import (
	"context"
	"encoding/json"
	"net/http"
	responsespkg "opencsg.com/csghub-server/aigateway/handler/responses"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	llmtrace "opencsg.com/csghub-server/aigateway/component/trace"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestStartResponsesTrace(t *testing.T) {
	recorder := &testGenerationRecorderWithMutex{}
	tracer := &testLLMTracerWithMutex{recorder: recorder}
	h := &OpenAIHandlerImpl{llmTracer: tracer}
	maxTokens := 128
	temperature := 0.2
	topP := 0.9
	req := &types.ResponsesRequest{
		Model:           "public-model",
		Stream:          true,
		MaxOutputTokens: &maxTokens,
		Temperature:     &temperature,
		TopP:            &topP,
		Tools: json.RawMessage(`[{
			"type":"function",
			"function":{"name":"lookup","description":"Lookup data","parameters":{"type":"object"}}
		}]`),
		ToolChoice: json.RawMessage(`"auto"`),
	}
	headers := http.Header{}
	headers.Set(sessionHeaderSessionID, "session-1")
	modelTarget := &resolvedModelTarget{
		Model:     &types.Model{BaseModel: types.BaseModel{ID: "internal-model"}, ExternalModelInfo: types.ExternalModelInfo{Provider: "openai"}},
		ModelName: "upstream-model",
	}

	_, got := h.startResponsesTrace(context.Background(), headers, "public-model", modelTarget, req, responsespkg.RoutingDecision{Mode: responsespkg.ResponsesModeNative}, "req-1", "user-1")

	require.Same(t, recorder, got)
	require.Equal(t, []bool{true}, tracer.Streams())
	starts := tracer.Starts()
	require.Len(t, starts, 1)
	require.Equal(t, "req-1", starts[0].RequestID)
	require.Equal(t, "session-1", starts[0].ConversationID)
	require.Equal(t, "user-1", starts[0].UserID)
	require.Equal(t, "openai", starts[0].Provider)
	require.Equal(t, "public-model", starts[0].RequestModel)
	require.Equal(t, "upstream-model", starts[0].ResolvedModel)
	require.Equal(t, types.GenerationModeStream, starts[0].Mode)
	require.Equal(t, int64(128), *starts[0].MaxTokens)
	require.Equal(t, 0.2, *starts[0].Temperature)
	require.Equal(t, 0.9, *starts[0].TopP)
	require.Equal(t, "auto", *starts[0].ToolChoice)
	require.Len(t, starts[0].Tools, 1)
	require.Equal(t, "lookup", starts[0].Tools[0].Name)
	require.Equal(t, 1, starts[0].ToolCount)
	require.Equal(t, "/v1/responses", starts[0].Metadata[llmtrace.TraceMetadataKeyAIGatewayAPI])
	require.Equal(t, "internal-model", starts[0].Metadata[llmtrace.TraceMetadataKeyAIGatewayModelID])
	require.Equal(t, string(responsespkg.ResponsesModeNative), starts[0].Metadata[llmtrace.TraceMetadataKeyResponsesExecutionMode])
}

func TestRecordResponsesTraceCompletion(t *testing.T) {
	firstWriteAt := time.Now()
	recorder := &testGenerationRecorderWithMutex{}
	input := responsesTracePostProcessInput{
		Recorder:     recorder,
		Completion:   true,
		Stream:       true,
		FirstWriteAt: firstWriteAt,
		StatusCode:   http.StatusOK,
	}

	recordResponsesTraceCompletion(input, "openai", "upstream-model", nil, []types.GenerationMessage{{Role: "user"}}, []types.GenerationMessage{{Role: "assistant"}}, commontypes.LLMLogTraceInfo{})

	response, firstChunk, errorCode, ended, events := recorder.traceSnapshot()
	require.NotNil(t, response)
	require.Equal(t, "openai", response.Provider)
	require.Equal(t, "upstream-model", response.Model)
	require.Len(t, response.Input, 1)
	require.Len(t, response.Output, 1)
	require.NotNil(t, firstChunk)
	require.Equal(t, firstWriteAt, firstChunk.At)
	require.Empty(t, errorCode)
	require.False(t, ended)
	require.Equal(t, []string{"first_chunk", "response"}, events)
}

func TestRecordResponsesTraceCompletionMarksHTTPError(t *testing.T) {
	recorder := &testGenerationRecorderWithMutex{}

	recordResponsesTraceCompletion(responsesTracePostProcessInput{
		Recorder:   recorder,
		Completion: true,
		StatusCode: http.StatusBadGateway,
	}, "openai", "upstream-model", nil, nil, nil, commontypes.LLMLogTraceInfo{})

	_, _, errorCode, ended, events := recorder.traceSnapshot()
	require.Equal(t, types.TraceErrUpstreamError, errorCode)
	require.False(t, ended)
	require.Equal(t, []string{"response", "error"}, events)
}

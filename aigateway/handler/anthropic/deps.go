package anthropic

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/shared"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

// Deps bundles all dependencies required by the anthropic Handler.
//
// The Handler owns its Execute-phase logic (proxy, auth headers, SSE
// headers, usage recording, metrics, usage-limit commit, preflight tracing,
// LLM trace, and LLM log publishing).  Each dependency is an explicit
// interface so the Handler has no compile-time dependency on the handler
// package — the handler package provides concrete adapters.
type Deps struct {
	// ProxyExecutor executes a reverse proxy request to the upstream.
	ProxyExecutor ProxyExecutor
	// UsageRecorder records token usage for billing/metering.
	UsageRecorder UsageRecorder
	// UsageLimiter commits usage quota after the upstream response.
	UsageLimiter UsageLimiter
	// MetricsRecorder records request-level metrics.
	MetricsRecorder MetricsRecorder

	// LLMTracer is optional. When set, the Handler starts an LLM generation
	// trace before proxying and records completion/error after.
	LLMTracer LLMTracer
	// LLMLogRecorderFactory creates an LLM log recorder for capturing
	// input/output messages for training log publishing.
	LLMLogRecorderFactory LLMLogRecorderFactory
	// LLMLogPublisher is optional. When set, the Handler publishes the
	// captured LLM log record asynchronously after the response completes.
	LLMLogPublisher LLMLogPublisher
}

// ProxyExecutor executes a reverse proxy request to the upstream and writes
// the response through the provided writer.
type ProxyExecutor interface {
	ServeProxy(c *gin.Context, backendURL, host string, responseWriter types.HTTPResponseWriter)
}

// UsageRecorder records token usage for billing/metering.
type UsageRecorder interface {
	RecordUsage(ctx context.Context, nsUUID string, model *types.Model, targetModelName string, inputTokens, outputTokens, cachedPromptTokens, cacheCreationPromptTokens int64, apikey string) error
}

// UsageLimiter commits usage quota limits after the upstream response.
type UsageLimiter interface {
	CommitUsageLimitFromUsage(ctx context.Context, nsUUID string, model *types.Model, inputTokens, outputTokens, cachedPromptTokens, cacheCreationPromptTokens int64) error
}

// MetricsRecorder records request-level metrics (model, provider, token usage).
type MetricsRecorder interface {
	SetModelTarget(c *gin.Context, modelID string, target *types.ModelTarget, isStream bool)
	RecordTokenUsage(c *gin.Context, inputTokens, outputTokens, cachedPromptTokens int64)
}

// PreflightTracer records a preflight span that covers the full request
// lifecycle.  The span is started by the entry-point handler (e.g.
// AnthropicHandlerImpl.Messages) and stored in the gin.Context so the
// protocol handler can record model-resolution attributes and errors
// during Execute and HandlePlanError.
type PreflightTracer interface {
	// RecordError records an error on the preflight span and ends it.
	RecordError(err error, errorType string)
	// SetTargetModel records the resolved model target attributes.
	SetTargetModel(requestModel string, target *types.ModelTarget)
	// End ends the preflight span.
	End()
}

// preflightTracerCtxKey is the gin.Context key for the per-request PreflightTracer.
const preflightTracerCtxKey = "anthropic.preflight_tracer"

// SetPreflightTracer stores a PreflightTracer in the gin.Context so that
// Execute and HandlePlanError can access it.  Called by the entry-point
// handler (AnthropicHandlerImpl.Messages) before Dispatch.
func SetPreflightTracer(c *gin.Context, t PreflightTracer) {
	c.Set(preflightTracerCtxKey, t)
}

// GetPreflightTracer retrieves the PreflightTracer from the gin.Context.
// Returns nil if none was set.
func GetPreflightTracer(c *gin.Context) PreflightTracer {
	v, exists := c.Get(preflightTracerCtxKey)
	if !exists {
		return nil
	}
	t, _ := v.(PreflightTracer)
	return t
}

// LLMTracer starts an LLM generation trace.  The returned GenerationRecorder
// captures usage, response, and error information for the trace.
type LLMTracer interface {
	StartGeneration(ctx context.Context, input types.GenerationStart) (context.Context, GenerationRecorder)
}

// GenerationRecorder records an LLM generation trace.
type GenerationRecorder interface {
	SetUsage(usage types.TokenUsage)
	SetResponse(response types.GenerationResponse)
	SetFirstChunk(firstChunk types.GenerationFirstChunk)
	SetError(err error, code string)
	End()
}

// LLMLogRecorder captures input/output messages for training log publishing.
type LLMLogRecorder interface {
	Completion(resp types.ChatCompletion)
	AppendCompletionChunk(chunk types.ChatCompletionChunk)
	Record() (*commontypes.LLMLogRecord, error)
	Messages() (input, output []commontypes.LLMLogMessage)
	TraceInfo() commontypes.LLMLogTraceInfo
}

// LLMLogRecorderFactory creates an LLM log recorder for a request.
type LLMLogRecorderFactory interface {
	NewLLMLogRecorder(requestID, modelID, userUUID string, req commontypes.LLMLogRequest, metadata map[string]any) (LLMLogRecorder, error)
}

// LLMLogPublisher publishes training log records.
type LLMLogPublisher interface {
	PublishTrainingLog(message []byte) error
}

// SensitiveResult is a protocol-agnostic representation of a sensitive
// check result.
type SensitiveResult struct {
	IsSensitive bool
	Message     string
}

// postProcessInput holds the data needed for async post-processing after
// the upstream response completes.  It mirrors handler.runChatPostProcessAsync.
type postProcessInput struct {
	NSUUID          string
	ApiKey          string
	Model           *types.Model
	TargetModelName string
	Usage           *tokenUsage
	LogCapture      LLMLogRecorder
	Trace           tracePostProcessInput
	StatusCode      int
}

type tracePostProcessInput struct {
	Recorder     GenerationRecorder
	Completion   bool
	Stream       bool
	FirstWriteAt time.Time
	StatusCode   int
}

// tokenUsage is the local usage struct used by the anthropic package.
type tokenUsage struct {
	PromptTokens              int64
	CompletionTokens          int64
	TotalTokens               int64
	CachedPromptTokens        int64
	CacheCreationPromptTokens int64
}

// anthropicRequestToLLMLogRequest converts an AnthropicMessagesRequest into
// the OpenAI-typed LLMLogRequest expected by the LLM log recorder.
// The recorder's normalizeLogMessages does a JSON round-trip, so the messages
// just need to serialize to valid OpenAI Chat message structures.
func anthropicRequestToLLMLogRequest(req *types.AnthropicMessagesRequest) (commontypes.LLMLogRequest, error) {
	chatMsgs, err := messagesToChatMessages(req, true)
	if err != nil {
		return commontypes.LLMLogRequest{}, err
	}

	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(chatMsgs))
	for _, m := range chatMsgs {
		role, _ := m["role"].(string)
		switch role {
		case "system":
			systemText := jsonStringFromAny(m["content"])
			messages = append(messages, openai.SystemMessage(systemText))
		case "tool":
			toolCallID, _ := m["tool_call_id"].(string)
			toolText := jsonStringFromAny(m["content"])
			messages = append(messages, openai.ToolMessage(toolText, toolCallID))
		case "assistant":
			msg := buildAssistantMessageParam(m)
			messages = append(messages, msg)
		default: // "user" and anything else
			userText := jsonStringFromAny(m["content"])
			messages = append(messages, openai.UserMessage(userText))
		}
	}

	var tools []openai.ChatCompletionToolUnionParam
	if len(req.Tools) > 0 {
		tools = make([]openai.ChatCompletionToolUnionParam, 0, len(req.Tools))
		for _, tool := range req.Tools {
			schema := tool.InputSchema
			if len(schema) == 0 {
				schema = json.RawMessage(`{"type":"object"}`)
			}
			var params shared.FunctionParameters
			if err := json.Unmarshal(schema, &params); err != nil {
				params = shared.FunctionParameters{"type": "object"}
			}
			fn := shared.FunctionDefinitionParam{
				Name:       tool.Name,
				Parameters: params,
			}
			if tool.Description != "" {
				fn.Description = param.NewOpt(tool.Description)
			}
			tools = append(tools, openai.ChatCompletionFunctionTool(fn))
		}
	}

	return commontypes.LLMLogRequest{
		Messages: messages,
		Tools:    tools,
		Stream:   req.Stream,
	}, nil
}

// jsonStringFromAny converts a content value (string or []any of text parts)
// to a plain string, mirroring normalizeMessageContent in llmlog_capture_ee.go.
func jsonStringFromAny(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			part, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if partType, _ := part["type"].(string); partType == "text" {
				if text, _ := part["text"].(string); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return joinStrings(parts, "")
	case map[string]any:
		data, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(data)
	default:
		return ""
	}
}

// buildAssistantMessageParam builds an assistant ChatCompletionMessageParamUnion
// from a converted chat message map, including tool_calls.
//
// Note: reasoning_content is not set because the openai-go v3 SDK's
// ChatCompletionAssistantMessageParam does not expose a reasoning_content
// field.  This only affects LLM training log capture (not the client-facing
// response), and is consistent with the SDK's own limitations.
func buildAssistantMessageParam(m map[string]any) openai.ChatCompletionMessageParamUnion {
	content, _ := m["content"].(string)
	msg := openai.ChatCompletionMessageParamUnion{}
	assistant := &openai.ChatCompletionAssistantMessageParam{}
	if content != "" {
		assistant.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
			OfString: param.NewOpt(content),
		}
	}
	if toolCalls, ok := m["tool_calls"].([]any); ok && len(toolCalls) > 0 {
		for _, tc := range toolCalls {
			tcMap, ok := tc.(map[string]any)
			if !ok {
				continue
			}
			fn, _ := tcMap["function"].(map[string]any)
			id, _ := tcMap["id"].(string)
			name, _ := fn["name"].(string)
			args, _ := fn["arguments"].(string)
			assistant.ToolCalls = append(assistant.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
					ID:   id,
					Type: "function",
					Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
						Name:      name,
						Arguments: args,
					},
				},
			})
		}
	}
	msg.OfAssistant = assistant
	return msg
}

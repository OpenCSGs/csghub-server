# AIGateway Agent Trace Plan

## Summary

Add AIGateway-side LLM generation observability for `/v1/chat/completions` through an AIGateway-owned trace abstraction.

AIGateway will trace the LLM generation part of an Agent run: model routing, provider call, token usage, latency, errors, streaming mode, and session correlation. It will not attempt to model full Agent Runtime steps such as planner, memory, sandbox, or tool server execution. Those belong to later Agent Runtime / Tool Runtime instrumentation.

The v1 implementation emits native OpenTelemetry GenAI spans and metrics through Grafana Sigil SDK instrumentation mode. Sigil is used as an implementation detail behind the AIGateway trace interface; this version does not configure or use Sigil generation export.

Existing OTel setup remains the baseline transport path:

```text
AIGateway
  -> existing OTel SDK / otelgin request span
  -> AIGateway LLM trace interface
       -> Sigil SDK implementation
       -> OTel GenAI spans + metrics
  -> OTLP Collector / Alloy
  -> Grafana Tempo / Prometheus / self-hosted Grafana
```

## Key Decisions

- Define an AIGateway-owned trace interface; handlers must not depend on Sigil types.
- Use native OpenTelemetry GenAI spans/metrics as the required v1 output.
- Use Sigil SDK as the v1 implementation of the AIGateway trace interface.
- Configure Sigil for instrumentation mode only; do not add Sigil generation export config in this version.
- When tracing is disabled, do not create or call a recorder from the handler path.
- Trace instrumentation must be independent of the llm log feature and must not require llm log config, types, publishers, or build tags.
- Do not store prompt, response, message text, tool arguments, or tool results in span attributes by default.

## Proposed Package Shape

```text
aigateway/component/trace
  llm_tracer.go     // AIGateway-owned trace contracts
  sigil.go          // Sigil SDK implementation for OTel instrumentation

aigateway/types/trace.go  // Shared trace DTOs used by handlers and trace implementations
```

Core interface:

```go
type LLMTracer interface {
    StartGeneration(ctx context.Context, input types.GenerationStart) (context.Context, GenerationRecorder)
    StartStreamingGeneration(ctx context.Context, input types.GenerationStart) (context.Context, GenerationRecorder)
    Shutdown(ctx context.Context) error
}

type GenerationRecorder interface {
    SetUsage(ctx context.Context, usage types.TokenUsage)
    SetResponse(ctx context.Context, response types.GenerationResponse)
    SetFirstChunk(ctx context.Context, firstChunk types.GenerationFirstChunk)
    SetError(ctx context.Context, err error, code string)
    End(ctx context.Context)
}
```

Core model:

```go
type GenerationStart struct {
    RequestID           string
    ConversationID      string
    ConversationTitle   string
    UserID              string
    AgentName           string
    AgentVersion        string
    Provider            string
    RequestModel        string
    ResolvedModel       string
    Mode                types.GenerationMode // sync | stream
    OperationName       string
    SystemPrompt        string
    Input               []GenerationMessage
    Tools               []GenerationToolDefinition
    ToolCount           int
    MaxTokens           *int64
    Temperature         *float64
    TopP                *float64
    ToolChoice          *string
    ThinkingEnabled     *bool
    ParentGenerationIDs []string
    EffectiveVersion    string
    Tags                map[string]string
    Metadata            map[string]any
    StartedAt           time.Time
}

type TokenUsage struct {
    InputTokens     int64
    OutputTokens    int64
    TotalTokens     int64
    ReasoningTokens int64
}

type GenerationResponse struct {
    Provider      string
    Model         string
    TraceID       string
    SpanID        string
    ResponseID    string
    ResponseModel string
    SystemPrompt  string
    Input         []GenerationMessage
    Output        []GenerationMessage
    Tools         []GenerationToolDefinition
    StopReason    string
    FinishReasons []string
    CompletedAt   time.Time
    Tags          map[string]string
    Metadata      map[string]any
    Artifacts     []GenerationArtifact
    CallError     string
}
```

## Trace Model

AIGateway creates one logical generation span per chat completion request:

```text
HTTP request span: POST /v1/chat/completions
  -> span: gen_ai.client.operation
       attributes:
         gen_ai.operation.name = "chat" or mode-aware operation name
         gen_ai.provider.name = provider
         gen_ai.request.model = requested model
         gen_ai.response.model = upstream model name
         gen_ai.usage.input_tokens
         gen_ai.usage.output_tokens
         gen_ai.usage.total_tokens
         http.request_id = request_id
         session.id = normalized session_id
         enduser.id = namespace/user uuid
         aigateway.model.id
```

Do not add AIGateway-owned attempt spans in v1. Retry/fallback behavior remains in the existing fallback reporter and logs. If attempt-level trace becomes necessary later, it should be designed separately or implemented through Sigil SDK support rather than adding custom OTel span logic to the chat proxy path.

Error handling:

- If all attempts fail, set the generation span status to error.
- Sensitive prompt block sets the generation span status to error with reason `sensitive_prompt`.
- Insufficient balance and usage limit errors set generation error with their existing error codes.
- Successful fallback after failed primary leaves the logical generation successful; fallback details remain available through existing fallback observability.

## Metrics

Emit OTel GenAI metrics through the existing meter provider:

- `gen_ai.client.operation.duration`
- `gen_ai.client.token.usage`
- Sigil SDK first-token / first-chunk metric support for streaming requests when first chunk timestamp is available.

Metric labels must remain low-cardinality. Do not use raw `session_id`, user UUID, request ID, upstream URL, or session key hash as metric attributes.

## Session ID Handling

Normalize session ID at the AIGateway boundary and use it only for trace/log correlation.

Precedence:

```text
1. X-Claude-Code-Session-Id
2. X-Session-ID
3. X-Conversation-ID
4. empty
```

Behavior:

- Attach normalized `session_id` to the generation telemetry as `session.id` or implementation equivalent.
- Use existing `RoutingPolicy.SessionHeader` behavior for routing, but do not require it to match trace `session_id`.
- Do not add `session_id` to upstream request bodies.
- Preserve current raw-body passthrough behavior for unknown fields.
- Forward provider-specific session headers only if already present in the incoming request or a later provider config explicitly enables forwarding.

## Configuration

Required baseline:

- `EnableLLMTrace bool` under `AIGateway`.
- Env `OPENCSG_AIGATEWAY_LLM_TRACE_ENABLE`, default `true`.
- Effective OTel export still depends on `OPENCSG_TRACING_OTLP_ENDPOINT`; without it, existing OTel setup remains no-op.

Sigil implementation:

- `LLMTraceContentCapture string`, env `OPENCSG_AIGATEWAY_LLM_TRACE_CONTENT_CAPTURE`, default `metadata_only`.

Configure Sigil with generation export protocol `none` and use it only to emit OTel spans/metrics through the existing tracer/meter providers.

## Sigil Mapping

Add dependency `github.com/grafana/sigil-sdk/go` and initialize one Sigil client during AIGateway startup or handler construction. Shut it down during server shutdown.

AIGateway contract to Sigil mapping:

```text
GenerationStart.RequestID       -> generation id
GenerationStart.ConversationID  -> conversation_id
GenerationStart.UserID          -> user_id
GenerationStart.AgentName       -> agent_name
GenerationStart.AgentVersion    -> agent_version
GenerationStart.Provider        -> model.provider
GenerationStart.ResolvedModel   -> model.name
GenerationStart.Mode            -> SYNC / STREAM, generateText / streamText
GenerationStart.OperationName   -> operation_name override
GenerationStart.MaxTokens       -> request max_tokens
GenerationStart.Temperature     -> request temperature
GenerationStart.TopP            -> request top_p
GenerationStart.ToolChoice      -> request tool_choice
GenerationStart.ThinkingEnabled -> thinking enabled
GenerationStart.Tags/Metadata   -> Sigil tags/metadata
GenerationStart.ToolCount       -> tool count metadata only by default
TokenUsage                      -> usage input/output/total/reasoning tokens
GenerationResponse              -> final provider/model, response model/id/content/tools/artifacts/finish reason
```

The mapping must keep AIGateway's model as the source of truth. If Sigil lacks a direct field, use bounded metadata keys under `aigateway.*`.

Content capture:

- Default `metadata_only`.
- `metadata_only` records structure, timing, model, token usage, tool names, and metadata, but not prompt/response text or tool arguments/results.
- Optional modes:
  - `metadata_only`
  - `no_tool_content`
  - `full`
- `full` must be treated as debug/private-deployment mode and requires explicit config.
- OTel spans and metrics must never include prompt/response/tool content by default.

## Implementation Notes

- Hook tracing in `OpenAIHandlerImpl.Chat` after request parsing and model target resolution, before the first proxy attempt.
- Use the existing `trace.GetTraceIDInGinContext(c)` as `request_id` so logs, traces, and future recorders can correlate without depending on llm log.
- Start a generation recorder before balance/sensitive/upstream paths that should be represented in trace.
- Keep `executeChatProxyAttempt` focused on proxy behavior; do not add trace-specific attempt control flow to it.
- Before calling `rp.ServeHTTP`, inject the current generation context into `c.Request.Header` with the global OTel propagator so upstream services receive the correct `traceparent`.
- Re-inject `traceparent` for each fallback attempt because `c.Request.Header` is reused.
- Reuse `tokenCounter.Usage(ctx)` after proxy completion to set token attributes and emit token metrics.
- Keep implementation in shared AIGateway code unless the Sigil dependency itself must be gated by edition policy.

## Production Readiness Requirements

- No prompt/response/tool argument/tool result text in spans or metrics by default.
- Sigil generation export is out of scope for this version.
- Recorder `End` must be idempotent and safe on early returns.
- Streaming must handle `[DONE]`, upstream EOF, client disconnect, moderation block, and provider error without leaking buffers or goroutines.
- Metric attributes must be low-cardinality.
- Secrets such as API keys and bearer tokens must never be recorded.

## Assumptions

- v1 scope is AIGateway LLM generation tracing only.
- Full Agent trace requires later Agent Runtime, Tool Runtime, Sandbox, and Memory instrumentation.
- AIGateway owns the trace interface and data model.
- Sigil SDK is an implementation detail, not the handler-layer contract.
- Native OTel spans/metrics are the required baseline output.
- Sigil generation export is out of scope for this version.
- Prompt and response content are not captured in OTel spans by default.
- No database schema or migration is needed for this plan.

# AIGateway Generation Pipeline Compatibility Spec

## Summary

AIGateway's usage metering, moderation, LLM tracing, and LLM log flow is still mostly centered on `/v1/chat/completions`. As AIGateway adds `/v1/responses` and later Anthropic-compatible `/v1/messages`, these cross-cutting behaviors should move into a shared generation pipeline.

The public API contracts stay unchanged. Each public API keeps its own request/response DTOs and provider execution logic. The shared layer owns the stable lifecycle: async usage calculation, usage-limit commit, accounting, trace completion/end, LLM log publishing, and eventually protocol-neutral output moderation.

Do not normalize every payload too early. Normalize lifecycle artifacts first.

## Actual Go Shape

The pipeline is a function call, not a middleware framework.

```text
Public API Handler
  -> protocol-specific execution function
  -> returns GenerationContext + GenerationArtifacts
  -> runGenerationPostProcessAsync(reqCtx, context, artifacts)
```

Current Responses shape:

```text
Responses()
  -> executeNativeResponses(...)      returns responsesPostProcessInput, ok
  -> executeAdapterResponses(...)     returns responsesPostProcessInput, ok
  -> runResponsesPostProcessAsync(ctx, postProcess)
```

Future shared shape:

```go
func (h *OpenAIHandlerImpl) runGenerationPostProcessAsync(
    reqCtx context.Context,
    gc GenerationContext,
    ga GenerationArtifacts,
)
```

Contract:

- caller passes the request context
- function calls `context.WithoutCancel(reqCtx)` internally
- function spawns one goroutine and returns immediately
- goroutine owns panic recovery
- goroutine uses a bounded timeout, initially 3 seconds, matching current Chat/Responses post-process behavior
- trace usage is recorded before usage-limit commit and accounting
- LLM log publishing runs in the same goroutine after usage and trace work

## Preflight vs Post-Process

Preflight and post-process are intentionally asymmetric.

Preflight is synchronous and gating:

- balance check
- usage-limit check
- prompt moderation
- trace start
- log capture start

Any preflight failure may short-circuit the request and write an API error.

Post-process is asynchronous and effect-recording:

- token usage calculation
- trace completion/end
- usage-limit commit
- usage metering event
- LLM log publish
- output moderation where safe

Post-process runs after the response path has completed or the stream has closed. It must not depend on the request context staying alive.

## Protocol Adapter Responsibility

Protocol adapters are responsible for protocol-specific parsing and conversion:

- Chat Completions adapter handles chat request, chat response, and chat stream chunks.
- Responses native adapter handles OpenAI-compatible Responses JSON/SSE when parsing is safe.
- Responses chat-adapter adapter converts synthesized Responses output/events from Chat Completions execution.
- Future Messages adapter handles Anthropic-compatible Messages request, response, stream events, and usage fields.

Adapters should not own accounting, log publishing, or trace finalization. They should return lifecycle artifacts to the pipeline.

Adapters do own protocol-specific identity and usage interpretation:

- Responses adapter owns response ID wrapping/unwrapping and route continuity.
- Runtime only sees the final public `ResponseID` string.
- Each adapter decides whether usage is present and non-empty for its protocol.
- Each adapter sets `UsageSource`; the pipeline records it and does not re-derive source from raw usage values.

## Generation Context

Use a small protocol-neutral context for request-level metadata:

```go
type GenerationContext struct {
    APIPath         string
    BackendAPI      string
    ExecutionMode   string
    Stream          bool
    NamespaceUUID   string
    APIKey          string
    PublicModelID   string
    TargetModelName string
    UpstreamID      int64
    Provider        string

    // Initially kept for compatibility with existing accounting/component APIs.
    // Later cleanup can replace this with GenerationModelRef once accounting no
    // longer requires *types.Model.
    Model *types.Model
}
```

Longer term, replace `Model *types.Model` with a smaller `GenerationModelRef` once the accounting APIs no longer require the full model object.

## Generation Artifacts

Use protocol-neutral artifacts for lifecycle outputs:

```go
type GenerationArtifacts struct {
    Counter       token.Counter
    UsageSource   string
    LogCapture    component.LLMLogRecorder
    TraceRecorder llmtrace.GenerationRecorder

    Input         []types.GenerationMessage
    Output        []types.GenerationMessage
    ResponseID    string
    FinishReasons []string
    StatusCode    int
    FirstChunkAt  time.Time
    Error         error
    ErrorCode     string
    Incomplete    bool
}
```

`UsageSource` values:

- `upstream_usage`
- `token_counter`
- `fallback_estimate`

This is required for billing/debugging because provider usage, adapter usage, and estimates may differ.

## Runtime Behavior

### Usage

- Prefer upstream usage when the adapter marks it as present and non-empty.
- Use token counter fallback when upstream usage is missing.
- Preserve cached prompt tokens, cache creation tokens, reasoning tokens, and total tokens where available.
- Do not block native passthrough only because usage parsing is incomplete.
- Record usage source metadata for observability and billing debugging.

Adapters define non-empty usage for their own protocol:

- Chat may treat any non-zero prompt/completion/total token count as non-empty.
- Responses may also treat detail buckets as non-empty, even when top-level token counts are zero.
- Messages will define its own rule when implemented.

### Upstream Stream Errors

When an adapter sees an upstream `event: error` or equivalent stream failure:

- set `GenerationArtifacts.Error` and `ErrorCode`
- set `StatusCode` to a non-2xx value if available, otherwise use a synthetic gateway error status
- mark `Incomplete=true`
- still preserve any usage already captured
- trace must close with an explicit upstream error code
- LLM log should either be skipped or marked incomplete; do not publish it as a successful completion

### Moderation

Prompt moderation is preflight and reads protocol-native request data or request-specific normalized prompt data.

Output moderation is stream/post-process and reads protocol-neutral artifacts or synthesized output events.

Rules:

- Keep existing Chat moderation unchanged during the first Responses release.
- Responses adapter output moderation is safe because AIGateway owns the synthesized output stream.
- Native Responses output moderation must remain best-effort and must not require buffering the full native stream.
- Raw native passthrough must continue logging that output moderation is unavailable.

### LLM Tracing

- Add Responses trace start/end using the same generation recorder model as Chat.
- Start trace after model target resolution.
- Trace metadata should include `api`, `backend_api`, `execution_mode`, public model, target model, upstream ID, and provider.
- Record response ID, finish reasons, usage, status code, first chunk time, parsed input/output when available, and upstream error events.
- Sensitive prompt/output, upstream failures, invalid upstream responses, usage-limit failures, and balance failures should close traces with explicit error codes.
- Trace usage must be recorded before usage-limit commit and accounting so traces remain useful when downstream accounting fails.

### LLM Logs

- Preserve existing Chat log behavior and sample format.
- Add Responses metadata: `api=/v1/responses`, `backend_api`, `responses_execution_mode`, `response_id`, `previous_response_id` presence, and usage source.
- Native Responses logs should only capture normalized text/refusal/tool metadata parsed from known Responses events.
- Do not store raw opaque native response bodies.
- If the stream is incomplete because of upstream error, skip LLM log publish or mark the log as incomplete.

## Implementation Phases

1. Responses-local shared post-process path.
   - Status: implemented in the current MR.
   - `Responses()` owns the single post-process call.
   - Native and chat-adapter modes return post-process inputs/artifacts.

2. Responses trace start/end.
   - Start trace after model target resolution.
   - End trace through shared Responses post-process.
   - Record parsed response ID, status, usage, and errors where available.

3. Responses LLM log metadata and native parsed log support.
   - Keep adapter LLM logs based on synthesized chat-compatible output.
   - Add native logs only for safely parsed Responses fields.
   - Safely parsed means known Responses JSON/SSE fields such as `response.id`, `response.status`, `response.usage`, `output_text`, `refusal`, and function-call metadata.

4. Responses adapter output moderation.
   - Check synthesized `response.output_text.delta` or final converted output.
   - Keep native output moderation unsupported unless parsed safely without full-stream buffering.

5. Shared generation post-process helper.
   - Generalize only the async usage/log/trace lifecycle.
   - Keep Chat request execution and Responses request execution separate.
   - Migrate Chat to the helper only after regression tests prove behavior is unchanged.

6. Future Anthropic-compatible Messages API.
   - Implement a Messages adapter.
   - Convert Messages request/response/stream usage into `GenerationContext` and `GenerationArtifacts`.
   - Reuse the shared post-process helper.
   - Do not add Messages-specific accounting/logging/tracing code unless the shared artifacts are insufficient.

7. Optional formal filter interfaces.
   - Do not build a large runtime framework upfront.
   - Add filter interfaces only after Chat, Responses, and Messages share enough lifecycle behavior that duplication is clear.

## First Implementation MR Plan

MR-1: Responses post-process hardening.

- Keep Phase 1 as implemented.
- Add Responses trace start/end.
- Add upstream `event: error` artifact handling.
- Add response ID, status, and usage metadata into post-process artifacts.
- Add tests for native and adapter post-process behavior.

MR-2: Responses observability and moderation.

- Add Responses LLM log metadata.
- Add native parsed log support for safe fields only.
- Add Responses adapter output moderation.
- Add tests for incomplete stream handling and log behavior.

MR-3: Shared generation post-process extraction.

- Introduce `GenerationContext` and `GenerationArtifacts`.
- Move Responses post-process to `runGenerationPostProcessAsync`.
- Migrate Chat post-process only after snapshot/regression tests confirm behavior is unchanged.

MR-4+: Messages API support.

- Add Anthropic-compatible Messages DTOs and adapter.
- Reuse shared post-process.
- Add Messages-specific tests for usage, stream errors, trace/log metadata, and moderation boundaries.

## Test Plan

Chat regression:

- stream and non-stream usage accounting unchanged
- chat output moderation still blocks sensitive stream output
- chat LLM trace/log content unchanged
- fallback and usage-limit behavior unchanged

Responses shared post-process:

- native and chat-adapter modes both call one post-process path from `Responses()`
- upstream usage is preferred over token-counter fallback
- missing upstream usage falls back to token counter
- trace usage is recorded before accounting commit
- usage commit failure does not prevent trace end
- LLM log publishing runs in the same async post-process path
- panic recovery ends trace when a recorder exists

Responses adapter:

- prompt moderation blocks with Responses-shaped error/event
- stream output moderation checks synthesized `response.output_text.delta` when implemented
- non-stream output moderation checks converted Responses output when implemented
- LLM trace/log contains `api=/v1/responses` and `backend_api=/v1/chat/completions`
- upstream stream error marks artifacts incomplete and closes trace as failed

Native Responses:

- non-stream parses `usage`, response ID, output text, and finish status when present
- stream parses `response.completed.usage`
- native passthrough does not block if output cannot be normalized
- native output moderation limitation is logged
- trace ends correctly on upstream error, invalid response, and sensitive prompt
- upstream `event: error` does not produce a successful completion trace/log

Future Messages:

- message request converts into generation context/artifacts
- message stream usage is metered through shared post-process
- message trace/log fields use the same lifecycle metadata

## Assumptions

- No public API shape changes for `/v1/chat/completions`, `/v1/responses`, or future `/v1/messages`.
- `/v1/messages` means a future Anthropic-compatible Messages API unless a different target is explicitly chosen.
- No gateway-owned Responses conversation state is introduced.
- Native Responses state remains provider-owned.
- AIGateway only parses native payloads when safe and needed for observability, moderation, or metering.
- Chat migration should not happen before the Responses path is stable.
- Lifecycle normalization is more stable than payload normalization; avoid lossy cross-protocol payload models until required.

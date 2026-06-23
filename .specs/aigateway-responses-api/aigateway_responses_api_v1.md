# AIGateway Responses API Version 1 Design

## Goal

Support OpenAI-compatible `POST /v1/responses` as a first-class AIGateway API while keeping `/v1/chat/completions` unchanged.

The target design has two execution modes behind one public API:

1. Native OpenAI Responses passthrough for upstreams that explicitly support `/v1/responses`.
2. Responses-to-Chat adapter for existing OpenAI-compatible Chat Completions upstreams.

These are execution modes behind one coherent version 1 API, not implementation phases. Features outside the implemented surface are listed explicitly as deferred.

## Sources

- OpenAI migration guide: https://developers.openai.com/api/docs/guides/migrate-to-responses
- OpenAI Responses create API reference: https://developers.openai.com/api/reference/resources/responses/methods/create
- Hugging Face OpenResponses announcement: https://huggingface.co/blog/open-responses
- OpenResponses spec site: https://www.openresponses.org/
- OpenRouter Responses API: https://openrouter.ai/docs/api/api-reference/responses/create-responses
- Portkey gateway docs/source: https://portkey.ai/docs/product/ai-gateway/responses-api and https://github.com/Portkey-AI/gateway
- `new-api` reference implementation: https://github.com/QuantumNous/new-api
- `csghub-lite` reference implementation: https://github.com/opencsgs/csghub-lite

## Concept Distinction

OpenAI Responses API and OpenResponses are related but not identical.

OpenAI Responses API is OpenAI's proprietary agentic API primitive. It is optimized for OpenAI models and OpenAI-operated capabilities: hosted web search, file search, code interpreter, computer use, remote MCP tools, stored responses, `previous_response_id`, conversation state, background execution, encrypted reasoning, reasoning summaries, and structured output.

OpenResponses is a provider-neutral specification. Its goal is portability: a shared schema for input items, output items, tool calls, reasoning items, streaming events, and agentic loops across OpenAI, Anthropic, local models, and gateway providers.

AIGateway should use OpenAI Responses API as the product-facing compatibility target. Users expect `/v1/responses` to behave like OpenAI.

AIGateway should use OpenResponses as an internal normalization and provider-adapter reference, not as the public contract name. It is useful for mapping OpenAI, Azure OpenAI, Claude, Gemini, Qwen, MiniMax, local models, and future providers into one gateway execution model.

Internally, AIGateway should still treat features in two groups:

- Portable Responses/OpenResponses subset: text and multimodal input items, output items, function tools, tool outputs, semantic streaming, structured output, and usage mapping.
- Native-provider features: hosted tools, stored responses, `previous_response_id`, provider-managed conversations, background jobs, and provider-specific reasoning semantics.

The chat adapter should target the portable subset. OpenAI-native hosted features require native passthrough or explicit AIGateway-owned infrastructure. AIGateway should not emulate provider-owned response storage by default.

## Source Findings

OpenAI treats Responses as the recommended API for new projects, but Chat Completions remains supported. The migration is not only a route rename: `messages` becomes `input`, `response_format` becomes `text.format`, outputs become typed `output[]` items, and stateful features depend on the provider storing responses.

OpenResponses and Hugging Face define a provider-independent Responses-style API for agent workflows. This supports using OpenResponses as AIGateway's internal normalized schema while keeping OpenAI-compatible `/v1/responses` as the external contract.

OpenRouter exposes Responses/OpenResponses-style parameters including `previous_response_id` and `session_id`. `session_id` is documented as a sticky routing key, which is useful evidence that gateways can keep response-state handling lightweight by focusing on routing continuity.

LiteLLM documents two important gateway patterns:

- Response ID security: response IDs are encrypted/bound to the user so another user cannot reuse them.
- Session/deployment affinity: requests with `previous_response_id` are routed back to the same deployment.

Portkey documents a simpler adapter boundary: `previous_response_id`, `store`, retrieve, and delete are native-only features. Adapter providers do not persist responses server-side; clients should pass full history in `input`.

`new-api` separates route mode, request conversion, upstream execution, and response handling. It does not require every model/channel to declare a broad list of supported APIs:

- `relay/responses_handler.go` creates a distinct Responses relay flow.
- `relay/channel/adapter.go` adds `ConvertOpenAIResponsesRequest` to the provider adapter interface.
- `relay/channel/openai/relay_responses.go` passes native OpenAI Responses bodies through while parsing usage and stream completion events.
- `service/openaicompat/*` keeps Chat/Responses compatibility conversion out of handlers.
- OpenAI and Azure channels use channel-type-specific native Responses URL handling.
- Global/channel `PassThroughRequestEnabled` and `PassThroughBodyEnabled` can bypass conversion and send the original request body.
- Unsupported provider adaptors fail through `ConvertOpenAIResponsesRequest` rather than relying on operator-maintained capability arrays.
- The separate `ChatCompletionsToResponsesPolicy` controls the opposite direction: routing Chat Completions through a native Responses backend for selected channels/models.

`csghub-lite` shows the adapter shape needed for Codex-like clients:

- It implements `POST /v1/responses` with native-like JSON and SSE responses.
- It converts Responses `input` into chat messages.
- It preserves reasoning items where possible through `reasoning_content`.
- It converts `function_call` and `function_call_output` items into chat assistant/tool messages.
- It generates Responses SSE events from chat results.
- It returns an explicit unsupported response for `GET /v1/responses`, avoiding accidental behavior for clients probing unsupported APIs.

## Design Decisions

- Add `POST /v1/responses` as a first-class route in AIGateway.
- Keep `/v1/chat/completions` unchanged.
- Do not add Responses capability configuration to model or upstream schemas.
- Classify the selected upstream from its exact invocation URL path.
- Disable `/v1/responses` for ambiguous or unsupported endpoint paths instead of guessing provider capability.
- Return deterministic OpenAI-style errors for unsupported adapter features.
- Keep response state owned by upstream providers.
- Use AIGateway-owned response ID wrapping only for auth and routing continuity.
- Do not silently drop fields that affect semantics.
- Treat OpenAI-native hosted tools as native-only unless AIGateway explicitly implements equivalent infrastructure.
- Keep existing gateway behavior in both modes where it is already API-neutral: auth, namespace, model routing, model-name rewrite, upstream auth headers, balance checks, usage limits, fallback behavior where applicable, and accounting.
- Defer Responses-specific moderation, LLM tracing, and LLM logs until those subsystems accept Responses-native inputs, outputs, and events.

## Routing Mode Selection

AIGateway upstream URLs are exact invocation URLs, not provider base URLs. Responses mode is therefore selected by the selected upstream URL path, not by LLM config metadata or provider labels.

Rules:

- Upstream URL path ending in `/responses` uses native passthrough.
- Upstream URL path ending in `/chat/completions` uses the Responses-to-Chat adapter.
- Any other upstream URL path returns an explicit unsupported error for `/v1/responses`.
- AIGateway must not derive `/responses` from `/chat/completions`; operators that want native Responses should configure the upstream URL as the exact native Responses endpoint.
- Query strings are preserved for native passthrough, including Azure-style `api-version` parameters.

Examples:

- `https://api.openai.com/v1/responses` -> native.
- `https://cloud.infini-ai.com/maas/v1/chat/completions` -> chat adapter.
- `https://cloud.infini-ai.com/maas/v1/embeddings` -> unsupported for `/v1/responses`.
- `https://opencsg-us.openai.azure.com/openai/deployments/csg-gpt4/chat/completions?api-version=2024-02-15-preview` -> chat adapter.
- Azure native Responses must be configured directly as a URL ending in `/responses`.

For `/v1/chat/completions` only, AIGateway applies a compatibility shortcut when the configured upstream URL ends in `/responses`: it replaces the terminal path with `/chat/completions`. This is URL rewriting, not a Chat-to-Responses adapter, and assumes the upstream exposes the sibling Chat Completions endpoint.

## Architecture

```text
Client
  |
  | POST /v1/responses
  v
+----------------------------------------------------------------+
| AIGateway                                                       |
|                                                                |
|  Route handler                                                  |
|  - auth / namespace / API key already applied by router          |
|  - bind and validate Responses request                          |
|  - unwrap previous_response_id before upstream selection         |
|  - resolve public model, forcing the original upstream if set    |
|  - select mode from the exact upstream URL path                  |
|  - check balance                                                |
|        |                                                       |
|        v                                                       |
|  Responses handler                                              |
|  - dispatch native or chat-adapter execution                    |
|        |                                                       |
|        +--------------------------+----------------------------+
|                                   |                            |
|                         native responses?                      |
|                                   |                            |
|                  yes              | no                         |
|                   v               v                            |
|       Native Responses backend    Chat adapter backend          |
|       - rewrite model             - convert request             |
|       - proxy /responses          - call chat backend           |
|       - proxy/rewrite SSE IDs     - convert response/events     |
|       - parse usage if safe       - normalize usage             |
|                                   |                            |
|        +--------------------------+----------------------------+
|        v                                                       |
|  Responses ID mapper (native mode)                              |
|  - wrap upstream response IDs using authenticated encryption    |
|  - verify namespace binding                                     |
|  - resolve original upstream ID and upstream response ID         |
|        |                                                       |
|        v                                                       |
|  Common post-processing                                         |
|  - usage accounting                                             |
|  - usage-limit commit                                           |
|  - Responses moderation, tracing, and LLM logs are deferred      |
+----------------------------------------------------------------+
  |
  v
Selected upstream
```

The route handler stays thin. AIGateway owns its Responses DTOs instead of using OpenAI SDK structs as handler DTOs. Known control fields remain typed, flexible semantic fields use `json.RawMessage`, and unknown fields are preserved in `ExtraFields` for native passthrough.

Implemented internal seams:

- `types.ResponsesRequest`, `types.ResponsesResponse`, and `types.ResponsesStreamEvent`: AIGateway-owned wire DTOs.
- `ResponsesExecutionMode`: `native`, `chat_adapter`, or `disabled`.
- `responsesRoutingDecision`: mode selected from the resolved upstream URL path.
- `ResponsesIDMapper`: native response ID wrapping, namespace authorization, and upstream route continuity.
- `token.ResponsesTokenCounter`: common request/response/event accounting for both execution modes.
- `responsesAdapterNonStreamWriter`: Chat Completions JSON to Responses JSON conversion.
- `responsesAdapterStreamWriter`: Chat Completions SSE to Responses SSE conversion.
- `responsesNativeStreamWriter` and `responsesNativeNonStreamWriter`: native transport-specific forwarding.
- `responsesNativePayloadTransformer`: native ID rewriting and usage/event capture.

## Request Flow

```text
POST /v1/responses
  |
  +-- bind request
  |
  +-- require model
  |
  +-- require input unless the supported request shape permits omission
  |
  +-- if previous_response_id is present:
        verify gateway-wrapped response ID
        enforce owner/namespace binding
        extract required upstream ID and upstream response ID
  |
  +-- resolve model target, forcing the required upstream when present
  |
  +-- choose execution mode from the selected upstream URL path
  |
  +-- apply balance check
  |
  +-- dispatch selected execution mode
        |
        +-- native: apply usage-limit check,
                     rewrite model and previous_response_id,
                     proxy native request to the original upstream
        |
        +-- adapter: validate adapter-compatible features,
                     convert to chat,
                     execute existing chat mechanics,
                     synthesize Responses output
```

## Native Responses Backend

Native backend is the highest-fidelity path.

Behavior:

- Preserve all request fields except `model`.
- Rewrite public model ID to upstream model name.
- Preserve unknown request fields for OpenAI/OpenResponses forward compatibility.
- Proxy to the selected upstream URL when it is already a `/responses` endpoint.
- For `previous_response_id`, unwrap the AIGateway response ID, restore the upstream response ID, and route to the original upstream provider/deployment.
- Do not reconstruct or replay stored context inside AIGateway.
- Preserve native non-stream response fields while parsing and re-encoding JSON only when ID rewriting is required.
- Preserve native Responses SSE event names.
- Rewrite upstream response IDs to gateway-wrapped response IDs in native SSE payloads when response ID wrapping is enabled.
- Parse native `usage` when safe and map it into existing accounting.
- For streams, parse `response.completed` usage opportunistically while forwarding events.
- Disable upstream content encoding for both stream and non-stream requests because AIGateway must inspect JSON/SSE payloads for ID rewriting and usage capture.
- Relay a native upstream `event: error` frame unchanged and stop forwarding later events; do not synthesize a terminal Responses event.
- Do not block native passthrough solely because usage parsing is incomplete; record best-effort usage and add observability.

Version 1 registers only `POST /v1/responses`. Stored-response retrieval, deletion, input-item listing, cancel, compact, and input-token endpoints are deferred and are not registered as placeholder routes.

## Chat Adapter Backend

Adapter backend lets Responses clients use chat-only upstreams.

```text
Responses request
  |
  v
Request adapter
  |
  v
Existing chat execution
  |
  v
Response or stream adapter
  |
  v
Responses response/events
```

Adapter principles:

- Support the common Codex/OpenAI SDK subset.
- Preserve conversation semantics when they can be represented as chat messages.
- Return explicit errors for unsupported native-only features.
- Keep conversion deterministic and covered by unit tests.
- Use the existing chat proxy, fallback, auth-header, and retry mechanics without changing `/v1/chat/completions` response behavior.

Request support:

- String `input` becomes one user chat message.
- Message-array `input` becomes chat messages.
- `instructions` is prepended as system/developer content.
- `max_output_tokens`, `temperature`, `top_p`, `stream`, `tools`, `tool_choice`, and `parallel_tool_calls` map to chat equivalents where supported.
- `text.format` maps to chat `response_format` where supported.
- `function_call` input items become assistant tool calls.
- `function_call_output` input items become chat tool messages.
- `reasoning` is rejected in adapter mode because the current chat conversion cannot preserve Responses reasoning semantics.
- Basic text and image input parts map to chat multimodal parts when the existing chat path supports them.

Response support:

- Chat assistant text becomes one Responses `message` output item with `output_text`.
- Chat tool calls become Responses `function_call` output items.
- Chat usage maps prompt/completion/total tokens to input/output/total tokens.
- Adapter-generated response IDs use `resp_agw_adapter_*` and are response-object identifiers only.
- Adapter output includes `object: response`, `status: completed`, `model`, `created_at`, `output`, `output_text`, and `usage` when available.

Stream support:

- The adapter owns the Responses SSE event sequence.
- Text chunks emit `response.output_text.delta` and end with `response.output_text.done`.
- Tool-call chunks emit `response.function_call_arguments.delta` and `response.function_call_arguments.done` when arguments are available.
- Refusal chunks emit Responses refusal content-part events instead of being silently dropped.
- The stream ends with `response.completed` followed by `data: [DONE]`.
- Adapter mode requests chat stream usage with `stream_options.include_usage=true` for supported upstreams.
- If the chat stream omits final usage, `ResponsesTokenCounter` estimates usage from the Responses request and synthesized response/events.
- Native SSE must not go through this adapter.

Minimum text event shape:

```text
response.created
response.in_progress
response.output_item.added
response.content_part.added
response.output_text.delta
response.output_text.done
response.content_part.done
response.output_item.done
response.completed
```

## Adapter Limits

The adapter must return `400 invalid_request_error` for features that cannot be faithfully represented by Chat Completions:

- `conversation`
- hosted `prompt` objects
- `background`
- `max_tool_calls`
- `reasoning`
- native built-in tools unless the selected chat upstream explicitly supports equivalent behavior:
  - `web_search`
  - `file_search`
  - `computer_use`
  - `code_interpreter`
  - MCP tools

The error must identify the unsupported field. Silent ignore is worse than a clear failure because clients will assume OpenAI-equivalent semantics.

`previous_response_id`, `store: true`, retrieve, delete, and input-item listing are native-provider state features. The chat adapter must not emulate them by storing prompts and outputs in AIGateway. Adapter clients should pass full conversation history in `input`.

## Response ID And State

AIGateway should not own Responses conversation state by default. Upstream providers own response state, state lifetime, stored response retrieval, deletion, and `previous_response_id` replay semantics when they support native Responses.

AIGateway owns only response ID authorization and routing continuity.

Core rule:

```text
AIGateway response state = upstream route mapping + auth binding.
Upstream response state = prompts, outputs, reasoning, tools, and replay context.
```

This keeps the gateway simple and avoids storing prompts, outputs, reasoning traces, or tool outputs only to emulate provider state.

Required behavior:

- Native mode supports `previous_response_id` and forwards `store` on `POST /v1/responses`; other stored-response APIs are not registered in version 1.
- Adapter mode does not support `previous_response_id` or `store: true`; other stored-response APIs are not registered in version 1.
- Adapter clients must pass full conversation history in `input`.
- In adapter mode, omitted `store` is treated as `false`.
- In adapter mode, explicit `store: false` is accepted.
- In adapter mode, explicit `store: true` returns `400 invalid_request_error`.
- AIGateway must return explicit `400 invalid_request_error` for adapter requests that require provider-owned state.
- AIGateway does not guarantee how long an upstream response ID remains valid.
- If the upstream response ID is expired, deleted, or unknown, AIGateway should pass through or normalize the upstream error.
- AIGateway must not silently fall back to another provider when `previous_response_id` is present.

Response ID wrapping:

```text
upstream response:
  id: resp_upstream_123

gateway response:
  id: resp_agw_v1.<opaque_token>
```

Example:

```text
resp_agw_v1.D7xYpQ2m5bL8rT9nA4s6cE1wZ0rKpLmN8yQ
```

The current implementation uses AES-GCM authenticated encryption. The SHA-256 digest of `OPENCSG_AIGATEWAY_RESPONSES_ID_SECRET` is the AES key, a fresh random nonce is generated for every wrapped ID, and `resp_agw_v1` is bound as AEAD associated data. The token is base64url-encoded nonce plus ciphertext; it is not plain base64 JSON.

`OPENCSG_AIGATEWAY_RESPONSES_ID_SECRET` has a default so local deployments start without additional configuration. Production deployments should set a stable, deployment-specific secret. All replicas that serve the same gateway must use the same value.

Encrypted claims are intentionally minimal:

- upstream response ID
- selected upstream database ID
- namespace UUID owner binding

Provider label, upstream URL, public model, upstream model, timestamps, and expiry are not embedded. AIGateway does not manage upstream response lifetime. Rotating `OPENCSG_AIGATEWAY_RESPONSES_ID_SECRET` invalidates previously wrapped IDs; multi-key rotation is deferred.

Recommended invalid ID errors:

```json
{
  "error": {
    "code": "invalid_response_id",
    "message": "previous_response_id is invalid",
    "type": "invalid_request_error"
  }
}
```

```json
{
  "error": {
    "code": "response_id_forbidden",
    "message": "previous_response_id is not owned by the current namespace",
    "type": "invalid_request_error"
  }
}
```

`previous_response_id` handling:

```text
POST /v1/responses
previous_response_id = resp_agw_v1.<opaque_token>
```

Flow:

1. Verify and decrypt the AEAD token.
2. Verify namespace UUID owner binding.
3. Extract the original upstream database ID and upstream response ID.
4. Resolve the public model while requiring the original upstream ID.
5. Replace `previous_response_id` with the upstream response ID.
6. Forward the request to the upstream.
7. Wrap the new upstream response ID before returning it to the client.

If the original upstream route is unavailable because the route was removed or disabled, return a deterministic error instead of routing to another provider. If the original upstream is temporarily failing, rate-limited, or overloaded, pass through or normalize the upstream `429`, `503`, timeout, or provider error.

Recommended error:

```json
{
  "error": {
    "code": "response_route_unavailable",
    "message": "previous_response_id was created by an upstream that is no longer available",
    "type": "invalid_request_error"
  }
}
```

Streaming behavior:

- When response ID wrapping is enabled, native SSE must be parsed event-by-event.
- Preserve native SSE event names and event order.
- Rewrite only response ID fields from upstream IDs to gateway-wrapped IDs.
- Do not leak raw upstream response IDs to clients on routes that advertise gateway-wrapped `previous_response_id` support.
- Adapter SSE uses generated gateway response IDs. These IDs are valid only for the current response object and must fail with `unsupported_feature` if used as `previous_response_id`.
- Adapter-generated IDs should be distinguishable from native wrapped IDs, for example `resp_agw_adapter_*`.

Stored-response endpoints are outside version 1. AIGateway does not register retrieve, delete, input-items, cancel, compact, or input-token routes. They may be added later as explicit native passthrough features, but version 1 must not expose placeholder handlers for them.

## Usage Accounting

Normalize all usage into the existing AIGateway token model.

- Responses `input_tokens` maps to prompt tokens.
- Responses `output_tokens` maps to completion tokens.
- Responses `total_tokens` maps to total tokens.
- Cached input, cache creation, and reasoning token details map into the existing token usage fields where supported.
- Both native and adapter execution use `token.ResponsesTokenCounter`; adapter mode does not use the Chat Completions counter for final accounting.
- The counter captures the original Responses request plus the final Responses response or synthesized stream events.
- Upstream usage is authoritative when available. Otherwise the counter estimates request and output usage with the selected model tokenizer.
- Native mode captures usage from response JSON and native stream event payloads, normally the terminal `response.completed` event.
- Usage-limit commit and metering publication run asynchronously with a detached request context and a three-second timeout.

## Moderation

Responses moderation is deferred in version 1 for both native and adapter modes. The existing moderation subsystem is Chat Completions-oriented; invoking it only in adapter mode would create inconsistent behavior and native parsing would not cover all Responses item types.

Future support should accept Responses-native request items, output items, hosted-tool payloads, and SSE events directly. It must not depend on lossy conversion to chat messages.

## LLM Tracing And Logs

Responses-specific LLM tracing and LLM log publication are deferred in version 1 for both execution modes. The current chat-oriented schemas cannot faithfully represent Responses output items, reasoning, hosted tools, or native event streams.

Operational structured logs still identify `/v1/responses`, execution mode, selected upstream ID, provider label, and upstream model where the handler already has those fields. They must not expose upstream credentials, request headers, or decrypted response-ID claims.

## Error Handling

Use OpenAI-style errors:

```json
{
  "error": {
    "code": "unsupported_feature",
    "message": "conversation is not supported by AIGateway Responses API",
    "type": "invalid_request_error"
  }
}
```

Recommended codes:

- `invalid_request_error`
- `unsupported_feature`
- `model_not_found`
- `model_not_running`
- `rate_limit_exceeded`
- `insufficient_balance`
- `invalid_response_id`
- `response_id_forbidden`
- `response_route_unavailable`
- `upstream_response_invalid`

Native mode passes upstream errors through when the upstream already returns OpenAI-compatible errors. Native and adapter stream writers relay an upstream `event: error` frame and stop finalization; they do not synthesize `response.failed`, `response.incomplete`, or `response.completed` after that error.

## Test Matrix

Minimum behavior tests:

- Native JSON response ID is wrapped before returning to the client.
- Native `previous_response_id` is unwrapped and routed to the original upstream.
- Raw upstream response ID is rejected when gateway wrapping is required.
- Wrapped response ID from another namespace is rejected.
- Malformed or unverifiable wrapped response ID returns `invalid_response_id`.
- Removed or disabled original upstream returns `response_route_unavailable`.
- Temporary upstream `429` or `503` is passed through or normalized as an upstream error, not `response_route_unavailable`.
- Adapter `previous_response_id` returns `unsupported_feature`.
- Adapter `store: true` returns `unsupported_feature`.
- Adapter omitted `store` and `store: false` succeed.
- Adapter-generated response ID fails if reused as `previous_response_id`.
- Native SSE response IDs are rewritten to gateway-wrapped IDs when wrapping is enabled.
- Repeated occurrences of the same native upstream response ID map to the same wrapped ID within one response or stream.
- Adapter text, refusal, and function-call streams emit ordered Responses events and terminate with `response.completed` plus `[DONE]`.
- Native and adapter stream error frames are relayed without a synthesized completion event.
- Native stream and non-stream payloads remain readable when the client sends `Accept-Encoding: gzip` because upstream compression is disabled.
- Upstream usage is preferred and tokenizer estimation is used when usage is absent.

## Version 1 Status

Implemented:

1. OpenAI-compatible `POST /v1/responses` request and response DTOs with unknown-field preservation.
2. Native and chat-adapter mode selection from the exact selected upstream URL.
3. Native JSON/SSE passthrough with model rewrite, response ID wrapping, route continuity, and usage capture.
4. Adapter text, function tools, tool calls, refusals, structured-output forwarding, JSON responses, and Responses SSE synthesis.
5. Responses-native token counting, usage-limit commit, and usage metering for both modes.

Deferred:

1. Stored-response endpoints, compact, and input-token APIs.
2. Responses moderation, LLM tracing, and LLM logs.
3. Multi-key response-ID rotation and gateway token expiry.
4. Provider-specific adapters beyond native passthrough and generic Chat Completions conversion.
5. Background execution, conversations, hosted prompts, and native hosted tools in chat-adapter mode.

## Implementation Boundaries

Version 1 surface:

- `POST /v1/responses`
- native execution mode
- chat adapter execution mode
- responses mode resolver using selected upstream URL path
- AIGateway response ID wrapping for native-provider `previous_response_id`
- AIGateway-owned Responses DTOs and unknown-field preservation
- Responses-native usage counting and accounting
- separate native/adapter and stream/non-stream response writers
- deterministic unsupported-feature errors

Deferred surface:

- retrieve response
- delete response
- list response input items
- cancel response
- compact response/context
- count response input tokens
- Responses moderation
- Responses LLM tracing and LLM logs

If stored-response passthrough is added later, AIGateway must first unwrap and authorize the gateway response ID and route to the original upstream. Compact and input-token APIs require separate provider-capability and tokenization decisions.

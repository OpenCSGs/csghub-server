# AIGateway Service

> This document is intended for **AI Agents** (and developers) who read or analyze this repository. The goal is to help the reader build a mental model as quickly as possible: what this service does, how the code is layered, where each kind of logic lives, and how a request flows through its lifecycle.

## 1. One-Sentence Summary

**AIGateway is an OpenAI-compatible (and Anthropic Messages) AI inference gateway**: it exposes a unified `/v1/*` API externally and internally handles "model resolution → protocol routing → reverse proxy to upstream inference services → usage/billing/LLM log/trace collection and accounting." It is the entry point for external clients calling AI models on the CSGHub platform.

Service directory: `aigateway/`; entry point: `cmd/csghub-server/cmd/aigateway/launch.go` (start with `go run -tags=saas cmd/csghub-server/main.go aigateway launch --config=common/config/local.toml`).

---

## 2. Directory Structure & Responsibilities

The layering follows the repository-wide convention (`handler → component → builder`), but AIGateway has its own extensions.

| Directory | Responsibility | Key Files / Types |
|---|---|---|
| `router/` | HTTP route registration, middleware wiring | `router/aigateway.go` is the **single** `/v1/*` route table (see §5) |
| `handler/` | HTTP handler layer: request parsing, protocol adaptation, reverse proxy, response transformation, recording | `handler/openai.go` (main handler, 1100+ lines) |
| `handler/plan/` | **Three-stage pipeline skeleton** (Extract → Plan → Execute), protocol-agnostic | `interfaces.go`, `orchestrator.go`, `planner.go` |
| `handler/protocol/` | Protocol route resolver (Native / Adapter / Disabled) | `adapt.go`'s `adapterMatrix` |
| `handler/anthropic/` | Anthropic Messages API (`/v1/messages`) implementation | `handler.go`, `to_chat_adapter.go`, `to_responses_adapter.go`, `native.go` |
| `handler/responses/` | Responses API helper sub-package (routing, llmlog normalization, ID mapping, sensitivity handling) | `responses_routing.go`, `responses_id_mapper.go` |
| `handler/streamdecoder/` | SSE stream decoder | `stream_decoder.go` |
| `component/` | Business logic layer (model management, usage, sensitivity, LLM logging) | `openai.go` (model resolution), `usage_limiter.go`, `safety_policy.go`, `moderation.go`, `llmlog_*.go` |
| `component/router/` | Upstream session routing, upstream catalog normalization | `session_router.go`, `upstream_catalog.go` |
| `component/adapter/` | Multimodal provider adapters (text2image / text2video / audio / ocr) | `*_adapter.go` + provider files in each sub-directory |
| `component/availability/` | Upstream health check, circuit breaking, state caching | `health_checker.go`, `circuit_breaker.go`, `availability_manager.go` |
| `component/metrics/` | Request metrics collection (Prometheus / DB sink) | `collector_ee.go`, `sink_ee.go` |
| `component/trace/` | LLM tracing (Sigil tracer) | `llm_tracer.go`, `sigil.go` |
| `token/` | Token counting (usage/billing) | `token_counter.go`, `*_token_counter.go`, `tokenizer_*.go` |
| `task/` | Async generation task (video) polling/billing orchestration | `orchestrator.go`, `metering.go`, `service.go` |
| `task/processor/` | Async task resource processor interface + implementations (video) | `processor.go`, `video/video.go` |
| `types/` | Service-internal shared data structures (protocol, request/response, model, trace) | see §4 |
| `middleware/` | Metrics middleware (EE) | `metrics_ee.go` |
| `http/response/wrapper/` | Response body wrapper/transformer (image, ocr) | `wrapper/image.go`, `wrapper/ocr.go` |

### Build Tags (CE / EE / SAAS)

The repository has three build variants, distinguished by **Go build tags**. Files typically end with `_ce.go` / `_ee.go` / `_saas.go` or use `//go:build` annotations:

- **CE** (Community Edition): `//go:build !ee && !saas`
- **EE** (Enterprise Edition): `//go:build ee` (or `ee || saas`)
- **SAAS**: `//go:build saas`

Typical examples:
- `component/llmlog_capture_ce.go` vs `component/llmlog_capture_ee.go` (LLM training logs only enabled in EE)
- `component/openai_model_filter_{ce,ee,saas}.go` (model filtering logic differs per variant)
- `handler/metrics_helpers_{ce,ee}.go`, `middleware/metrics_ee.go` (metrics only compiled in EE/SAAS)
- `router/api_{ce,ee}.go` (`extendRoutes` extends routes per variant)
- `handler/mcp_*.go`, `handler/agent_ee.go`, `handler/sandbox_ee.go` etc. (MCP / Agent / Sandbox are EE features)

> **Search tip**: When a feature appears to have "per-variant implementations," search for `_ce/_ee/_saas` variants of the same file name first. Interfaces are typically defined in non-suffixed files, with implementations spread across suffixed files.

---

## 3. Core Architecture

### 3.1 Three-Stage Pipeline (Standard Pattern for New Protocols)

`handler/plan/` defines a unified three-stage flow: **protocol-agnostic Planner + protocol-specific Handler** combination:

```
Extract (protocol-specific)  →  Plan (protocol-agnostic)  →  Execute (protocol-specific)
     │                              │                              │
 Parse request body,            Resolve model target,           Adapt request → reverse proxy →
 extract identity               protocol routing, balance/       finalize → record usage/metrics/
 (MetadataExtractor)            quota/content safety (Planner)   LLM trace/LLM log
```

- **Interface definitions**: `handler/plan/interfaces.go`
  - `MetadataExtractor.Extract(c) (*types.RequestMetadata, error)`
  - `Planner.Plan(ctx, meta) (*types.RequestPlan, error)`
  - `ProtocolHandler.Execute(c, meta, p) error` + `HandlePlanError(...)`
- **Orchestrator**: `handler/plan/orchestrator.go`'s `Orchestrator.Dispatch()` — only calls the three stages in order, contains no business logic.
- **Planner implementation**: `handler/plan/planner.go`'s `plannerImpl.Plan()`, steps in order: model resolution → protocol routing → balance check → usage limit → content safety. Each step failure fills `RequestPlan.ErrorCode` for the handler to render the appropriate error.
- **Dependency injection**: `handler/planner_adapter.go`'s `newPlannerDeps()` adapts `OpenAIHandlerImpl` into the four interfaces the Planner needs (`ModelResolver` / `BalanceChecker` / `UsageLimitChecker` / `ContentSafetyChecker`).

### 3.2 Two Handler Architectures Coexist

The repository currently has **two** handler styles — understanding this prevents misreading:

1. **Traditional monolithic handler** (legacy, majority): `handler/openai.go`'s `OpenAIHandlerImpl` directly handles `/v1/chat/completions`, `/v1/responses`, `/v1/embeddings`, `/v1/rerank`, images/audio/video/ocr etc. within a single large struct, with hand-written "parse → validate → proxy → post-process" logic.
2. **Three-stage pipeline** (new pattern, currently only for `/v1/messages`): `handler/anthropic/`'s `anthropic.Handler` implements `MetadataExtractor` + `ProtocolHandler`, orchestrated by `Orchestrator`.

The bridge between them: `handler/anthropic_handler.go`'s `AnthropicHandlerImpl` **embeds** `*OpenAIHandlerImpl` to reuse its shared infrastructure (model resolution, balance, quota, sensitivity, metrics, usage, trace, llmlog publishing), then layers on the Anthropic-specific handler + Orchestrator.

> **Agent tip**: When adding a new protocol (e.g. Gemini), follow the `AnthropicHandlerImpl` pattern: embed `OpenAIHandlerImpl` → create protocol-specific handler → use Orchestrator. Currently `/v1/messages` is the only instance of this pattern.

### 3.3 Protocol Routing Matrix (Native > Adapter > Reject)

`handler/protocol/adapt.go` defines the "client protocol × upstream protocol → execution mode" matrix:

| Client Protocol \ Upstream Protocol | Chat | Responses | Messages |
|---|---|---|---|
| **Chat** | Native | — | — |
| **Responses** | `responses_to_chat` | Native | — |
| **Messages** | `messages_to_chat` | `messages_to_responses` | Native |

- `ResolveRouting()` logic: first `DetectUpstreamProtocol()` (priority: explicit metadata declaration → URL path inference → CSGHub-managed default Chat → fallback Chat), then look up the matrix.
- Three result states: `ModeNative` (protocols match, pass through directly), `ModeAdapter` (select adapter for conversion), `ModeDisabled` (no adapter available, reject with error rather than silently dropping parameters).
- `AdapterKind` constants (`protocol/adapt.go`): `AdapterMessagesToChat`, `AdapterMessagesToResponses`, `AdapterResponsesToChat`. Unimplemented adapters are commented out as placeholders.
- Anthropic adapter implementations: `handler/anthropic/to_chat_adapter.go` (Messages→Chat), `handler/anthropic/to_responses_adapter.go` (Messages→Responses), `handler/anthropic/native.go` (Messages native passthrough).

---

## 4. Key Data Structures (`types/`)

| Type | File | Description |
|---|---|---|
| `RequestMetadata` | `request_plan.go` | Output of stage 1: protocol, task, model, tenant/user, APIKey, streaming, headers, `ParsedBody any` |
| `RequestPlan` | `request_plan.go` | Output of stage 2: `ModelTarget`, `BackendURL`, `RouteMode`, `AdapterKind`, `UpstreamProtocol`, `ErrorCode` |
| `ModelTarget` | `request_plan.go` | Resolved model target (`Model`, `Upstream`, `Target`, `Host`, `ModelName`) |
| `Model` | `openai.go` | Model entity (includes `Upstreams`, provider, `RuntimeFramework`, `ImageID`, etc.), has behavior methods like `SkipBalance()` |
| `Protocol` / `ProtocolCapability` | `protocol.go` | Protocol enum (chat/responses/messages/...) and capability bits (prompt_caching/thinking/vision/tools) |
| `AnthropicMessagesRequest/Response` | `messages.go` | Anthropic Messages protocol structures (includes `Validate()`, `UnmarshalJSON` handling unknown fields) |
| `ResponsesRequest/Response` | `responses.go` | OpenAI Responses protocol structures |
| `ChatCompletionRequest` | `handler/requests.go` | Chat Completions request (in handler package, not types package) |
| `GenerationStart/Response/Message` | `trace.go` | LLM trace input/output structures |
| `TokenUsage` | `trace.go` | Token usage structure |
| `HTTPResponseWriter` | `request_plan.go` | Response writer abstract interface (for adapters to implement) |
| `AdaptResult` | `request_plan.go` | Execute stage adaptation output: `{Body, Writer}` |

---

## 5. Route → Handler Method → Key File Quick Reference

All routes are registered in `router/aigateway.go`'s `NewRouter()`. Middleware chain: `MustUserOrgApiKey` (auth) + `metricsMw` (metrics, EE) + optional `modalAPIRateLimiter` (rate limiting).

| Route | Handler Method | Implementation File |
|---|---|---|
| `GET /v1/models` | `ListModels` | `handler/openai.go` |
| `GET /v1/models/*model` | `GetModel` | `handler/openai.go` |
| `POST /v1/chat/completions` | `Chat` | `handler/openai.go` (+ `chat_retry.go`, `chat_metrics.go`, `chat_trace.go`) |
| `POST /v1/responses` | `Responses` | `handler/openai_responses.go` (+ `openai_responses_native.go`, `openai_responses_adapter.go`) |
| `POST /v1/messages` | `AnthropicHandlerImpl.Messages` | `handler/anthropic_handler.go` → `handler/anthropic/` |
| `POST /v1/embeddings` | `Embedding` | `handler/openai.go` (+ `embedding_trace.go`) |
| `POST /v1/rerank` | `Rerank` | `handler/rerank.go` |
| `POST /v1/images/generations` | `GenerateImage` | `handler/openai_image.go` |
| `POST /v1/images/edits` | `EditImage` | `handler/openai_image_edit.go` |
| `POST /v1/audio/transcriptions` | `Transcription` | `handler/openai_audio.go` |
| `POST /v1/audio/speech` / `batch` | `Speech` / `SpeechBatch` | `handler/openai_speech.go` |
| `GET/POST/PUT/DELETE /v1/audio/voices...` | `ListVoices` etc. | `handler/openai_speech_voices.go` |
| `POST /v1/videos`, `GET /v1/videos/:id...` | `CreateVideo`(Deprecated) etc. | `handler/openai_video.go` |
| `POST /v1/video/generations` | `CreateVideo` | `handler/openai_video.go` |
| `POST /v1/ocr` | `OCR` | `handler/openai_ocr.go` |
| `/v1/mcp/*` | MCP proxy | `handler/mcp_proxy_handler.go` (EE) |

---

## 6. Full Request Lifecycle (using `/v1/messages` as example)

This is the most complete flow; other protocols (chat/responses) are simplified variants:

1. **Routing + Middleware**: `router/aigateway.go` → auth, metrics, rate limiting.
2. **Extract**: `anthropic.Handler.Extract()` parses the Anthropic request body, producing `RequestMetadata` (`handler/anthropic/handler.go`).
3. **Plan**: `plannerImpl.Plan()` executes in order:
   - Model resolution `ResolveModelTarget` → `handler/model_target.go` (underlying `component/openai.go`)
   - Protocol routing `protocol.ResolveRouting` → `handler/protocol/adapt.go`
   - Balance `CheckBalance`, usage `CheckUsageLimit`, content safety `Check` (`component/`)
4. **Execute**: `anthropic.Handler.Execute()` (`handler/anthropic/handler.go`):
   - Start LLM trace (`startMessagesTrace`)
   - Create LLM log recorder (`createLogCapture`)
   - Adapt request: `adapt()` dispatches to `adaptNative` / `adaptToChat` / `adaptToResponses` based on `RouteMode`
   - Set request body → apply auth headers → set SSE headers → reverse proxy `ServeProxy`
   - Finalize response writer → extract usage → sync record token metrics
   - Async post-processing `runPostProcessAsync`: LLM trace finalization, commit usage limit, billing, publish LLM training log
5. **Error path**: Plan stage failure → `HandlePlanError()` renders protocol-specific error response based on `ErrorCode` (`handler/anthropic/handler.go`).

---

## 7. Code Location Index by Concern

> This is a quick reference for "where to find a particular type of logic."

| Concern | Entry File | Notes |
|---|---|---|
| **Model resolution / target selection** | `handler/model_target.go` | `resolveModelTarget` → `resolvedModelTarget`; includes upstream selection, auth headers, tokenizer target |
| **Model listing / CRUD / availability** | `component/openai.go` | `OpenAIComponent` interface; `GetAvailableModels` / `ListModels` / `GetModelByID` |
| **Model filtering (per variant)** | `component/openai_model_filter_{ce,ee,saas}.go` | Differentiated by build tag |
| **Balance check / billing** | `component/openai.go` | `CheckBalance`, `RecordUsageFromTokenUsage`, `BuildUsageMeteringEvent` |
| **Usage limit (quota)** | `component/usage_limiter.go` | `UsageLimiter`, `IsUsageLimitExceeded` |
| **Content safety / sensitive words** | `component/safety_policy.go`, `component/moderation.go` | `SensitivePolicy`, `Moderation` (streaming/sync checks) |
| **Token counting (usage/billing)** | `token/` | `CounterFactory` → `*_token_counter.go`; underlying `tokenizer_*.go` (tiktoken/vllm/sglang/tgi/tei/llamacpp) |
| **LLM training log capture** | `component/llmlog_capture{,_ce,_ee}.go` | `LLMLogRecorder`; normalization in `handler/responses/responses_llmlog_normalize.go` |
| **LLM training log publishing** | `component/llmlog_publisher{,_ce,_ee}.go` | `LLMLogPublisher.PublishTrainingLog` |
| **LLM tracing (trace)** | `component/trace/` + `handler/*_trace.go` | `SigilTracer`; `llmlog_to_trace.go` converts logs → traces |
| **Upstream health / circuit breaking / session routing** | `component/availability/`, `component/router/session_router.go` | Health check, circuit breaker, `SessionRouter` (consistent-hash routing to multiple upstreams) |
| **Async generation tasks (video)** | `task/` + `task/processor/` | `AsyncGenerationService` polls pending tasks → refreshes status → bills |
| **Multimodal provider adaptation** | `component/adapter/{text2image,text2video,audio,ocr}/` | Each sub-package has a `Registry` + multiple provider implementations |
| **SSE stream decoding** | `handler/streamdecoder/stream_decoder.go` | Generic SSE event parsing |
| **Response transformation writer** | `handler/response_writer_wrapper*.go`, `http/response/wrapper/` | Non-streaming/streaming/embedding/rerank/speech/audio response wrappers |
| **Metrics collection** | `component/metrics/`, `middleware/metrics_ee.go`, `handler/chat_metrics.go` | Request lifecycle metrics (EE/SAAS) |
| **MCP gateway** | `handler/mcp*.go`, `component/mcp_*.go` | EE only |

---

## 8. Coding Conventions (inherited from root `AGENTS.md`, AIGateway specifics)

- **Layering**: handler → component → builder. Interfaces should not return lower-layer structures across layers (component layer interfaces should not return `database.*` structures).
- **Interfaces + dependency injection**: `component` layer extensively uses interfaces (`OpenAIComponent`, `SensitivePolicy`, `UsageLimiter`, etc.), handler injects via constructors.
- **Build tags**: Per-variant features use `_ce/_ee/_saas` suffix + `//go:build` tags; interface definitions in non-suffixed files.
- **Testing**: Every `.go` has a corresponding `_test.go`; mocks are generated with mockery, **do not hand-edit `_mocks/`** — update `.mockery*.yaml` then run `make mock_gen GO_TAGS={go.buildTags}`.
- **Error codes**: `handler/plan/planner.go`'s `categorizePlanError` classifies errors into `PlanErrorCategory` (`types/request_plan.go`), handler selects HTTP status code accordingly; `unsupportedFeature` errors use `unsupported_feature:` prefix for `extractUnsupportedCapabilities` to identify.

---

## 9. Build / Test Commands

```bash
# Run all tests (mind the build tags)
make test GO_TAGS={go.buildTags}

# Generate mocks
make mock_gen GO_TAGS={go.buildTags}

# Build only aigateway-related packages
go build ./aigateway/...

# Unit test a specific package
go test ./aigateway/handler/anthropic/... ./aigateway/types/...

# Start the service
go run -tags=saas cmd/csghub-server/main.go aigateway launch --config=common/config/local.toml
```
---

## 10. Quick Mental Model (TL;DR)

1. **Entry**: `router/aigateway.go`'s `NewRouter()` is the single route table.
2. **Main handler**: `handler/openai.go`'s `OpenAIHandlerImpl` (traditional monolith) + `handler/anthropic/` (new three-stage pipeline, embeds the former via `handler/anthropic_handler.go` to reuse infrastructure).
3. **Core abstractions**: Three-stage `Extract → Plan → Execute` (`handler/plan/`) + protocol routing matrix (`handler/protocol/adapt.go`).
4. **Business logic**: All in `component/` (models, balance, quota, sensitivity, logging, tracing, availability).
5. **Billing/usage**: `token/` counting → `component` billing.
6. **Variant differences**: `_ce/_ee/_saas` suffix + build tag differentiation.

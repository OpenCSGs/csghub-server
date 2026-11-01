# AIGateway Sample Provider Design

## Context

The `aigateway/sample` package builds and executes protocol-specific sample
requests for two consumers:

- scheduled upstream health checks, which run L7 every scheduler tick and
  inference according to the protocol's health-check cadence; and
- manual upstream connection tests, which run an inference sample and return a
  masked request/response summary.

The existing provider boundary is useful: consumers depend only on
`types.SampleProvider`, while the sample package owns endpoint selection,
request construction, and HTTP execution. The design must preserve that
boundary while solving these problems:

1. Providers currently construct JSON bodies with `map[string]any`, duplicating
   schemas already owned by request DTOs.
2. Reusable DTOs are split between `aigateway/handler` and `aigateway/types`,
   and `ImageGenerationRequest` exists in both packages.
3. Endpoint matching and base URL extraction rely on raw string suffixes and
   assumptions about `/v1`.
4. The shared L7 behavior assumes every upstream supports
   `GET {base}/models`.
5. Multipart inference routes do not yet have sample providers.
6. AIGateway supports the Anthropic Messages API, but the sample registry does
   not yet recognize or exercise explicit `/v1/messages` upstream endpoints.

## Design principles

- A provider is a long-lived, stateless protocol strategy.
- A DTO contains data for one request and is constructed locally for every
  `Build` call.
- Providers reuse DTO serialization by passing the DTO directly to
  `json.Marshal`; providers do not embed mutable DTOs or duplicate their JSON
  schemas.
- Consumers remain protocol-agnostic.
- Every request helper receives `types.SampleInput` and clones its headers, so
  authentication survives JSON, multipart, and L7 request construction.
- Routes are compared as parsed path segments.
- Every protocol explicitly declares both its L7 and inference factories.
- Protocols using `/models` fall back to their minimal inference request when
  the upstream explicitly reports that `/models` is unsupported with HTTP 404
  or 405. Authentication failures, rate limits, timeouts, and server failures
  remain L7 failures.

## Architecture

```text
Health-check scheduler                 Manual TestUpstream
  L7 -> inference                         inference only
          |                                  |
          +----------- SampleInput ----------+
                             |
                    Registry.Find(endpoint)
                             |
                 stateless protocolProvider
                 route + L7/inference factories
                             |
                       SampleRequest
                             |
                    shared HTTP executor
                             |
                 SampleExecutionResult
```

Package ownership:

```text
aigateway/types
  Shared wire DTOs and sample contracts
          ^
aigateway/sample
  Route selection, request factories, HTTP execution
          ^
component / aigateway/component/availability
  Manual connection tests and scheduled health checks
```

The dependency direction is:

```text
handler -> types <- sample
```

The sample package must not import the handler package.

## Shared DTO ownership

Move these reusable request DTOs from `aigateway/handler/requests.go` to
`aigateway/types/requests.go`:

- `ChatCompletionRequest` and `StreamOptions`
- `EmbeddingRequest`
- `RerankRequest`
- `SpeechRequest`
- `BatchSpeechRequest`, including `InputTexts()`

Keep the `ImageGenerationRequest` in `aigateway/types/image.go` and merge the
handler implementation's `UnmarshalJSON` behavior into it. Handler code should
refer to these types as `types.X`.

DTO marshal/unmarshal tests belong in `aigateway/types`; handler behavior tests
remain in `aigateway/handler`.

## Stateless protocol provider

Providers are configured with a canonical route, separate request factories,
and the inference timeout:

```go
type requestFactory func(types.SampleInput) (*types.SampleRequest, error)

type protocolProvider struct {
    route            string
    l7               requestFactory
    inference        requestFactory
    inferenceTimeout time.Duration
    multimodal       bool
}

func newProtocolProvider(
    route string,
    l7 requestFactory,
    inference requestFactory,
    inferenceTimeout time.Duration,
) protocolProvider {
    return protocolProvider{
        route: route,
        l7: l7,
        inference: inference,
        inferenceTimeout: inferenceTimeout,
    }
}
```

`NewDefaultRegistry` registers provider values. Providers contain no per-request
state, so the same registry can be reused safely by concurrent health checks.
After executing an L7 factory, the provider recognizes an unsupported models
API only when the generated request is `GET .../models` and the response is HTTP
404 or 405. It reports that an inference fallback is required without executing
the fallback itself. The health checker then executes the provider's inference
factory with the inference policy's timeout. This keeps the registry compact,
gives each request phase its own deadline, and prevents inference-based L7
requests from being retried.

`SampleProvider` exposes each request kind's execution policy, while
`SampleRequestBuilder` continues to expose the primary request used by that
kind.

## Multimodal health-check cadence

The sample provider's inference execution policy controls two scheduler inputs:

- `Timeout` is the maximum duration of the protocol's inference request.
- `Multimodal` selects the throttled multimodal cadence and latency threshold.

The scheduler remains on the configured L7 interval, defaulting to 60 seconds.
Chat, Responses, Messages, embeddings, and rerank run inference on every
successful L7 tick. Image, audio, and video providers declare their inference
policy as multimodal and use
`OPENCSG_AIGATEWAY_HEALTH_CHECK_MULTIMODAL_INFERENCE_INTERVAL`, which defaults
to 3600 seconds.

The default inference timeout is 30 seconds. Image generation and image editing
use the shared long-running inference timeout of five minutes. The scheduler
passes the provider's inference timeout directly to the inference request, so
long-running protocols are not constrained by the L7 timeout. L7 requests keep
using `OPENCSG_AIGATEWAY_HEALTH_CHECK_L7_API_TIMEOUT`, which defaults to 15
seconds.

The elected leader keeps a read-through in-memory cache of per-upstream cadence
state backed by Redis. Redis stores only the probe mode and next inference time;
health facts and failure counts remain in the database. A leader loads each
upstream once after acquiring leadership, uses the local state on subsequent
scheduler ticks, and writes cadence transitions through to Redis. It does not
read Redis on every 60-second tick. Losing or acquiring leadership clears the
local cache, so a new leader resumes the shared deadline instead of starting
every multimodal inference immediately.

A missing Redis state is bootstrapped from the persisted health result. An
upstream without a health result is new and runs its first inference
immediately. Otherwise, the inference dimension's persisted last-check time and
failure count rebuild the retry or multimodal deadline. This avoids a request
burst during the first deployment of the shared scheduler or after Redis state
loss.

Before starting a due inference, the leader atomically reserves its Redis
deadline for the inference timeout plus one L7 interval. Only the successful
claimant sends the request. Completion replaces the reservation with the next
retry or multimodal deadline. The Redis key TTL is twice the larger of the
multimodal interval and reservation window. If Redis scheduling operations fail,
the checker skips that expensive probe without changing upstream health. When
Redis is not configured, the scheduler retains the previous in-memory-only
behavior.

A success schedules the next check at the multimodal interval. Failures retry
at the L7 interval until the configured consecutive-failure threshold is
reached, then return to the multimodal interval. Removing or disabling an
upstream lets its Redis cadence state expire; changing a cached upstream to a
regular protocol removes its cadence state.

A successful cheap L7 request updates only the persisted L7 health dimension;
it does not overwrite inference failures when inference is not due. When
`/models` returns 404 or 405, the provider marks that an inference fallback is required in
`SampleExecutionResult`. The scheduler starts that fallback with the provider's
inference timeout, counts it as the inference attempt, and does not send a
duplicate request. For multimodal upstreams, it then throttles the whole check
because no cheap L7 path exists. Each scheduled attempt still probes `/models`
first and restores 60-second L7 checks when that endpoint becomes available.
If `/models` becomes unsupported before inference is due, the scheduler records
inference-only mode while preserving the shared deadline instead of sending an
early fallback request.

### Health-state classification

Protocol execution and health-state classification remain separate. Before a
scheduled result is persisted, the scheduler copies the provider's `Multimodal`
policy into the transient health-check result. This applies to both a normal
inference request and an inference request used as the `/models` fallback.

A successful request is classified using the threshold selected by that policy:

| Inference policy | Configuration | Default |
|---|---|---:|
| Regular | `OPENCSG_AIGATEWAY_HEALTH_CHECK_LATENCY_DEGRADED_MS` | 10000 ms |
| Multimodal | `OPENCSG_AIGATEWAY_HEALTH_CHECK_MULTIMODAL_LATENCY_DEGRADED_MS` | 120000 ms |

Latency strictly greater than the selected threshold produces `degraded`.
Latency equal to the threshold remains `healthy`. A non-positive threshold
disables latency-based degradation for that policy. This allows a successful
40-second image generation to remain healthy without relaxing the latency rule
for chat and other regular inference protocols.

For multimodal upstreams, the database metadata stores independent L7 and
inference health facts under `metadata.multimodal`. Each dimension contains its
own consecutive-failure count, last check and success times, last error, and
latency. A probe event updates only its own dimension. A pure projection then
derives the legacy top-level row fields and the overall state: either dimension
reaching `OPENCSG_AIGATEWAY_HEALTH_CHECK_CONSECUTIVE_FAILURES` makes the
upstream `unhealthy`, while a successful slow probe in either dimension makes
it `degraded` when neither dimension is unhealthy. The top-level consecutive
failure count is the maximum of the two dimension counts.

Every health event is applied by the database store in a transaction that locks
the upstream health row with `SELECT ... FOR UPDATE`. The transaction reads the
latest row, reduces one event, derives the compatibility projection, and writes
the result before releasing the lock. This prevents an old leader's long-running
inference result and a new leader's L7 result from overwriting each other's
dimension. Cache publication and circuit-breaker side effects happen only after
the transaction commits.

Probe timestamps are recorded when each request starts. Within the locked
mutation, regular health events must be strictly newer than the row's
`last_check_at`, while multimodal events must be strictly newer than their own
L7 or inference dimension's `last_checked_at`. Older and equal-timestamp events
are ignored without updating the database, cache, circuit breaker, or Redis
cadence. This ordering assumes health-check nodes keep synchronized clocks;
leader fencing is not part of this design.

Legacy rows without `metadata.multimodal` are initialized lazily. Until the
first inference fact is committed, their projected state remains `unknown`,
unless one dimension independently reaches the unhealthy failure threshold.
This prevents one successful L7 probe from incorrectly recovering a legacy
unhealthy row when inference is interrupted or cannot be reserved. If the
`multimodal` key already exists but cannot be parsed or lacks either dimension,
the event is rejected and logged instead of replacing the persisted facts with
an empty state.

This separation means an L7 recovery clears only the L7 failure sequence and
never hides pending inference failures. Redis write failures cannot merge or
discard health sequences because database persistence occurs before cadence is
updated. A `/models` 404 or 405 is a capability fallback rather than an L7
failure, so only the resulting inference event is persisted.

When the persisted state changes, the structured log includes
`latency_threshold_ms` and `multimodal` alongside the measured latency and
check type. These fields show which policy produced the transition.

### Health-check configuration

| Environment variable | Purpose | Default |
|---|---|---:|
| `OPENCSG_AIGATEWAY_HEALTH_CHECK_L7_API_INTERVAL` | L7 scheduler interval | 60 seconds |
| `OPENCSG_AIGATEWAY_HEALTH_CHECK_L7_API_TIMEOUT` | L7 request timeout | 15 seconds |
| `OPENCSG_AIGATEWAY_HEALTH_CHECK_MULTIMODAL_INFERENCE_INTERVAL` | Successful multimodal inference interval | 3600 seconds |
| `OPENCSG_AIGATEWAY_HEALTH_CHECK_CONSECUTIVE_FAILURES` | Failures required for `unhealthy` | 3 |
| `OPENCSG_AIGATEWAY_HEALTH_CHECK_LATENCY_DEGRADED_MS` | Regular inference degraded threshold | 10000 ms |
| `OPENCSG_AIGATEWAY_HEALTH_CHECK_MULTIMODAL_LATENCY_DEGRADED_MS` | Multimodal inference degraded threshold | 120000 ms |

## Request factories

Each JSON factory constructs a local DTO:

```go
func chatCompletionsRequest(
    input types.SampleInput,
) (*types.SampleRequest, error) {
    dto := types.ChatCompletionRequest{
        Model: input.Model,
        Messages: []openai.ChatCompletionMessageParamUnion{
            openai.UserMessage(sampleText(input)),
        },
        MaxTokens: 1,
        Stream:    false,
    }
    return jsonRequest(input, dto)
}
```

`json.Marshal(dto)` invokes the DTO's own `MarshalJSON`, including `RawJSON`
merging. Embedding the DTO in a provider is unnecessary and would mix a
long-lived strategy with mutable request data.

Shared helpers accept the complete sample input:

```go
func jsonRequest(
    input types.SampleInput,
    dto any,
) (*types.SampleRequest, error)

func multipartRequest(
    input types.SampleInput,
    fields map[string]string,
    files map[string]fileSpec,
) (*types.SampleRequest, error)

func modelsL7Request(route string) requestFactory
```

Each helper clones `input.Headers`. JSON helpers set `application/json`, and
multipart helpers set the generated content type including its boundary.

`SampleInput.Other` must not be used for protocol selection or L7 strategy. Add
an explicit typed field if future providers need more selection data.

## Route matching

Use two segment-aware helpers:

```go
func endpointMatchesRoute(endpoint, route string) bool
func endpointBaseURL(endpoint, route string) (string, error)
```

Both helpers parse the URL and canonical route, trim empty and trailing
segments, and compare the same number of trailing path segments. This provides
the required behavior:

- `.../api/v1/chat/completions` matches `/chat/completions`.
- `.../v1/chat/completions/` matches `/chat/completions`.
- `.../chat/completions` matches `/chat/completions`.
- `/audio/speech/batch` does not match `/audio/speech`.
- `.../v1/messages` and `.../anthropic/v1/messages` match `/v1/messages`.
- `/messages`, `/v1/messages/count_tokens`, a bare host, and
  `/model/{modelID}/invoke` do not match `/v1/messages`.

`endpointBaseURL` strips exactly the matched canonical route segments. It keeps
any provider prefix and version path before the route and clears query and
fragment fields.

Example:

```text
endpoint: https://dashscope.example/compatible-mode/v1/responses
route:    /responses
base:     https://dashscope.example/compatible-mode/v1
```

## Endpoint factories

### JSON inference

| Route | DTO | Minimal inference data | L7 request |
|---|---|---|---|
| `/chat/completions` | `types.ChatCompletionRequest` | model, user message, `max_tokens: 1`, `stream: false` | `GET {base}/models` |
| `/responses` | `types.ResponsesRequest` | model, input `"hi"`, `max_output_tokens: 16`, `stream: false` | `GET {base}/models` |
| `/v1/messages` | `types.AnthropicMessagesRequest` | model, one user message containing `"hi"`, `max_tokens: 1`, `stream: false` | `GET` sibling versioned `/models` |
| `/embeddings` | `types.EmbeddingRequest` | model, string input `"hi"` | `GET {base}/models` |
| `/rerank` | `types.RerankRequest` | model, query `"hi"`, documents `["hi"]` | `GET {base}/models` |
| `/images/generations` | `types.ImageGenerationRequest` | model, prompt `"hi"` | `GET {base}/models` |
| `/audio/speech` | `types.SpeechRequest` | model, input `"hi"`, `stream: false` | `GET {base}/models` |
| `/audio/speech/batch` | `types.BatchSpeechRequest` | model, one item containing input `"hi"` | `GET {base}/models` |
| `/video/generations` | `types.VideoGenerationRequest` | model, prompt `"hi"` | `GET {base}/models` |

Responses and Messages use a models-first L7 check. When a compatible provider
such as DashScope does not expose `/models`, HTTP 404 or 405 asks the health
checker to run the protocol's minimal inference request with its inference
timeout. The fallback result is marked as an inference attempt, so the scheduler
persists it without sending a duplicate inference request.

### Anthropic Messages endpoints

The Messages provider uses the canonical route `/v1/messages`. Segment-aware
matching allows provider-specific prefixes while requiring the complete
versioned protocol suffix. Verified compatible endpoint shapes include:

- Anthropic direct: `https://api.anthropic.com/v1/messages` ([API reference](https://platform.claude.com/docs/en/api/messages/create)).
- Amazon Bedrock Mantle:
  `https://bedrock-mantle.{region}.api.aws/anthropic/v1/messages`
  ([AWS documentation](https://docs.aws.amazon.com/bedrock/latest/userguide/inference-messages-api.html)).
- Cloudflare AI Gateway:
  `https://gateway.ai.cloudflare.com/v1/{account}/{gateway}/anthropic/v1/messages`
  ([Cloudflare provider documentation](https://developers.cloudflare.com/ai-gateway/usage/providers/anthropic/)).
- Cloudflare's Anthropic-compatible REST endpoint:
  `https://api.cloudflare.com/client/v4/accounts/{account}/ai/v1/messages`
  ([Cloudflare REST documentation](https://developers.cloudflare.com/ai-gateway/usage/rest-api/)).

The provider does not claim bare URLs or transports that merely carry an
Anthropic-shaped body. For example, Bedrock Runtime uses
`/model/{modelID}/invoke`, AWS SigV4, and an `anthropic_version` request-body
field. That is a distinct transport and is outside the generic `/v1/messages`
sample provider.

The Messages factory constructs a local DTO and delegates serialization to its
existing `MarshalJSON` implementation:

```go
func messagesRequest(input types.SampleInput) (*types.SampleRequest, error) {
    content, err := json.Marshal(sampleText(input))
    if err != nil {
        return nil, fmt.Errorf("marshal messages sample content: %w", err)
    }

    dto := types.AnthropicMessagesRequest{
        Model:     input.Model,
        Messages:  []types.AnthropicMessage{{Role: "user", Content: content}},
        MaxTokens: 1,
        Stream:    false,
    }

    sampleInput := input
    sampleInput.Headers = input.Headers.Clone()
    if sampleInput.Headers.Get("anthropic-version") == "" {
        sampleInput.Headers.Set("anthropic-version", "2023-06-01")
    }
    return jsonRequest(sampleInput, dto)
}
```

The factories preserve configured authentication headers such as `x-api-key`.
Both the models L7 request and inference request supply
`anthropic-version: 2023-06-01` when the upstream configuration did not provide
an explicit version. A configured value takes precedence.

Messages endpoint matching remains explicitly versioned, while its models L7
factory removes only the final `/messages` segment. For example,
`.../anthropic/v1/messages` probes `.../anthropic/v1/models`. Providers without
that endpoint use the existing 404/405 inference fallback.

### Multipart inference

| Route | Fields | File part | L7 request |
|---|---|---|---|
| `/images/edits` | model, prompt, `response_format=b64_json` | `image`, tiny PNG | `GET {base}/models` |
| `/audio/transcriptions` | model | `file`, tiny WAV | `GET {base}/models` |
| `/audio/voices` | model, name, `consent=consent-id` | `audio_sample`, tiny WAV | `GET {base}/models` |

Minimal valid sample files are immutable package-level byte slices. Every
factory creates a new multipart body and boundary.

This iteration covers upstream inference routes, not management or result
retrieval routes such as listing/deleting voices or fetching asynchronous video
results.

`/ocr` remains a follow-up. Its gateway input is multipart, but its upstream
request is JSON built through the OCR adapter. A valid OCR sample must exercise
that adapter rather than behave as a generic multipart proxy.

## HTTP execution and errors

The shared sample request executor remains the only HTTP execution
implementation. It:

1. builds the protocol request;
2. creates a context-bound HTTP request;
3. clones request headers;
4. executes through the injected `HTTPDoer`;
5. records latency, status, and a bounded response body; and
6. returns transport and response-read failures in
   `SampleExecutionResult.Error`.

Request-construction failures are returned directly from `Build`. Execution
failures remain inside the result so consumers can report the attempted request.
For a models-based L7 strategy, HTTP 404 and 405 mark
`InferenceFallbackRequired` on the L7 result. The health checker then executes
one request to the protocol's minimal inference endpoint using a fresh context
with the provider's inference timeout. The final health result describes the
fallback attempt, includes the latency of both attempts, and sets
`UsedInferenceFallback` for scheduler bookkeeping. Other statuses and transport
failures never trigger fallback.

## File organization

Group files by responsibility and request encoding rather than creating one file
per endpoint:

- `aigateway/sample/route.go` and `route_test.go` own route matching and base URL
  derivation.
- `aigateway/sample/provider.go` and `provider_test.go` own
  `requestFactory`, `protocolProvider`, sample-kind dispatch, and shared HTTP
  execution.
- `aigateway/sample/json.go` and `json_test.go` own the header-preserving JSON
  helper and JSON protocol request factories.
- `aigateway/sample/multipart.go` and `multipart_test.go` own the multipart
  helper, immutable PNG/WAV sample assets, and multipart protocol request
  factories.
- `aigateway/sample/default_registry.go` and `default_registry_test.go` own the
  default protocol registrations and registry lookup coverage.

Keep provider dispatch separate from request encoding. In particular,
`provider.go` must not become a container for JSON helpers, multipart helpers,
or binary sample assets. A single `factories.go` containing both JSON and
multipart endpoints is also avoided because it would mix unrelated encoding
responsibilities and produce an oversized test file.

Existing superseded files such as `openai_chat_completions.go` and
`openai_responses.go` can be removed after their behavior is covered by the new
JSON factories and tests. Keep shared sample text behavior with the provider
layer unless it becomes specific to one encoding.

Reusable DTO files remain organized separately:

- `aigateway/types/requests.go` and `requests_test.go` contain the DTOs moved
  from the handler package and their serialization tests.
- `aigateway/types/image.go` and `image_test.go` contain the consolidated
  `ImageGenerationRequest` behavior.

Handler files and tests are updated only for the `types.X` references and retain
handler behavior coverage.

## Test design

Tests must verify:

- direct, versioned, prefixed, and trailing-slash endpoint selection;
- exact segment matching between `/audio/speech` and
  `/audio/speech/batch`;
- explicit Messages matching for `/v1/messages` and prefixed
  `.../anthropic/v1/messages` endpoints;
- rejection of `/messages`, `/v1/messages/count_tokens`, bare URLs, and
  Bedrock Runtime `/model/{modelID}/invoke` as Messages sample endpoints;
- exact route removal while preserving provider prefixes;
- JSON bodies equal `json.Marshal` of their populated DTOs;
- `RawJSON` and custom `MarshalJSON` behavior remains intact;
- authorization headers survive JSON, multipart, models-based L7, and
  inference-based L7 construction;
- the Messages sample preserves authentication, defaults
  `anthropic-version: 2023-06-01`, and preserves an explicitly configured
  Anthropic version;
- the Messages body equals `json.Marshal` of the populated
  `types.AnthropicMessagesRequest`, contains one valid user message, and uses
  `max_tokens: 1` with streaming disabled;
- multipart boundaries, fields, filenames, content types, and bytes are valid;
- the voice sample includes `consent` and `audio_sample`;
- Responses L7 derives the sibling `/models` endpoint and falls back to the
  minimal Responses request on HTTP 404 or 405;
- Messages L7 preserves the versioned provider prefix when deriving `/models`
  and falls back to the minimal Messages request on HTTP 404 or 405;
- models-based L7 checks fall back on HTTP 404 and 405 only, without masking
  authentication, rate-limit, timeout, or server failures;
- concurrent builds with different models do not leak request state;
- standard scheduled checks still run L7 followed by inference;
- multimodal checks keep cheap L7 at the base interval while throttling
  inference to the configured cadence;
- inference failures retry at the base interval until the unhealthy threshold;
- inference fallback is counted once and throttles upstreams without `/models`;
- unsupported protocols retain their existing skip/rejection behavior;
- response body limits and manual request-header masking remain intact; and
- existing compile-time interface assertions continue to hold.

Every new production Go file has the focused corresponding test file listed
above, following repository convention.

Suggested validation:

```bash
go test ./aigateway/sample/... \
  ./aigateway/types/... \
  ./aigateway/handler/... \
  ./aigateway/component/availability/... \
  ./component/...

make test GO_TAGS=ce
make lint
git diff --check
```

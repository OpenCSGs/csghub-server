# Support Responses API for CSGHub-Hosted Models

## Summary

Extend Responses routing so CSGHub-hosted deployments are not judged only by stored endpoint suffix. Hosted vLLM models using the repo's current `v0.24.0` runtime should route natively to `/v1/responses`; older or uncertain hosted runtimes should fall back to the existing chat adapter when they are OpenAI chat-compatible.

## Key Changes

- Extend `responses.RoutingTarget` to include hosted-model context: whether the model is CSGHub-hosted, runtime framework, image ID, and optional metadata/tags.
- Extend `responses.RoutingDecision` to carry the resolved backend URL for both native and chat-adapter modes.
- Keep existing external behavior unchanged:
  - upstream URL ending `/v1/responses` routes native.
  - upstream URL ending `/v1/chat/completions` routes chat adapter.
  - unsupported suffix remains disabled.
- Add hosted-model routing:
  - hosted vLLM image/runtime `>= 0.24.0` routes native with backend URL `base + /v1/responses`.
  - older hosted vLLM variants route chat adapter with backend URL `base + /v1/chat/completions`.
  - hosted SGLang image/runtime `>= 0.5.0` routes native with backend URL `base + /v1/responses`.
  - hosted SGLang variants without a detectable SGLang semver, including NGC-style tags, route chat adapter.
  - unknown hosted runtime remains disabled.
- Wire `OpenAIHandlerImpl.Responses` to pass hosted-model fields into routing and use `RoutingDecision`'s backend URL when executing native or adapter mode.

## Implementation Notes

- Treat `len(model.SvcName) > 0` as the hosted-model signal.
- Normalize base deploy target before appending `/v1/responses` or `/v1/chat/completions`, preserving scheme/host and avoiding duplicate slashes.
- Version detection should parse known image/runtime strings conservatively:
  - native only for clear `v0.24.0`, `0.24.0`, or higher vLLM versions.
  - native only for clear `v0.5.0`, `0.5.0`, or higher SGLang versions.
  - otherwise adapter if the runtime is vLLM- or SGLang-compatible.
- Do not change `previous_response_id` semantics:
  - native mode keeps current upstream affinity behavior.
  - adapter mode still rejects adapter response IDs as before.

## Test Plan

- Add routing tests for:
  - external `/v1/responses` remains native.
  - external `/v1/chat/completions` remains chat adapter.
  - CSGHub hosted vLLM `v0.24.0` routes native to `/v1/responses`.
  - CSGHub hosted vLLM `v0.9.2`, `v0.8.5`, and CPU `0.4.12-fix1` route chat adapter.
  - CSGHub hosted SGLang `v0.5.14` routes native to `/v1/responses`.
  - CSGHub hosted SGLang without detectable SGLang semver routes chat adapter.
  - unknown hosted runtime remains disabled.
- Add handler-level tests proving hosted model routing uses the generated backend URL and does not require the original deploy endpoint to end with an OpenAI API suffix.
- Run focused tests:
  - `go test ./aigateway/handler/responses ./aigateway/handler`

## Assumptions

- Current default hosted vLLM runtime is `v0.24.0`, so native Responses is acceptable for that runtime.
- Current default hosted SGLang runtime is `v0.5.14`, so native Responses is acceptable for that runtime.
- No database migration is needed; routing can derive behavior from existing model fields.

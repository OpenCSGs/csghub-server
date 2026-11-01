# Compact Auto routing input (EE/SaaS)

Gateway reuses AutoRouteContext protocol conversion, current-turn splitting and the existing 2,000-message history window. CE does not provide Auto routing. The upstream inference request is not modified.

## Wire contract

POST /v1/rank uses `lightgbm-rank-input/1`: `query_text` contains normalized current input; `text_features` contains five 13-dimensional groups (query, history, execution, control, previous_assistant); `count_features` contains seven counts. History, system prompt and tool definitions contribute local statistics rather than being sent as full context. Session/Turn identity remains in HTTP headers, and the response remains `model_list`.

Canonical tool JSON and Unicode statistics match the Router extractor. Standard JSON serialization is not a direct substitute for the deterministic normalization used by the fixed fixtures.

## Input validation and error semantics

At most **200 tools** are accepted. This mirrors `freeze_turn_input` in `router_service/turn_contract.py` of the semantic_model repository, which rejects more than 200 tools or non-object tool definitions. This is a source-code contract reference, not confirmation of the currently deployed Router version.

Gateway now performs this validation locally. Invalid tool JSON, trailing JSON data, non-object definitions and numbers outside finite float64 representation (for example `1e999`) are also rejected during canonicalization. The request builder deliberately preserves the existing invalid-routing-input convention: `autoRouteCodeNotFound` / `model_not_found`, mapped to **HTTP 404**, rather than introducing a new 400 response in this MR. Here 404 indicates invalid routing input, not necessarily an absent model. The error message identifies the validation failure. Moving validation locally does not imply that the previous remote error response was identical.

The final serialized rank body may contain at most **2,000,000 bytes**. Larger bodies are rejected before sending. Local size errors and remote HTTP 413 responses map to `routing_input_too_large` / HTTP 413 without marking Router readiness unhealthy. This byte limit is distinct from tool validation and its retained 404 semantics.

## Compatibility and scope

Confirm Router supports compact v1 before enabling this Gateway version. A Router accepting only the old full-context body is incompatible. Coordinate rollback across both services. Candidate authorization, health checks and normal Router failure handling remain unchanged.

This MR does not include generation v2, shared ranking caches, lease renewal, feedback, training or deployment.

## Verification

Fixed fixtures and Unicode parity tests preserve the 72-feature contract. The extraction path counts words and matches folded keywords without allocating full-text copies; canonical message text is written into a single builder. No generation, cache or feedback functionality is added by this optimization.

Targeted EE tests passed for routingfeatures and semanticrouter. Targeted SaaS tests passed for extraction, ranking/health, the 200/201-tool boundary and invalid tool JSON, serialized size boundaries and remote 413 handling. Extraction allocation-budget tests passed under EE and SaaS; statistics allocate zero times for the fixed text fixture. No full repository suite or full lint was rerun for this update; CI must validate the new commit.

On the fixed 1 MiB text fixture, Extract cumulative allocation dropped from 26,387,792 to 1,056,866 bytes/op. With an additional 4 MiB historical image-text fixture, it dropped from 64,307,360 to 5,259,394 bytes/op (median of three runs with three iterations each). Tests enforce 2 MiB and 7 MiB allocation budgets respectively. These are helper-level cumulative allocations, not process RSS, peak memory or production concurrency guarantees. Fixture construction is outside benchmark timing. No deployment or real-model integration was performed.

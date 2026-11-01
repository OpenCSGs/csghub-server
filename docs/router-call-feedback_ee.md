# Auto routing execution feedback (EE/SaaS)

This increment builds on generation routing and the shared turn cache. It records
execution evidence; it does not train or publish a model.

## Request and delivery flow

1. Register the authenticated request before ranking or cache lookup.
2. Bind the request to its Session/Turn, configuration identity and snapshot.
3. Observe each actual upstream attempt after protocol adaptation and model
   selection. Chat, Responses and Messages use the common observer, including
   streaming responses and model fallback attempts.
4. Preserve each attempt's model, upstream, protocol, parameter hash/summary,
   token usage, duration, first-token timing and terminal status. Missing or
   invalid usage remains unknown. Prompts and response bodies are not persisted.
5. End the request after the entire planning/execution lifecycle. Starting the
   next turn closes the preceding receipt; delivery waits for its open requests.
6. Claim a sealed receipt from Redis and POST it to Router's /v1/feedback.
   Gateway accepts HTTP 204 as success.

Feedback association failures must not block otherwise valid pinned-upstream
inference. A missing/expired existing-turn cache snapshot retains local model
fallback; it must not invent a new rank or training binding. Real cancellation
and upstream errors retain their ordinary semantics.

## Configuration and dependency

Feedback is disabled by default. To enable it, configure:

- OPENCSG_AIGATEWAY_ROUTER_FEEDBACK_ENABLED=true
- OPENCSG_AIGATEWAY_ROUTER_GENERATION_ENABLED=true
- OPENCSG_AIGATEWAY_ROUTER_CACHE_ENABLED=true
- Shared Redis and consistent cache environment/namespace.
- Router service credentials: an explicit API token file takes precedence over
  the platform API token. File errors do not silently fall back.

The generation and cache defaults and credential handling come from the preceding
increments and are not redefined by feedback. CE contains only neutral DTOs and
inert observation hooks.

## Evidence and delivery limits

- Only a new turn closes the preceding turn. A final turn without a successor
  is not automatically sent by an idle timer.
- Redis receipt expiry is seven days from creation. Abandoned pending requests
  expire after six hours and mark a closed receipt incomplete.
- A receipt retains at most 256 attempts, 60,000 bytes of serialized call data
  and a bounded number of hash fields. Omitted calls mark it incomplete;
  omitted_attempts is a lower bound, not an exact count.
- Missing configuration identity or snapshot prevents transmission. The local
  receipt is compacted and marked unsupported_input_contract, rather than
  sending invalid training evidence.
- Two workers poll pending receipts. A claim becomes visible again after
  30 seconds if a sender disappears. Transient failures are rescheduled after
  60 seconds; 429 is retryable. Other 4xx responses are terminal.
- Successful or permanently rejected receipts keep a small sealed tombstone
  until the original expiry; call/pending payloads are removed. Transient
  failures retain their payload for retry.
- Delivery is retryable, not exactly-once transport. Stable Session/Turn and
  request/attempt identifiers allow Router to deduplicate. Local real-Router
  validation confirmed identical deliveries return 204, changed receipts return
  409, and this behavior survives a Router container restart.
- Responses exceeding the 1 MiB metadata parsing limit and encoded responses
  pass through without inventing usage. Selected controls are bounded to
  16 KiB; the feedback transport rejects a body exceeding 64 KiB.

## Optional sharding

OPENCSG_AIGATEWAY_ROUTER_FEEDBACK_SHARDING_ENABLED defaults to false.
When enabled, scopes use 16 Redis hash tags with per-shard pending indexes.
Existing legacy sessions retain their tag, and workers poll both legacy and
sharded indexes. Each Lua operation stays within one hash tag.

Enable consistently only after every instance supports sharding. Do not mix
enabled and disabled writers during the switch. Before rolling back to an older
binary, stop new feedback writes and drain all shard indexes with capable
workers. The undelivered backlog has no global byte cap; monitor memory, pending
age and permanent delivery failures.

## Validation status

This branch is a selective source migration from the complete feedback branch
onto the cache increment. Scoped CE/EE/SaaS compilation and affected-package
regression tests passed on 2026-10-26 with Go 1.26.0. Tests exercised protocol
parsing, per-attempt identity, turn closure, missing binding, cancellation, Redis
state transitions, retry/restart fixtures and service credentials.

Compilation used go test -run '^$'; targeted tests used -count=1 and excluded
existing TestE2E-prefixed cases. A local isolated Redis and HTTP test servers were
used. A separately deployed local Docker Router with a synthetic two-model
artifact accepted Gateway-generated receipts for all three protocols. Duplicate
delivery returned 204; changed content returned 409, including after restarting
the Router container. Redis transient-failure recovery passed using a recreated
Gateway delivery worker (not a full Gateway process restart).

CE/EE/SaaS affected-package lint reported zero issues. Full-repository tests,
a full Gateway application build and production configuration/LLM integration
were not run. No full Gateway deployment or matching frontend was available.

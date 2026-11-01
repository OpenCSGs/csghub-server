# Auto route shared turn cache (EE / SaaS)

## Request behavior

Gateway resolves current model permissions and pinned upstream affinity before
Auto selection. For trusted callers, enabled caching checks L1 memory then shared
Redis L2 before constructing a Rank request or processing generation input.
A hit reuses the ordered ranking for environment + namespace + authenticated
caller scope + Session/Turn. Current permissions and availability are applied on
every request; a cached ranking is never an authorization grant or an upstream
binding. The same turn reuses its first ranking even if generation controls or
tool history change. The lower-level GetVariant API is not the business entry.

A scoped existing turn whose snapshot is lost/expired or whose cache is busy
never calls Rank with changed history. It falls back to at most three currently
authorized, available models in deterministic model-ID string order, preserving
Session/Turn. That order is not a Router score: rank and benchmark metadata are
cleared, and the model can differ from the lost snapshot. No usable candidate
returns model_unavailable after entering fallback. An empty authorized candidate
set is rejected earlier with the existing model_not_found; cancellation takes
precedence once a routing operation has started. Other input or Router
errors are not swallowed by this fallback. Redis read errors count as misses;
a still-valid L1 entry remains usable.

Anonymous, bypass and identity-less callers do not share snapshots: each request
uses compact v1 Rank even when it carries a turn ID. No anonymous scope is made.
Cache disabled preserves generation/v1 routing from the generation contract MR.
Pinned upstream resolution remains unchanged. This feature contains no execution
observation, previous_call, receipt aggregation, feedback storage or delivery.

## Configuration

Cache defaults to disabled. Generation keeps its enabled default. Configure the
same Router authority and scope on replicas that share Redis. Use distinct scope
values for independent deployments. Authentication reuses STARHUB_SERVER_API_TOKEN;
an explicit token file takes precedence, including file read/empty-file errors.
Enabling either generation or cache requires a service credential and nonempty
environment/namespace. User API keys are never used as service credentials.

All variables below use the prefix OPENCSG_AIGATEWAY_.

| Suffix | Default and validation |
| --- | --- |
| ROUTER_CACHE_ENABLED | false |
| ROUTER_GENERATION_ENABLED | true |
| ROUTER_CACHE_ENVIRONMENT | default |
| ROUTER_CACHE_NAMESPACE | csghub |
| ROUTER_API_TOKEN_FILE | empty; optional platform-token override |
| ROUTER_CACHE_ENTRIES | 4096; minimum 1 |
| ROUTER_CACHE_BYTES | 16777216 estimated L1 bytes; minimum 1024 |
| ROUTER_CACHE_FALLBACK_CONCURRENCY | 16; minimum 1 |
| ROUTER_CACHE_RENEW_CONCURRENCY | 4; minimum 1 |
| ROUTER_CACHE_REDIS_TIMEOUT_MS | 100; 1 through 1000 |
| SEMANTIC_ROUTER_TIMEOUT_SECONDS | 5; effective cache timeout positive and at most 30 seconds |

Nonpositive configured RPC seconds use the existing 5-second default. Internal
safety is 50 ms and must be positive. Invalid limits or missing shared Redis fail
initialization. Capacity guards have no additional implied upper bounds.

## Lease authority, concurrency and failure

Router owns leases. Gateway validates snapshot identity, generation, revision,
expiry and TTL before caching; absent or malformed leases leave a usable Rank
response usable but uncached. Rank responses are limited to 65536 bytes; lease
renewal responses to 4096 bytes; Redis records to 65536 bytes. These are Gateway
limits, not provider limits. Input overflow remains HTTP 413, as described in
[the compact input contract](semantic-router-compact-input_ee.md).

Gateway schedules independent renewal when ten minutes or less remain. Renewal
uses POST /v1/rank/lease/renew with the same service token, scope, Session/Turn,
snapshot/generation/revision and configuration identity. Router controls the new
expiry; Gateway never extends a lease locally. Supported TTL is at most one hour,
and maximum expiry at most 24 hours plus one minute of clock tolerance from the
current validation time. Synchronize clocks across Gateway, Redis and Router.

Redis uses atomic GET+PTTL, generation/revision CAS and owner-checked unlock.
Decimal generation strings avoid Lua integer precision loss. Reads and same
revision writes do not extend TTL. Safety is deducted before publication and
Redis TIME caps absolute deadlines; L1 deadlines use monotonic time.

L1 is bounded by entry count and estimated bytes. Complete-identity misses
coalesce bounded Rank work; inference calls never coalesce. Waiters can cancel
individually, and shutdown cancels/joins workers. New-turn rank failures trigger
250 ms backoff; caller cancellation and local overload do not poison readiness.
Renewal has bounded workers, no waiting queue and at most two attempts with
jitter. Busy workers skip scheduling without discarding valid data.

Lease 409/410 invalidates only the matching snapshot/generation/revision.
Transient failures retain still-valid data. Cache-write failures do not reject
successful rankings. A scoped lost turn uses local fallback, never reranking.
Metrics use aigateway_auto_cache_ with fixed low-cardinality labels and never
include user, session, prompt or credential values.

## Rollout and rollback

Upgrade Router to support leases and matching service authentication first.
Verify Redis, scope isolation and recovery before enabling caching. Disable
ROUTER_CACHE_ENABLED to restore uncached ranking for unpinned requests; disabling
generation returns to compact v1. Do not restore old authority state into an
active namespace: use a new namespace after database resets or backup rollback.
CE does not execute Auto routing. Shared DTO/config declarations do not enable it.

## Validation entry points

Component tests cover cache-hit extraction bypass, current-candidate filtering,
local fallback, cancellation, unscoped v1 and configuration. Set
ROUTECACHE_TEST_REDIS to an isolated Redis address for cross-Gateway, CAS and
lease-renewal tests in component/routecache and builder/store/cache.
TestCacheHTTPReuseAndLostTurnFallback exercises real HTTP Rank and Redis through
the component constructor with generation both enabled and disabled. The manual
integration harness in tests/integration/routecache also requires a compatible
Router. These checks do not replace deployment-specific authorization/billing QA.

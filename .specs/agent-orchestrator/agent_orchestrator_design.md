# Agent Orchestrator Design

Status: proposed implementation contract, updated 2026-09-25. Issue: [product/starhub/starhub-server#1509](https://git-devops.opencsg.com/product/starhub/starhub-server/-/issues/1509). Edition: EE/SaaS (`//go:build ee || saas`).

Related:

- [Agent Registry](../agent-registry/agent_registry_design.md)
- [CSGClaw instance lifecycle](../agent-csgclaw/agent-instance-lifecycle-api.md)
- [End-to-end API testing](./agent_orchestrator_e2e_api_testing.md)

## 1. Decision

`AgentInstance` represents product-visible agents. Immutable `kind: logical | runtime` distinguishes catalog definitions from independently created runtime agents. `type` remains the implementation, such as `csgclaw`, and is independent of `kind`.

A logical instance is one immutable executable definition plus mutable catalog presentation and visibility. It is not a version container, and there is no separate logical-agent version resource or table. Template, image, model, resource, allowed environment, mode, storage policy, and blueprint are fixed when the logical agent is created. A user who needs different executable settings creates another logical agent with another `content_id`.

Mutable logical fields are limited to visibility plus catalog and bounded access metadata such as name, description, custom UI, and guest usage policy. Visibility changes never replace the executable definition.

An orchestrated actor runtime is not an `AgentInstance`. `AgentAllocation` owns its identity, resource key, generation, lifecycle capacity accounting, and SandboxV2 binding. The logical agent never owns a sandbox, and the publisher's sandbox is never used to serve consumers.

There is no separate publication object and no public allocation API. `/agent/instances` contains logical agents and independently created runtime agents only; allocation lifecycles are internal.

## 2. Ownership boundaries

- Agent Server manages agent definitions, catalog state, authorization, allocation, capacity accounting, admission, and lifecycle coordination.
- CSGClaw implements the Harness: model and tool execution, conversation state, context management, and execution recovery.
- SandboxV2/Runner manages compute, storage, networking, and workload readiness.
- PostgreSQL is authoritative for allocation, capacity accounting, admission, and durable operation state.
- Temporal executes lifecycle operations. It does not relay chat data or run the model/tool loop.
- AIGateway authenticates callers, invokes the in-process orchestration component, and proxies admitted traffic directly to CSGClaw.

`AgentAllocation` is the stable physical identity for an orchestrated runtime across sandbox replacement. Agent Server must not grow a second Harness or a generic provider layer in this milestone.

## 3. Goals and boundaries

Goals:

- Create a usable catalog agent without starting or exposing the creator's sandbox.
- Give each actor a dedicated runtime and reuse it for the same logical agent.
- Preserve runtime and session identity across retries and sandbox replacement.
- Recover provisioning and deletion without leaking resources or capacity reservations.
- Support `read_only` with no PVC and `read_write` with actor-isolated persistent storage.
- Keep logical identifiers and physical routes distinct.

V1 boundaries:

- Dedicated execution only. Warm pools and shared replicas are future optimizations.
- No publisher PVC, workspace, memory, MCP credentials, or private knowledge-base mount is copied into a consumer runtime.
- The platform-managed creator CSGHub/CSGClaw token is resolved immediately before provisioning and injected only into the protected runtime environment. It is never accepted from a client or returned by a sandbox API.
- No generic Run API, server-owned tool loop, checkpoint engine, context compactor, or subagent scheduler.
- Anonymous visitors execute a public logical agent through any valid share link by sending `X-CSG-Guest-ID` (UUIDv4). Each guest receives one derived opaque actor and one allocation per logical agent. The allocation is accounted to the creator's runtime principal and follows the normal lifecycle.
- CE does not expose logical-agent APIs.

## 4. Domain model

| Object | Responsibility |
|---|---|
| `AgentTemplate` | Existing authoring input |
| `AgentInstance(kind=logical)` | Immutable executable definition plus mutable catalog/publication fields; no sandbox |
| `AgentInstance(kind=runtime)` | Independently created, user-managed runtime agent |
| `AgentAllocation` | Internal logical-agent/actor runtime identity, resource key, generation, capacity accounting, and lifecycle progress |
| Existing instance session | Product session addressed through the logical agent and internally bound to an allocation |
| Execution admission | Durable execution state and outcome history; never rewritten to free capacity |
| Execution lease | Ephemeral per-session execution exclusion and allocation capacity hold; the only rows that expire or get reconciled |
| Sandbox binding | Existing deploy identity and runtime generation |

```text
AgentInstance(kind=logical, immutable execution definition)
                         |
              +----------+----------+
              |                     |
     AgentAllocation(Alice) AgentAllocation(Bob)
              |                     |
         Sandbox/CSGClaw       Sandbox/CSGClaw
```

Standalone runtime agents remain ordinary `AgentInstance(kind=runtime)` rows outside this graph. An allocation references its logical agent and actor; it does not create a child AgentInstance. The logical owner gains no access to another actor's allocation.

Invariants:

1. `kind` and logical executable fields are immutable.
2. At most one active allocation exists for `(logical_instance_id, actor_kind, actor_id)`.
3. Each allocation and resource key belongs to one actor and is never reassigned.
4. Allocation, session binding, and capacity reservation commit before external provisioning.
5. A pending runtime may exist without a deploy; missing deploy metadata never determines kind.
6. Routing requires current authorization, allocation binding, verified workload identity, application readiness, and execution admission.
7. Sessions are addressed through the logical AgentInstance and remain internally bound to their allocation.
8. Making a logical agent private blocks new cross-user entry but does not silently detach or mutate existing runtimes.
9. `public` is visibility, not kind.

## 5. Logical-agent contract

### 5.1 Creation

`POST /agent/instances` with `kind=logical` requires:

- user-supplied immutable `content_id`;
- supported `type` and accessible template;
- optional `mode: read_only | read_write` (omitted mode defaults to `read_only`; the server persists the derived `storage_policy` and returns both effective fields);
- product-level `metadata.provision_request` such as model and resource selection.

Clients do not send a blueprint, image, port, mounts, protected credentials, or integration internals. The type-specific adapter resolves the template and server runtime profile into a canonical credential-free blueprint and stores it directly on the logical instance.

Creation atomically stores:

- canonical blueprint and blueprint hash;
- server-managed runtime image reference, command, port, readiness contract, and profile fingerprint;
- template/artifact identifier;
- model, resource, and permitted non-secret environment;
- effective mode and derived storage policy.

The executable fields cannot be updated. A request that tries to change template, image, model, resource, environment, mode, storage, or blueprint returns `400` and directs the client to create a new logical agent. This product does not expose logical-agent versioning.

Logical creation produces a complete, immediately usable definition and no sandbox. The server rejects unsupported required runtime capabilities from the resolved blueprint before insertion. It relies on live content-ID/name uniqueness and does not require an idempotency header. A client that loses the response resolves the agent by content ID.

### 5.2 Mutable catalog fields

Name, description, `public`, `custom_ui`, `guest_usage_limit`, and explicitly approved catalog/access metadata remain mutable through the logical update API. Updates are last-write-wins: the store locks the row, verifies owner and kind, and rejects updates once deletion has started. Generic updates cannot mutate executable fields.

`public=false` permits owner use and blocks new cross-user discovery/use. `public=true` permits cross-user discovery/use under the existing catalog authorization rules. Changing a logical agent from public to private does not detach or delete already allocated actor runtimes; those runtimes remain usable through the authorized logical-agent route until explicit deletion.

### 5.3 Runtime image and artifact pinning

The server runtime profile owns the shared CSGClaw image reference. V1 stores the configured value directly: a deployment tag, `latest`, or a digest. Clients cannot select it. Sandbox pods use Kubernetes' default image pull policy.

A mutable tag can resolve to different bytes after a restart. That is an explicit staging/development tradeoff. Reproducible production rollout requires a stable platform-managed tag or digest. This image choice is independent of the logical agent's product identity.

Repository artifacts must eventually be pinned by an immutable commit or artifact identifier that CSGClaw actually consumes and confirms at readiness. Recording a hash while booting a mutable branch is insufficient. Until that contract exists, production activation remains gated even if logical creation succeeds.

## 6. Serving modes and storage

| Contract | `read_only` | `read_write` |
|---|---|---|
| Runtime ownership | Dedicated to actor | Dedicated to actor |
| PVC | None | Actor-isolated persistent storage |
| Local files | Writable disposable container/scratch state | Persistent actor workspace plus bounded scratch |
| Publisher storage | Never mounted | Never mounted |

`read_only` means no PVC, including generic ephemeral volumes that create PVCs. It does not require a read-only root filesystem, and it does not automatically make tools or network side effects read-only. Runner must fail closed if defaults, init containers, sidecars, or admission injection attach a PVC.

`read_write` provisions new storage owned by the actor/allocation and seeds only approved artifacts. It never clones or mounts the publisher's live workspace.

The allocation workflow applies the logical instance's effective mode and storage policy to its sandbox. These fields cannot drift through generic updates or detach.

## 7. Allocation and provisioning

### 7.1 Reservation

Use of a logical agent:

1. Authenticate the caller and derive actor identity; never trust a client actor field.
2. Lock and verify the logical instance is not deleting and is either public or owned by the actor.
3. Find or insert the live allocation for `(logical_instance_id, actor_kind, actor_id)`.
4. Under the same transaction, lock capacity counters, reserve capacity, insert the allocation, bind the logical-agent session, and insert a durable lifecycle operation. Orchestrated CSGClaw capacity is unbounded by agent configuration; the locked counter grows before each new reservation so lifecycle accounting remains consistent.
5. Commit before calling Temporal or Runner.
6. Trigger best-effort immediate Temporal dispatch. The scheduled dispatcher remains the recovery path.

The reserved runtime resource key is DNS-safe, randomized, no more than 63 characters, and derived from the logical content ID plus NanoID: `aor-{logical-prefix}-{random}`. It is never the logical slug alone and is not predictable from a database ID.

Repeated use by the same actor and logical agent returns the same allocation. Different logical agents receive different allocations and sandboxes, even for the same actor. Session UUID reuse with a different actor, logical agent, or allocation is rejected.

### 7.2 Durable lifecycle

`agent_allocation_operations` is the dispatch source of truth. Dispatch uses operation keys and leases so crash-after-commit, duplicate dispatch, and signal failure converge. Provision retries repair the same live allocation. A permanent resource-free failure ends the binding and releases its capacity reservation atomically; ambiguous ownership retains both until reconciliation.

Sandbox ensure uses the reserved resource key as deploy and service name. Runner adopts an existing workload only when the `agent_allocation_id`, `agent_runtime_generation`, `agent_blueprint_sha256`, and `agent_storage_policy` annotations match. `agent_operation_key` is retained for lifecycle traceability but does not prevent a retry operation from adopting the same allocation generation. A mismatch is a conflict, never an adoption.

An adopted workload that is still starting remains in a non-pending deploy state and is polled with an image-startup-scale deadline. It is not recreated merely because readiness has not yet reached `Running`.

Readiness requires:

- matching ownership and generation annotations;
- matching no-PVC or actor-storage policy;
- running workload and application readiness;
- CSGClaw compatibility and accepted configuration when that API is available.

## 8. Data plane, sessions, and admission

```text
Client
  |
  | agent address + session + credentials + input
  v
AIGateway -- authenticate/derive actor --> Agent Orchestrator component + PostgreSQL
  |                                             |
  |                              reserve/find runtime and durable operation
  |                                             |
  |                                      Temporal worker
  |                                             |
  |                                      SandboxV2 / Runner
  |                                             |
  |                                      CSGClaw readiness
  |
  | recheck authorization/readiness + acquire execution admission
  v
CSGClaw in the actor runtime -- HTTP/SSE --> AIGateway --> Client
```

Gateway calls to the orchestrator are in-process component calls backed by PostgreSQL, not chat proxy hops. An already-ready runtime skips lifecycle work but never skips authorization or admission. The orchestrator and Temporal never relay response chunks.

Sessions use an internal `agent_allocation_sessions` registry keyed by `(allocation_id, session_uuid)`. Actor identity is carried by the parent allocation, not duplicated onto product `agent_instance_sessions`. `POST /agent/instances/:id/use` is the mutating start/repair entry point: it returns `202` with public `status=provisioning` and `retry_after_seconds=5` while the sandbox is created or repaired, and `200` with `status=ready` when the runtime fence is already satisfied. Clients must not poll by repeating `/use`. Instead they poll the read-only `GET /agent/instances/:id/runtime-status`, which never reserves capacity, creates operations, dispatches Temporal, or repairs a failed provision. Allocation IDs and resource keys are never public response fields. Pending does not authorize forwarding.

Logical-agent chat (`POST .../responses`) calls `UseLogicalAgent` once, then waits up to the separately configured gateway wait timeout (default 30 seconds) for the pinned allocation fence. It never returns `202`. When the wait expires, the gateway returns `503` with `Retry-After: 5`. An open-binding provision failure returns `503` without retry guidance. Deletion, release, ended binding, or a pinned-generation change returns `409`. Actor or runtime-principal mismatch returns `403`. No SSE bytes are emitted before readiness. The Temporal/Sandbox provision deadline remains the existing 1,800-second readiness timeout and is independent of the HTTP wait.

Public runtime-status mapping:

| Allocation row | Public status |
|---|---|
| No row, active logical | `not_started` |
| No row, deleting logical, owner | `deleting` |
| Live pending/provisioning, desired running | `provisioning` |
| Live ready, current and accepted generations match | `ready` |
| Live open-binding failed | `failed` |
| Live deleting, or desired released | `deleting` |
| Ended binding, observed released | `released` |
| Ended binding, observed failed | `failed` |

Status authorization is the logical-agent owner, a caller with a live or ended allocation for that logical agent, or a non-owner when the agent is public and deletion has not started. After the logical soft-delete is visible to ordinary queries, status returns `404`.

Every accepted execution receives a server-generated `execution_id`. Execution admission is scoped per **session**: CSGClaw safely runs different sessions concurrently, but overlapping turns within one session are unsafe. PostgreSQL enforces one open lease per `(allocation_id, session_id)` (`agent_execution_leases ... WHERE ended_at IS NULL`) plus a configurable cap of concurrent executions per allocation (`OPENCSG_AGENT_ORCHESTRATOR_MAX_CONCURRENT_EXECUTIONS_PER_ALLOCATION`, default `4`, a temporary static value until load-aware admission exists). A same-session collision returns `409`; an exhausted allocation cap returns `429` with `Retry-After: 5`; lifecycle conflicts return `409`. The gateway strips caller-supplied orchestration headers and forwards only admitted allocation identity and generation headers. `X-CSG-Agent-Runtime-ID` remains the compatibility header name and carries the allocation ID as an opaque value.

Leases reconcile lazily inside the admission transaction (the allocation row lock serializes the decision; executions themselves are not serialized): a stale `reserved` admission (no proxy start within `..._EXECUTION_RESERVED_TIMEOUT_SECONDS`, default 60s) is canceled; a stale `running` admission (no heartbeat within `..._EXECUTION_HEARTBEAT_TIMEOUT_SECONDS`, default 90s; the gateway heartbeats every `..._EXECUTION_HEARTBEAT_INTERVAL_SECONDS`, default 30s) becomes `unknown`. The capacity count is computed in SQL and excludes expired holds even when reconciliation has not touched a lease yet. All writers lock the admission row before its lease row. Heartbeats hold the admission lock while refreshing the lease and are rejected unless the admission is still `running`, so a heartbeat racing stale reconciliation always wins for a live execution and a late heartbeat can never revive a closed or unknown lease. A completion racing the other way still resolves the execution: `unknown` accepts a late `completed`/`failed` while its lease is open and the admission's generation still matches the allocation's accepted generation. The heartbeat interval/timeout relationship (`timeout > interval`) is validated at service startup.

Transport completion alone is not proof of CSGClaw completion. An `unknown` admission keeps fencing its own session and counts against allocation capacity only for a bounded hold (`..._EXECUTION_UNKNOWN_CAPACITY_HOLD_SECONDS`, default 1h) that starts at the transition to `unknown`. It never blocks sibling sessions forever. Retrying an unknown-fenced session begins (or reuses) one durable `restart` operation: the hidden runtime is deleted and re-provisioned under an advanced runtime generation, all previous-generation open admissions are marked `interrupted`, and their leases are closed. The restart is the authoritative termination for an ambiguous execution; it may interrupt the actor's sibling sessions, which is acceptable because an allocation belongs to exactly one actor — no other tenant's runtime or sessions can be affected. The gateway waits for the new generation outside any database transaction (up to `OPENCSG_AGENT_ORCHESTRATOR_RESUME_WAIT_TIMEOUT_SECONDS`, default 60s), then retries the admission once. `unknown` outcomes are never auto-rewritten to a terminal state to free capacity. Agent Server stores admission bookkeeping, not model output or CSGClaw checkpoint state.

Suspension is a runtime power state, separate from allocation provisioning: the Runner idle sweeper suspends an idle labeled sandbox while its in-flight request counter is zero, and the allocation keeps its identity, session bindings, and runtime generation. The Runner webhook reports `Sleeping`; the API then marks the allocation `suspended` and closes remaining leases (`reserved` → `canceled`, `running`/`unknown` → `interrupted`). The recording is idempotent and attempted on every `Sleeping` event, because the Runner drops failed webhook pushes and the first attempt may fail transiently. The next chat triggers an idempotent generation-preserving `resume` operation and waits up to the resume timeout; a timeout returns `503` with `Retry-After: 5`, and deletion, release, or a generation change returns `409`. `POST /use` may return `202`; chat never returns `202`.

Runner topology invariant: the in-flight request counter is process-local to one Runner instance, so all proxy traffic and idle sweeping for a sandbox must reach the same Runner process. Distributed activity coordination is required before horizontally scaling Runner for the same cluster.

Orchestrated sandboxes have no public AgentInstance ID or content ID. Generic sandbox get/start/stop/update/SSE/proxy routes reject a live allocation resource key; only the logical route may resolve it after authorization and admission. Owner authorization alone is insufficient.

`CSGCLAW_ADVERTISE_BASE_URL` uses the logical address `/v1/agent/csgclaw/agents/{logical_content_id}`. The internal allocation resource key is never advertised.

## 9. Listing, lookup, and route dispatch

`GET /agent/instances` preserves the existing owner-or-public listing scope and may filter `kind=logical|runtime`. It returns logical agents and independently created runtime agents; orchestrated allocations never appear in this list.

Logical detail never exposes allocation IDs, resource keys, internal proxy targets, or another actor's runtime state. Standalone runtime detail keeps the existing runtime projection.

Dispatch by kind before any deploy lookup:

| Route family | Logical | Runtime |
|---|---|---|
| list/get | Catalog projection | Runtime projection |
| visibility update | Last-write-wins `public` update | Existing runtime rules |
| `/use` | Resolve/create caller runtime; return status/retry guidance only | Unsupported |
| `/runtime-status` | Read-only caller runtime status; no repair | Unsupported |
| sessions via logical ID | Operate on logical sessions with internal allocation binding | Existing standalone runtime sessions |
| restart/suspend/resume/tasks/schedulers | `400` unless explicitly resolved by contract | Existing standalone runtime operation |
| by-content-ID update/delete | Logical catalog behavior | Runtime behavior |
| sandbox proxy/get/start/stop/update/SSE | Never treat slug as deploy | Existing standalone runtime behavior; allocation resource keys reject |

Existing `/agent/shared/instance?share_uuid=` remains the share-UUID catalog API and returns `kind` without presenting a logical content ID as a sandbox name. `/agent/instances/shared/:share_name` is a separate catalog/resolution route and must be registered before `/:id`.

Logical writes remain disabled until every ID, content-ID, share, session, template-instance, status, monitor, task, scheduler, and gateway route is kind-aware. A public lookup grants catalog visibility. Execution of a public logical agent by an anonymous visitor is allowed only on the exact shared CSGClaw responses path, and every request revalidates the share, visibility, deletion fence, type, and agent name.

## 10. Deletion and retention

Deleting a logical agent is a durable cascade:

1. Verify owner and kind, set the internal `deletion_started_at` fence, set `public=false` for immediate visibility shutdown, and enqueue delete operations for every live allocation in one transaction.
2. Temporal deletes each owned sandbox, confirms the result, clears protected deploy credentials, releases capacity accounting, ends bindings, and removes orchestrated session bindings.
3. Finalize the logical tombstone only after all allocation cleanup completes.

HTTP deletion returns pending while work remains. Retries are idempotent. A Kubernetes timeout or partial failure does not report final success and does not abandon the remaining runtimes.

Logical cascade deletion is addressed by numeric instance ID only. The by-content-ID delete route rejects logical agents: live `(type, content_id)` uniqueness is partial, so a deleted agent's content ID can be reused by a replacement, and a content-ID lookup cannot bind a retry to the original resource.

Logical status is derived as `Running` or `deleting` from the internal deletion fence: a logical agent is an always-available definition and reports `Running` until deletion starts, matching always-on runtime agent types such as langflow. There is no draft, published, unpublished, or publication-status field in persistence or API responses.

The deletion fence, not `public`, is authoritative for update, use, share, retry, and finalization. A visibility change is not delete and does not end bindings. Detach, if added later, must quiesce execution, reconfigure and verify private addressing, transfer capacity accounting atomically, then end the serving binding. It must preserve storage mode and provenance.

## 11. Schema

| Table | Required shape |
|---|---|
| `agent_instances` | Immutable `kind`; logical-only executable definition and internal deletion fence; runtime rows are independent user-managed agents |
| `agent_allocations` | `logical_instance_id`, actor identity, runtime principal, billing owner, unique resource key, runtime generation, lifecycle state, operation sequence, error/TTL/release fields |
| existing `agent_instance_sessions` | Legacy runtime-agent chat/history sessions only; not used for orchestrated admission |
| `agent_allocation_sessions` | Orchestrator execution-session registry scoped by `(allocation_id, session_uuid)` |
| `agent_guest_usage_counters` | One row per logical agent and guest actor. Limited guest policy increments `use_count` in the admission transaction |
| `agent_allocation_operations` | Durable provision/restart/resume/delete operation, lease, attempts, operation key, generation, and terminal result |
| capacity counters/reservations | Locked orchestrated-runtime lifecycle accounting with reserved, consumed, and released state; not a configured quota gate |
| `agent_execution_admissions` | Allocation/session/actor/logical binding, execution ID, runtime generation, usage reservation, heartbeat, and observed status (durable history) |
| `agent_execution_leases` | Per-session open-lease uniqueness, heartbeat, bounded unknown capacity hold, and end reason (ephemeral concurrency state) |

Required uniqueness includes:

```sql
CREATE UNIQUE INDEX agent_allocations_live_actor_logical
ON agent_allocations (logical_instance_id, actor_kind, actor_id)
WHERE binding_ended_at IS NULL;
```

Keep `(type, content_id)` unique for live instances in this development schema. Allocation resource keys are unique and server-generated; logical content IDs are user-supplied.

Logical executable JSON belongs on the logical row because it is the single immutable product definition. State and relationships remain columns; JSONB is limited to the typed blueprint document, with `blueprint_sha256`, `effective_mode`, and `storage_policy` as companion columns.

## 12. API contract

| API | Logical behavior | Runtime behavior |
|---|---|---|
| `POST /agent/instances` | `kind=logical` creates a complete immutable executable definition with requested visibility; no sandbox | omitted kind preserves legacy runtime creation |
| `GET /agent/instances`, `GET /:id` | catalog projection | runtime projection |
| `PUT /agent/instances/:id` | catalog-only update; executable changes rejected | existing runtime update rules |
| `POST /agent/instances/:id/use` | reserve/find caller allocation and session; return status/retry guidance only | unsupported |
| `GET /agent/instances/:id/runtime-status` | read-only caller runtime status; never starts or repairs provisioning | unsupported |
| `DELETE /agent/instances/:id` | durable cascade | existing delete plus allocation coordination |

There is no logical-agent version/list API. OpenAPI and clients must not expose one.

Status mapping:

- `200`: ready/use succeeded, catalog mutation completed, or idempotent terminal result.
- `202`: control-plane provisioning (`POST /use`) or durable deletion is pending; include bounded retry guidance. Never used for chat `/responses`.
- `400`: invalid kind, immutable executable change, unsupported operation, or malformed identifier.
- `403`: unauthorized catalog/runtime action, forbidden capability, or actor/runtime-principal mismatch.
- `404`: resource not found in the caller's authorized scope.
- `409`: binding, lifecycle, adoption, configuration, deletion, release, pinned-generation conflict, or a same-session execution collision.
- `429`: allocation execution capacity exhausted (`Retry-After: 5`) or bounded waiter/provisioning capacity exhausted.
- `503`: chat readiness wait timed out (`Retry-After: 5`) or open-binding provision failed (no retry guidance).
- `500`: unexpected internal failure.

## 13. Security and capacity

- Derive actor identity from authenticated login or a verified scoped guest credential.
- Treat logical `public` visibility as catalog access, not runtime authorization.
- Never pass a logical slug to `FindActiveSandboxByDeployName` or `GetProxyURL`.
- Allocation-owned sandbox get/start/stop/update/SSE routes reject with `409` unless an explicit admission-aware contract exists.
- Redact `CSGHUB_USER_TOKEN`, `CSGCLAW_ACCESS_TOKEN`, `CSGCLAW_LLM_API_KEY`, and future platform credentials from every response and log.
- Orchestrated CSGClaw capacity is not limited by `csgclaw_instance_quota_per_user`. Transactionally locked counters retain lifecycle accounting, grow before a new allocation is reserved, and are consumed or released exactly once by Temporal.
- Logical rows do not count as runtime capacity. Pending, failed-with-owned-resource, suspended, and superseded live allocations remain accounted until confirmed cleanup.

## 14. Rollout and verification

Rollout order:

1. Schema, kind-aware reads, route gates, and generated mocks.
2. Logical create and catalog/visibility update as always-on EE/SaaS capabilities.
3. Allocation, locked capacity accounting, durable provision/delete, SandboxV2 ensure/adopt, and live no-PVC verification.
4. Signed-in AIGateway admission and CSGClaw execution identity/status/cancel integration.
5. Portal flows and operational acceptance.
6. Guests, detach, warm pools, and shared replicas only as separately designed milestones.

Behavioral acceptance includes:

- logical creation stores one complete immutable executable definition and creates no sandbox;
- executable-field updates fail and catalog-field updates apply last-write-wins while rejecting updates after deletion starts;
- owner use works for private logical agents, while non-owner use requires `public=true`;
- unsupported runtime capabilities fail logical creation before persistence;
- concurrent first use creates one allocation and no runtime AgentInstance for an actor and logical agent;
- different actors receive different runtimes; different logical agents never share an allocation;
- read-only create/adopt/restart/resume contains no PVC anywhere in the final workload;
- creator credentials are present only inside the protected runtime and cannot be read from ordinary sandbox APIs;
- retries after commit, create ambiguity, dispatch loss, readiness delay, and deletion failure converge without leaks or double capacity release;
- logical deletion eventually removes every allocation sandbox and releases its capacity accounting;
- all list/detail/share/session/task/scheduler/status/proxy paths dispatch by kind before deploy lookup;
- a logical slug matching an unrelated deploy never proxies that deploy;
- allocation execution uses one admission path with per-session exclusivity: same-session collisions return `409`, exhausted allocation capacity returns `429`;
- CE, EE, and SaaS compile, migrations move forward and backward on a disposable database, and generated contracts contain no removed API.

Validation commands for implementation work:

```sh
make mock_gen
make swag
make test_all
make lint_all
```

Run focused package checks first and all builds sequentially on local developer machines. Do not include generated Swagger artifacts in the final change.

## 15. Implementation status

The current branch implements the management-plane safety slice as an always-on EE/SaaS capability. Logical-agent creation, catalog updates, sharing, execution, lifecycle management, guest access, and durable deletion are available without enablement flags; timeout, concurrency, image, and recovery settings remain configurable.

- logical and independent runtime `AgentInstance` rows, with orchestrated sandboxes owned by allocations;
- immutable executable fields stored directly on logical rows;
- catalog create and update without a draft/publication lifecycle;
- actor/logical allocation with randomized `aor-{logical-prefix}-{nanoid}` runtime names;
- locked capacity accounting, durable Temporal lifecycle operations, SandboxV2 ensure/adopt, and durable cascade deletion;
- no-PVC read-only propagation and Runner verification;
- creator-token runtime injection with attached-runtime sandbox API blocking and response redaction;
- signed-in execution admission with per-session leases, heartbeats, and a temporary per-allocation concurrency cap;
- anonymous execution of a public logical agent through any share link, with client-supplied `X-CSG-Guest-ID`, one guest allocation per logical agent, and creator-principal capacity accounting;
- automatic runtime recovery for unknown-fenced sessions via durable restart operations, and suspension-aware resume driven by the Runner idle sweeper and webhook.

Clearing `opencsg_guest_id` from client storage creates a new anonymous identity and can bypass the per-guest question limit and create another dedicated runtime. External rate limiting and operational resource controls remain the abuse boundary. Detach, warm pools, and shared replicas remain out of scope.

## References

- [OpenAI Agents API architecture](https://developers.openai.com/api/docs/guides/agents-api/architecture) — architectural separation of management, Harness, and execution environment; not a backend requirement.
- [OpenAI session recovery](https://developers.openai.com/api/docs/guides/agents-api/sessions) — distinguishes live delivery from saved execution state.
- [OpenAI self-hosted environments](https://developers.openai.com/api/docs/guides/agents-api/environments/self-hosted) — clarifies infrastructure and Harness ownership boundaries.

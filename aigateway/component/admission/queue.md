# Admission Reservation Queue — Architecture & Flow

This document describes the v2 admission reservation queue of the AIGateway
capacity admission control. It extends the v1 lease-based admission
(immediate admit / reject, see `README.md`) with a distributed waiting
structure: when an upstream is saturated, requests wait in a bounded
per-upstream priority queue and are atomically promoted into a real
admission lease when capacity frees up.

The queue is **not** a task queue. The accurate name is the one used
throughout the code: an *admission reservation queue*. Waiting does not
change the execution model — it only changes *when* a reservation is taken:
the request that would have been rejected with 429 instead parks a ticket in
Redis and blocks in its own goroutine until Redis grants it the reservation.

---

## 1. Design goals and invariants

1. **Distributed by construction.** The gateway is deployed with multiple
   replicas; all queue state lives in Redis, shared across replicas, under
   the same `{m:<modelID>}` cluster hash tag as the v1 lease keys so every
   upstream's queue, leases and windows stay in one cluster slot and remain
   atomically scriptable.
2. **Single-owner ownership transitions.** Exactly like v1, every state
   transition takes ownership via a successful `ZREM` before touching
   reservation aggregates. Three transitions matter:
   - `Ticket → Lease` (promote — the scheduler grants the slot),
   - `Lease → Finalize` (v1 release, unchanged semantics),
   - `Ticket → Cancel` (all exits: timeout, disconnect, reroute).
   Every transition is exactly-once across replicas; racing operations
   resolve to exactly one winner.
3. **No central scanner.** QueueWait timeouts are request-local timers;
   ticket/lease cleanup piggybacks on every script run; the fallback poll
   drives scheduling. No goroutine scans upstreams.
4. **Pub/Sub is acceleration, polling is correctness.** Redis Pub/Sub wakes
   the designated waiter faster; a lost message is recovered by the
   waiter's fallback poll within `QueueFallbackPollMs`. Nothing depends on
   message delivery.
5. **The queue never routes.** A ticket is bound to one upstream. If that
   upstream turns unavailable while waiting, the ticket is cancelled and
   the planner re-runs the full Router → Admission sequence. The queue
   itself never picks another upstream.
6. **Fallback never queues.** The attempt/fallback path (`Acquire`) always
   rejects immediately. An availability failover must not pile queue
   latency onto an already failing request.

### The Queue Mode Invariant

```text
Queue enabled  ⇔  CapacityPolicy.MaxQueueDepth > 0 (and QueueWaitSeconds > 0)

I1  Execution admission (direct admit, enqueue, dequeue, promote) depends
    ONLY on:  currentConcurrency < MaxConcurrency
I2  RPM/TPM MUST NOT block enqueue, dequeue or promote. In queue mode they
    are accounting and observability dimensions, never scheduling gates.
I3  RPM/TPM MUST NOT reject a request that is already queued.
I4  Queue-OFF upstreams keep the v1 three-dimension
    (Concurrency + RPM + TPM) immediate-429 semantics, byte for byte.
```

Rationale: the real inference load is carried by concurrency × request
duration — a large request naturally holds its execution slot for its whole
runtime and releases it on completion, which is a real-time capacity signal.
The fixed-window RPM/TPM counters are lagging proxies and would turn the
queue into a resource-aware distributed scheduler (head-of-line stalls until
a window rolls, permanently-infeasible heads). With the invariant, the
architecture is exactly Router + Admission/RateLimit + Concurrency Queue +
Lease. RPM/TPM keep being counted in queue mode (window gauges, usage
accounting, future provider quota protection); quota protection, if ever
needed, belongs at admission time BEFORE the queue (the reserved
`budget_exceeded` admission reason) — never in the dequeue path. The
promote script structurally cannot read RPM/TPM: the ticket metadata only
carries `maxConc`.

---

## 2. Data structures (per upstream)

All keys share the model's hash tag: `aigateway:capacity:{m:<modelID>}:u:<id>:…`

| Key | Type | Content |
|---|---|---|
| `…:conc` | ZSET | v1 lease set: member = lease token, score = expiry (epoch ms, Redis TIME authority) |
| `…:lease` | HASH | v1 lease metadata: token → `<est>:<windowStart>` |
| `…:queue` | ZSET | **the queue**: member = ticket token (uuid), score = `priority×1e13 + enqueueMs` |
| `…:qmeta` | HASH | ticket metadata: token → `<prio>:<est>:<enqueueMs>:<waitDeadlineMs>:<maxConc>` |

Design notes:

- **One ZSET serves priority + FIFO.** Score composition
  (`priority×1e13 + enqueueMs`, priorities `high=0`, `low=1`) orders the
  queue by priority first and by enqueue time within a priority. A single
  pop path (`ZRANGE 0 0`) is the head of line; adding priorities later is a
  constant change. Epoch-ms stays below the 1e13 stride until the year
  2286, so classes never mix.
- **Ticket metadata is self-contained.** It carries the priority, the TPM
  estimate (used only for the lease pre-reservation at promotion), the wait
  deadline and the upstream's `MaxConcurrency`, so the promote step
  (possibly executed by another replica that never saw the policy) needs no
  policy lookup — and structurally cannot gate on anything but concurrency.
- **`waitDeadlineMs` is the ghost-ticket backstop.** A ticket whose
  deadline passed belongs to a waiter that is gone (crashed replica; a live
  waiter cancels its own ticket). Cleanup scripts remove such tickets, so a
  dead ticket can never block the queue head forever.
- **`ZCARD` after cleanup is the queue depth.** It counts valid tickets,
  not waiting HTTP requests.

---

## 3. End-to-end flow

```text
                     Router
                        │  candidate set
                        ▼
              Initial Admission (Check, planner step 8)
                        │
        ┌───────────────┴────────────────┐
        │ feasible AND queue empty       │ infeasible, or queue non-empty
        ▼                                ▼
   Direct admit                    admit script enqueues a TICKET
   (lease acquired                 (ZADD score=prio×1e13+enqueueMs,
    immediately, v1 path)           HSET qmeta; queue full → 429 queue_full)
        │                                │
        │                          Waiting (request goroutine blocks)
        │                                │
        │                    ┌───────────┼─────────────┐
        │                    │           │             │
        │              Pub/Sub wake   fallback poll  request-local
        │              (promotion     (1s±jitter,    QueueWait timer
        │               publish)       also drives   / client ctx
        │                            the scheduler)  / availability
        │                    └───────────┼─────────────┘
        │                                ▼
        │                     promote (in release/finalize script):
        │                     cleanup expired leases+tickets
        │                       → head ticket → conc check ONLY
        │                         (currentConcurrency < maxConc — the
        │                          Queue Mode Invariant; RPM/TPM unread)
        │                       → ZREM ticket + create REAL lease
        │                         (expiry = claim deadline, 5s)
        │                       → PUBLISH queue-key ticket-token
        │                                │
        │                                ▼
        │                     claim (waiter): ZSCORE conc == my token
        │                       → extend expiry to full LeaseTTL
        │                       → register with lease renewer
        │                       → NEVER re-acquire (no herd, no race)
        │                                │
        └──────────────┬─────────────────┘
                       ▼
                   Execute → Finalize (release + promote next head, atomic)
```

Key points in the flow:

- **Unified queue discipline.** A candidate is directly admissible only when
  it is feasible **and** its queue is empty. Once a queue holds tickets,
  every new request enqueues and is scheduled by (priority, enqueue time) —
  a fresh high-priority request does not bypass already-waiting requests;
  it only sorts ahead of them inside the queue. Documented consequence:
  sustained high-priority traffic can starve low-priority tickets. Aging
  (promoting long-waiting tickets) is a deliberate non-goal for v2. For a
  queue-ON candidate "feasible" means the concurrency gate only (the
  invariant); for a queue-OFF candidate it keeps the v1 three-dimension
  gate.
- **Head-of-line blocking is bounded by construction.** Under the Queue
  Mode Invariant the promote gate is a single comparison
  (`currentConcurrency < maxConc`), so every slot release can promote the
  head immediately — no multi-dimension feasibility can stall the head, no
  request can be "permanently infeasible" for the queue. Slots freed
  without an explicit release (crashed replicas' leases reclaimed by the
  expired-lease cleanup that precedes promotion in the same script) are
  picked up by the same promote step.
- **Promotion is an ownership grant, not a hint.** The promote step of the
  combined release/finalize script converts the head ticket into a real
  lease with a short *claim-deadline* expiry (`QueueClaimDeadlineSeconds`,
  default 5s) and publishes the token. The waiter then *claims* it
  (extends to the full lease TTL, registers with the renewer) — it never
  re-acquires, so there is no thundering herd and no grant race. A grant
  that is never claimed (waiter crashed between grant and claim) expires at
  the claim deadline and is reclaimed by the regular expired-lease
  cleanup; the slot cannot leak. The promoted lease takes the ticket's TPM
  estimate as its pre-reservation (v1 lease creation semantics —
  accounting only, never a gate).
- **Progress without releases.** Every waiter's fallback poll runs one
  promote-ready step for the whole queue. Slots freed without a release
  event — crashed replicas' leases reclaimed by the expired-lease cleanup
  that runs inside the same script — are therefore picked up too.

---

## 4. Exit paths (one cancel script, three triggers)

All three exits run the same atomic `capacityQueueCancelScript`, which
covers both waiting states idempotently:

| State | Script behavior |
|---|---|
| Ticket still queued | `ZREM`-guarded removal from `queue` + `qmeta` |
| Ticket already promoted (lease granted, waiter gone) | ownership-guarded lease release + TPM reservation reclaim from the original window |

| Trigger | Detection | Response | Metrics |
|---|---|---|---|
| **QueueWait timeout** | request-local timer over the upstream's `QueueWaitSeconds` (Redis-computed deadline is authoritative; the local timer is a hint) | HTTP 408 `queue_timeout` | `queue_timeout` event + wait histogram `outcome=timeout` |
| **Client disconnect** | request context cancelled (gin `c.Request.Context()`) | no response (client is gone) | `queue_disconnect` event + `outcome=disconnected` |
| **Upstream unavailable** | waiter polls the injected availability checker on each fallback tick | planner re-runs Router → Admission (bounded by `QueueMaxRePlans`, default 1; exhaustion → 503 `model_unavailable`) | `queue_reroute` event + `outcome=reroute` |

Because the disconnect exit's context is already cancelled, the cancel
script runs on a detached context with its own short budget.

Race guarantee: cancel and promote are both atomic ownership transitions
on the same ticket. Exactly one wins — if cancel removes the ticket first,
the late promote finds nothing to grant; if promote granted first, cancel
releases the granted lease instead. A request can never both lose its
ticket and keep a grant.

---

## 5. Lua atomic operations

| Script | Responsibility |
|---|---|
| `capacityAdmissionScript` (admit/enqueue) | expired lease + ghost-ticket cleanup, mode-aware gating (conc-only for queue-ON, three-dimension v1 gate for queue-OFF), selection; direct admit into an empty queue, otherwise enqueue (depth check → `queue_full`) |
| `capacityQueueReleasePromoteScript` | Finalize: release lease (v1 semantics) **and in the same atomic execution** cleanup + head promotion under the concurrency-only gate (`Ticket → Lease`) + `PUBLISH`; also used with an empty release token as the per-tick scheduler step |
| `capacityQueueClaimScript` | waiter takes the granted lease: extends claim-deadline expiry to full LeaseTTL; tri-state return (claimed / still queued / ownership lost) |
| `capacityQueueCancelScript` | the dual-state idempotent cancel (Section 4) |
| `capacityAdmissionRenewScript` / `Commit` / `Observe` | unchanged v1 lease lifecycle |

All scripts run on one upstream's same-slot keys and use Redis server TIME
as the single time authority (replica clock skew is irrelevant).

---

## 6. Priority resolution

The queue priority comes from exactly ONE source — the API key's
server-side priority scope (`types.AdmissionPriorityFromSource`):

- The quota middleware reads the key's quota configuration
  (`AccountingQuotaValueTypePriority` scope, values `high`/`low`) and puts
  it into the request context; the Extract phase copies it to
  `RequestMetadata.PriorityScope`.
- Unrecognized or missing scope values default to `low`.
- **Edition note:** the quota middleware is currently SaaS-only
  (`quota_saas.go`; the `!saas` build ships a no-op). On EE every request
  resolves to `low` until an EE quota implementation populates the scope —
  the queue scheduling itself is edition-agnostic (`ee || saas`).

Clients cannot influence queue scheduling in any way: there is no
request-declared priority. This closes the reviewed self-declaration
attack surface — a key scoped to `low` can never jump the line, and an
anonymous client can never declare itself `high`. Scope management (which
keys get `high`) is an administrative decision made in the key's quota
configuration. Priority only affects waiting order; it never affects
immediate admission.

Operational note: the quota middleware currently mounts on the text routes
(chat/responses/messages/embeddings/rerank/anthropic); modal routes
(images/video/ocr/audio) do not mount it, so on those routes every key
resolves to `low` until the middleware is added there.

---

## 7. Degradation and compatibility

- **Feature gating.** Queueing activates per upstream when its
  `CapacityPolicy` has `MaxQueueDepth > 0` **and** `QueueWaitSeconds > 0`;
  otherwise behavior is byte-for-byte v1 (immediate reject). Existing
  deployments without queue fields keep working unchanged.
- **Redis failures.** v1 fail-open applies: an enqueue-time Redis error
  fails open (admit without a lease, per `FailOpen`); an error while
  waiting fails open on the next claim; Pub/Sub unavailability just
  removes the acceleration.
- **Rolling deploys.** Old replicas keep using v1 keys; the new keys are
  additive. Old-format tickets (no metadata) are dropped by cleanup.

---

## 8. HTTP semantics

| Situation | Status | Error code | Retry-After |
|---|---|---|---|
| Queue full (bounded backpressure) | 429 | `queue_full` | yes (hint) |
| QueueWait exceeded | 408 | `queue_timeout` | no |
| Left queue via disconnect | 408 | `queue_cancelled` | no |
| Capacity dimensions (v1) | 429 | `capacity_exceeded` / dimension reason | yes (hint) |

Queue waiting happens before any SSE header is written, so 429/408 reuse
the existing error rendering for both stream and non-stream requests.

---

## 9. Configuration

Global (`AIGateway.CapacityAdmission`):

| Field | Default | Meaning |
|---|---|---|
| `QueueFallbackPollMs` | 1000 | per-waiter fallback poll / scheduler tick |
| `QueueClaimDeadlineSeconds` | 5 | grant-to-claim window before the grant is reclaimed |
| `QueueMaxRePlans` | 1 | re-plan budget for reroute exits |

Per upstream (`CapacityPolicy`, existing fields now enforced):
`MaxQueueDepth` (queue bound), `QueueWaitSeconds` (max wait tolerance).

---

## 10. Metrics

| Metric | Type | Labels |
|---|---|---|
| `csghub_aigateway_admission_queue_depth` | Gauge | model, upstream_id |
| `csghub_aigateway_admission_queue_wait_ms` | Histogram | model, outcome(admitted/timeout/disconnected/reroute) |
| `csghub_aigateway_admission_queue_events_total` | Counter | model, event(enqueued/promoted/queue_full/queue_timeout/queue_disconnect/queue_reroute/claim_lost/cancel) |

The existing admission decision/latency/Redis-error metrics are reused
(queue script failures count into `admission_redis_errors_total` with
`queue_claim` / `queue_promote` / `queue_cancel` operations).

---

## 11. Testing

- **Unit (miniredis, real Lua + real Pub/Sub):** `capacity_queue_ee_test.go` —
  direct admit, enqueue on saturation, queue_full, priority + FIFO
  ordering, unified discipline (no jump-ahead into a non-empty queue), full
  release → promote → Pub/Sub hand-off, claim deadline grant/extend/
  reclaim, ghost-ticket cleanup, cancel dual-state idempotency,
  cancel-vs-promote race (exactly one winner), disconnect exit, reroute
  exit, lost-wake-up recovery, and `TestQueue_ChaosConservation_MixedExits`
  (mixed exits must leave the Redis ledger exactly zero).
- **Invariant guards:** `TestQueue_Invariant_TPMDimensionNeverGatesQueueMode`
  (an estimate above `MaxTPM` is admitted directly, and a queued head is
  promoted while the TPM window sits far over its limit — the regression
  test that keeps RPM/TPM out of the queue path) and
  `TestQueue_MixedCandidates_QueueOnSelectedByConcurrencyOnly` (mixed
  candidate sets: queue-OFF candidates keep the v1 gate, queue-ON
  candidates are gated by concurrency only).
- **Handler/planner:** 408/429 mapping (`admission_lease_test.go`),
  bounded reroute re-planning (`planner_admission_test.go`), priority
  resolution (`types/admission_test.go`).
- **Live verification:** `aigateway/cmd/admission_queue_loadgen` (a `//go:build ignore` dev tool, run via `go run aigateway/cmd/admission_queue_loadgen/main.go`) drives the real
  gateway (staggered high/low arrivals, mid-queue disconnects, timeouts)
  against fake upstreams; its README documents the scenario recipes and
  the conservation checks (`conc`/`queue`/`qmeta` zeroed after drain,
  `tpm` = Σ actual usage, `rpm` = acquire count).

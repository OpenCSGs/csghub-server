# AIGateway MCP Gateway: Dual-Serve Stateless (2026-07-28) + Legacy Stateful Design

Edition: `ee || saas` only.

## Decision Summary

The gateway serves two protocol families on the existing `/mcp` endpoint:

- MCP `2026-07-28` uses a shared, stateless server with no client-session
  registry.
- Earlier revisions continue through the existing stateful handler, Redis
  ownership registry, and peer proxy.
- The gateway terminates both the client and backend protocol connections. It
  adapts metadata, errors, caching, capabilities, and MRTR at that boundary.
- Stateless catalog generations are immutable. Configuration or capability
  changes publish a new generation and retire the previous session pool.
- Backend sessions are shared only when the negotiated backend protocol is
  stateless. Legacy backend sessions are isolated by effective credentials.

The remainder of this document defines these decisions, their implementation
boundaries, and the tests required to preserve them.

## Background

The MCP `2026-07-28` revision makes the protocol core fully stateless:

- No protocol-level sessions: `Mcp-Session-Id`, the `initialize`/`initialized`
  handshake, standalone GET streams, and `Last-Event-ID` resumability are
  removed. Capability discovery is an optional `server/discover` RPC.
- Every POST MUST carry `MCP-Protocol-Version`; the value MUST match
  `_meta.io.modelcontextprotocol/protocolVersion` in the body. `Mcp-Method`
  (all requests) and `Mcp-Name` (`tools/call`, `resources/read`, `prompts/get`)
  are REQUIRED headers so intermediaries can route without parsing bodies.
- List results carry `ttlMs`/`cacheScope` freshness hints. `ping`,
  `logging/setLevel`, `resources/subscribe` are removed; roots, sampling and
  logging are deprecated; server-to-client interactions become in-band MRTR
  (`InputRequiredResult`).
- Absent `MCP-Protocol-Version` header implies a pre-2025-06-18 (legacy-era)
  client.

Our current gateway (`ANY /mcp`) is stateful: SDK `StreamableHTTPHandler` with
`Stateless: false`, per-client-session `sdkmcp.Server` built on `initialize`,
a Redis session registry mapping session → instance, and a reverse-proxy layer
routing requests to the owning instance. None of that is needed for 2026
clients, but legacy clients (2025-11-25 and earlier) must keep working.

## Goals

- Serve both MCP protocol revisions on the same `/mcp` endpoint:
  - `2026-07-28` clients: native stateless and horizontally scalable, with no
    Redis session-ownership or correctness dependency. Redis is used only as a
    fail-open fingerprint-cache optimization.
  - Legacy clients (`2025-11-25` and earlier): today's stateful path,
    bit-for-bit unchanged (session IDs, SSE resumability, Redis peer routing).
- Bridge to all backends: 2026 clients see tools from every configured backend
  regardless of the backend's protocol revision (SSE, stateful streamable, or
  stateless streamable); the gateway holds backend state in its session pool.
- Record each backend's negotiated protocol version in the capability cache
  for observability.

## Non-goals

- Mid-call elicitation / sampling from **legacy** backends (2025-style
  server→client requests): unsupported on both paths — there is no channel to
  the client for them. Legacy-negotiated backend sessions therefore declare
  no interactive capabilities, so conforming legacy backends never issue them
  (status quo preserved). Interactive MRTR **is** bridged for 2026-negotiated
  backends — see Protocol Adaptation Boundary → MRTR bridging.
- `subscriptions/listen` fan-out to clients, and progress-notification
  forwarding (`progressToken` is not forwarded). Clients rely on `ttlMs`
  freshness hints instead of change notifications.
- Per-version tool inventories (hiding legacy-backend tools from 2026
  clients).
- Config flag / kill switch for the stateless path (decision: always on).
- Admin-set backend protocol profile override (auto-detection via SDK
  negotiation suffices; add only if misclassification appears).
- Wrapping/rewriting backend `requestState` (MRTR): passed through opaquely;
  its integrity and scoping are the backend's responsibility (the spec treats
  it as attacker-controlled input; backends must scope it server-side, not
  session-side, since retries may arrive on a different pooled connection).

## SDK Compatibility Requirements

`github.com/modelcontextprotocol/go-sdk` v1.3.1 → v1.8.0 (latest stable).

- v1.7.0 added 2026-07-28 support: stateless server (`Stateless: true`),
  `server/discover`, MRTR, cacheable list results, header standardization,
  automatic legacy `initialize` fallback on the client side.
- v1.8.0 adds `ServerOptions.SupportedProtocolVersions`, `SetCacheable`, and
  makes stateful handlers answer 2026 requests with `-32022
  UnsupportedProtocolVersion` + advertised versions (safe client
  renegotiation).
- Do not rely on `MCPGODEBUG` escape hatches; they are removed in v1.9.0.
- Blast radius: only `builder/mcp/*` and
  `aigateway/handler/mcp_gateway_ee.go` import the SDK.

### Required v1.8.0 behavior

Confirmed by reading `go-sdk@v1.8.0/mcp` source, not release-note summaries:

<details>
<summary>SDK source verification</summary>

- `serveStateless` (streamable.go:373): calls `getServer(req)` **per
  request**; each request gets a temporary session closed on completion;
  GET/DELETE → 405; `Mcp-Session-Id` ignored by default. ⇒ `getServer` must
  be cheap: return a cached shared `*sdkmcp.Server`.
- Signatures we rely on are unchanged:
  `NewStreamableHTTPHandler(func(*http.Request) *Server, *StreamableHTTPOptions)`,
  `Client.Connect(ctx, transport, opts)`,
  `StreamableClientTransport{Endpoint, HTTPClient}`,
  `Tools/Resources/ResourceTemplates/Prompts` iterators, `Server.Sessions()`,
  `ServerOptions.KeepAlive` / `CompletionHandler`, `ToolHandler` func type,
  `ResourceNotFoundError`.
- Type changes requiring edits:
  - `Meta` is `type Meta map[string]any` (shared.go:504) — constructions need
    `mcp.Meta(meta)` conversion; ranging over `Params.Meta` still works.
  - `CallToolRequest = ServerRequest[*CallToolParamsRaw]` (requests.go:10);
    `CallToolParamsRaw.Arguments` is `json.RawMessage` (matches our existing
    `json.Unmarshal(callReq.Params.Arguments, &args)`).
  - `Tool.InputSchema` is still `any` — raw-schema passthrough unchanged.
  - `ServerRequest.Extra.Header` (`http.Header`) is available in handlers.
- Protocol behaviors verified in source that drive the adaptation boundary:
  - `setMCPHeaders` (streamable.go:2428): an outgoing message's
    `_meta["io.modelcontextprotocol/protocolVersion"]` **takes priority** over
    the connection's negotiated version. ⇒ Forwarding inbound client `_meta`
    to a legacy-negotiated backend would emit a 2026 version header and be
    rejected. The gateway must construct downstream `_meta` itself.
  - `ClientSession.CallTool` → `lookupTool` (client.go:1292/599): x-mcp-header
    `Mcp-Param-*` generation needs the tool definition from the session's
    tools cache, populated only by `tools/list` results **on that session**.
    ⇒ Pooled sessions must be warmed (see below).
  - `CallToolParamsRaw` carries `InputResponses` and `RequestState`
    (protocol.go:241), so MRTR retry fields can be passed through.
  - MRTR middleware is **on by default** on clients and servers
    (mrtr.go:21-25) and would try to fulfill backend input requests
    internally; a `Disabled` option exists. ⇒ Disable on the backend client
    so `InputRequiredResult` passes through to the caller.
  - `cacheScope` absent means **public** (protocol.go:1185). ⇒ Cache scope
    must be set explicitly on outward results.
  - `ResourceNotFoundError` now defaults to `-32602` (was `-32002` pre-v1.7;
    the `MCPGODEBUG=customresnotfounderrcode=1` restore is removed in v1.9).
    ⇒ Legacy-path error codes must be pinned explicitly, not inherited.
- `InitializeResult.ProtocolVersion` (protocol.go:1093) is the negotiated
  backend version source persisted by `Client.Inspect`.

</details>

## Architecture

A version dispatcher fronts two SDK handlers on the same route:

```
ANY /mcp  (NeedLogin)
   │
   ├─ MCP-Protocol-Version: 2026-07-28 ──► stateless handler (new)
   │     sdkmcp.NewStreamableHTTPHandler(getStatelessServer,
   │         &StreamableHTTPOptions{Stateless: true,
   │           PropagateRequestCancellation: true})
   │
   ├─ absent / known legacy ───────────────► legacy path (unchanged)
   │     MCPProxyAwareHandler → stateful SDK handler
   │     (Redis registry, peer proxy, per-session BuildUserGateway)
   │
   └─ present but unknown version ────────► 400 UnsupportedProtocolVersion
         listing the endpoint's full supported set (dispatcher-owned)
```

Dispatch rules:

| Request | Header | Route |
|---|---|---|
| 2026 client, any POST | `MCP-Protocol-Version: 2026-07-28` | stateless |
| legacy `initialize`, pre-2025-06-18 | absent | legacy |
| legacy 2025-06-18 … 2025-11-25 | matching 2025 version | legacy |
| future / unrecognized version | any other value | dispatcher `400` + supported set |
| GET / DELETE | absent / known legacy version | legacy |
| GET / DELETE | `MCP-Protocol-Version: 2026-07-28` | stateless handler `405 Method Not Allowed` |

If a 2026 request ever reaches the stateful handler, SDK v1.8 replies
`-32022` with supported versions and conforming clients renegotiate — a safe
degrade, not a hang. Header/body mismatch validation (`-32020`) is performed
by the SDK stateless handler.

Unknown versions are **not** equated with legacy clients: a present but
unrecognized `MCP-Protocol-Version` (e.g. a future revision) gets a
dispatcher-level `400` + `UnsupportedProtocolVersionError` listing the whole
endpoint's supported set (`2026-07-28`, `2025-11-25`, `2025-06-18`,
`2025-03-26`, `2024-11-05`) — the per-handler SDK errors only know their own
sub-handler's versions. The supported-version list lives in one constant used
by the dispatcher. Note: `server/discover` is optional for clients to call,
but servers must implement it (the SDK stateless handler does).

## Protocol Adaptation Boundary

The gateway is the **terminus of two independent protocol connections**, not
a pipe. Layer duties: handler = inbound protocol (dispatch, identity into
ctx, pass RPC inputs); component = identity policy (token injection) and
backend/isolation selection; builder = backend connections, version
negotiation, HTTP encoding. Cross-layer calls use a request struct that can
carry MRTR and protocol context (below), not the legacy `arguments + meta`
pair.

### `_meta` handling (stateless path)

Downstream `_meta` is **constructed by the gateway, never copied** from the
inbound request. Rationale: 2026 clients MUST send
`_meta.io.modelcontextprotocol/protocolVersion` on every call, and the SDK's
`setMCPHeaders` prefers the message's `_meta` version over the negotiated one
— forwarding it verbatim would emit a 2026 header at a 2025 backend.

Rules: start from an empty map; apply the `metaInjection` credential policy;
merge `mcp_server` (existing behavior); then merge a fixed client allowlist —
currently only `CSGHubAccessToken`, preserving the legacy "client `_meta`
wins over injection" precedence. Everything else, especially all
`io.modelcontextprotocol/*` keys and `progressToken`, is dropped. The SDK
then generates correct protocol `_meta`/headers per backend revision (it
auto-injects them on 2026-negotiated sessions via `injectRequestMeta`).
Legacy path keeps copying client `_meta` as today.

### Stable capability naming (stateless path)

`buildIndexes` currently lets map iteration order decide which backend keeps
an unprefixed name on clash — the same name can route to different backends
on different instances or after availability changes. The stateless shared
gateway therefore uses **deterministic naming independent of order and
availability**:

- Every tool and prompt is exposed as `{serverName}_{name}` (always prefixed;
  server names are already constrained by `ValidMCPServerName` for this).
- Name ownership is computed from the **full known catalog** — every
  configured backend's last-known tool/prompt set, including currently
  unavailable backends (cached capability rows persist on error), backends
  sorted by ID. Ownership is therefore fixed by configuration, not by which
  backends happened to be reachable during this build.
- Collisions (e.g. backend `a_b` tool `c` vs backend `a` tool `b_c`) resolve
  to the lowest backend ID **in the ownership pass over the full catalog**;
  the loser is dropped with an error log. If the owner is unavailable in a
  partial build, the name is **absent from the catalog** (or a call to it
  returns backend-unavailable) — it is never transferred to the other
  backend, so a client-cached name never silently changes targets.
- Resources/templates (URIs can't be prefixed): same fixed-ownership rule on
  URI clash; conflicts logged.
- List output is sorted by name for cache-friendly deterministic catalogs.
- Legacy path keeps today's dynamic clash behavior (unchanged).

### Request boundary structs (all execution paths)

Every execution RPC crosses layers as a struct, so identity policy, MRTR
fields, and pool selection apply uniformly — no per-method patching. The
handler fills wire fields; the component rewrites `Meta` per the `_meta`
rules and sets `IsolationKey`; the builder executes:

```go
type CallContext struct {
    Meta               map[string]any   // gateway-constructed; see `_meta` rules
    ClientCapabilities map[string]any   // inbound client's caps, extracted by handler
    InputResponses     InputResponseMap // echoed from client retry (2026)
    RequestState       string           // echoed opaque backend state (2026)
    IsolationKey       string           // auth-context pool shard (below)
}
type ToolCall     struct { DisplayName string; Arguments json.RawMessage; CallContext }
type ResourceRead struct { URI string; CallContext }
type PromptGet    struct { DisplayName string; Arguments map[string]string; CallContext }
type CompleteCall struct { Params *mcp.CompleteParams; CallContext } // no MRTR fields on CompleteParams; credential policy + isolation still apply
```

`ReadResourceParams` and `GetPromptParams` carry `InputResponses`/
`RequestState` in the SDK (verified), so resource/prompt MRTR retries pass
through the same way. `CompleteParams` has none — it still carries
`IsolationKey` so completion uses the caller's pool shard.

### Session warm-up for header generation (builder)

`Client.NewSession` additionally populates the SDK session's tools cache for
2026-negotiated backends: one `tools/list` iteration right after connect, so
`CallTool`'s `lookupTool` can generate `Mcp-Param-*` headers declared via
`x-mcp-header`. Cost is one list RPC per pooled session, amortized by the
pool. Without this, a strict 2026 backend can reject first calls from cold or
newly-added pooled sessions with `-32020 HeaderMismatch`.

### MRTR bridging (interactive, 2026 backends only)

MRTR is in-band (`InputRequiredResult` on the response), so the stateless
gateway can fully bridge it for 2026-negotiated backends — no server→client
channel is needed. Capability advertisement is **revision-scoped per
connection and intersected per request**, because (a) declaring interactive
capabilities toward a legacy backend would invite 2025-style server→client
requests we cannot bridge, and (b) declaring capabilities the *external
client* doesn't have would let a backend legitimately issue input requests
the client cannot fulfill:

- The backend client keeps two SDK client instances: `modern` (declares the
  bridgeable capabilities — elicitation, sampling) and `legacy` (declares
  none). `NewSession` selects by the backend's last-known negotiated revision
  (capability cache `protocol_version`, recorded by `Inspect`); unknown
  backends default to `legacy` (safe side) and upgrade automatically once
  inspection records 2026.
- Revision mismatches are corrected **at acquire time, before any business
  RPC**: after `Connect`, if the `modern` client negotiated legacy, the
  session is closed immediately and reconnected with the `legacy` client; the
  downgrade is cached per backend for the rest of the generation. A legacy
  backend therefore never sees a tool call on a connection that declared
  interactive capabilities (tests assert no `tools/call` on the first
  connection — not just that it was eventually closed).
- Per-request downstream `clientCapabilities`: the handler extracts the
  inbound `_meta.io.modelcontextprotocol/clientCapabilities` into an explicit
  `CallContext.ClientCapabilities` field (not raw `_meta` passthrough); the
  component sets the downstream meta's clientCapabilities to
  `inbound ∩ gateway-bridgeable`, intersected deeply including
  sub-capabilities (e.g. individual elicitation modes). The key is **always
  set** for 2026-bound calls — including an explicit empty object when the
  intersection is empty — because the SDK's `injectRequestMeta` only fills
  *missing* meta keys (verified, client.go:528) and would otherwise refill it
  from the `modern` client's static declaration, re-advertising interactions
  the external client never declared.
- Result adaptation at the server boundary (verified, mrtr.go:39): the SDK's
  `handleMultiRoundTripResult` rewrites `resultType` to `complete` whenever
  `InputRequests == nil`, even on results that carry only `requestState`.
  Before returning through the SDK server, results with
  `NeedsInput() == true && InputRequests == nil` are copied and given an
  empty non-nil `InputRequests` map so the `input_required` result type
  survives.
  Tests assert the final `resultType` on tools/resources/prompts outputs.
- The SDK's default MRTR auto-fulfill middleware is **disabled** on both
  clients, so `InputRequiredResult` is returned to the gateway instead of
  being consumed internally.
- `InputResponses`/`RequestState` pass through end to end (handler copies
  them verbatim from `CallToolParamsRaw` / `ReadResourceParams` /
  `GetPromptParams`); retries route to the same backend via stable naming;
  `InputRequiredResult` from a backend is returned to the client unchanged
  (after the adaptation above) — the SDK server-side shim passes it to
  MRTR-capable clients.
- `requestState` is never wrapped or inspected by the gateway; its integrity
  and scoping are the backend's responsibility (the spec treats it as
  attacker-controlled input; backends must scope it server-side since retries
  may arrive on a different pooled connection).
- Trust note: declaring elicitation means an admin-configured backend can
  prompt end users through the client. Backends are admin-vetted; no
  additional gate this iteration.

### Outward cache & subscription contract

Set via `ServerOptions.SetCacheable` on the stateless server (absent
`cacheScope` defaults to public, so nothing is left unset):

- Catalog lists (`tools/list`, `prompts/list`, `resources/list`,
  `resources/templates/list`, `server/discover`) from a **complete** snapshot:
  `ttlMs = 60_000`, `cacheScope = private` (conservative default; the catalog
  is currently global, promoting to `public` is a separate product decision).
- Incomplete snapshot or `emptyServer()` degrade responses: `ttlMs = 0`,
  `cacheScope = private`.
- `resources/read`: the closure always forces `cacheScope = private`;
  backend TTL hints are passed through **including an explicit zero** (the
  `SetCacheable` hook only fills catalog defaults when `cacheScope` is unset,
  so it never overrides closure-set values).
- The stateless server does **not** declare `ListChanged` capabilities (the
  legacy server declares them today; nothing actually notifies). Freshness is
  conveyed via `ttlMs` only.

### Error normalization at the boundary

Distinguish three error classes: transport/auth failures (pass as-is), tool
execution results with `isError` (pass as-is), and JSON-RPC protocol errors
(normalize by revision at the boundary):

- Stateless path: known legacy codes from upstream legacy backends are
  translated to their 2026 forms before reaching the client — explicitly
  `-32002` (legacy resource-not-found) → `-32602`. The mapping table is
  small, explicit, and tested.
- Legacy path: pin today's observable codes instead of inheriting new SDK
  defaults — replace `sdkmcp.ResourceNotFoundError` (now `-32602` in v1.8)
  with an explicit `-32002` construction, so the legacy contract survives the
  SDK upgrade. Regression tests assert both directions.

### Backend session isolation across users

The legacy path pools per client session (effectively per user). The shared
gateway pools per instance, so a legacy backend session could carry
user-scoped state (cwd, project, identity) from one authorization context
into another's call. Contract and mechanism:

- Gateway backends must authorize per request via the injected credential;
  the gateway does not assume backend sessions are user-neutral.
- Backends whose last-known negotiated revision is `2026-07-28` are
  **shared** — they are stateless by definition. Because this flag now drives
  an isolation decision, `protocol_version` is **part of the rebuild
  fingerprint**, not observability-only: a recorded 2026→legacy downgrade
  (same caps, same status) still triggers a rebuild.
- Shareability is re-validated against reality at **acquire time** (before
  any business RPC): a session opened under a "shareable"/modern assumption
  that actually negotiates legacy is closed immediately, reconnected with the
  legacy client, and the backend is treated as isolated + non-modern for the
  rest of the generation; cron/inspection persists the correction. The cache
  is a hint; the wire decides.
- All non-shareable backends use **per-authorization-context sub-pools** keyed
  by the *effective credential*, not the login identity:
  - If a token will be sent to the backend (directive-injected, or
    client-supplied via the `_meta` allowlist — the override case):
    `IsolationKey = sha256(backendID || effectiveToken)`. Different tokens or
    scopes of the same user, and client overrides, land in different pools.
  - If no credential is sent: `IsolationKey = sha256(backendID || userUUID)`.
  - Raw tokens never appear in pool names, logs, or metrics labels (truncated
    hash at most).
  - Sub-pools per backend are bounded (LRU evict + close). Empty userUUID and
    empty token (unreachable behind NeedLogin, guarded) → ephemeral unpooled
    session.
- `snapshot` derives the key inside `CallTool`/`ReadResource`/`GetPrompt`/
  `Complete` — all execution paths shard, not just tool calls.
- Legacy data-plane path is unchanged (per-session pools).

## Components

### `builder/mcp` (client, gateway, pool)

- `Client.NewSession` accepts a last-known protocol version hint;
  `BackendSession.ProtocolVersion()` exposes the actual negotiated version.
  The SDK negotiates backend revisions internally (`server/discover` first,
  legacy `initialize` fallback).
- `Client.Inspect` additionally returns the negotiated protocol version; the
  two callers (aigateway component, cron component) persist it.
- `Gateway` interface unchanged for indexing; `GatewayImpl`,
  `GatewaySessionPool` bridging semantics unchanged — a pooled "session" to a
  stateless backend is a reusable HTTP connection; to a legacy backend, a real
  stateful session.
- Pool lifecycle gains a retired state for hot swap (`GatewaySessionPool.Retire`):
  - `Retire()` marks the pool retired and closes idle sessions.
  - `Release()` on a retired pool closes the session instead of returning it
    to the idle list, so in-flight sessions at swap time are reclaimed on
    completion and never reused.
  - `Acquire()` on a retired pool is **allowed**: a request that loaded the
    old snapshot but has not yet acquired must still complete. Idle sessions
    were already closed at retire, so this opens a fresh session under the
    old generation's config; its later `Release()` closes it (retired). The
    guarantee is "no reuse after retire", not "no use" — retirement never
    breaks an in-flight request.
  - Each gateway generation owns its pool; a rebuild creates a new pool and
    retires the old one at publish time. Stale backend configs (URL/headers
    changed) can therefore never be served from a previous generation's pool.
- Pool sharding for cross-user isolation: pools key on
  `(backendID, isolationKey)` (see Protocol Adaptation Boundary → Backend
  session isolation). Bounded sub-pool count per backend with LRU evict-close.
- `Client.NewSession` warms the SDK tools cache for 2026-negotiated sessions
  (one `tools/list` after connect) so `x-mcp-header` / `Mcp-Param-*`
  generation works on pooled sessions. `NewSession` selects between the
  client's `modern` / `legacy` SDK instances by the backend's last-known
  revision hint, and **validates at acquire time**: if the `modern` client
  negotiated legacy, it closes the session immediately and reconnects with
  `legacy` before returning — no business RPC ever runs on a mismatched
  connection. The downgrade is remembered for the rest of the generation.
- `GatewayImpl` gains a stable-naming mode used only by the shared gateway
  (always-prefixed tools/prompts, deterministic collision resolution, sorted
  catalog output); legacy per-session gateways keep dynamic naming.
- `Gateway.CallTool` takes the `ToolCall` struct (Protocol Adaptation
  Boundary) and performs the legacy→2026 error-code normalization for
  upstream errors on the stateless path.
- New `SharedSnapshot` interface defined here (the port package), not in
  `aigateway/component`, exposing read+call only:

  ```go
  type SharedSnapshot interface {
      Generation() uint64
      IsComplete() bool
      ListTools(ctx context.Context) ([]*mcp.Tool, error)
      ListResources(ctx context.Context) ([]*mcp.Resource, error)
      ListResourceTemplates(ctx context.Context) ([]*mcp.ResourceTemplate, error)
      ListPrompts(ctx context.Context) ([]*mcp.Prompt, error)
      ReadResource(ctx context.Context, call ResourceRead) (*mcp.ReadResourceResult, error)
      GetPrompt(ctx context.Context, call PromptGet) (*mcp.GetPromptResult, error)
      Complete(ctx context.Context, call CompleteCall) (*mcp.CompleteResult, error)
      CallTool(ctx context.Context, call ToolCall) (*mcp.CallToolResult, error)
  }
  ```

  Reason: the mocked `MCPGatewayComponent.GetSharedSnapshot` must not return a
  type declared in the mocked package — the generated mock would import
  `aigateway/component`, while same-package tests
  (`aigateway/component/safety_policy_test.go`) import that mock, forming an
  import cycle (the AGENTS.md mock rule). The mock imports only
  `builder/mcp`; the component keeps the unexported implementation and the
  credential policy. Index-mutating `Gateway` methods
  (`RegisterBackend`, `IndexFromCaps`, …) are unreachable through this
  interface.

### `aigateway/component/mcp_gateway_ee.go`

- Component-owned `sharedSnapshotImpl` implements `mcp.SharedSnapshot`:
  `{generation uint64, gateway mcp.Gateway, backends map[string]GatewayBackend,
  shareable map[string]bool, failed []string}`. Every execution method
  (`CallTool`, `ReadResource`, `GetPrompt`, `Complete`) — not the handler —
  owns the per-request policy:
  - builds the downstream `Meta` per the `_meta` rules in Protocol Adaptation
    Boundary (fresh map, `metaInjection` directive check, `mcp_server`, client
    allowlist; never copies inbound `_meta`);
  - derives the `IsolationKey` per the isolation rules (effective-credential
    fingerprint for non-shareable backends; shareability from the capability
    row's `protocol_version` captured at build time, with acquire-time
    downgrade on revision mismatch);
  - constructs downstream `clientCapabilities` as
    `CallContext.ClientCapabilities ∩ gateway-bridgeable` (deep, including
    sub-capabilities) for 2026-bound calls;
  - passes `InputResponses`/`RequestState` through untouched.
  The handler passes ctx + RPC inputs and never interprets the directive or
  the (package-private) user gateway context.
- New `GetSharedSnapshot(ctx) (mcp.SharedSnapshot, error)` replaces
  per-session building for the stateless path. Snapshot publication protocol:
  - A short mutex elects at most one detached refresh worker. The worker uses
    `context.WithoutCancel` plus a 30-second timeout, reloads database inputs,
    recomputes the fingerprint, and builds outside the lock. Warm requests
    continue serving the published generation while it runs; only cold-start
    requests wait for completion.
  - Publish is one atomic store (`atomic.Pointer`) of
    `{generation, fingerprint, gateway, pool}`; `generation` increments per
    publish for tests and observability.
  - Every request loads the snapshot once and holds the matched
    gateway/pool/server triple for its whole lifetime.
- A Redis fingerprint hint avoids hashing full capability payloads on the hot
  path. The hint has a three-second TTL and is fail-open: Redis is an
  optimization, not a correctness or session dependency. A complete snapshot
  is returned directly until its next check, so Redis is not queried on every
  request. Configuration visibility is bounded by the TTL plus refresh time.
- **Shared builds are write-free** (no self-triggering rebuilds): the build
  reads the capability cache and inspects cache-miss backends **in memory
  only** — it never persists capability rows and never spawns the async
  cache-hit refresh. Cron and admin CRUD remain the only cache writers, so the
  act of building cannot move the fingerprint that triggered it.
- Rebuild fingerprint covers **both** tables and is **content-based**, so
  capability changes propagate but routine re-inspection does not churn
  generations: hash over backends `(id, config_hash, updated_at)` plus
  capability rows `(mcp_server_id, config_hash, status, protocol_version,
  sha256(capabilities JSON))` — `refreshed_at` is deliberately excluded (it
  changes on every cron run even when nothing changed). `protocol_version`
  is included because it drives the session-isolation policy, not just
  observability. Needs a new cheap store method
  `GatewayMCPServerCapabilityStore.ListRevisions(ctx)` returning those
  columns. What moves the fingerprint: admin CRUD, backend config change,
  capability content change, status transitions (connected ↔ error),
  recorded protocol-version transitions.
- Incomplete snapshots get **bounded retry**, independent of the fingerprint.
  A transient live-inspect failure can happen entirely between two cron runs,
  and cron's later success write can be content-identical — so DB state alone
  cannot guarantee recovery. Therefore: a snapshot records which backends
  failed during its build; `GetSharedSnapshot` triggers a rebuild when the
  current snapshot is incomplete and a backoff has elapsed (e.g. 30s → 1m →
  5m cap per consecutive failure, still coalesced to one refresh worker). A
  rebuild that clears all failures publishes a complete
  snapshot; one that doesn't extends the backoff. Complete snapshots are
  rebuilt only on fingerprint change, so content-stable systems never churn.
- `fetchAndStoreCapabilities` and cron `InspectAndPersist` persist the
  negotiated backend protocol version into the new
  `gateway_mcp_server_capabilities.protocol_version` column. It is a policy
  input (isolation shareability) and therefore part of the fingerprint above.
- New `CloseSharedGateway()` retires the current shared pool; called from the
  gateway handler's `Shutdown()`. Shutdown cancels any refresh and waits for
  its completion; the wait is bounded by the 30-second refresh timeout.

### `aigateway/handler/mcp_gateway_ee.go`

- New `getStatelessServer(req)`: loads `GetSharedSnapshot`, returns a cached
  `*sdkmcp.Server` for that snapshot's generation (built once per generation
  from the snapshot's list methods). Closures capture the snapshot and call
  `snapshot.CallTool(...)` / `snapshot.ReadResource(...)` etc., so the
  advertised catalog and the execution routing always come from the same
  generation. Closures pass `InputResponses`/`RequestState` through from
  `CallToolParamsRaw` verbatim, extract inbound `clientCapabilities` into the
  explicit struct field, and apply the resultType adaptation (empty
  `InputRequests` map when a result is `input_required` with nil requests).
  OTel spans and structured logging unchanged.
- The stateless server declares **no** `ListChanged` capabilities (unlike the
  legacy server) and sets `ServerOptions.SetCacheable` per the cache contract
  in Protocol Adaptation Boundary.
- Stateless transport options: `StreamableHTTPOptions{Stateless: true,
  PropagateRequestCancellation: true}` — the latter is required (verified in
  v1.8.0 `serveStateless`) for a client disconnect to cancel the in-flight
  backend call, implementing the spec rule that closing the SSE response
  stream is the cancellation signal.
- `Handler()` returns
  `mcpVersionDispatcher{stateless: ..., legacy: MCPProxyAwareHandler(...)}`;
  the dispatcher owns the supported-version constant and answers unknown
  versions with the endpoint-wide `UnsupportedProtocolVersionError`.
- Legacy closures replace `sdkmcp.ResourceNotFoundError` with an explicit
  `-32002` construction (pin today's code; v1.8 changed the default).
- `Shutdown()` additionally calls `comp.CloseSharedGateway()` so shared
  backend sessions are reclaimed on graceful stop.
- Redis registry, `MCPProxyAwareHandler`, advertise-addr resolution,
  `pruneDeadServers`: unchanged, legacy path only.

### Migration

Add nullable/text `protocol_version` column (default `''`) to
`gateway_mcp_server_capabilities`, and extend `gateway_mcp_servers_view` with
the new column **appended last** (`CREATE OR REPLACE VIEW` can only append).
No backfill; cron repopulates on next inspect. Rollback **drops and recreates**
the view without the column (replace-view cannot remove columns) before
dropping the table column. The SQL follows the repository's generated
migration structure and existing migration-validation conventions.

## Data Flow

- 2026 `tools/list` → dispatcher → stateless handler → snapshot's shared
  server → index only; no backend traffic.
- 2026 `tools/call` → `snapshot.CallTool` injects the user token per the
  backend's `metaInjection` directive → snapshot's gateway → snapshot's pool
  acquire → backend (any revision) → release (or close, if the pool was
  retired mid-call).
- Legacy `initialize` → no version header → proxy-aware handler captures
  session ID → Redis register → per-session `BuildUserGateway` (unchanged).
- Admin CRUD or cron inspect → capability rows written → Redis hint expires
  within three seconds → one detached, write-free refresh reloads inputs and
  atomically publishes a new generation; warm requests use the previous
  generation until publication and the old pool is then retired.

## Error Handling

- Shared gateway build failure: keep serving the last published snapshot; on
  cold start with none, return `emptyServer()` (today's degrade behavior).
  Never 500 a request because a backend is down.
- Rebuild concurrency: one elected refresh worker performs the database read,
  fingerprint check, and optional build. Incomplete snapshots may rebuild
  without a fingerprint change after backoff. Publish is a single atomic swap;
  warm and in-flight requests finish on their existing snapshot. Retire
  semantics: idle sessions closed at retire; `Acquire` after retire is allowed
  (fresh session, old config) so in-flight requests complete; `Release` after
  retire closes the session — nothing is ever repooled.
- No rebuild loops: builds are write-free and the fingerprint is
  content-based (excludes `refreshed_at`), so neither the build itself nor a
  no-change cron run can trigger a new generation. Backoff retries only occur
  while the current snapshot is incomplete, and each retry has a real cost
  bound (one live inspect per failed backend), so retries cannot spin.
- Backend inspect failure has two distinct recording paths. Legacy path
  (request time) and cron/admin paths persist `status=error`, last cached
  capabilities, and the error message to the DB — unchanged. The stateless
  shared build is write-free: it records failures only **in memory** on the
  published snapshot (the missing-backends set driving backoff retry) and
  never writes status or error rows to the DB.
- Client disconnect mid-call (stateless): `PropagateRequestCancellation`
  cancels the tool handler ctx, which propagates to the pooled backend call.
- Dispatch edges: absent version header or known legacy version → legacy
  path (today's behavior); present-but-unknown version → dispatcher-level
  `UnsupportedProtocolVersionError` listing the endpoint's full supported set.
- Token injection with empty ctx token (impossible behind NeedLogin, but
  guarded): forward without token; backend auth failure surfaces as a normal
  tool error. Client-supplied `CSGHubAccessToken` in `_meta` always wins over
  injection.

## Testing

Validation is organized around five contracts:

1. Version dispatch and protocol adaptation.
2. Deterministic catalogs and safe pool lifecycle.
3. End-to-end MRTR across tools, resources, and prompts.
4. Snapshot rebuild, publication, and recovery.
5. Legacy compatibility and cross-user isolation.

Protocol behavior requires end-to-end tests; shared-state concurrency tests
alone are insufficient. The detailed verification matrix follows.

<details>
<summary>Detailed verification matrix</summary>

| Scenario | Assertion |
|---|---|
| 2026 client → 2025 backend | downstream carries the negotiated version in header and `_meta`; inbound client version never leaks |
| same tool name on two backends / two instances / partial builds | the name always resolves to the same backend; an unavailable owner's names disappear instead of transferring |
| cold pool, concurrent new sessions | `x-mcp-header`-declared params produce complete `Mcp-Param-*` headers |
| MRTR round trip (tools + resources + prompts) | interactive MRTR works end-to-end on 2026 backends: downstream `clientCapabilities` = inbound ∩ bridgeable (per sub-capability); `inputResponses`/`requestState` pass through; retry reaches the same backend; requestState-only results keep `resultType=input_required` through the SDK boundary; legacy/unknown backends are never sent interactive capability declarations |
| modern-hint backend negotiates legacy | connection closed before any business RPC (fake backend asserts no `tools/call` on it), reconnect with `legacy` client, downgrade cached for the generation |
| unknown-version request | error lists the endpoint's full supported set |
| incomplete catalog, user-scoped resource | `ttlMs`/`cacheScope` match the cache contract |
| legacy error through modern entry | `-32002` → `-32602` translation; legacy path keeps `-32002` |
| users A/B and token variants share a legacy backend | sessions shard by effective credential (incl. `_meta` override); backend session state never crosses keys |
| backend downgrades 2026 → legacy | fingerprint moves on recorded `protocol_version`; a live session negotiating legacy on a "shareable" backend is closed at acquire time and reconnected with the legacy client, and the backend is treated as isolated for that generation |

Layer tests:

- `builder/mcp`: client negotiation against fake 2026-stateless and
  legacy-stateful backends; `Inspect` returns negotiated version; session
  warm-up populates the tools cache (x-mcp-header generation on cold and
  concurrently-added sessions, incl. encoded/nested values); pool retirement
  — sessions released after `Retire` are closed, never repooled; new
  generation never serves stale backend config; interleaving test — load
  snapshot → retire → acquire (allowed, old config) → release (closed);
  isolation-key sub-pools don't share sessions across keys; stable naming —
  always-prefixed names identical across build orders and partial
  availability, deterministic collision resolution; legacy→2026 error-code
  mapping.
- `aigateway/component`: fingerprint moves on capability content/status
  change with unchanged backend config, and does NOT move on
  `refreshed_at`-only updates; shared build performs no cache writes (store
  mock asserts zero `CreateOrUpdate` calls); serialized rebuild — concurrent
  triggers publish one generation, monotonic, no out-of-order overwrite;
  incomplete-snapshot recovery — cache says connected-but-expired, live
  inspect fails transiently between cron runs, snapshot lacks the tools,
  backoff retry rebuilds and restores them; content-identical cron write
  after such an episode does not by itself trigger a rebuild of a complete
  snapshot; `_meta` construction — inbound protocol keys stripped, directive
  on/off, client-supplied token wins, empty ctx token; isolation key
  derivation — effective-credential fingerprint incl. `_meta` override, no raw
  token in logs/labels, shareable 2026 backends exempt, in-memory downgrade on
  negotiated-revision mismatch; user A/B interleaving across CallTool AND
  ReadResource/GetPrompt — sessions and tokens never cross; capability
  persistence includes protocol version.
- `aigateway/handler`: dispatcher routing table (2026 header → stateless;
  absent / `2025-11-25` → legacy; unknown present version → endpoint-wide
  error; GET/DELETE → legacy); stateless end-to-end with a fake backend
  (list + call, token lands in `_meta`, MRTR fields round-trip); cache
  contract assertions (`SetCacheable` outcomes per snapshot state); stateless
  server declares no `ListChanged`; catalog/routing generation match (server
  built from snapshot N routes via snapshot N); client disconnect cancels the
  backend call; legacy-path regression incl. pinned `-32002`.
- Mock integrity: `make mock_gen` output compiles and `aigateway/component`
  same-package tests still build (no component-declared types in mocked
  signatures).
- Migration: review the SQL up/down ordering and cover the `ListRevisions`
  store path.
- Validation: the handler/component/builder-mcp changes are `ee || saas`
  tagged, but the DB store, migration, and `common/types` changes are
  untagged shared code — run `make test_all`, and `make lint_all` when
  practical, rather than only the ee/saas editions.

</details>

## Appendix A: Repository Evidence

Facts established by reading the codebase during design (worktree
`feat-stateless-mcp-gateway` @ origin/main `9a4f4ee8a`):

- SDK importers (full list): `builder/mcp/{client_ee,gateway_ee,session_pool_ee}.go`
  and `aigateway/handler/mcp_gateway_ee.go` (+ their tests). No other package
  imports `modelcontextprotocol/go-sdk`; `go.mod` pins v1.3.1.
- Legacy wiring to preserve: `NewMCPGatewayHandler`, `getServer`, `Shutdown`,
  `GatewayHandler` (`aigateway/handler/mcp_gateway_ee.go`);
  `mcp_proxy_handler.go`; `mcp_session_registry.go`; router at
  `aigateway/router/api_ee.go:23-55` returns `func() { gwHandler.Shutdown() }`
  for graceful shutdown.
- Shared-gateway inputs:
  `database.GatewayMCPServersStore.GetMCPServers` returns full rows including
  `ConfigHash` and `times` (CreatedAt/UpdatedAt) — sufficient for the
  rebuild fingerprint during the coalesced refresh; normal requests reuse the
  published generation and a short-lived Redis fingerprint hint.
- Capability persistence: `GatewayMCPServerCapability`
  (`builder/store/database/gateway_mcp_server_capability.go:13`) upserts on
  `(mcp_server_id, config_hash)`; both tables were created by migration
  `builder/store/database/migrations/20260314003305_create_mcp_gateway_tables.go`.
- `Client.Inspect` callers (both must persist the negotiated version):
  `aigateway/component/mcp_gateway_ee.go` (`fetchAndStoreCapabilities`,
  `InspectMCPServer`) and `component/gateway_mcp_server_ee.go` (cron
  `InspectAndPersist`).
- Mockery coverage: `aigateway/component` mocked in `.mockery.yaml`,
  `.mockery_ee.yaml`, `.mockery_saas.yaml`; `builder/mcp` mocked in
  `.mockery_saas.yaml` only. Interface changes require `make mock_gen`.
  Handler tests already import both mock packages
  (`aigateway/handler/mcp_gateway_ee_test.go`).
- Constants/types: `common/types/mcp_gateway.go` (`CSGHubAccessToken`,
  `GatewayCapabilityCacheTTL` = 1h, `MCPServerStatus*`);
  `common/utils/trace.HeaderMcpSessionID` = `Mcp-Session-Id`.
- Migration generation command:
  `go run cmd/csghub-server/main.go migration create_go <name>`.

## Appendix B: References

- Spec: https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http
- go-sdk releases: https://github.com/modelcontextprotocol/go-sdk/releases (v1.7.0, v1.8.0)
- ToolHive dual-protocol epic: https://github.com/stacklok/toolhive/issues/5743
- Kuadrant MCP Gateway 0.8 (dual routers): https://kuadrant.io/blog/mcp-gateway-0.8/

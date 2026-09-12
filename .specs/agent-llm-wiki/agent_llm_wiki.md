# Agent LLM-Wiki Specification

## Status

This document defines the intended CSGHub integration with llmservice (LLM-Wiki) for EE and SaaS builds. It covers:

1. an authenticated API proxy for per-KB management traffic;
2. an authenticated AIGateway proxy for per-KB MCP traffic; and
3. namespace-owned Agent knowledge bases with personal and organization RBAC.

The document separates committed product behavior from unresolved integration contracts. An unresolved release gate must not be replaced with an implicit fallback.

## Goals

- Expose llmservice management APIs through the CSGHub API without exposing llmservice directly to clients.
- Expose each LLM-Wiki knowledge base as an MCP endpoint through AIGateway.
- Distinguish Langflow/Csgbot and LLM-Wiki knowledge bases explicitly.
- Allow a knowledge base to belong to a personal or organization namespace.
- Allow organization members to query organization knowledge bases and organization administrators to manage them.
- Preserve proxy request bodies and streaming responses.

## Non-goals

- Aggregating all organization knowledge bases visible to a user into one feed.
- Implementing the llmservice typed CRUD client or backend adapter in this phase.
- Changing accounting, billing, portal UI, or deployment templates.
- Adding MCP tool aggregation; this design is a path-based reverse proxy.
- Silently treating an LLM-Wiki knowledge base as a Csgbot knowledge base when llmservice routing is unavailable.

## Terminology

| Term | Meaning |
| --- | --- |
| llmservice | The service that implements LLM-Wiki management and per-knowledge-base MCP endpoints. |
| Langflow KB | An Agent knowledge base backed by the existing Csgbot integration. |
| LLM-Wiki KB | An Agent knowledge base backed by llmservice. |
| Control plane | Authenticated knowledge-base management traffic through the API service. |
| Data plane | MCP traffic sent through AIGateway to a knowledge-base MCP endpoint. |
| Namespace owner | The personal or organization namespace identified by `ns_uuid`. |
| Actor | The authenticated user performing an operation, identified by username, user UUID, and personal namespace UUID. |

## Architecture

```text
Client
  |
  +-- /api/v1/llmwikis/{kb_id}/*
  |       |
  |       +-- API service -- /internal/v1/knowledge-bases/{kb_id}/* --> llmservice
  |              management control plane
  |
  +-- /v1/llmwikis/{kb_id}/mcp
          |
          +-- AIGateway -- stored metadata.mcp_endpoint_url --> per-KB MCP service
                 MCP data plane

Agent knowledge base
  |
  +-- type=langflow --> Csgbot
  +-- type=llmwiki  --> llmservice

Ownership
  |
  +-- ns_uuid --> personal namespace
  +-- ns_uuid --> organization namespace
```

The API service owns management traffic while AIGateway owns the exact LLM-Wiki MCP endpoint. This keeps MCP traffic on the data-plane ingress and its independent online-proxy rate-limit and streaming policy. Both entry points use the same local authorization component, but only management traffic uses the shared llmservice endpoint configuration; MCP traffic uses the private endpoint stored on its KB record.

## Configuration

The shared configuration contains an `LLMWiki` section:

```go
LLMWiki struct {
    Host string `env:"OPENCSG_LLMWIKI_SERVER_HOST" default:"http://127.0.0.1"`
    Port int    `env:"OPENCSG_LLMWIKI_SERVER_PORT" default:"8100"`
}
```

The deployment configures the llmservice management endpoint through these environment variables. No static management API path list is configured because the API service forwards the authorized per-KB subresource path. Per-KB MCP endpoints are supplied during KB registration rather than through this shared configuration.

The public management entry point returned as `metadata.base_url` uses the same API origin as CSGClaw's `CSGHUB_API_BASE_URL`: `Model.DownloadEndpoint` (`STARHUB_SERVER_MODEL_DOWNLOAD_ENDPOINT`). It is independent from `APIServer.PublicDomain`, which may point to the portal, and from `AIGateway.PublicAIGatewayURL`, which is used only for the public MCP URL.

## Management API Proxy

### Public contract

The API service exposes only per-KB subresource routes anchored to an existing knowledge base:

```text
ANY /api/v1/llmwikis/{kb_id}/*path
```

The root route `/api/v1/llmwikis/{kb_id}` and collection routes without `{kb_id}` are not exposed. The wildcard path must contain a non-empty subresource, so a trailing-slash root request also cannot reach llmservice.

The route is available only in EE and SaaS builds and uses these middleware checks in order:

1. `License.Check`;
2. `Auth.NeedLogin`.

The management proxy does not require a separate access-token gate: `Auth.NeedLogin`
authenticates the request, and the actor's personal namespace UUID is resolved from
the authenticated user UUID when the authentication context does not already provide it.

The proxy rewrites:

```text
/api/v1/llmwikis/{kb_id}/<path>
    -->
/internal/v1/knowledge-bases/{kb_id}/<path>
```

For example:

```text
/api/v1/llmwikis/kb-1/sources/source-1
    -->
/internal/v1/knowledge-bases/kb-1/sources/source-1
```

`kb_id` is `agent_knowledge_bases.content_id`. The handler validates the ID and rejects empty paths, dot segments, repeated separators, backslashes, and control characters before resolving authorization or constructing the upstream path.

### Authorization

Before contacting llmservice, the handler loads the canonical `agent_knowledge_bases` row by `content_id` and requires `type=llmwiki`.

- `GET` and `HEAD` require read permission: personal namespace ownership or an organization role for which `CanRead` is true.
- All other methods require write permission: personal namespace ownership or an organization role for which `CanWrite` is true. Exceptions: the read-only evidence query subresources `/query` (LLMWiki internal #34 queryKnowledgeBase) and `/preview-queries` (LLMWiki internal #13 previewQuery) require read permission even though they use `POST`.
- `CONNECT` and `TRACE` always return `405`.

A missing or wrong-type target returns `404`; insufficient permission returns `403`. LLM-Wiki KBs are visible only to their personal owner or organization members, so the stored `public` value never bypasses this authorization check.

### Forwarding behavior

The proxy preserves:

- the HTTP method;
- query parameters;
- the request body;
- the upstream status code and headers; and
- buffered and streaming response bodies.

The API authenticates the client with the CSGHub user access token, then removes the inbound `Authorization` and `Cookie` headers before contacting llmservice.

The proxy injects trusted platform headers:

| Header | Value |
| --- | --- |
| `X-Request-ID` | The current CSGHub trace request ID. |
| `X-CSGHub-Actor-ID` | The authenticated user's UUID. |
| `X-CSGHub-Actor-Name` | The authenticated user's login name. Sent on non-GET methods (`POST`/`PUT`/`PATCH`/`DELETE`, including the read-only `/query` and `/preview-queries` POSTs) so LLM-Wiki can record the operator in its operation logs. |

Caller-supplied values for trusted platform headers must not override the platform-derived values. `X-CSGHub-Actor-Name` is always removed first; management requests set it again only for non-GET/HEAD methods.

The proxy removes caller-supplied `X-CSGHub-Decision-ID` and `X-CSGHub-Policy-Version` values because this route does not yet have an authoritative decision record to inject.

Response bodies are forwarded unchanged, including llmservice fields such as `mcp_endpoint_url`. When llmservice returns a `Location` header for an asynchronous resource, the proxy accepts only a location under the same upstream origin and authorized KB, then rewrites it back to `/api/v1/llmwikis/{kb_id}/*path`. An external or cross-KB location is treated as an invalid upstream response and returns `502`.

### Failure behavior

| Condition | Response |
| --- | --- |
| Missing or invalid authentication | `401` |
| License check rejected | Existing license middleware response |
| Invalid KB ID or subresource path | `400` |
| Missing or wrong-type KB | `404` |
| Insufficient KB permission | `403` |
| `CONNECT` or `TRACE` | `405` |
| llmservice unavailable before response headers | `502` |
| Client cancels the request | Cancel upstream work and preserve the platform's client-cancel behavior |

## AIGateway MCP Proxy

### Public contract

AIGateway exposes one exact Streamable HTTP MCP endpoint per LLM-Wiki KB:

```text
ANY /v1/llmwikis/{kb_id}/mcp
```

No wildcard subpath is exposed. MCP `GET`, `POST`, and `DELETE` requests use the same endpoint; query parameters and MCP session headers are forwarded unchanged. `CONNECT` and `TRACE` return `405`.

The route is available only in EE and SaaS builds and applies `License.Check`, `Auth.NeedLogin`, and `Auth.NeedAccessToken` before the handler.

Each CSGHub KB record stores its private per-KB MCP endpoint in `metadata.mcp_endpoint_url` during registration. AIGateway resolves that value only after read authorization and proxies directly to its origin and `/mcp` path:

```text
/v1/llmwikis/{kb_id}/mcp
    -->
http://{per-kb-mcp-service}:42002/mcp
```

The LLM-Wiki management backend is not contacted to resolve the MCP target for each request. Existing records without a valid stored endpoint return `503` until an administrator supplies one through the KB update API.

### Authorization and forwarding

`kb_id` is `agent_knowledge_bases.content_id`. AIGateway calls the shared LLM-Wiki access component with read mode, so a personal owner or organization member with `CanRead` may use every MCP transport method. The stored `public` value does not bypass access control.

The proxy preserves the method, query, body, MCP session headers, upstream status, and streaming response. It removes inbound `Authorization`, `Cookie`, and `X-CSGHub-Actor-Name`, replaces `X-CSGHub-Actor-ID` with the authenticated user UUID, propagates `X-Request-ID`, and removes caller-supplied decision and policy headers when no authoritative values exist.

## Agent Knowledge-Base Domain Model

### Knowledge-base type

The common layer defines a typed enum:

```go
type AgentKnowledgeBaseType string

const (
    AgentKnowledgeBaseTypeLangflow AgentKnowledgeBaseType = "langflow"
    AgentKnowledgeBaseTypeLLMWiki  AgentKnowledgeBaseType = "llmwiki"
)
```

The API validates input against these two values. Existing records and create requests that omit `type` default to `langflow` for compatibility. An unsupported or unregistered type fails explicitly and must never fall through to Csgbot.

### Namespace ownership

`agent_knowledge_bases.user_uuid` remains the immutable UUID of the user who created the KB. Ownership is stored separately in `ns_uuid` and `namespace_type`.

`ns_uuid` identifies either:

- the user's personal namespace UUID; or
- an organization namespace UUID.

`namespace_type` uses the repository namespace values `user` and `organization`. It is resolved from the trusted namespace record during creation and cannot be supplied independently by the client. Existing rows are personal KBs, so migration backfills `ns_uuid = user_uuid` and `namespace_type = 'user'`. The platform has a rare collision fallback where identity and namespace UUID can diverge; new authorization code must use the actor's resolved namespace UUID instead of assuming equality solely from the user UUID.

### Storage model

The database model contains:

| Field | Contract |
| --- | --- |
| `id` | Internal primary key. |
| `user_uuid` | Immutable UUID of the authenticated user who created the KB. |
| `ns_uuid` | Personal or organization namespace owner. |
| `namespace_type` | Trusted namespace type: `user` or `organization`. |
| `type` | `langflow` or `llmwiki`. |
| `name` | Display name, unique within the namespace. |
| `description` | Optional description. |
| `content_id` | Unique identifier returned by the backing service. |
| `public` | Whether non-members may read the KB. |
| `metadata` | Backend-specific JSON metadata. |
| `is_pinned`, `pinned_at` | Per-actor derived list fields. |

Name uniqueness is enforced by `(ns_uuid, name)`. `content_id` remains globally unique unless the backing-service contract later proves that IDs are only unique within a type.

Lifecycle mutations complete all request validation before changing OpenFGA relationships. Because OpenFGA, the database, and backend services do not share a transaction, ambiguous relationship write/delete errors and later backend/database failures trigger best-effort inverse relationship compensation while preserving the primary operation error.

### API representation

Create input contains:

```json
{
  "name": "Engineering Wiki",
  "description": "Internal engineering knowledge",
  "public": false,
  "type": "langflow",
  "namespace": "my-organization"
}
```

`namespace` is optional. An empty value selects the actor's personal namespace. A non-empty value resolves an organization namespace by path.

`type` is optional for backward compatibility and defaults to `langflow`. It is immutable after creation. The list API accepts an optional `type` query filter.

Registering an existing LLM-Wiki KB additionally requires its `kb_id` as `content_id` and its private per-KB MCP service URL in `metadata.mcp_endpoint_url`. CSGHub stores that private URL for authorized AIGateway routing.

LLM-Wiki create, list, and detail responses expose two client entry points:

- `metadata.base_url` is the absolute management-proxy base URL: `{Model.DownloadEndpoint}/api/v1/llmwikis/{content_id}`;
- `metadata.mcp_endpoint_url` is the public AIGateway route: `{AIGateway.PublicAIGatewayURL}/llmwikis/{content_id}/mcp`.

The stored private MCP endpoint is never returned to API clients. If live `resource_state` contains `mcp_endpoint_url`, that duplicate field is omitted from the public projection; clients use the stable top-level `metadata.mcp_endpoint_url` entry point.

Metadata updates are partial. `base_url` and `resource_state` are response-only. An administrator may replace the MCP target with another valid private endpoint; returning the exact public `metadata.mcp_endpoint_url` for the same KB is normalized to a no-op so the stored private target is not overwritten. Other public or cross-KB URLs remain invalid.

List and detail responses include `type` and namespace-aware ownership. The response contract must expose a namespace UUID, namespace type, and namespace path or display owner so an organization-owned KB does not appear to be owned by a user. `user_uuid` retains user-only semantics and identifies the creator; it must never be populated with an organization UUID.

List and detail responses also expose the caller's permissions on the KB: `editable`/`can_manage` report whether the caller may update or delete the record, and `can_write` reports whether the caller may write KB content through the management proxy. Reading is implied by visibility, so no separate `can_read` field is returned.

Creator data is resolved through `user_uuid`. Owner and avatar data for an organization KB are resolved from the namespace. A missing `ns_uuid = users.uuid` relation must not produce an empty organization owner.

## Actor Identity

Authorization uses a common actor value:

```go
type AgentKnowledgeBaseActor struct {
    Username      string
    UserUUID      string
    NamespaceUUID string
}
```

`Username` is used for organization membership lookup, `UserUUID` is the stable user identity, and `NamespaceUUID` is the actor's resolved personal namespace. `GetMemberRoleByUUID` accepts an organization UUID and a username; passing a user UUID in the username argument is invalid.

The handler derives the entire value from trusted authentication context. Client request fields cannot select or replace actor identity, and the actor value does not carry the access token.

## Authorization

### RBAC matrix

| Operation | Personal KB | Organization KB |
| --- | --- | --- |
| Create | Logged-in actor, own personal namespace | Organization admin (`CanAdmin`) |
| Read by ID/content ID | Owner or public | Organization member (`CanRead`) or public |
| Update by ID/content ID | Owner | Organization admin (`CanAdmin`) |
| Delete by ID/content ID | Owner | Organization admin (`CanAdmin`) |
| List | Actor's personal KBs plus public KBs | Explicit org scope and organization member |

Public visibility grants read access only. It never grants update or delete access.

### Authorization resolution

For an existing knowledge base:

1. If `kb.ns_uuid == actor.personal_ns_uuid`, treat the actor as the personal owner.
2. Otherwise, resolve the namespace through `GetNameSpaceInfoByUUID`.
3. Require the namespace to be an organization namespace.
4. Resolve membership through `GetMemberRoleByUUID(ctx, ns.UUID, actor.Username)`.
5. Apply `CanRead` or `CanWrite` according to the operation.
6. If resolution fails to establish the required permission, return Forbidden.

For organization creation or org-scoped listing:

1. resolve the request's namespace path with `GetNameSpaceInfo`;
2. require an organization namespace;
3. obtain the actor's membership role by organization UUID and username; and
4. require `CanAdmin` for create or `CanRead` for list.

### Editable projection

`editable` (and its alias `can_manage`) is authorization-derived:

- personal KB: `true` only for the personal owner;
- organization KB: `true` only for an organization admin; and
- public KB owned by another namespace: `false` unless the actor separately has management rights.

`can_write` reports whether the caller may write KB content through the management proxy:

- personal KB: `true` for the personal owner;
- organization KB: `true` for an organization writer or admin; and
- public KB owned by another namespace: `false` unless the actor separately has write rights.

`can_manage` is always identical to `editable`. Reading a KB is implied by visibility, so the
response does not carry a separate `can_read` field.

The `editable` list filter follows the same semantics. An org-scoped list cannot use `ns_uuid == actor UUID` as a substitute for admin permission.

## List Semantics

### Default list

The default authenticated list returns:

```text
kb.ns_uuid = actor.personal_ns_uuid OR kb.public = true
```

Search, public, editable, pagination, and type filters apply to that visible set.

### Ordering

All knowledge-base list scopes use one global ordering before pagination:

1. pinned knowledge bases before unpinned knowledge bases;
2. within each pinned state, `llmwiki` before `langflow`; and
3. within each type, `updated_at` descending.

The type priority is explicit rather than lexical. Unknown future types sort after `langflow`. Pin time does not affect ordering within the pinned group.

### Organization-scoped list

An organization-scoped list resolves one organization path, validates membership, and returns:

```text
kb.ns_uuid = organization.ns_uuid
```

The API may represent this scope as `?namespace=<org>` on the existing list endpoint. A second organization-specific route is unnecessary unless the portal contract requires it.

The first release does not implement an "all knowledge bases from all my organizations" feed. Membership is stored in the User service, so that feed requires a User-service API that enumerates the actor's organizations.

## Backend Routing and Lifecycle

Knowledge bases follow the same adapter/factory pattern as Agent Instances. The component owns common authorization, validation, and persistence; adapters own type-specific backend lifecycle operations.

```go
type AgentKnowledgeBaseAdapter interface {
    Type() types.AgentKnowledgeBaseType
    Create(ctx context.Context, actor types.AgentKnowledgeBaseActor, req types.CreateAgentKnowledgeBaseReq) (*types.AgentKnowledgeBaseCreationResult, error)
    Update(ctx context.Context, actor types.AgentKnowledgeBaseActor, target types.AgentKnowledgeBaseBackendTarget, req types.UpdateAgentKnowledgeBaseRequest) error
    Delete(ctx context.Context, actor types.AgentKnowledgeBaseActor, target types.AgentKnowledgeBaseBackendTarget) error
}
```

An `AgentKnowledgeBaseAdapterFactory` registers one adapter per type:

| Type | Backend |
| --- | --- |
| `langflow` | `CsgbotKnowledgeBaseAdapter` using `CsgbotSvcClient` |
| `llmwiki` | `LLMWikiKnowledgeBaseAdapter` using `LLMWikiSvcClient` |

Create resolves the adapter from the validated request type, creates the backend resource, and persists the returned `content_id`. If local persistence fails after backend creation, the component asks the same adapter to delete the backend resource as compensation. Update and delete load the stored row first and resolve the adapter from its immutable persisted type.

The builder RPC layer defines `LLMWikiSvcClient` with create, update, and delete operations plus builder-owned request and response types. The exact llmservice HTTP paths, payloads, service authentication, retry policy, and error decoding remain deferred until the wire contract is confirmed. Until a working LLM-Wiki adapter is registered, LLM-Wiki mutations return an explicit unavailable-type error; read and list may still return existing local LLM-Wiki records.

The existing Csgbot client is user-scoped. Organization-owned Langflow creation and subsequent update/delete therefore require a confirmed external ownership and credential contract. The choices include an organization identity understood by Csgbot or the creating administrator as a recorded backend actor. This decision is a release gate; partial implementation must not authorize an operation and then call Csgbot with an unrelated acting user's UUID.

## Migration

The migration must be generated with the repository migration generator; it must not be created manually.

Up migration:

1. retain `agent_knowledge_bases.user_uuid` as creator attribution;
2. drop the existing `(user_uuid, name)` unique constraint;
3. add `ns_uuid`, backfill it from `user_uuid`, and make it non-null;
4. add `namespace_type VARCHAR NOT NULL DEFAULT 'user'`;
5. add `type VARCHAR NOT NULL DEFAULT 'langflow'`;
6. add the `(ns_uuid, name)` unique constraint; and
7. add only indexes justified by query plans.

The namespace-scoped unique constraint provides an index whose leading column is `ns_uuid`, so a second standalone equality index may be redundant.

Down migration restores the `(user_uuid, name)` constraint and removes `type`, `namespace_type`, and `ns_uuid` in dependency-safe order.

Migration verification must confirm that all pre-existing rows retain `user_uuid`, read as `ns_uuid=user_uuid`, `namespace_type=user`, and `type=langflow`.

## API Errors

| Condition | HTTP status |
| --- | --- |
| Invalid request, namespace, KB type, or MCP KB ID | `400` |
| Missing authentication | `401` |
| Non-member read or non-admin management operation | `403` |
| Knowledge base or namespace not found | `404` |
| Upstream proxy connection failure | `502` |
| Unexpected storage or service failure | `500` |

Create and org-scoped List handlers must map `ErrForbidden` to `403`; the current generic server-error path is not sufficient. All ID-based and content-ID-based variants follow the same error contract.

Errors returned to clients must not expose llmservice network addresses, credentials, access tokens, or internal authorization headers.

## Observability

Proxy logs include the request ID, public route, upstream service name, response status, and elapsed time. They do not log authorization headers or request bodies by default.

Authorization failure logs include the operation, actor user UUID, target namespace UUID, and KB ID/content ID where available. They do not treat expected Forbidden decisions as internal server errors.

Metrics should distinguish API management traffic, AIGateway LLM-Wiki MCP traffic, upstream failures, and authorization rejections.

## Testing

### Migration and store

- Up migration preserves existing ownership values and assigns `type=langflow`.
- Down migration restores the original schema.
- Name uniqueness remains namespace-scoped.
- Default List returns personal plus public KBs.
- Organization List returns only the selected namespace's KBs.
- Type, public, search, editable, and pagination filters compose with visibility.

### Component

- Create defaults an omitted type to Langflow and dispatches explicit types through the matching adapter.
- Update and delete dispatch from the immutable persisted type.
- An unsupported or unregistered type never calls the Langflow/Csgbot adapter.
- A failed local insert after backend creation triggers compensating deletion through the same adapter.
- Personal owner create/read/update/delete succeeds.
- A different user can read a public personal KB but cannot modify it.
- Organization admin create/update/delete succeeds once the backend ownership contract is implemented.
- Organization non-admin create/update/delete returns Forbidden.
- Organization member reads private org KBs.
- Organization non-member cannot read private org KBs.
- Public org KBs remain readable to non-members but not editable.
- Username, rather than user UUID, is passed to membership lookup.
- ID and content-ID operations enforce identical authorization.
- `editable` and `editable` filtering follow personal-owner/org-admin semantics.
- `type=llmwiki` returns an unavailable-type error until llmservice routing exists and never calls Csgbot.

### Backend adapters

- The adapter factory registers exactly one adapter for each supported type.
- Langflow lifecycle operations use `CsgbotSvcClient`.
- LLM-Wiki lifecycle operations use `LLMWikiSvcClient` once its wire contract is implemented.
- Adapter and RPC failures prevent the corresponding local mutation.

### Handler and router

- Type and namespace inputs are parsed and validated; List accepts the optional type filter.
- Actor identity comes from authentication context.
- Create and org-scoped List map Forbidden to `403`.
- Existing Get/Update/Delete Forbidden mappings remain intact.
- EE and SaaS routes compile and carry the required middleware.

### Management API proxy

- Methods, query parameters, bodies, statuses, and headers are forwarded.
- Only target routes containing `kb_id` are exposed and rewritten exactly once to `/internal/v1/knowledge-bases/{kb_id}/*`.
- Read and management methods enforce the corresponding KB permission before proxying.
- Platform actor and request-ID headers replace caller values.
- `Location` is rewritten only when it remains within the authorized KB; cross-KB and external targets produce `502`.
- Upstream response bodies, including `mcp_endpoint_url`, remain unchanged.
- Streaming responses are not buffered.
- Connection failures produce `502`.

### AIGateway MCP proxy

- Only the exact `/v1/llmwikis/{kb_id}/mcp` route is registered; no wildcard route is exposed.
- Every MCP transport method uses shared read authorization.
- The authorized KB's stored private endpoint selects the upstream; the public `kb_id` is not appended to that endpoint.
- The `/mcp` path, query, body, MCP session headers, status, and streaming response pass through.
- Platform credentials and caller-controlled trust headers are not forwarded.
- Invalid, missing, wrong-type, and unauthorized targets return the documented `400`, `404`, and `403` responses; a missing or corrupt stored MCP endpoint returns `503`.

### Verification commands

At minimum, verification includes focused tests plus EE and SaaS compilation. Commands must use the repository's supported `GO_TAGS` form, for example:

```bash
make test GO_TAGS=ee
make test GO_TAGS=saas
make build GO_TAGS=ee
make build GO_TAGS=saas
```

Focused component, handler, store, and proxy tests may be run during development, but passing only focused tests is not sufficient release evidence.

## Rollout

1. Confirm the llmservice endpoint and per-KB internal paths.
2. Land shared configuration and the authenticated management API proxy.
3. Register each private per-KB MCP endpoint and land the exact AIGateway data-plane proxy using shared read authorization.
4. Land the ownership/type migration and personal behavior compatibility.
5. Resolve Csgbot organization identity before enabling organization-owned Langflow management.
6. Implement the llmservice typed adapter before enabling `type=llmwiki` creation.

Each capability should be independently disableable during rollout when its upstream contract or security boundary is not ready.

## Release Gates and Deferred Work

| Item | Status | Required resolution |
| --- | --- | --- |
| llmservice endpoint | Defined | Configure host and port through `OPENCSG_LLMWIKI_SERVER_HOST` and `OPENCSG_LLMWIKI_SERVER_PORT`. |
| Control-plane upstream paths | Defined | Proxy only `/internal/v1/knowledge-bases/{kb_id}/*path`. |
| Per-KB MCP path | Defined | Store `metadata.mcp_endpoint_url` during registration and proxy exact `/v1/llmwikis/{kb_id}/mcp` directly to that service's `/mcp` endpoint. |
| MCP authorization | Defined | Apply shared read permission regardless of the MCP transport method. |
| LLM-Wiki typed CRUD adapter | Deferred | Implement before accepting `type=llmwiki` creation. |
| Csgbot organization ownership | Blocking | Define backend owner and credentials for create/update/delete. |
| Organization owner API representation | Defined | Keep `user_uuid` as creator and expose namespace ownership separately. |
| Cross-organization list | Deferred | Add a User-service organization enumeration API if required. |

## Acceptance Criteria

The feature is complete only when:

- proxy paths and configuration match confirmed llmservice contracts;
- control-plane requests are authenticated and carry trusted actor identity;
- AIGateway MCP traffic authenticates the user, enforces shared read access, strips platform credentials, preserves stream semantics, and rejects unsafe targets;
- existing knowledge bases migrate without ownership loss and default to Langflow;
- personal KB behavior remains compatible;
- organization RBAC is enforced consistently for ID and content-ID operations;
- owner and editable response fields are correct for personal and organization namespaces;
- Agent KB create/update/delete dispatch through the adapter selected by KB type;
- unsupported LLM-Wiki creation cannot fall through to Csgbot;
- create and list authorization failures return `403`; and
- relevant focused tests and EE/SaaS builds pass.

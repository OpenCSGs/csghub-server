# Agent LLM-Wiki API

## Overview

CSGHub represents an LLM-Wiki knowledge base as an Agent knowledge-base record whose `type` is `llmwiki` and whose `content_id` is the LLM-Wiki `kb_id`.

The integration has three API surfaces:

| Surface | Purpose |
| --- | --- |
| `/api/v1/agent/knowledge-bases` | Register, list, view, update, and remove CSGHub KB records. |
| `/api/v1/llmwikis/{content_id}/*path` | Access management APIs for one authorized LLM-Wiki KB. |
| `/v1/llmwikis/{content_id}/mcp` | Query one authorized LLM-Wiki KB through AIGateway MCP. |

All routes require an authenticated CSGHub user. LLM-Wiki KB access is limited to the personal owner or an authorized member of the owning organization.

## Main Flow

```text
Register existing LLM-Wiki KB
  -> POST /api/v1/agent/knowledge-bases
  -> CSGHub stores ownership, visibility, type, content_id, and the private MCP endpoint

List or get KB
  -> CSGHub loads the local KB record
  -> CSGHub fetches the current resource state from LLM-Wiki
  -> response metadata contains management and MCP URLs

Manage KB content
  -> client calls metadata.base_url + a management subresource
  -> CSGHub checks access and proxies the request to LLM-Wiki

Query KB
  -> client calls metadata.mcp_endpoint_url
  -> AIGateway checks access and proxies directly to the stored per-KB MCP service
```

## Register an LLM-Wiki KB

The LLM-Wiki KB already exists when it is registered in CSGHub. Creating the Agent KB record does not create another resource in LLM-Wiki.

```http
POST /api/v1/agent/knowledge-bases
Authorization: Bearer <user_access_token>
Content-Type: application/json
```

```json
{
  "name": "Engineering Wiki",
  "description": "Internal engineering knowledge",
  "content_id": "kb-east",
  "type": "llmwiki",
  "namespace": "engineering-team",
  "public": false,
  "metadata": {
    "mcp_endpoint_url": "http://kb-east-llm-wiki-mcp.llm-wiki.svc.cluster.local:42002/mcp"
  }
}
```

`content_id` must be the target LLM-Wiki `kb_id`.

`metadata.mcp_endpoint_url` is required for an LLM-Wiki KB because every KB has its own MCP service. CSGHub stores this private address but replaces it with the public AIGateway route in API responses. The value must be an absolute `http` URL whose path is exactly `/mcp`, without credentials, query parameters, or a fragment.

The hostname is not allowlisted. Supplying or updating this value is therefore a trusted internal-routing capability granted to the namespace writer or KB administrator, and deployments must account for that SSRF boundary.

`namespace` selects the personal or organization namespace that owns the CSGHub KB record:

- omitted or empty: use the authenticated user's personal namespace;
- personal namespace name: it must resolve to the authenticated user's namespace; and
- organization namespace name: the authenticated user must have write permission on the organization namespace.

Organization namespaces support only `llmwiki` knowledge bases. Personal namespaces support both `llmwiki` and `langflow` knowledge bases.

CSGHub resolves the namespace name to its trusted UUID and type before creating the record. The resolved namespace is stored as the owner, while the authenticated user's UUID is stored as the user who performed the creation.

## List and Get KBs

```http
GET /api/v1/agent/knowledge-bases
GET /api/v1/agent/knowledge-bases/{id}
Authorization: Bearer <user_access_token>
```

The list API supports the `search`, `type` (`llmwiki` or `langflow`), `public`, `editable`, `per`, and `page` query parameters. The `search` term matches the KB `name` and `description` (case-insensitive substring). Results are ordered by pinned state, then `llmwiki` before `langflow`, then `updated_at` descending.

For an LLM-Wiki item, `metadata` includes two client entry points:

```json
{
  "id": 42,
  "name": "Engineering Wiki",
  "content_id": "kb-east",
  "type": "llmwiki",
  "editable": true,
  "can_write": true,
  "can_manage": true,
  "metadata": {
    "base_url": "https://api.example.com/api/v1/llmwikis/kb-east",
    "mcp_endpoint_url": "https://aigateway.example.com/v1/llmwikis/kb-east/mcp",
    "resource_state": {
      "readiness": "ready",
      "mcp_status": "ready"
    }
  }
}
```

The example abbreviates `resource_state`; the API may return additional current resource-state fields.

- `editable`/`can_manage` report whether the caller may update or delete the KB record (admin permission).
- `can_write` reports whether the caller may write KB content through the management proxy (write permission). A writer can use the management proxy but cannot update/delete the record.
- Reading a KB is implied by visibility, so there is no separate `can_read` field.
- `metadata.base_url` is the absolute base URL for LLM-Wiki management subresources. Its public API origin comes from `Model.DownloadEndpoint` (`STARHUB_SERVER_MODEL_DOWNLOAD_ENDPOINT`), the same configuration used for CSGClaw's `CSGHUB_API_BASE_URL`; it does not use `APIServer.PublicDomain`. The base URL itself is not an API operation.
- `metadata.mcp_endpoint_url` is the public AIGateway MCP endpoint backed by the private URL stored during registration.
- `metadata.resource_state` omits its upstream `mcp_endpoint_url` field to avoid duplicating the stable top-level client entry point.
- Resource-state enrichment has a ten-second timeout. If LLM-Wiki is unavailable or times out, CSGHub still returns the local KB record without `metadata.resource_state`.
- The list flow uses one batch resource-state request for the LLM-Wiki KBs on the current page.

## Management Proxy

Clients append a LLM-Wiki management subresource to `metadata.base_url`:

```http
GET /api/v1/llmwikis/kb-east/sources
POST /api/v1/llmwikis/kb-east/sources
GET /api/v1/llmwikis/kb-east/pages/{page_id}
GET /api/v1/llmwikis/kb-east/jobs/{job_id}
Authorization: Bearer <user_access_token>
```

The API service:

1. resolves `content_id` to the canonical CSGHub KB record;
2. requires the record type to be `llmwiki`;
3. checks read permission for `GET`/`HEAD`, or write permission for management mutations;
4. removes the user's authorization credentials from the upstream request;
5. adds trusted request and actor headers; and
6. proxies the method, query parameters, body, status, headers, and response stream to the same KB in LLM-Wiki.

CSGHub always removes a caller-supplied `X-CSGHub-Actor-Name`. Non-GET requests then carry the authenticated caller's login name so LLM-Wiki can record the operator in its operation logs; GET/HEAD reads do not include this header. The MCP proxy also removes caller-supplied Actor Name values and does not inject one.

Only paths bound to the selected `content_id` are exposed. The proxy does not expose an unscoped LLM-Wiki root API.

The read-only evidence query subresources use `POST` but require only read permission: `/query` (LLMWiki #34) and `/preview-queries` (LLMWiki #13 previewQuery).

## MCP Proxy

The MCP endpoint is exact and has no wildcard suffix:

```http
POST /v1/llmwikis/kb-east/mcp
Authorization: Bearer <user_access_token>
Content-Type: application/json
```

AIGateway authorizes the user against the CSGHub KB record, loads its stored private `metadata.mcp_endpoint_url`, and proxies directly to that per-KB MCP service. It does not call the LLM-Wiki management backend to resolve the endpoint for each request.

## Update and Delete

```http
PUT /api/v1/agent/knowledge-bases/{id}
DELETE /api/v1/agent/knowledge-bases/{id}

PUT /api/v1/agent/knowledge-bases/content-id/{content_id}
DELETE /api/v1/agent/knowledge-bases/content-id/{content_id}
```

These operations update or remove the CSGHub Agent KB record. They do not update or delete the LLM-Wiki resource. LLM-Wiki content changes use the management proxy.

`metadata` updates are partial. Clients should omit response-only `base_url` and `resource_state` fields. An administrator may replace `metadata.mcp_endpoint_url` through either update route by supplying a private endpoint that follows the registration constraints. If a client sends back the exact public MCP URL returned for the same KB, CSGHub treats it as unchanged and preserves the stored private endpoint. A different public or cross-KB URL is rejected. The endpoint cannot be removed from an LLM-Wiki record. Existing records without this metadata remain readable, but their MCP route returns `503` until an administrator configures the endpoint.

## Authorization Summary

| Operation | Required access |
| --- | --- |
| List/get personal KB | Personal owner |
| List/get organization KB | Organization member with read permission |
| Management proxy read | Personal owner or organization read permission |
| Management proxy write | Personal owner or organization write permission |
| MCP | Personal owner or organization read permission |
| Update/delete CSGHub record | Personal owner or organization admin permission |

Authenticated users may read public KB records regardless of the owning namespace. Public visibility does not grant write or admin permission. Detail and list responses expose the caller's permissions: `editable`/`can_manage` mean the user has admin permission on the KB (can update/delete the record), and `can_write` means the user may write KB content through the management proxy. Reading is implied by visibility, so no separate `can_read` field is returned.

The authenticated access token identifies the user to CSGHub. It is not forwarded to LLM-Wiki. CSGHub sends its trusted actor and request identifiers upstream after authorization.

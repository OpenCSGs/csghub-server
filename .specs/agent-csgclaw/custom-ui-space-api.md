# Agent Custom UI Space API

This document describes the endpoints the portal uses to create and select **custom UI spaces** for CSGClaw agents.

These endpoints are available in the **Enterprise (EE)** and **SaaS** editions only (`//go:build ee || saas`).

Related spec: [custom-ui-space.md](custom-ui-space.md) (binding model, routing, trusted proxy context).

## Conventions

### Base path & auth

Both endpoints are under `/agent/instances` and require an authenticated user. `POST /agent/instances/ui-spaces` additionally requires a verified phone number (`NeedPhoneVerified`).

### Response envelope

```json
{
  "code": "AGENT-ERR-16",        // omitted on success; present on error
  "msg": "OK",
  "data": {},
  "total": 0,                    // list endpoints only
  "trace_id": "..."
}
```

### Prerequisites

Custom UI space creation is enabled only when the server operator pins a framework bundle in `configs/agent_runtime/csgclaw.json` (`custom_ui` block). When it is not configured, `POST /agent/instances/ui-spaces` fails with `ErrResourceUnavailable`. The portal may treat a creation failure with that error as "feature disabled in this environment".

---

## Endpoints

### 1. List Agent UI options

Lists the public spaces tagged `task/agent-ui` that the current user can select as an agent's custom UI.

```
GET /agent/instances/ui-options?namespace={namespace}&per={per}&page={page}
```

**Query parameters**

| Name        | Type   | Description                                                       |
|-------------|--------|-------------------------------------------------------------------|
| `namespace` | string | Optional. Only return spaces under this namespace (user or org).  |
| `per`       | int    | Page size.                                                        |
| `page`      | int    | Page number (1-based).                                            |

**Response** `200 OK`

```json
{
  "msg": "OK",
  "data": [
    {
      "space_id": 123,
      "path": "alice/my-agent-ui",
      "name": "my-agent-ui",
      "private": false,
      "sdk": "gradio",
      "status": "Running",
      "endpoint": "my-agent-ui.spaces.example.com"
    },
    {
      "space_id": 124,
      "path": "my-org/team-ui",
      "name": "team-ui",
      "private": true,
      "sdk": "gradio",
      "status": "Unavailable",
      "endpoint": ""
    }
  ],
  "total": 2
}
```

| Field       | Type    | Description |
|-------------|---------|-------------|
| `space_id`  | int64   | Space ID. Use as `metadata.provision_request.custom_ui_space_id` when creating the agent. |
| `path`      | string  | `namespace/name`. The namespace prefix tells the portal ownership (current user vs. one of their orgs). |
| `name`      | string  | Space name. |
| `private`   | bool    | Repo visibility of the space. Public spaces are listed for everyone; private spaces only when the current user has access. See "Private UI spaces" below. |
| `sdk`       | string  | Space SDK (e.g. `gradio`). |
| `status`    | string  | Aggregated state: `Running` or `Unavailable`. `Unavailable` covers building, failed, stopped, and never-deployed. |
| `endpoint`  | string  | Public endpoint of the space, present only when `status` is `Running`; empty otherwise. |

**Private UI spaces**

A private space can be bound as a custom UI, but binding one requires **write permission** on the space (public spaces only need read). After binding, the space's page is anonymously reachable **only through the agent's custom domain** — its own endpoint stays access-controlled. Binding a private space to a public agent is a deliberate publish action; the portal should warn accordingly.

**Selection rules for the portal**

- Only `Running` options can be bound successfully: agent creation re-validates that the space is public, tagged, readable, and has a Running deployment. `Unavailable` options are shown for visibility (a just-created space is `Unavailable` until its build finishes) and will fail binding if selected early.
- Poll or refresh this endpoint after creating a space (see below) until the new space's `status` becomes `Running`.
- **Open/edit gating**: the API does not return a permission flag. The portal infers it from `path`: spaces under the current user's namespace or their orgs are the user's own; open/edit actions should be offered only for those.

### 2. Create a custom UI space

Creates a public space seeded with the platform-pinned agent UI framework, auto-selects a free compute resource, and tags it `task/agent-ui`. When `name` is omitted, the server generates a unique repo name (`agent-ui-<random 8-char suffix>`) under the requested namespace. The space is created with `min_replica = 1` (auto-sleep disabled) so a bound agent's custom domain keeps serving. The space builds and deploys automatically after creation.

```
POST /agent/instances/ui-spaces
```

**Request body**

```json
{
  "namespace": "alice",
  "name": "my-agent-ui",
  "nickname": "My Agent UI",
  "description": "Custom UI for my agent"
}
```

| Field         | Type   | Required | Description |
|---------------|--------|----------|-------------|
| `namespace`   | string | yes      | Owner of the space; may name the current user or an organization the user belongs to. |
| `name`        | string | no       | Repo name; must be unique within the namespace. If omitted, the server generates `agent-ui-<random 8-char suffix>`. |
| `nickname`    | string | no       | Display name; defaults to the space name. |
| `description` | string | no       | Repo description. |

The portal cannot choose the SDK, framework files, visibility (`private` is always `false`), or compute resource — the server decides all of these.

**Response** `200 OK`

```json
{
  "msg": "OK",
  "data": {
    "space_id": 123,
    "path": "alice/agent-ui-3f9k2mdq"
  }
}
```

| Field      | Type   | Description |
|------------|--------|-------------|
| `space_id` | int64  | The new space's ID. |
| `path`     | string | `namespace/name` of the new space; the name segment is the provided name or the server-generated `agent-ui-<suffix>` name. |

**Errors**

| Condition | HTTP | Notes |
|-----------|------|-------|
| `namespace` missing | 400 | Body validation. |
| `name` already exists in the namespace | 400 | `ErrDatabaseDuplicateKey`; only reachable with a client-provided `name` — choose another name or omit it. Generated names are retried server-side. |
| No free compute resource available | 400 | `ErrResourceUnavailable`; also returned when the feature is not configured. |
| Framework bundle unavailable or digest mismatch | 500 | Server-side config issue; retry later or contact ops. |
| Tag `task/agent-ui` could not be attached | 500 | The space is left in place; retry tagging via the repo tags API instead of recreating. |

**After creation**

The new space appears in `GET /agent/instances/ui-options` immediately, initially as `Unavailable`. It becomes `Running` when its build and deployment complete — no further portal action is needed. Suggested flow: create → poll ui-options until `Running` → preselect it in the agent creation form.

---

## Complete portal flow

```text
1. GET  /agent/instances/ui-options          (show existing options; offer "create new")
2. POST /agent/instances/ui-spaces           (create; response has space_id and path)
3. GET  /agent/instances/ui-options          (poll until the new space is Running)
4. POST /agent/instances                     (create agent with
        metadata.provision_request.custom_ui_space_id = <space_id>,
        optionally metadata.provision_request.subdomain = <custom host label>)
```

Agent creation with a custom UI binding is documented in [custom-ui-space.md](custom-ui-space.md#main-flow).

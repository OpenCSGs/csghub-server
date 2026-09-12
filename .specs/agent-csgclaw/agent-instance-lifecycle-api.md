# Agent Instance Lifecycle API

This document describes the agent-instance lifecycle management endpoints for the **CSGClaw** agent type: **Update**, **Restart**, **Suspend**, and **Resume**.

These endpoints are available in the **Enterprise (EE)** and **SaaS** editions only (`//go:build ee || saas`). Restart, suspend, and resume are backed by the SandboxV2 / Knative sandbox lifecycle.

> **Type support:** Suspend, resume, and restart are supported **only** for instances whose `type` is `csgclaw`. Requests against any other agent type return an error (`suspend`/`resume`/`restart is not supported for agent type ...`). Update works for all agent types.

## Conventions

### Base path & auth

All endpoints are under `/agent/instances` and require an authenticated user with a verified phone number (the `NeedPhoneVerified` middleware). Requests are authorized per instance: callers may only operate on instances they own; otherwise the API returns `403 Forbidden`.

### Response envelope

Every response uses the standard JSON envelope:

```json
{
  "code": "AGENT-ERR-16",        // omitted on success; present on error (see Error reference)
  "msg": "OK",                   // "OK" on success, human-readable message on error
  "data": {},                    // omitted when empty/null
  "trace_id": "...",             // omitted unless set
  "context": {}                  // omitted unless an error carries context
}
```

On success, the four endpoints described here return `HTTP 200` with an empty body:

```json
{ "msg": "OK" }
```

### Type scoping

| Operation | Supported types | Behavior for other types |
|-----------|-----------------|--------------------------|
| Update    | all            | Metadata is merged generically; provisioning-metadata immutability rules below apply only to `csgclaw`. |
| Restart   | `csgclaw` only | Returns an error (`restart is not supported for agent type ...`). |
| Suspend   | `csgclaw` only | Returns an error (`suspend is not supported for agent type ...`). |
| Resume    | `csgclaw` only | Returns an error (`resume is not supported for agent type ...`). |

---

## Endpoints

### 1. Update an agent instance

Updates an existing instance. Its primary use is **moving a `csgclaw` instance to a different hardware resource** by changing `metadata.provision_request.resource_id`, which triggers an automatic sandbox restart onto the new resource. It also accepts registry-only fields (`name`, `description`, `public`).

```
PUT /agent/instances/{id}
```

**Path parameters**

| Name | Type   | Description                       |
|------|--------|-----------------------------------|
| `id` | int64  | Instance ID.                       |

**Updating `resource_id` (move to a new resource)**

To move a `csgclaw` instance to a different hardware resource, send the new resource ID under `metadata.provision_request.resource_id`:

```json
{
  "metadata": {
    "provision_request": {
      "resource_id": 7
    }
  }
}
```

| Field                                    | Type  | Description |
|------------------------------------------|-------|-------------|
| `metadata.provision_request.resource_id` | int64 | Target hardware resource ID. Must be a positive integer. |

Behavior:

1. The component resolves the instance's **current** resource ID from the pre-merge metadata `metadata.provision_request.resource_id` (recorded at creation); for legacy instances created before `resource_id` was persisted, it falls back to the sandbox deploy's SKU. It then compares this value with the requested value.
2. If the value differs, the sandbox is restarted onto the new resource. The runner's same-name redeploy preserves the shared PVC and keeps the deploy ID stable, so instance data and the stored `deploy_id` survive the move. The new resource ID is then recorded back into `metadata.provision_request.resource_id`.
3. If the value equals the current resource ID, the request is a **no-op** — no restart is performed.
4. The target resource is resolved and validated (`resolveResource`): it must exist, and the caller's account must be able to use it. An invalid or unavailable resource fails the update.

> To restart the sandbox in place (without moving to a new resource), use [`POST /agent/instances/{id}/restart`](#2-restart-an-agent-instance), which always restarts onto the instance's current resource.

**Other fields (registry-only)**

| Field         | Type            | Required | Description |
|---------------|-----------------|----------|-------------|
| `name`        | string          | no       | New instance name. Must be unique per user. |
| `description` | string          | no       | New description. |
| `public`      | bool            | no       | New visibility. |
| `metadata`    | map[string]any  | no       | Metadata delta. See **Metadata merge semantics** below. |

The `type` and `content_id` fields cannot be changed via this endpoint: `content_id` is discarded and a `type` mismatch is rejected.

**Metadata merge semantics**

`metadata` is merged into the stored instance metadata at the top level using `MergeMapWithDeletion`:

- A key with a non-`null` value **sets or overwrites** the stored key.
- A key with a `null` value **deletes** the stored key.
- Omitted keys are left untouched.

For `csgclaw`, the merged metadata is then validated against the provisioning-immutability rules (below). For other types, the merge is applied directly with no sandbox interaction.

**Semantics for `csgclaw`**

Provisioning metadata is baked into the sandbox at creation time. Most `provision_request` fields cannot be changed on a live sandbox, with four exceptions:

1. **`metadata.provision_request.resource_id`** — moves the sandbox to a different hardware resource (auto-restart).
2. **`metadata.provision_request.llm.model`** — switches the LLM model (auto-restart, swapping only the `CSGCLAW_LLM_MODELS` environment variable on top of the current environment).
3. **`metadata.provision_request.custom_ui`** — display-only. Stored on the instance and surfaced on the shared page; never applied to the sandbox. Updating it is a metadata-only change (no restart).
4. **`metadata.provision_request.guest_usage_limit`** — display-only. Stored on the instance and surfaced on the shared page; never applied to the sandbox. Updating it is a metadata-only change (no restart).

Any other `provision_request` change (for example `env`, `repo_path`) is rejected with `AGENT-ERR-16` (`ErrInstanceProvisioningMetadataImmutable`) and `400 Bad Request`.

A registry-only update (`name`, `description`, `public` with no metadata) has no sandbox counterpart and is a no-op at the sandbox layer.

**Responses**

| Status | Description |
|--------|-------------|
| `200`  | Updated successfully (including a resource move, or a no-op when the resource is unchanged). |
| `400`  | Invalid instance ID, malformed body, an immutable provisioning-metadata change (`AGENT-ERR-16`), a non-positive `resource_id`, or a cross-cluster resource move (`SANDBOX-ERR-4`). |
| `403`  | Caller does not own the instance. |
| `500`  | Any other failure, including an unavailable resource or name conflict. |

---

### 2. Restart an agent instance

Recreates the instance's backing sandbox **in place**, on its current hardware resource. The runner's same-name redeploy preserves the shared PVC, so instance data survives the restart. The current resource is resolved from the instance's stored `metadata.provision_request.resource_id`; there is no request body and no way to move to a different resource via this endpoint (use [`Update`](#1-update-an-agent-instance) to move).

```
POST /agent/instances/{id}/restart
```

**Path parameters**

| Name | Type   | Description |
|------|--------|-------------|
| `id` | int64  | Instance ID. |

**Request body**

None.

**Responses**

| Status | Description |
|--------|-------------|
| `200`  | Restart completed. |
| `400`  | Invalid instance ID. |
| `403`  | Caller does not own the instance. |
| `500`  | Instance not found, unsupported agent type, missing stored `resource_id`, or sandbox restart failure. |

> Only `csgclaw` instances support restart. Restart does not mutate instance metadata.

---

### 3. Suspend an agent instance

Suspends the instance's backing sandbox (the pod is stopped while the PVC and deployment are preserved). No request body.

```
POST /agent/instances/{id}/suspend
```

**Path parameters**

| Name | Type   | Description |
|------|--------|-------------|
| `id` | int64  | Instance ID. |

**Responses**

| Status | Description |
|--------|-------------|
| `200`  | Sandbox suspended. |
| `400`  | Invalid instance ID. |
| `403`  | Caller does not own the instance. |
| `500`  | Instance not found, unsupported agent type, or sandbox suspend failure. |

> Only `csgclaw` instances support suspend. Suspend does not mutate instance metadata.

---

### 4. Resume an agent instance

Resumes a previously suspended instance's backing sandbox. No request body.

```
POST /agent/instances/{id}/resume
```

**Path parameters**

| Name | Type   | Description |
|------|--------|-------------|
| `id` | int64  | Instance ID. |

**Responses**

| Status | Description |
|--------|-------------|
| `200`  | Sandbox resumed. |
| `400`  | Invalid instance ID. |
| `403`  | Caller does not own the instance. |
| `500`  | Instance not found, unsupported agent type, or sandbox resume failure. |

> Only `csgclaw` instances support resume. Resume does not mutate instance metadata.

---

## Data models

### `AgentInstance`

The instance resource. The request body for `PUT /agent/instances/{id}` accepts a subset of these fields.

```json
{
  "id": 42,
  "template_id": 7,
  "name": "my-agent",
  "description": "A demo agent",
  "type": "csgclaw",
  "content_id": "csgclaw-abc123",
  "public": false,
  "is_running": true,
  "metadata": {
    "template_name": "repo/agent",
    "provision_request": {
      "resource_id": 5,
      "repo_path": "owner/repo",
      "env": { "LOG_LEVEL": "debug" },
      "llm": { "model": "deepseek-v4-pro" },
      "custom_ui": { "theme": "dark" },
      "guest_usage_limit": { "mode": "limited", "max_count": 10 }
    }
  },
  "created_at": "2026-08-27T00:00:00Z",
  "updated_at": "2026-08-27T00:00:00Z"
}
```

| Field         | Type            | Description |
|---------------|-----------------|-------------|
| `id`          | int64           | Instance ID. |
| `template_id` | int64           | Associated agent-template ID. |
| `name`        | string          | Instance name. |
| `description` | string          | Instance description. |
| `type`        | string          | Agent type (`langflow`, `code`, `csgclaw`, …). |
| `content_id`  | string          | Unique ID of the backing sandbox resource. |
| `public`      | bool            | Whether the instance is public. |
| `is_running`  | bool            | Whether the sandbox is running. |
| `metadata`    | map[string]any  | Instance metadata. |

### `provision_request` (csgclaw metadata)

Nested under `metadata.provision_request` for `csgclaw` instances. Created once at instance creation; persists for the lifetime of the instance.

| Field               | Type              | Mutable? | Description |
|---------------------|-------------------|----------|-------------|
| `resource_id`       | int64             | yes      | Hardware resource ID. Changing it (via update) moves the sandbox to a new resource (auto-restart). |
| `llm.model`         | string            | yes      | LLM model. Changing it (via update) restarts the sandbox with the new `CSGCLAW_LLM_MODELS`. |
| `custom_ui`         | map[string]string | yes      | Display-only UI customization for the shared page. Metadata-only change; never written to the sandbox. |
| `guest_usage_limit` | object            | yes      | Display-only guest usage policy (`{mode, max_count}`) for the shared page. Metadata-only change; never written to the sandbox. |
| `repo_path`         | string            | no       | Source repository path. Immutable after creation. |
| `env`               | map[string]string | no       | Custom environment variables. Immutable after creation. |

### `UpdateAgentInstanceRequest`

Used by the alternative update endpoint `PUT /agent/instances/by-content-id/{type}/{content_id}`. It carries the same mutable fields and follows the same csgclaw immutability rules.

```json
{
  "name": "renamed",
  "description": "updated",
  "public": true,
  "metadata": { "provision_request": { "llm": { "model": "another-model" } } }
}
```

---

## Error reference

Errors are returned with an HTTP status and, where available, a `code` of the form `AGENT-ERR-{code}` plus a `context` object carrying structured fields.

| `code` | Error | HTTP | Notes |
|--------|-------|------|-------|
| `AGENT-ERR-16` | `ErrInstanceProvisioningMetadataImmutable` | 400 | A `csgclaw` update attempted to change immutable provisioning metadata. |
| `AGENT-ERR-18` | `ErrAgentProvisionRequestFieldNull` | 400 | A `provision_request` field is `null`. |
| `AGENT-ERR-19` | `ErrAgentProvisionRequestFieldType` | 400 | A `provision_request` field has an invalid type. |
| `AGENT-ERR-20` | `ErrAgentProvisionRequestFieldEmpty` | 400 | A `provision_request` field is empty. |
| `AGENT-ERR-21` | `ErrAgentProvisionRequestModelUnavailable` | 400 | The pinned `llm.model` is not in the available model catalog. |
| `SANDBOX-ERR-4` | `ErrSandboxCrossClusterRestart` | 400 | A `csgclaw` update targets a `resource_id` on a different cluster. |
| — | `ErrForbidden` | 403 | Caller does not own the instance. |
| — | other | 500 | Not found, unsupported type, or internal sandbox failure. |

---

## Notes & caveats

- **Immutability guard**: the update merges the incoming `metadata` into the stored metadata, then diffs the merged `provision_request` against the pre-merge snapshot. Any immutable key that is added, removed, or whose value changed is rejected with `AGENT-ERR-16`; the four mutable fields (`resource_id`, `llm`, `custom_ui`, `guest_usage_limit`) are excluded from the diff. Because the diff compares before/after values (not mere presence), deleting an immutable key is also detected — the metadata merge turns a nested `null` into a deletion, so an absent key is not mistaken for "untouched".

- **Suspend/resume and status**: suspend/resume persist the deploy record's status (`Sleeping`/`Running`) and `StatusUpdateAt`, so status queries and `is_running` reflect the suspended/resumed state rather than a stale prior value.

- **Restart is in place**: `POST /agent/instances/{id}/restart` always restarts onto the instance's current resource (read from `metadata.provision_request.resource_id`); it cannot move to a different resource. Moving to a different resource is done via `PUT` (update), which rejects a target on a different cluster with `SANDBOX-ERR-4`.

- **Shared-page display fields and legacy instances**: `custom_ui` and `guest_usage_limit` are surfaced on the shared page only when present in `metadata.provision_request`. Instances created before these fields existed carry neither, so the shared-page response omits both keys entirely (`omitempty` on `custom_ui` / `guest_usage_limit`); no error is raised and no default value is substituted. Clients must treat a missing key as "no custom UI configured" / "no guest usage limit configured". (Edge case: an explicitly empty `guest_usage_limit: {}` is coerced to `{"mode":"unlimited"}` and is therefore emitted, whereas an absent field is omitted.)

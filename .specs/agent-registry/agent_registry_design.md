# Agent Registry Design

## Overview

The Agent Registry uses existing `AgentTemplate` and `AgentInstance` records.

```text
 Owner: publish CSGClaw Agent Template to Code repository
                  │
                  ▼
     agent.toml committed to Code repository
                  │
                  ▼
     parse agent.toml and validate runtime_kind
     (supported runtime kind: codex)
                  │
                  ▼
      EE/SaaS Git callback ─────────────► AgentTemplate
                  │                         - name = Code repository repo_path
                  │                         - description = Code repository description
                  │                         - metadata.repo_path
                  │                         - metadata.agent_file
                  │                         - public mirrors repository visibility
                  │
                  │ (Template only; never deploys a Sandbox)
                  │
                  ▼
 Owner: POST /agent/instances
     - template_id, or
     - metadata.provision_request
       (repo_path, resource_id, env, llm)
                  │
                  ▼
       resolve repository-managed Template ◄── template_id / repo_path
                  │
                  ▼
       CSGClaw AgentInstance adapter ◄──── approved runtime profile
                  │                         configs/agent_runtime/csgclaw.json
                  │                                  │
                  │                                  ▼
                  │                         runtime profile initializer
                  │                                  │ content-hash synchronization
                  │                                  ▼
                  │                         agent_configs.sandbox_runtime.csgclaw
                  │
                  ├────► LLM environment
                  │        base URL / API key from AIGateway (platform-derived)
                  │        default models: eligible AIGateway text-generation model
                  │        model pin: metadata.provision_request.llm.model
                  │        → CSGCLAW_LLM_BASE_URL / API_KEY / MODELS
                  │
                  ▼
 Sandbox resource selection
     resource_id supplied ───────────────► use the requested resource
     resource_id omitted ────────────────► auto-select a Sandbox-scenario resource
                                             that satisfies CPU and memory requirements
                  │
                  ▼
 Sandbox create ─────────────────────────► Sandbox
                  │                         image, env, selected resource, PVC mount
                  ▼
 AgentInstance
     - content_id = Sandbox name
     - metadata.runtime_snapshot
     - public controls share eligibility
                  │
                  ▼
 Owner: POST /agent/instances/{id}/share
                  │
                  ▼
 AgentShare
     - share_uuid = management / display lookup id
     - share_name = short Sandbox proxy alias
                  │
                  ▼
 AIGateway Sandbox proxy
     Owner: authenticated request ─► owner Sandbox lookup
     Anonymous user: request        ─► agent_shares.share_name lookup
                                      → public CSGClaw instance → target Sandbox
                                      → all Sandbox paths
```

A repository synchronizes an Agent Template only. It never creates, updates, stops, or restarts an Agent Instance or Sandbox. An Agent Instance is created explicitly by its owner and owns the Sandbox lifecycle.

`AgentTemplate.public` controls Template visibility. `AgentInstance.public` controls whether the CSGClaw instance can be shared and whether an existing share remains valid. The two flags have separate purposes.

An owner can create an agent share only for a public CSGClaw instance. A share does not expose the real Sandbox name: `agent_shares.share_name` is a generated, unique proxy alias. Anonymous Sandbox access must use that `share_name`, not the AgentInstance `content_id`. If the owner later makes the instance private, both the shared-instance API and the share alias return `404`.

## Repository-managed Templates

The EE/SaaS Git callback observes `agent.toml` changes.

| Repository event | Result |
| --- | --- |
| Add or modify `agent.toml` | Create or update the matching Agent Template. |
| Delete `agent.toml` | Delete the matching Agent Template only. Existing instances continue running. |
| Delete Code repository | Delete matching repository-managed Agent Templates only. Existing instances continue running. |
| Code repository visibility update | Synchronize the public flag of Templates linked to that repository. |
| Repository-managed Template public update | Update the linked Code repository visibility through `CodeComponent`. |

Repository-managed Templates use the Code repository `repo_path` as their name and its description as their description. `AgentTemplate.user_uuid` is the Code repository owner's user UUID, including when the repository path belongs to an organization; it is not the organization namespace UUID and it is not the last user who updated the repository. Template lookups use `type` and `name`; `metadata.repo_path` remains provenance and preserves the user or organization namespace path. `metadata.owner_type` records whether that namespace is `user` or `org`. Repository synchronization records the parsed manifest, namespace owner type, and runtime kind:

```json
{
  "repo_path": "namespace/repository",
  "owner_type": "org",
  "runtime_kind": "codex",
  "agent_file": {
    "name": "assistant",
    "runtime_kind": "codex"
  }
}
```

Repository-managed CSGClaw Templates keep `content` empty. The parsed `agent.toml` is stored in `metadata.agent_file`; Git remains the source of the raw manifest.

Private repository-managed Templates are visible through the Agent Template API only to `AgentTemplate.user_uuid`. Public repository-managed Templates are visible and usable by everyone. Organization-backed Templates do not grant private visibility to all organization members in this design; the organization identity is represented by `metadata.repo_path` and `metadata.owner_type`.

The visibility synchronization is limited to Code repositories and repository-managed Templates. It is implemented by `CodeComponent` and `AgentTemplateComponent`, not by the generic `RepoComponent`.

CE keeps the Git callback as a no-op for Agent Templates.

## CSGClaw Instances

`csgclaw` is the Sandbox-backed Agent Instance type. It replaces the unreleased `general` type.

Creating a CSGClaw instance requires a repository-managed Template. The request supplies `template_id`, or `metadata.provision_request.repo_path` resolves the Template by its repository path. When both are supplied, they must identify the same Template. The resolved Template ID is stored on the instance. The adapter extracts the repository name from `template.metadata.repo_path` and creates a Sandbox name in this form:

```text
<sanitized-repository-name>-<12-character-lowercase-nanoid>
```

The random suffix is generated before Sandbox deployment and is stored as the instance `ContentID`. The instance display name is not used as the Sandbox name. The generated name is constrained to the Kubernetes DNS-label length limit.

The adapter creates the Sandbox, then the Agent Instance record stores the Sandbox name, user metadata, and a runtime snapshot. The provisioning metadata (environment, resource, LLM settings, and the runtime snapshot) is baked into the Sandbox at creation time and is immutable afterwards: there is no Sandbox update or recreate path. Updating an instance is limited to the registry fields `name`, `description`, and `public`; supplying `metadata` in an update request is rejected with `400`.

Deleting an instance stops its Sandbox. Deleting the source Template does not affect an existing instance.

## Runtime Profile

CSGClaw runtime configuration is Git-managed in [csgclaw.json](../../configs/agent_runtime/csgclaw.json).

```text
configs/agent_runtime/csgclaw.json
               │
               ▼
agent_configs: sandbox_runtime.csgclaw
```

API runtime initialization reads the profile and synchronizes it to `agent_configs` by content hash. The profile contains the CSGClaw image, version, Sandbox port, container command, HTTP health check, and `default_env`. Ordinary Agent Config API operations cannot modify `sandbox_runtime.*` records.

The current profile supplies the complete image reference:

```text
opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/csgclaw-server-sandbox:2026080504
```

When an instance is created, the adapter records the selected image, version, port, command, health check, and `default_env` as `metadata.runtime_snapshot`. The snapshot is fixed at creation and cannot be changed, and there is no Sandbox update or recreate path. New profile versions affect future instances only.

The CSGClaw profile declares its image command as an argv array and its HTTP liveness endpoint as `GET /healthz`; the health check uses the runtime `port`. The current Sandbox create API does not yet accept command or health-check fields, so the adapter stores them in the runtime snapshot and will forward them when that API supports them.

`CSGCLAW_PVC_MOUNT_PATH` and `SKILLS_POLL_INTERVAL` are runtime-profile `default_env` values. The adapter uses the profile mount path for its PVC volume and does not overwrite either value with hard-coded defaults.

## Sandbox Provisioning

The adapter merges Sandbox environment values in this order:

```text
runtime snapshot default_env
  → metadata.provision_request.env
  → protected platform environment
```

Protected CSGClaw, CSGHub, OpenCSG, and port variables cannot be overridden by user custom environment values.

An optional Sandbox resource can be supplied in the same metadata convention used by OpenClaw:

```json
{
  "provision_request": {
    "resource_id": 123
  }
}
```

`resource_id` is optional. When present it must be a positive integer and is forwarded to `SandboxCreateRequest`. The metadata is persisted with the instance at creation and, like all provisioning metadata, cannot be changed later.

## LLM Configuration

The CSGClaw adapter always derives the LLM environment from the platform: `CSGCLAW_LLM_BASE_URL` is the configured AIGateway OpenAI-compatible URL and `CSGCLAW_LLM_API_KEY` is the owner's built-in AIGateway API key. The owner cannot override these two values.

By default, the adapter discovers models through `LLMServiceComponent` instead of using a hard-coded model name. It selects the first LLM configuration that satisfies all of the following:

- The configuration has the AIGateway LLM type.
- The configuration is enabled.
- The configuration is currently available.
- `metadata.task` or `metadata.tasks` contains `text-generation`.

The selected model is encoded into `CSGCLAW_LLM_MODELS` as a JSON array. If no configuration matches, the adapter logs a warning and starts CSGClaw with an empty model list (`[]`); the owner can configure a model in CSGClaw later.

Owners can pin a single model through typed provision metadata:

```json
{
  "provision_request": {
    "llm": {
      "model": "qwen-plus"
    }
  }
}
```

The pinned model must be a non-empty string and be in the available AIGateway text-generation catalog. Omitting `llm.model` (or the whole `llm` object) keeps the discovered default; supplying a non-string, empty, or `null` `model` — such as `123`, `""`, or `null` — fails instance creation with `400`, as does an unknown model name. When pinned, `CSGCLAW_LLM_MODELS` is set to `["model"]`; otherwise it keeps the discovered default.

| Sandbox environment | Value |
| --- | --- |
| `CSGCLAW_LLM_BASE_URL` | AIGateway OpenAI-compatible URL, always platform-derived. |
| `CSGCLAW_LLM_API_KEY` | Owner's built-in AIGateway API key, always platform-derived. |
| `CSGCLAW_LLM_MODELS` | `provision_request.llm.model` when pinned; otherwise the first eligible AIGateway text-generation model. |

Raw `CSGCLAW_*`, `CSGHUB_*`, `OPENCSG_*`, `PORT`, and `TEMPLATE_ID` values remain protected and cannot be supplied through `metadata.provision_request.env`.

## Sandbox Proxy

The AIGateway splits Sandbox proxy access into two routes:

```text
/v1/sandboxes/{sandbox_name}
/v1/sandboxes/{sandbox_name}/*
/v1/shared/sandboxes/{share_name}/*
```

`/v1/sandboxes/{sandbox_name}` requires login and resolves only the caller's own Sandbox; anonymous requests are rejected with `401`. `/v1/shared/sandboxes/{share_name}/*` is unauthenticated: AIGateway resolves `agent_shares.share_name=share_name`, verifies that the target remains a public CSGClaw instance, and uses the target's internal `content_id` for the Runner request. Only Sandbox API sub-paths are proxied — the Sandbox root is not accessible anonymously. Anonymous requests cannot use the real AgentInstance `content_id`. Missing, private, non-CSGClaw, or unavailable Sandboxes return `404`.

All Sandbox paths are forwarded. AIGateway removes caller-supplied internal authorization headers and injects the platform authorization used to access the Sandbox proxy target.

## CSGClaw Agent Chat Proxy

Authenticated CSGClaw chat uses the existing generic AIGateway Agent proxy route:

```text
POST /v1/agent/csgclaw/agents/{sandbox_name}/sessions/{session_id}/responses
```

`sandbox_name` is the CSGClaw instance `content_id`, which identifies the Sandbox. AIGateway resolves the visible instance and its owner Sandbox proxy URL, then replaces that path segment with `metadata.template_metadata.agent_file.name` when forwarding to CSGClaw:

```text
POST {sandbox proxy URL}/api/v1/agents/{agent.toml.name}/sessions/{session_id}/responses
```

The request body, response status, response body, and Responses-compatible SSE stream are passed through unchanged. Before forwarding, AIGateway creates the corresponding CSGHub AgentInstance session and replaces caller authorization headers with the platform Sandbox-proxy credential.

## API

All Agent Registry management APIs are available in EE and SaaS under `/api/v1/agent`. `GET /agent/templates` is public and returns only public Templates to an anonymous caller. Other read APIs require the normal authenticated API session; create, update, and delete operations also require phone verification.

### Templates

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/v1/agent/templates` | List Templates. An anonymous caller receives public Templates only; an authenticated caller also receives its own Templates. Supports `search`, `type`, `per`, and `page`. |
| `POST` | `/api/v1/agent/templates` | Create a user-managed Template. |
| `GET` | `/api/v1/agent/templates/{id}` | Get a Template. |
| `PUT` | `/api/v1/agent/templates/{id}` | Update a Template owned by the current user. Updating `public` on a repository-managed Template also updates its linked Code repository visibility. |
| `DELETE` | `/api/v1/agent/templates/{id}` | Delete an owned Template. |

`GET /api/v1/agent/templates` intentionally has no login middleware. The store applies `public = true` when the request has no authenticated user; authenticated callers receive both public Templates and Templates they own. The list response contains template metadata and display fields but omits `content`. It is filtered by `search` (name or description), `type`, `per`, and `page`.

Example anonymous CSGClaw Template list item:

```json
{
  "id": 42,
  "type": "csgclaw",
  "name": "namespace/repository",
  "description": "Repository description",
  "public": true,
  "metadata": {
    "repo_path": "namespace/repository",
    "runtime_kind": "codex",
    "agent_file": {
      "name": "assistant",
      "runtime_kind": "codex"
    }
  }
}
```

`POST /api/v1/agent/templates` creates a user-managed Template. It requires phone verification; `type`, `name`, and `content` are required. The server always assigns the current user as owner. Example:

```json
{
  "type": "langflow",
  "name": "My flow",
  "description": "Optional description",
  "content": "{\"nodes\":[]}",
  "public": false,
  "metadata": {
    "tags": ["example"]
  }
}
```

`GET /api/v1/agent/templates/{id}` requires login and returns `content`; a private Template is available only to its owner. `PUT` and `DELETE` require phone verification and ownership. A `PUT` may update supplied `name`, `description`, `content`, `metadata`, and `public` fields. Changing `public` on a repository-managed Template updates the linked Code repository visibility first, then persists the Template flag. Deleting a repository-managed CSGClaw Template deletes its linked Code repository; the Code repository deletion deletes matching repository-managed Templates. Neither direction stops or deletes existing Agent Instances.

Repository-managed Templates are created and updated by the Git callback. The ordinary `POST /api/v1/agent/templates` API rejects `metadata.repo_path`, so it cannot create repository-managed Templates. Their `content` is empty, and their `name`, `description`, `metadata.agent_file`, and `public` are synchronized from the Code repository and `agent.toml` on a later callback.

### CSGClaw Instances

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/agent/instances` | Create an explicit CSGClaw instance and its Sandbox. |
| `GET` | `/api/v1/agent/instances` | List instances. Supports `search`, `type`, `public`, `built_in`, `editable`, `per`, and `page`. |
| `GET` | `/api/v1/agent/instances/{id}` | Get an instance. |
| `PUT` | `/api/v1/agent/instances/{id}` | Update registry fields only: `name`, `description`, `public`. Provisioning metadata is immutable; supplying `metadata` returns `400`. |
| `DELETE` | `/api/v1/agent/instances/{id}` | Stop and delete an instance Sandbox. |
| `GET` | `/api/v1/agent/instances/status` | Get the status of instances. |

`POST /api/v1/agent/instances` requires phone verification and Agent access. The server sets the owner from the authenticated request, creates and starts the Sandbox, and then creates the AgentInstance record. Clients must not supply `content_id`; CSGHub generates it as the Sandbox name.

#### Create request

| Field | Required | Description |
| --- | --- | --- |
| `type` | Yes | Must be `"csgclaw"`. |
| `name` | Yes | Owner-visible instance name. It must be unique for the owner. It is not the Sandbox name. |
| `description` | No | Instance description. |
| `public` | No | Defaults to `false`. A public CSGClaw instance may be shared through the instance-share API. |
| `template_id` | One of `template_id` or `metadata.provision_request.repo_path` | ID of a repository-managed CSGClaw Template. |
| `metadata.provision_request.repo_path` | One of `template_id` or `repo_path` | Code repository path, such as `namespace/repository`, used to resolve the CSGClaw Template. When both identifiers are supplied, they must resolve to the same Template. |
| `metadata.provision_request.resource_id` | No | Positive integer Sandbox resource ID. If omitted, Sandbox chooses a compatible resource from the Sandbox scenario. |
| `metadata.provision_request.env` | No | String-to-string CSGClaw environment overrides. Platform-managed `CSGCLAW_*`, `CSGHUB_*`, `OPENCSG_*`, `PORT`, and `TEMPLATE_ID` names are rejected. |
| `metadata.provision_request.llm.model` | No | Pin a single available AIGateway text-generation model, encoded as `CSGCLAW_LLM_MODELS=["model"]`. When supplied, `model` must be a non-empty string; a non-string, empty, or `null` value (e.g. `123`, `""`, or `null`) fails with `400`. An unknown model also fails with `400`. `llm.base_url` and `llm.api_key` are always platform-derived and are ignored if supplied. |

Create with a repository path:

```json
{
  "type": "csgclaw",
  "name": "My CSGClaw",
  "description": "Optional description",
  "public": false,
  "metadata": {
    "provision_request": {
      "repo_path": "namespace/repository",
      "resource_id": 123,
      "env": {
        "LOG_LEVEL": "debug"
      },
      "llm": {
        "model": "qwen-plus"
      }
    }
  }
}
```

Create with a Template ID instead:

```json
{
  "type": "csgclaw",
  "name": "My CSGClaw",
  "template_id": 42,
  "public": false
}
```

CSGHub always supplies the configured AIGateway URL and the owner’s built-in AIGateway API key. `CSGCLAW_LLM_MODELS` uses the pinned model when supplied; otherwise it is the discovered eligible model list, or `[]` if no eligible model exists — CSGClaw can then be configured later by its owner.

The response contains the generated instance `id` and `content_id`. For CSGClaw, `content_id` is the generated Sandbox name and is the stable identifier used by the Sandbox proxy.

The list (`GET /api/v1/agent/instances`) and detail (`GET /api/v1/agent/instances/{id}`) responses include `is_shared`, a boolean that is `true` when an `agent_shares` record references the instance (`agent_shares.instance_id`). It is derived from the share table, not stored on the instance, so clients can reliably display the shared state without re-deriving it from a prior `POST /api/v1/agent/instances/{id}/share` response.

#### Update request

`PUT /api/v1/agent/instances/{id}` (and the by-content-id variant) updates only `name`, `description`, and `public`. CSGClaw provisioning metadata — `metadata.provision_request`, `metadata.runtime_snapshot`, and any other keys — was baked into the Sandbox at creation and cannot be changed. An update request that includes `metadata` fails with `400`; the Sandbox is never reconfigured or recreated.

### CSGClaw Chat

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/v1/agent/csgclaw/agents/{sandbox_name}/sessions/{session_id}/responses` | Run a CSGClaw agent turn through the instance Sandbox. |

`sandbox_name` is the CSGClaw instance `content_id`; AIGateway resolves the `agent.toml` name retained in instance metadata and uses it as the upstream CSGClaw agent selector. `session_id` is also stored as the CSGHub AgentInstance session ID. The endpoint accepts and forwards CSGClaw Responses request bodies, including `stream: true` SSE requests.

### Anonymous Sandbox Access

The following AIGateway endpoint does not require login:

```text
ANY /v1/shared/sandboxes/{share_name}/*
```

`/v1/shared/sandboxes/{share_name}/*` is accessible when `share_name` is an `agent_shares.share_name` alias for a public `csgclaw` instance. It is not accessible by the real AgentInstance `content_id` for anonymous callers. Only Sandbox API sub-paths are proxied; the Sandbox root path (bare `/v1/shared/sandboxes/{share_name}` or a bare trailing slash) returns `404`. The proxy returns `404` for private or unknown Sandboxes. The caller must not send platform authorization headers; AIGateway supplies its internal authorization to the Sandbox target.

The owner route `ANY /v1/sandboxes/{sandbox_name}` requires login and resolves only the caller's own Sandbox; anonymous requests are rejected with `401`.

### Instance Shares

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/agent/instances/{id}/share` | Create an anonymous share for an owner-owned public CSGClaw instance. |
| `GET` | `/api/v1/agent/shared/instance?share_uuid={share_uuid}` | Get shared instance display data and its short Sandbox proxy alias. |

`agent_shares` stores the share target and access alias:

| Field | Purpose |
| --- | --- |
| `share_uuid` | Opaque management/display lookup id used by `/agent/shared/instance`. |
| `share_name` | Short anonymous Sandbox proxy alias used as `{share_name}` in `/v1/shared/sandboxes/{share_name}`. |
| `instance_id` | Internal target AgentInstance id. |
| `type` | Share type. Currently `instance`. |

The create response contains the opaque management identifier `share_uuid` and generated `share_name`. The public response contains basic instance information plus `shared_sandbox_name`, whose value is `agent_shares.share_name`; Portal uses that value with the AIGateway shared Sandbox proxy route `/v1/shared/sandboxes/{share_name}`. The real CSGClaw `content_id` is never returned by the shared-instance API.

## Edition Boundaries

CSGClaw Sandbox deployment and anonymous AIGateway proxy behavior are built for EE and SaaS. Runtime-profile synchronization is initialized in CE, EE, and SaaS so `sandbox_runtime.csgclaw` is available consistently, but CE does not register the CSGClaw Sandbox adapter or anonymous proxy route.

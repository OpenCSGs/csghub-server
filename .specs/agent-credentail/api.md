# Agent Credential API

## Overview

Agent credential APIs let a logged-in user store credentials and grant a runtime agent session temporary access to selected credentials.

For the end-user workflow with diagrams and examples, see [user-guide.md](./user-guide.md).

The credential feature is part of the main API service and is only built for EE/SaaS:

```go
//go:build ee || saas
```

Base path:

```text
/api/v1/agent/credentials
```

The public API uses `credential_name` instead of numeric credential IDs. The name is unique for each user and is the client-facing credential identifier.

## Auth

Management APIs require normal user auth:

```http
Authorization: Bearer <user_auth_token>
```

Runtime APIs require the runtime credential token returned by the session grant API:

```http
Authorization: Bearer <runtime_credential_token>
```

## Credential Name

`credential_name` must match:

```text
^[a-z][a-z0-9_-]{2,62}$
```

Reserved names:

```text
credentials
sessions
```

Examples:

```text
gitlab-devops
internal-api
prod_gitlab_read
```

## Providers

Supported providers:

| Provider | Purpose |
| --- | --- |
| `gitlab` | First-class GitLab credential. Supports bearer token auth and requires `metadata.base_url`. |
| `generic` | Generic secret for bearer token, custom header, API-key header, or static key/value use. |

### List Providers

```http
GET /api/v1/agent/credentials/providers
Authorization: Bearer <user_auth_token>
```

Response:

```json
{
  "msg": "OK",
  "data": [
    {
      "name": "gitlab",
      "auth_types": ["bearer_token"],
      "credential_fields": [
        {"name": "token", "required": true, "secret": true}
      ],
      "metadata_fields": [
        {"name": "base_url", "required": true}
      ]
    },
    {
      "name": "generic",
      "auth_types": ["bearer_token", "api_key", "custom_header", "static_secret"]
    }
  ]
}
```

## Secret Backend

The public API does not let clients choose the storage backend per credential. Operators choose it for the service with:

```text
OPENCSG_CREDENTIAL_SECRET_BACKEND
```

Supported values:

| Backend | Notes |
| --- | --- |
| `postgres_encrypted` | Default. Stores ciphertext in `credential_secrets`; `credentials.secret_ref` is `pgp://credential_secrets/{id}`. |
| `hashicorp_vault_kv2` | Stores secret material in HashiCorp Vault KV v2; `credentials.secret_ref` is `vault://kv2/{mount}/agent-credentials/{namespace_uuid}/{credential_name}#value`. |

Vault backend config:

```text
OPENCSG_CREDENTIAL_VAULT_ADDRESS
OPENCSG_CREDENTIAL_VAULT_TOKEN
OPENCSG_CREDENTIAL_VAULT_NAMESPACE
OPENCSG_CREDENTIAL_VAULT_KV_DEFAULT_MOUNT
OPENCSG_CREDENTIAL_VAULT_TIMEOUT_SECONDS
```

## Response Envelope

Normal successful responses use the existing API response envelope:

```json
{
  "msg": "OK",
  "data": {}
}
```

List responses may include `total`:

```json
{
  "msg": "OK",
  "data": [],
  "total": 0
}
```

## Credential APIs

### Create Credential

```http
POST /api/v1/agent/credentials
Authorization: Bearer <user_auth_token>
Content-Type: application/json
```

GitLab request:

```json
{
  "credential_name": "gitlab",
  "provider": "gitlab",
  "auth_type": "bearer_token",
  "description": "GitLab token for agent tasks",
  "credential": {
    "token": "replace_with_gitlab_token"
  },
  "metadata": {
    "base_url": "https://git-devops.opencsg.com/"
  }
}
```

Notes:

- `description` is optional. When omitted, the credential is still created and `description` is returned as an empty string.
- `credential_name` must be unique for the current user. Creating another credential with the same name returns a duplicate-name error.
- UI can auto-fill `credential_name` with the selected provider name for first-class providers like `gitlab`.
- The access token is stored through the configured secret backend.
- Runtime resolution returns an `Authorization: Bearer ...` header.

Custom header request:

```json
{
  "credential_name": "internal-api",
  "provider": "generic",
  "auth_type": "custom_header",
  "description": "Internal API key for agent tasks",
  "credential": {
    "value": "replace_with_api_key"
  },
  "metadata": {
    "base_url": "https://internal.example.com",
    "header_name": "X-API-Key"
  }
}
```

Bearer token request:

```json
{
  "credential_name": "external-api",
  "provider": "generic",
  "auth_type": "bearer_token",
  "description": "External API bearer token",
  "credential": {
    "token": "replace_with_token"
  },
  "metadata": {
    "base_url": "https://api.example.com"
  }
}
```

API key request:

```json
{
  "credential_name": "vendor-api",
  "provider": "generic",
  "auth_type": "api_key",
  "description": "Vendor API key",
  "credential": {
    "api_key": "replace_with_api_key"
  },
  "metadata": {
    "base_url": "https://vendor.example.com",
    "header_name": "X-API-Key"
  }
}
```

Static key/value secret request:

```json
{
  "credential_name": "aliyun-oss",
  "provider": "generic",
  "auth_type": "static_secret",
  "description": "Aliyun OSS access key",
  "credential": {
    "access_key_id": "replace_with_access_key_id",
    "access_key_secret": "replace_with_access_key_secret"
  },
  "metadata": {
    "base_url": "https://oss-cn-hangzhou.aliyuncs.com"
  }
}
```

Supported `auth_type` values:

| Provider | Supported auth types |
| --- | --- |
| `gitlab` | `bearer_token` |
| `generic` | `bearer_token`, `custom_header`, `api_key`, `static_secret` |

Unsafe custom header names are rejected, including hop-by-hop and transport headers such as `Host`, `Content-Length`, `Connection`, and `Transfer-Encoding`.

### Credential Response

Create, get, and rotate return a credential record. Secret values are not returned.

```json
{
  "id": 1,
  "credential_name": "gitlab",
  "namespace_uuid": "namespace-uuid",
  "provider": "gitlab",
  "auth_type": "bearer_token",
  "description": "GitLab token for agent tasks",
  "secret_backend": "postgres_encrypted",
  "metadata": {
    "base_url": "https://git-devops.opencsg.com/"
  },
  "status": "active",
  "created_at": "2026-04-15T00:00:00Z",
  "updated_at": "2026-04-15T00:00:00Z"
}
```

### List Credentials

```http
GET /api/v1/agent/credentials?per=10&page=1
Authorization: Bearer <user_auth_token>
```

Query parameters:

| Name | Required | Notes |
| --- | --- | --- |
| `per` | No | Page size. |
| `page` | No | Page number. |
| `search` | No | Case-insensitive search on `credential_name`. |

Response:

```json
{
  "msg": "OK",
  "data": [
    {
      "id": 1,
      "credential_name": "gitlab",
      "namespace_uuid": "namespace-uuid",
      "provider": "gitlab",
      "auth_type": "bearer_token",
      "description": "GitLab token for agent tasks",
      "secret_backend": "postgres_encrypted",
      "metadata": {
        "base_url": "https://git-devops.opencsg.com/"
      },
      "status": "active",
      "created_at": "2026-04-15T00:00:00Z",
      "updated_at": "2026-04-15T00:00:00Z"
    }
  ],
  "total": 1
}
```

Search example:

```http
GET /api/v1/agent/credentials?per=10&page=1&search=gitlab
Authorization: Bearer <user_auth_token>
```

### Get Credential

```http
GET /api/v1/agent/credentials/{credential_name}
Authorization: Bearer <user_auth_token>
```

Response:

```json
{
  "msg": "OK",
  "data": {
    "id": 1,
    "credential_name": "gitlab",
    "namespace_uuid": "namespace-uuid",
    "provider": "gitlab",
    "auth_type": "bearer_token",
    "description": "GitLab token for agent tasks",
    "secret_backend": "postgres_encrypted",
    "metadata": {
      "base_url": "https://git-devops.opencsg.com/"
    },
    "status": "active",
    "created_at": "2026-04-15T00:00:00Z",
    "updated_at": "2026-04-15T00:00:00Z"
  }
}
```

### Update Credential

Only `description` and `metadata` are mutable. `credential_name`, `provider`, `auth_type`, and secret material are immutable. Use rotate to replace secret material.

```http
PATCH /api/v1/agent/credentials/{credential_name}
Authorization: Bearer <user_auth_token>
Content-Type: application/json
```

Request:

```json
{
  "description": "Updated GitLab token for agent tasks",
  "metadata": {
    "base_url": "https://git-devops.opencsg.com/"
  }
}
```

`metadata` supports partial updates. Only the keys in the request are changed.

- A metadata key with a non-`null` value adds or updates that key.
- A metadata key with `null` deletes that key.
- Keys omitted from the request are preserved.
- Updated metadata must still be valid for the credential provider and auth type after the merge.

Partial metadata update example:

```json
{
  "metadata": {
    "base_url": "https://git-devops.opencsg.com/",
    "timeout_secs": 30
  }
}
```

Metadata deletion example:

```json
{
  "metadata": {
    "timeout_secs": null
  }
}
```

Response:

```json
{
  "msg": "OK",
  "data": {
    "id": 1,
    "credential_name": "gitlab",
    "namespace_uuid": "namespace-uuid",
    "provider": "gitlab",
    "auth_type": "bearer_token",
    "description": "Updated GitLab token for agent tasks",
    "secret_backend": "postgres_encrypted",
    "metadata": {
      "base_url": "https://git-devops.opencsg.com/"
    },
    "status": "active",
    "created_at": "2026-04-15T00:00:00Z",
    "updated_at": "2026-04-15T00:05:00Z"
  }
}
```

### Rotate Credential

```http
POST /api/v1/agent/credentials/{credential_name}/rotate
Authorization: Bearer <user_auth_token>
Content-Type: application/json
```

Request:

```json
{
  "credential": {
    "token": "replace_with_new_provider_token"
  }
}
```

For `bearer_token`, `api_key`, and `custom_header`, rotate replaces the whole secret value.

For `static_secret`, `credential` supports partial updates.

- A key with a non-empty value adds or updates that key.
- A key with an empty string value deletes that key.
- Keys omitted from the request are preserved.
- After merge and deletion, the final `static_secret` map must still be non-empty.

Partial `static_secret` update example:

```json
{
  "credential": {
    "access_key_secret": "replace_with_new_access_key_secret"
  }
}
```

`static_secret` deletion example:

```json
{
  "credential": {
    "security_token": ""
  }
}
```

Rotation is supported for both `postgres_encrypted` and `hashicorp_vault_kv2` credentials.

### Revoke Credential

```http
POST /api/v1/agent/credentials/{credential_name}/revoke
Authorization: Bearer <user_auth_token>
```

Marks the credential as `revoked`.

### Delete Credential

```http
DELETE /api/v1/agent/credentials/{credential_name}
Authorization: Bearer <user_auth_token>
```

Deletes the credential record and its internal postgres-encrypted secret when applicable.

## Session Grant APIs

A session grant authorizes an agent runtime session to use one or more credential names. The response includes one runtime credential token for the session.

### Create Session Grants

```http
POST /api/v1/agent/credentials/sessions/{session_id}/grants
Authorization: Bearer <user_auth_token>
Content-Type: application/json
```

Request:

```json
{
  "agent_id": "email-agent",
  "task_id": "task-credential-demo-001",
  "credential_names": ["gitlab"],
  "duration_secs": 900
}
```

Fields:

| Field | Required | Notes |
| --- | --- | --- |
| `agent_id` | Yes | Agent that will use the runtime credential token. |
| `task_id` | No | Metadata for audit and traceability. |
| `credential_names` | Yes | Credential names owned by the current user. |
| `duration_secs` | No | Defaults to `900`. Maximum defaults to `3600` and can be configured with `OPENCSG_CREDENTIAL_RUNTIME_SESSION_MAX_DURATION_SECONDS`. |

Response `data`:

```json
{
  "grants": [
    {
      "id": 4,
      "task_id": "task-credential-demo-001",
      "session_id": "session-credential-demo-001",
      "agent_id": "email-agent",
      "credential_name": "gitlab",
      "expires_at": "2026-04-15T00:15:00Z",
      "created_at": "2026-04-15T00:00:00Z"
    }
  ],
  "runtime_credential_token": "eyJhbGciOiJIUzI1NiIs...",
  "token_type": "Bearer",
  "expires_at": "2026-04-15T00:15:00Z"
}
```

The runtime credential token is an HS256 JWT signed by `OPENCSG_CREDENTIAL_RUNTIME_SESSION_SIGNING_KEY`. It includes:

```json
{
  "namespace_uuid": "namespace-uuid",
  "task_id": "task-credential-demo-001",
  "session_id": "session-credential-demo-001",
  "agent_id": "email-agent",
  "exp": 1770000000
}
```

### List Session Grants

```http
GET /api/v1/agent/credentials/sessions/{session_id}/grants
Authorization: Bearer <user_auth_token>
```

## Runtime Credential APIs

Runtime endpoints resolve credential material for an already granted runtime session. They do not use user auth. They use the runtime credential token returned by create session grants.

Missing, invalid, or expired runtime credential tokens return `401` with code `AGENT-ERR-11`.

### Get Runtime Credential

```http
GET /api/v1/agent/credentials/runtime/{credential_name}
Authorization: Bearer <runtime_credential_token>
```

Status codes and errors:

| HTTP Status | Error Code | Meaning |
| --- | --- | --- |
| `200` | - | Runtime credential material returned successfully. |
| `401` | `AGENT-ERR-11` | Runtime credential token is missing, invalid, or expired. |
| `403` | `AGENT-ERR-12` | Runtime credential token is valid, but the requested credential is not granted, revoked, expired, or unavailable. |
| `500` | varies | Internal server error. |

Example `401` response:

```json
{
  "code": "AGENT-ERR-11",
  "msg": "AGENT-ERR-11: Runtime credential token is invalid or expired"
}
```

Example `403` response:

```json
{
  "code": "AGENT-ERR-12",
  "msg": "AGENT-ERR-12: Runtime credential grant is unavailable"
}
```

Response `data` for GitLab:

```json
{
  "credential_name": "gitlab",
  "provider": "gitlab",
  "auth_type": "bearer_token",
  "credential": {
    "Authorization": "Bearer glpat_xxx"
  },
  "metadata": {
    "base_url": "https://git-devops.opencsg.com/"
  },
  "expires_at": "2026-04-15T00:15:00Z"
}
```

Response `data` for generic custom header:

```json
{
  "credential_name": "internal-api",
  "provider": "generic",
  "auth_type": "custom_header",
  "credential": {
    "X-API-Key": "secret_xxx"
  },
  "metadata": {
    "base_url": "https://internal.example.com",
    "header_name": "X-API-Key"
  },
  "expires_at": "2026-04-15T00:15:00Z"
}
```

Response `data` for generic static key/value secret:

```json
{
  "credential_name": "aliyun-oss",
  "provider": "generic",
  "auth_type": "static_secret",
  "credential": {
    "access_key_id": "LTAI_xxx",
    "access_key_secret": "secret_xxx"
  },
  "metadata": {
    "base_url": "https://oss-cn-hangzhou.aliyuncs.com"
  },
  "expires_at": "2026-04-15T00:15:00Z"
}
```

Runtime callers should use the `credential` map directly. For HTTP credentials, each key/value pair is a request header. For static credentials, each key/value pair is provider-specific secret material.

### Revoke Runtime Session

```http
POST /api/v1/agent/credentials/runtime/session/revoke
Authorization: Bearer <runtime_credential_token>
```

Expires all unexpired grants for the runtime session in the token.

## End-To-End Flow

### 1. User Saves GitLab Credential

```http
POST /api/v1/agent/credentials
Authorization: Bearer <user_auth_token>
Content-Type: application/json
```

```json
{
  "credential_name": "gitlab",
  "provider": "gitlab",
  "auth_type": "bearer_token",
  "description": "GitLab token for agent tasks",
  "credential": {
    "token": "test"
  },
  "metadata": {
    "base_url": "https://git-devops.opencsg.com/"
  }
}
```

### 2. User Grants Credential To Agent Session

```http
POST /api/v1/agent/credentials/sessions/session-credential-demo-001/grants
Authorization: Bearer <user_auth_token>
Content-Type: application/json
```

```json
{
  "agent_id": "email-agent",
  "task_id": "task-credential-demo-001",
  "credential_names": ["gitlab"],
  "duration_secs": 900
}
```

Save `data.runtime_credential_token` from the response.

### 3. Agent Runtime Resolves Credential

```http
GET /api/v1/agent/credentials/runtime/gitlab
Authorization: Bearer <runtime_credential_token>
```

The agent receives a normalized `credential` map and non-secret `metadata` for the granted credential.

## Route Summary

| Method | Path | Auth |
| --- | --- | --- |
| `POST` | `/api/v1/agent/credentials` | User token |
| `GET` | `/api/v1/agent/credentials?per=&page=` | User token |
| `GET` | `/api/v1/agent/credentials/providers` | User token |
| `GET` | `/api/v1/agent/credentials/{credential_name}` | User token |
| `PATCH` | `/api/v1/agent/credentials/{credential_name}` | User token |
| `POST` | `/api/v1/agent/credentials/{credential_name}/rotate` | User token |
| `POST` | `/api/v1/agent/credentials/{credential_name}/revoke` | User token |
| `DELETE` | `/api/v1/agent/credentials/{credential_name}` | User token |
| `POST` | `/api/v1/agent/credentials/sessions/{session_id}/grants` | User token |
| `GET` | `/api/v1/agent/credentials/sessions/{session_id}/grants` | User token |
| `GET` | `/api/v1/agent/credentials/runtime/{credential_name}` | Runtime credential token |
| `POST` | `/api/v1/agent/credentials/runtime/session/revoke` | Runtime credential token |

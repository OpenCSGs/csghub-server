# CSGHub Memory REST API

This document defines the CSGHub Memory REST API used by `csghub-server`.

## Conventions

- Base path: `/api/v1/memory`
- Requests and responses are JSON.
- Response envelope:
  - `msg`: string, `"OK"` on success.
  - `data`: object or list, actual payload.
  - `error`: optional object for failures.

Success envelope example:

```json
{
  "msg": "OK",
  "data": {}
}
```

Error envelope example:

```json
{
  "msg": "ERROR",
  "error": {
    "code": "SYS-ERR-1",
    "message": "remote service call failed",
    "details": {}
  }
}
```

## Common Types

### MemoryMessage

```json
{
  "uid": "string",
  "content": "string",
  "timestamp": "RFC3339 timestamp",
  "role": "user|assistant|system",
  "scopes": {
    "agent_id": "string",
    "org_id": "string",
    "project_id": "string",
    "session_id": "string"
  },
  "user_id": "string",
  "meta_data": {
    "key": "value"
  },
  "similarity": 0.72
}
```

Notes:

- `content` is mandatory for each message.
- `uid` is returned by the service; it may be omitted in requests.
- `scopes` may be omitted in requests (see endpoint notes).
- `meta_data` is a free-form key/value map.
- `similarity` is optional and only returned on search results.
- `omitEmpty`: properties with no meaningful value (e.g. `""` or `null`) are omitted from responses.

### MemoryType

Allowed values: `"episodic"`, `"semantic"`.

## Endpoints

### Health

`GET /api/v1/memory/health`

Response `data`:

```json
{
  "status": "healthy",
  "service": "memmachine"
}
```

### Project Create

`POST /api/v1/memory/projects`

Request:

```json
{
  "org_id": "string",
  "project_id": "string",
  "description": "string"
}
```

Response `data`:

```json
{
  "org_id": "string",
  "project_id": "string",
  "description": "string"
}
```

### Project Get

`POST /api/v1/memory/projects/get`

Request:

```json
{
  "org_id": "string",
  "project_id": "string"
}
```

Response `data`: same as project create response.

### Project List

`POST /api/v1/memory/projects/list`

Request:

```json
{}
```

Response `data`:

```json
[
  {
    "org_id": "string",
    "project_id": "string"
  }
]
```

### Project Delete

`POST /api/v1/memory/projects/delete`

Request:

```json
{
  "org_id": "string",
  "project_id": "string"
}
```

Response `data`:

```json
{
  "deleted": true
}
```

Notes:

- `org_id` and `project_id` are required; defaults are not applied.

### Memories Add

`POST /api/v1/memory/memories`

Request:

```json
{
  "agent_id": "string",
  "session_id": "string",
  "org_id": "string",
  "project_id": "string",
  "types": ["episodic", "semantic"],
  "messages": [
    {
      "content": "string",
      "role": "user",
      "user_id": "string",
      "timestamp": "RFC3339 timestamp",
      "meta_data": {
        "key": "value"
      }
    }
  ]
}
```

Notes:

- All messages in a request share the same scope fields (`agent_id`, `session_id`, `org_id`, `project_id`).
- `types` is optional; if omitted or empty, each message is added to both episodic and semantic memories.

Response `data`:

```json
{
  "created": [
    {
      "uid": "string",
      "content": "string",
      "timestamp": "RFC3339 timestamp",
      "role": "string",
      "scopes": {
        "agent_id": "string",
        "org_id": "string",
        "project_id": "string",
        "session_id": "string"
      },
      "user_id": "string",
      "meta_data": {
        "key": "value"
      }
    }
  ]
}
```

### Memories Search

`POST /api/v1/memory/memories/search`

Request:

```json
{
  "agent_id": "string",
  "org_id": "string",
  "project_id": "string",
  "session_id": "string",
  "content_query": "string",
  "user_id": "string",
  "role": "user|assistant|system",
  "top_k": 5,
  "page_size": 10,
  "page_num": 1,
  "filter": "metadata.prop_1=123",
  "min_similarity": 0.5,
  "types": ["episodic", "semantic"]
}
```

Notes:

- `agent_id`, `org_id`, `project_id`, `session_id`, `user_id`, and `role` are optional filters. If omitted, they default to no restriction.
- Scope-related defaulting and interpretation may vary by backend implementation.
- `min_similarity` filters results to those with similarity greater than or equal to the value.
- `types` is optional; if omitted or empty, results include all types.

Response `data`:

```json
{
  "status": 0,
  "content": [
    {
      "uid": "string",
      "content": "string",
      "timestamp": "RFC3339 timestamp",
      "role": "string",
      "scopes": {
        "agent_id": "string",
        "org_id": "string",
        "project_id": "string",
        "session_id": "string"
      },
      "user_id": "string",
      "meta_data": {
        "key": "value"
      },
      "similarity": 0.72
    }
  ]
}
```

### Memories List

`POST /api/v1/memory/memories/list`

Request:

```json
{
  "agent_id": "string",
  "org_id": "string",
  "project_id": "string",
  "session_id": "string",
  "user_id": "string",
  "role": "user|assistant|system",
  "types": ["episodic", "semantic"],
  "page_size": 10,
  "page_num": 1
}
```

Notes:

- `agent_id`, `org_id`, `project_id`, `session_id`, `user_id`, and `role` are optional filters. If omitted, they default to no restriction.
- Scope-related defaulting and interpretation may vary by backend implementation.
- `types` is optional; if omitted or empty, results include all types.

Response `data`:

```json
{
  "status": 0,
  "content": [
    {
      "uid": "string",
      "content": "string",
      "timestamp": "RFC3339 timestamp",
      "role": "string",
      "scopes": {
        "agent_id": "string",
        "org_id": "string",
        "project_id": "string",
        "session_id": "string"
      },
      "user_id": "string",
      "meta_data": {
        "key": "value"
      }
    }
  ]
}
```

### Memories Delete

`POST /api/v1/memory/memories/delete`

Request:

```json
{
  "agent_id": "string",
  "org_id": "string",
  "project_id": "string",
  "session_id": "string",
  "uid": "string",
  "uids": ["string"]
}
```

Response `data`:

```json
{
  "deleted": true
}
```

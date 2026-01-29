# CSGHub Memory REST API

本文档定义了 `csghub-server` 使用的 Memory REST API。

## 约定

- 基础路径：`/api/v1/memory`
- 请求与响应均为 JSON。
- 响应封装格式：
  - `msg`: 字符串，成功时为 `"OK"`。
  - `data`: 对象或列表，实际数据。
  - `error`: 失败时的错误对象（可选）。

成功响应示例：

```json
{
  "msg": "OK",
  "data": {}
}
```

错误响应示例：

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

## 通用类型

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

说明：

- `content` 为必填。
- `uid` 由服务端返回，请求中可省略。
- `scopes` 可在请求中省略（参见各接口说明）。
- `meta_data` 为自由键值对。
- `similarity` 为可选字段，仅搜索结果返回。
- `omitEmpty`：响应中省略无意义的值（如 `""` 或 `null`）。

### MemoryType

允许的值：`"episodic"`、`"semantic"`。

## 接口

### Health

`GET /api/v1/memory/health`

响应 `data`：

```json
{
  "status": "healthy",
  "service": "memmachine"
}
```

### Project Create

`POST /api/v1/memory/projects`

请求：

```json
{
  "org_id": "string",
  "project_id": "string",
  "description": "string"
}
```

响应 `data`：

```json
{
  "org_id": "string",
  "project_id": "string",
  "description": "string"
}
```

### Project Get

`POST /api/v1/memory/projects/get`

请求：

```json
{
  "org_id": "string",
  "project_id": "string"
}
```

响应 `data`：同 Project Create。

### Project List

`POST /api/v1/memory/projects/list`

请求：

```json
{}
```

响应 `data`：

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

请求：

```json
{
  "org_id": "string",
  "project_id": "string"
}
```

响应 `data`：

```json
{
  "deleted": true
}
```

说明：

- `org_id` 和 `project_id` 必填，不使用默认值。

### Memories Add

`POST /api/v1/memory/memories`

请求：

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

说明：

- 同一请求内的所有消息共享同一组作用域字段（`agent_id`、`session_id`、`org_id`、`project_id`）。
- `types` 可选；为空或省略时，默认同时写入 episodic 和 semantic。

响应 `data`：

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

请求：

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

说明：

- `agent_id`、`org_id`、`project_id`、`session_id`、`user_id`、`role` 为可选过滤条件；省略则不限制。
- 与 scope 相关的默认值与解释行为可能因后端实现而异。
- `min_similarity` 表示相似度下限。
- `types` 可选；为空或省略时，包含所有类型。

响应 `data`：

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

请求：

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

说明：

- `agent_id`、`org_id`、`project_id`、`session_id`、`user_id`、`role` 为可选过滤条件；省略则不限制。
- 与 scope 相关的默认值与解释行为可能因后端实现而异。
- `types` 可选；为空或省略时，包含所有类型。

响应 `data`：

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

请求：

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

响应 `data`：

```json
{
  "deleted": true
}
```

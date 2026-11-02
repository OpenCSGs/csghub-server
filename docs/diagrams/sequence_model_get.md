# 查询模型流程

```mermaid
sequenceDiagram
    actor User
    participant API as API Handler
    participant Component as Model Component
    participant Repo as Repo Component
    participant DB as Database
    participant Git as Git Server
    participant Deploy as Deployer

    %% 获取模型列表流程
    User->>API: 获取模型列表请求
    API->>API: 获取当前用户
    API->>API: 解析查询参数(标签、分页等)
    API->>API: 验证请求参数
    API->>Component: Index(filter, per, page, needOpWeight)
    Component->>DB: 查询符合条件的模型
    DB-->>Component: 模型列表和总数
    Component-->>API: 模型列表和总数
    API-->>User: 返回模型列表响应

    %% 获取单个模型详情流程
    User->>API: 获取模型详情请求
    API->>API: 获取当前用户
    API->>API: 获取命名空间和名称
    API->>API: 解析查询参数
    API->>Component: Show(namespace, name, currentUser, needOpWeight, needMultiSync)
    Component->>DB: 查询模型信息
    DB-->>Component: 模型基本信息
    Component->>DB: 查询模型关联信息
    DB-->>Component: 模型关联信息
    Component->>Repo: 获取仓库信息
    Repo-->>Component: 仓库信息
    Component-->>API: 完整模型详情
    API-->>User: 返回模型详情响应
```

## 获取单个模型详情API

### 接口定义

Endpoint:  
`GET /api/v1/models/{namespace}/{name}`

#### 请求头
| 参数名 | 类型 | 必填 | 描述 |
|--------|------|------|------|
| Authorization | string | 是 | Bearer token格式: `Bearer <access_token>` |

#### 查询参数
| 参数名 | 类型 | 必填 | 描述 |
|--------|------|------|------|
| need_op_weight | boolean | 否 | 是否需要操作权重，默认false |
| need_multi_sync | boolean | 否 | 是否需要多端同步信息，默认false |

#### 路径参数
| 参数名 | 类型 | 必填 | 描述 |
|--------|------|------|------|
| namespace | string | 是 | 模型所属命名空间 |
| name | string | 是 | 模型名称 |

### 响应参数
成功响应 (200):
```json
{
  "id": "string",
  "name": "string",
  "namespace": "string",
  "description": "string",
  "license": "string",
  "private": boolean,
  "created_at": "string",
  "updated_at": "string",
  "default_branch": "string",
  "tags": [
    {
      "name": "string",
      "category": "string"
    }
  ],
  "repo_info": {
    "id": "string",
    "name": "string",
    "path": "string"
  },
  "related_models": [
    {
      "id": "string",
      "name": "string",
      "namespace": "string"
    }
  ]
}
```

错误响应 (404):
```json
{
  "error": "string",
  "message": "string"
}
```

### 调用示例

```bash
curl -X GET "https://api.example.com/api/v1/models/user123/my-llm-model" \
  -H "Authorization: Bearer your_access_token" \
  -H "Content-Type: application/json"
```

成功响应示例:
```json
{
  "id": "model_123",
  "name": "my-llm-model",
  "namespace": "user123",
  "description": "My first language model",
  "license": "apache-2.0",
  "private": false,
  "created_at": "2025-04-19T13:17:22Z",
  "updated_at": "2025-04-19T13:17:22Z",
  "default_branch": "main",
  "tags": [
    {
      "name": "llama",
      "category": "base_model"
    }
  ],
  "repo_info": {
    "id": "repo_456",
    "name": "my-llm-model",
    "path": "user123/my-llm-model"
  },
  "related_models": [
    {
      "id": "model_789",
      "name": "base-llama",
      "namespace": "meta"
    }
  ]
}
```
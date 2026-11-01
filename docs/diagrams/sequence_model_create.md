# 创建模型API

```mermaid
sequenceDiagram
    actor User
    participant API as API Handler
    participant Component as Model Component
    participant Repo as Repo Component
    participant DB as Database
    participant Git as Git Server
    participant Deploy as Deployer

    %% 创建模型流程
    User->>API: 创建模型请求
    API->>API: 获取当前用户
    API->>API: 验证请求参数
    API->>Component: 敏感内容检查
    Component-->>API: 检查结果
    API->>Component: Create(req)
    Component->>DB: 查询用户信息
    DB-->>Component: 用户信息
    Component->>Repo: CreateRepo(req)
    Repo->>DB: 创建仓库记录
    DB-->>Repo: 仓库记录
    Repo-->>Component: 仓库信息
    Component->>DB: 创建模型记录
    DB-->>Component: 模型记录
    Component->>Git: 创建README.md文件
    Git-->>Component: 创建结果
    Component->>Git: 创建.gitattributes文件
    Git-->>Component: 创建结果
    Component-->>API: 模型信息
    API-->>User: 创建成功响应
```

## 接口定义

Endpoint:  
`POST /api/v1/models`

### 请求头
| 参数名 | 类型 | 必填 | 描述 |
|--------|------|------|------|
| Authorization | string | 是 | Bearer token格式: `Bearer <access_token>` |
| Content-Type | string | 是 | 必须为`application/json` |

### 请求参数
```json
{
  "name": "string",
  "namespace": "string",
  "description": "string",
  "license": "string",
  "base_model": "string",
  "private": boolean,
  "default_branch": "string",
  "readme": "string",
  "nickname": "string"
}
```

字段说明：
- `name`: 模型名称 (必填)
- `namespace`: 命名空间/用户名 (必填)  
- `description`: 模型描述
- `license`: 许可证类型
- `base_model`: 基础模型名称
- `private`: 是否私有
- `default_branch`: 默认分支名
- `readme`: README内容
- `nickname`: 模型昵称

### 响应参数
成功响应 (200):
```json
{
  "id": "string",
  "name": "string",
  "path": "string",
  "namespace": "string",
  "description": "string",
  "created_at": "string",
  "updated_at": "string",
  "private": boolean,
  "default_branch": "string"
}
```

错误响应 (400):
```json
{
  "error": "string",
  "message": "string"
}
```

## 调用示例

```bash
curl -X POST "https://api.example.com/api/v1/models" \
  -H "Authorization: Bearer your_access_token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-llm-model",
    "namespace": "user123",
    "description": "My first language model",
    "license": "apache-2.0",
    "base_model": "llama-2",
    "private": false,
    "default_branch": "main",
    "readme": "# My Model\nThis is my first model",
    "nickname": "My LLM"
  }'
```

成功响应示例:
```json
{
  "id": "model_123",
  "name": "my-llm-model",
  "path": "user123/my-llm-model",
  "namespace": "user123",
  "description": "My first language model",
  "created_at": "2025-04-19T13:17:22Z",
  "updated_at": "2025-04-19T13:17:22Z",
  "private": false,
  "default_branch": "main"
}
```
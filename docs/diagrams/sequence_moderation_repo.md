# Repository Content Moderation Workflow

```mermaid
sequenceDiagram
    participant Workflow as Push Workflow
    participant Activity as SensitiveCheck Activity
    participant ModClient as Moderation Client
    participant ModAPI as Moderation API
    participant Component as Repo Component
    participant Git as Git Repository
    
    %% Workflow initialization
    Workflow->>Activity: Execute SensitiveCheck(pushReq)
    
    %% Repository content check
    Activity->>ModClient: SubmitRepoCheck(repoType, namespace, name)
    ModClient->>ModAPI: POST /api/v1/repo
    ModAPI->>Component: Full Check Request
    Component->>Git: Fetch repository content
    Git-->>Component: Repository files
    
    %% File content moderation loop
    loop For each file
        Component->>ModClient: PassTextCheck("repo_content", fileContent)
        ModClient->>ModAPI: POST /api/v1/text
        ModAPI-->>ModClient: CheckResult
        ModClient-->>Component: Moderation result
        alt Content is sensitive
            Component-->Activity: Report sensitive content
            Activity-->Workflow: Return error
        end
    end
    
    %% Completion handling
    Component-->>ModAPI: Check completion
    ModAPI-->>ModClient: Success response
    ModClient-->>Activity: Check completed
    Activity-->>Workflow: Return success
```

## API 文档

### POST /api/v1/repo

提交仓库内容审核请求

**请求方法**: POST

**请求路径**: /api/v1/repo

**请求参数**:
```json
{
  "repo_type": "model|dataset|space|codes",  // 仓库类型枚举
  "namespace": "string",                     // 命名空间
  "name": "string"                           // 仓库名称
}
```

**请求示例**:
```bash
curl -X POST \
  http://localhost/api/v1/repo \
  -H 'Content-Type: application/json' \
  -d '{
    "repo_type": "model",
    "namespace": "example",
    "name": "test-repo"
  }'
```

**响应**:
- 成功: 200 OK (无响应体)
- 错误: 返回相应错误码和消息

**错误码**:
- 400: 请求参数错误
- 500: 服务器内部错误
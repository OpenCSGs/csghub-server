## 模型推理Chat接口流程

```mermaid
sequenceDiagram
    actor User
    participant API as API Handler
    participant Component as Model Component
    participant Moderation as Moderation Service
    participant Proxy as Reverse Proxy
    participant Model as Model Endpoint
    participant DB as Database

    %% 发起Chat请求
    User->>API: POST /v1/chat/completions
    API->>API: 解析请求体
    API->>API: 获取当前用户
    
    %% 模型验证流程
    API->>Component: 根据model_id获取模型
    Component->>DB: 查询模型信息
    DB-->>Component: 模型详情
    alt 模型不存在或未运行
        Component-->>API: 返回错误
        API-->>User: 404 Not Found
    else 模型可用
        Component-->>API: 返回模型端点
        
        %% 敏感内容检查
        loop 每条消息
            API->>Moderation: 检查消息内容
            Moderation-->>API: 检查结果
            alt 敏感内容
                API-->>User: 返回敏感内容错误
            end
        end
        
        %% 代理请求
        API->>Proxy: 转发请求到模型端点
        Proxy->>Model: 发送Chat请求
        Model-->>Proxy: 返回响应
        Proxy-->>API: 返回结果
        
        %% 记录使用量
        API->>Component: 记录token使用量
        Component->>DB: 更新使用记录
        
        %% 返回响应
        API-->>User: 返回Chat响应
    end
```

## Chat API 接口文档

### 接口定义

Endpoint:  
`POST /v1/chat/completions`

#### 请求头
| 参数名 | 类型 | 必填 | 描述 |
|--------|------|------|------|
| Authorization | string | 是 | Bearer token格式: `Bearer <access_token>` |
| Content-Type | string | 是 | 必须为`application/json` |

#### 请求体 (ChatCompletionRequest)
```json
{
  "model": "string",
  "messages": [
    {
      "role": "string",
      "content": "string"
    }
  ],
  "temperature": number,
  "top_p": number,
  "n": integer,
  "stream": boolean,
  "stop": ["string"],
  "max_tokens": integer,
  "presence_penalty": number,
  "frequency_penalty": number,
  "logit_bias": object
}
```

#### 响应参数 (ChatCompletionResponse)
```json
{
  "id": "string",
  "object": "string",
  "created": integer,
  "model": "string",
  "choices": [
    {
      "index": integer,
      "message": {
        "role": "string",
        "content": "string"
      },
      "finish_reason": "string"
    }
  ],
  "usage": {
    "prompt_tokens": integer,
    "completion_tokens": integer,
    "total_tokens": integer
  }
}
```

### 调用示例

```bash
curl -X POST "https://api.example.com/v1/chat/completions" \
  -H "Authorization: Bearer your_access_token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {
        "role": "user",
        "content": "Hello!"
      }
    ],
    "temperature": 0.7,
    "max_tokens": 100
  }'
```

### 响应示例

成功响应 (200):
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "gpt-3.5-turbo",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "Hello there! How can I help you today?"
    },
    "finish_reason": "stop"
  }],
  "usage": {
    "prompt_tokens": 9,
    "completion_tokens": 12,
    "total_tokens": 21
  }
}
```

错误响应 (400):
```json
{
  "error": "invalid_request_error",
  "message": "The model does not exist"
}
```
# 评估任务完整调用流程时序图

```mermaid
sequenceDiagram
    participant Client as 客户端
    participant EvalHandler as EvaluationHandler
    participant Sensitive as SensitiveComponent
    participant EvalComp as EvaluationComponent
    participant Deployer as Deployer
    participant Scheduler as FIFOScheduler
    participant RemoteRunner as RemoteRunner
    participant RunnerService as RunnerService

    Client->>EvalHandler: POST /evaluations
    EvalHandler->>EvalHandler: 获取当前用户
    alt 用户未登录
        EvalHandler-->>Client: 返回401未授权
    else
        EvalHandler->>EvalHandler: 绑定请求参数(EvaluationReq)
        EvalHandler->>Sensitive: 检查敏感信息(CheckRequestV2)
        alt 敏感信息检查失败
            Sensitive-->>EvalHandler: 返回错误
            EvalHandler-->>Client: 返回400错误
        else
            EvalHandler->>EvalComp: 创建评估任务(CreateEvaluation)
            EvalComp->>Deployer: 提交部署请求
            Deployer->>Scheduler: 添加部署任务
            Scheduler->>RemoteRunner: 触发任务执行
            RemoteRunner->>RunnerService: RPC调用(RunService)
            RunnerService->>RunnerService: 处理部署逻辑
            alt 部署失败
                RunnerService-->>RemoteRunner: 返回错误
                RemoteRunner-->>Scheduler: 标记任务失败
                Scheduler-->>Deployer: 通知失败
                Deployer-->>EvalComp: 返回错误
                EvalComp-->>EvalHandler: 返回错误
                EvalHandler-->>Client: 返回500服务器错误
            else
                RunnerService-->>RemoteRunner: 返回成功
                RemoteRunner-->>Scheduler: 标记任务完成
                Scheduler-->>Deployer: 通知完成
                Deployer-->>EvalComp: 返回deployID
                EvalComp-->>EvalHandler: 返回评估结果
                EvalHandler-->>Client: 返回200成功
            end
        end
    end
```

# FinetuneCreate 接口文档

## 接口说明
创建微调实例

## 请求方法
POST

## 请求路径
/models/{namespace}/{name}/finetune

## 请求参数

### Path 参数
| 参数名 | 类型 | 必填 | 描述 |
|-------|------|------|------|
| namespace | string | 是 | 命名空间 |
| name | string | 是 | 模型名称 |

### Query 参数
| 参数名 | 类型 | 必填 | 描述 |
|-------|------|------|------|
| current_user | string | 是 | 当前用户 |

### Body 参数 (application/json)
```json
{
  "deploy_name": "string",
  "cluster_id": "string",
  "resource_id": 0,
  "runtime_framework_id": 0,
  "revision": "string",
  "order_detail_id": 0,
  "engine_args": "string"
}
```

字段说明：
- deploy_name: 部署名称
- cluster_id: 集群ID
- resource_id: 资源ID
- runtime_framework_id: 运行时框架ID
- revision: 版本/分支
- order_detail_id: 订单详情ID
- engine_args: 引擎参数

## 响应

### 成功响应 (200)
```json
{
  "deploy_id": 0
}
```

### 错误响应
- 400: 请求参数错误
- 401: 未授权
- 403: 禁止访问
- 500: 服务器内部错误
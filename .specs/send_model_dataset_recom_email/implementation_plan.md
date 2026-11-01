# 实现方案：支持群发模型数据集推荐邮件

## 1. 概述

根据需求，需要修改 Notification 服务的消息发送接口，支持群发模型数据集推荐的邮件。前端传递运营手动筛选的模型和数据集 ID，后台根据用户标签补充推荐一个模型和一个数据集。

## 2. 技术方案

### 2.1 新增场景定义

在 `common/types/notification_scenario.go` 中添加新的场景常量：

```go
// 模型数据集推荐场景
// @Scenario model-dataset-recommend
// @Channels email
// @PayloadFields model_ids, dataset_ids
// @Description 群发模型数据集推荐邮件
MessageScenarioModelDatasetRecommend MessageScenario = "model-dataset-recommend"
```

### 2.2 新增数据结构

在 `common/types/notification.go` 中添加相关的数据结构：

```go
// 模型数据集推荐请求
type ModelDatasetRecommendRequest struct {
	ModelIDs   []string `json:"model_ids" binding:"required"`
	DatasetIDs []string `json:"dataset_ids" binding:"required"`
}

// 模型数据集推荐响应
type ModelDatasetRecommendResponse struct {
	MsgUUID string `json:"msg_uuid"`
}
```

### 2.3 创建场景实现

在 `notification/scenariomgr/scenario/` 目录下创建新的场景实现：

#### 2.3.1 创建目录结构

```
notification/scenariomgr/scenario/modeldatasetrecommend/
├── email.go
└── internal.go
```

#### 2.3.2 实现邮件数据获取函数

在 `email.go` 中实现：

```go
package modeldatasetrecommend

import (
	"context"
	"encoding/json"
	"log/slog"

	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/notification/scenariomgr"
)

// GetEmailData 获取模型数据集推荐邮件数据
func GetEmailData(ctx context.Context, scenarioMsg types.ScenarioMessage) (map[types.MessageChannel][]scenariomgr.ChannelData, error) {
	var req types.ModelDatasetRecommendRequest
	if err := json.Unmarshal([]byte(scenarioMsg.Parameters), &req); err != nil {
		slog.Error("Failed to unmarshal model dataset recommend request", slog.Any("error", err))
		return nil, err
	}

	// 实现获取邮件数据的逻辑
	// 1. 获取所有用户
	// 2. 对每个用户，根据标签推荐模型和数据集
	// 3. 生成邮件内容

	// 这里需要调用相关服务获取用户数据、模型数据和数据集数据

	return map[types.MessageChannel][]scenariomgr.ChannelData{
		types.MessageChannelEmail: {
			// 实现邮件数据生成
		},
	}, nil
}
```

### 2.4 注册新场景

在 `notification/scenarioregister/register.go` 中注册新场景：

```go
// 注册模型数据集推荐场景
scenariomgr.RegisterScenario(types.MessageScenarioModelDatasetRecommend, &scenariomgr.ScenarioDefinition{
	Channels: []types.MessageChannel{
		types.MessageChannelEmail,
	},
	ChannelGetDataFunc: map[types.MessageChannel]scenariomgr.GetDataFunc{
		types.MessageChannelEmail: modeldatasetrecommend.GetEmailData,
	},
})
```

### 2.5 实现异步处理

利用现有的 NATS 队列机制，确保大量用户的情况下也能异步处理：

1. 消息发送到 NATS 队列
2. 消息处理器异步消费
3. 批量处理用户数据

### 2.6 标签匹配算法

实现标签匹配算法，根据用户标签推荐模型和数据集：

1. 获取用户的标签
2. 获取模型和数据集的标签
3. 计算标签相似度
4. 选择相似度最高的模型和数据集

## 3. 接口修改

### 3.1 请求参数

前端调用 `/api/v1/notifications` [POST] 接口时，需要传递以下参数：

```json
{
	"scenario": "model-dataset-recommend",
	"parameters": {
		"model_ids": ["model1", "model2"],
		"dataset_ids": ["dataset1", "dataset2"]
	},
	"priority": "normal"
}
```

### 3.2 响应参数

接口返回：

```json
{
	"code": 200,
	"message": "OK",
	"data": {
		"msg_uuid": "uuid-string"
	}
}
```

## 4. 实现步骤

1. **添加场景常量**：在 `common/types/notification_scenario.go` 中添加新的场景常量
2. **添加数据结构**：在 `common/types/notification.go` 中添加相关的数据结构
3. **创建场景实现**：创建新的场景实现目录和文件
4. **注册场景**：在 `notification/scenarioregister/register.go` 中注册新场景
5. **实现标签匹配**：实现标签匹配算法，用于推荐模型和数据集
6. **测试**：编写测试用例，确保功能正常

## 5. 注意事项

1. **异步处理**：确保大量用户的情况下使用异步处理，避免阻塞接口调用
2. **标签匹配**：实现高效的标签匹配算法，确保推荐质量
3. **错误处理**：完善错误处理机制，确保系统稳定性
4. **性能优化**：对大量用户的情况进行性能优化，如批量处理、并发处理等
5. **测试覆盖**：编写充分的测试用例，确保功能正常

## 6. 技术依赖

1. **NATS**：用于消息队列，实现异步处理
2. **Redis**：用于缓存，提高性能
3. **数据库**：用于存储用户数据、模型数据和数据集数据
4. **标签系统**：用于用户标签和模型/数据集标签的管理

## 7. 预期效果

1. 前端可以通过调用 `/api/v1/notifications` [POST] 接口，传递模型和数据集 ID
2. 后台异步处理大量用户的邮件发送
3. 每个用户收到包含推荐模型和数据集的邮件
4. 系统能够根据用户标签智能推荐相关内容

## 8. 实施计划

| 步骤 | 任务 | 负责人 | 时间估计 |
|------|------|--------|----------|
| 1 | 添加场景常量 | 开发工程师 | 0.5 天 |
| 2 | 添加数据结构 | 开发工程师 | 0.5 天 |
| 3 | 创建场景实现 | 开发工程师 | 1 天 |
| 4 | 注册场景 | 开发工程师 | 0.5 天 |
| 5 | 实现标签匹配 | 开发工程师 | 1 天 |
| 6 | 测试 | 测试工程师 | 1 天 |
| 7 | 部署 | 运维工程师 | 0.5 天 |

总计：5 天

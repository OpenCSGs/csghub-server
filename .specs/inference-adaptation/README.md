# 适配一种新的推理能力

本文说明如何把一种新的模型任务（pipeline task）接到平台：从 **模型标签** → **部署 / Docker** → **configs 运行时框架** → **AIGateway API**。

以 **text-ranking（文本排序 / Rerank）** 为完整示例。其它任务（TTS、embedding、ASR 等）路径相同，只是 API 形态与引擎参数不同。

---

## 1. 整体链路

```text
模型仓库打 task tag (text-ranking)
        │
        ▼
Deploy: resolveDeployTask (task tag) → deploy.Task
        │
        ▼
K8s Pod 注入 HF_TASK=text-ranking
        │
        ▼
推理镜像启动脚本按 HF_TASK 选择引擎参数
  (如 vllm --runner pooling)
        │
        ▼
引擎暴露下游 API (如 POST /rerank)
        │
        ▼
AIGateway 对外暴露 OpenAI/Jina 兼容路由
  POST /v1/rerank → 反代到部署实例 → 计量
```

关键组件：

| 层 | 职责 | 典型路径 |
|---|---|---|
| 模型 / Tag | 声明任务类型，驱动 `HF_TASK` | `common/types/repo.go`, migrations, `seeds/tags.yml` |
| Deploy | 把 tag 写成 `deploy.Task`，注入环境变量 | `component/model.go`, `api/workflow/activity/deploy_activity.go` |
| configs | 声明引擎镜像、支持的 `architectures`、engine args | `configs/inference/*.json` |
| Docker | 镜像 + 启动脚本，按 `HF_TASK` 分支 | `docker/inference/` |
| AIGateway | 对外 API、鉴权、反代、计费 | `aigateway/router`, `aigateway/handler` |

---

## 2. 适配清单（按顺序做）

1. **定义 PipelineTask 常量**（若是全新任务）
2. **注册 built-in tag**（migration + seeds + i18n）。若该任务是**派生任务**、不对应仓库 tag（例如 `typed-decisions`），跳过本步
3. **任务映射**：在 `resolveDeployTask` 里加 tag 判据，需要时再改 `GetPipelineTaskFromTags`
4. **configs**：在目标引擎的 `extra_archs` / `supported_archs` 中加入模型 architecture；需要按引擎版本区分的启动参数放进 `engine_images[].extra_engine_args`（见 3.3）
5. **Docker**：必要时改启动脚本，按 `HF_TASK` 设置引擎参数；专用引擎则新建 Dockerfile + config
6. **AIGateway**：路由 + handler + request/response + usage 计量
7. **单测** + 本地/联调验证

---

## 3. 以 text-ranking 为例

### 3.1 对模型的要求

部署 / 路由能否选中正确引擎，取决于模型仓库上的 **tag** 和 **config.json 里的 architecture**。

#### Task tag（必须）

模型仓库需要有 category=`task` 的标签：

| 字段 | 值 |
|---|---|
| name | `text-ranking` |
| category | `task` |
| scope | `model` |
| group | `natural_language_processing`（示例） |

平台用该 tag 推导任务：

```go
// component/model.go — Deploy 时
task := resolveDeployTask(m.Repository.Tags)
// ...
dp.Task = task
```

`resolveDeployTask` 先看派生判据（例如 decision 类 tag），未命中再走内置 tag 映射。派生任务
（如 `typed-decisions`）只在这里产生 `deploy.Task`，不需要
注册成仓库 tag；`HF_TASK` 会跟着变成派生值，启动脚本需能识别它。

随后在 deploy activity 注入：

```go
// api/workflow/activity/deploy_activity.go
envMap["HF_TASK"] = string(deployInfo.Task) // → text-ranking
```

**没有正确的 task tag → `HF_TASK` 不对 → 启动脚本走错分支 → API 可能不可用。**

#### Architecture（必须，用于匹配 Runtime Framework）

模型 `config.json` 的 `architectures` 必须被某个推理引擎的 config 声明支持，例如：

- `Qwen3ForSequenceClassification`
- `XLMRobertaForSequenceClassification`
- `JinaVLForRanking`
- …

平台扫描模型后，按 architecture 关联可用的 runtime framework（`vllm` / `tei` / `llama.cpp` 等）。  
架构未登记 → 前端/API 选不到对应引擎。

#### 特殊模型注意（Qwen3-Reranker）

部分模型上游标成 `Qwen3ForCausalLM`，实际要用 sequence classification。  
vLLM 启动脚本会在 `HF_TASK=text-ranking` 时自动加 `--hf-overrides` 与 chat template（见 `docker/inference/vllm/single-node.sh`）。

---

### 3.2 注册 PipelineTask 与 Tag

#### 常量

```go
// common/types/repo.go
TextRanking PipelineTask = "text-ranking"
```

#### Migration（存量库）

参考：`builder/store/database/migrations/20260710023953_add_text_ranking_tag.go`

```go
tag := Tag{
    Name:     "text-ranking",
    Category: "task",
    Group:    "natural_language_processing",
    Scope:    "model",
    ShowName: "文本排序",
    BuiltIn:  true,
}
```

用生成器创建 migration，不要手写文件名：

```bash
go run cmd/csghub-server/main.go migration create_go add_text_ranking_tag
```

#### Seeds（新环境）

`builder/store/database/migrations/seeds/tags.yml`：

```yaml
- category: task
  group: natural_language_processing
  name: text-ranking
  scope: model
  show_name: 文本排序
  built_in: true
```

#### i18n

`common/i18n/{en-US,zh-CN,zh-HK}/tags.json` 增加 `Tag.I18nKey.text-ranking`。

#### 任务映射

```go
// component/model.go — resolveDeployTask（内置 tag 映射）
if tag.Name == string(types.TextRanking) {
    return tag.Name
}
```

若任务的判据不是单个内置 tag（例如多个 tag 组合），加在 `resolveDeployTask`，并在
`component/model_task_test.go` 覆盖该组合。

Git callback 侧若也要识别该任务，同步改 `GetPipelineTaskFromTags`（`component/callback/git_callback.go`）。  
并补单测：`component/model_task_test.go`。

---

### 3.3 configs：让引擎“认识”这些模型

配置目录：`configs/inference/`。

以 vLLM 为例，把 ranking 相关 architecture 写进 `engine_images[].extra_archs`（或顶层 `supported_archs`）：

```json
// configs/inference/vllm.json（节选）
"extra_archs": [
  "Qwen3ForSequenceClassification",
  "XLMRobertaForSequenceClassification",
  "JinaVLForRanking",
  "..."
]
```

AMD / 其它变体同步改：`amd-vllm.json` 等。

TEI 若也支持部分 rerank 模型，改 `configs/inference/tei.json` 的 `supported_archs` / `supported_models`。

配置变更后，runtime framework 同步逻辑会更新 DB 中的 framework ↔ architecture 关系（见 `component/runtime_architecture.go`）。部署环境需要加载新配置并触发同步。

#### engine_args 与 extra_engine_args

同一引擎的多个镜像默认共用一份参数模板：

- 顶层 `engine_args`：该引擎**所有镜像版本都认**的启动参数
- `engine_images[].extra_engine_args`：只有**这个镜像版本**支持的参数

同步时两者按 `name` 合并（`component/runtime_architecture.go` 的 `mergeEngineArgs`）：同名时镜像级的替换共享级并保持原位置，新名字追加在共享参数之后（同一个 `name` 以镜像级为准，配置里不要写重名）。合并结果写进 `runtime_frameworks.engine_args`，部署时按 `frame_image` 读回并渲染成 `ENGINE_ARGS`，所以前端和部署链路都不需要感知 `extra_engine_args`。

什么时候用它：某个 flag 只在部分版本存在。传了不认识的 flag，引擎会在启动时直接退出，所以共享列表里只能放"所有版本都认"的参数。例如 `vllm.json` 的 `async-scheduling` 只进了 v0.24.0 / v0.31.0 镜像，`swap-space` 和 `guided-decoding-backend` 只进了 v0.8.5 / v0.9.2 镜像。

```json
// configs/inference/vllm.json（节选）
"engine_args": [
  { "name": "max-model-len", "value": "8192", "format": "--max-model-len %s" }
],
"engine_images": [
  {
    "image": "opencsghq/vllm:v0.31.0",
    "engine_version": "v0.31.0",
    "extra_engine_args": [
      { "name": "async-scheduling", "value": "enable", "format": "--async-scheduling" }
    ]
  }
]
```

写参数时注意几点：

- `format` 含 `%s` 表示带值参数，渲染成 `--flag <value>`；不含 `%s` 表示布尔开关，按 `format` 原样输出
- `value` 是**表单默认值，不是强制值**。`setEngineArgs` 只在部署请求里出现该 key 时才渲染，且请求值等于模板默认值时会被跳过。所以"平台必须生效的默认行为"不能只写在 `value` 里，要写进启动脚本（`docker/inference/<engine>/*.sh`）或 `custom-options`
- 带值参数不要留空 `value`：请求提交空字符串时会渲染出没有值的 flag，引擎启动即失败（`TestEngineConfigValueTakingArgsHaveDefaults` 会拦住这种配置）
- 值里不能有空格：启动脚本是不加引号展开 `$ENGINE_ARGS` 的，带空格的值会被拆成多个参数
- `custom-options` 的 `format` 是裸 `%s`，用来透传任意参数，是"临时逃生口"

改完参数跑 `go test ./component/`：`TestEngineConfigEngineArgsPerVersion` 会按配置文件逐个校验"声明的版本是否真的支持这些 flag"，`TestEngineConfigValueTakingArgsHaveDefaults` 校验带值参数是否有默认值。

---

### 3.4 Docker：按 `HF_TASK` 启动正确模式

#### 复用现有引擎（text-ranking 的做法）

多数情况不必新镜像，只需在启动脚本里按任务分支：

```bash
# docker/inference/vllm/single-node.sh
if [ "$HF_TASK" == "text-ranking" ]; then
    # Qwen3-Reranker 特殊 overrides（如需要）
    # ...
    if [[ ! $ENGINE_ARGS == *"--runner"* ]]; then
        ENGINE_ARGS="$ENGINE_ARGS --runner pooling"
    fi
fi
```

llama.cpp：

```bash
# docker/inference/llama.cpp/serve.sh
if [[ "$HF_TASK" == "text-ranking" ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --reranking"
fi
```

辅助文件（如 `docker/inference/vllm/qwen3_reranker.jinja`）需打进镜像，并在 Dockerfile / COPY 中保证路径与脚本一致。

#### 新建专用引擎（对比：AudioFly / LongCat）

若上游无法用 vLLM/TEI 直接服务，需要：

1. `docker/inference/Dockerfile.<engine>`
2. `docker/inference/<engine>/`（server / entry / serve.sh）
3. `configs/inference/<engine>.json`（及 `amd-*.json`）
4. `docker/inference/build.sh` + `README.md` 构建说明

专用服务应尽量暴露与 AIGateway 约定一致的路径（OpenAI / Jina 兼容），减少网关适配成本。

---

### 3.5 AIGateway：对外 API

text-ranking 对外是 Jina 兼容的 rerank：

| 项目 | 值 |
|---|---|
| 路由 | `POST /v1/rerank` |
| 注册 | `aigateway/router/aigateway.go` |
| Handler | `aigateway/handler/rerank.go` |
| Request | `RerankRequest`（`aigateway/handler/requests.go`） |
| 计量 | `response_writer_wrapper_rerank.go` |

典型 handler 流程（可套用到其它任务）：

1. 绑定并校验请求（`model` / `query` / `documents`）
2. `resolveModelTarget` 解析用户可见 model id → 部署地址
3. `CheckBalance`
4. 改写 body 中的 `model` 为上游真实名
5. Reverse proxy 到下游（默认 `/rerank`；TEI 无 `/v1/rerank`）
6. 包装 ResponseWriter，解析 usage 并 `RecordUsage`

注册示例：

```go
v1Group.POST("/rerank", middlewareCollection.Auth.MustUserOrgApiKey, openAIhandler.Rerank)
```

请求体示例：

```json
{
  "model": "user/my-reranker",
  "query": "什么是向量检索？",
  "documents": ["文档 A", "文档 B"],
  "top_n": 2
}
```

---

## 4. 端到端验收（text-ranking）

1. **Tag**：模型页能看到 / 能打上 `text-ranking`
2. **Framework**：模型详情可选 `vllm`（或 tei / llama.cpp），且 architecture 已匹配
3. **Deploy**：Pod 环境变量 `HF_TASK=text-ranking`
4. **引擎**：容器日志出现 pooling / rerank 相关参数；`/rerank` 或 `/v1/rerank` 可直接打通
5. **Gateway**：

```bash
curl -s "$AIGATEWAY/v1/rerank" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "<deployed-model-id>",
    "query": "hello",
    "documents": ["hello world", "goodbye"]
  }'
```

6. **计量**：accounting / usage 有对应记录

---

## 5. 新任务适配时怎么套用

把下表中的「text-ranking」换成你的任务名即可：

| 步骤 | text-ranking 落点 | 你要改的 |
|---|---|---|
| Task 常量 | `types.TextRanking` | 新 `PipelineTask` |
| Tag | migration + `tags.yml` + i18n | 同结构新 tag；派生任务（如 `typed-decisions`）跳过 |
| 任务映射 | `resolveDeployTask` | 在 `resolveDeployTask` 增加 tag 判据 |
| 引擎匹配 | `vllm.json` `extra_archs` | 登记 architecture；或新建 engine config |
| 启动参数 | `single-node.sh` 的 `HF_TASK` 分支 | 新分支或专用 serve 脚本 |
| 网关 API | `POST /v1/rerank` | 新路由 + handler + usage wrapper |
| 模型侧 | 打 `text-ranking` tag | 打对应 task tag |

### 两种常见模式

**A. 复用现有引擎（本例）**  
同一镜像服务多种任务，靠 `HF_TASK` + architecture 区分。适合 vLLM pooling、Omni TTS、embedding 等。

**B. 专用引擎**  
新 Dockerfile + 独立 OpenAI 兼容 HTTP 服务。适合 AudioFly、LongCat Video 等上游无法直接挂到 vLLM 的模型。

---

## 6. 常见坑

1. **只加了 Gateway，没打 tag**  
   部署 `HF_TASK` 仍是空/别的任务，引擎不会开 rerank。

2. **只加了 tag，没登记 architecture**  
   模型关联不到 runtime framework，无法选引擎部署。

3. **只加了 `fix_*` migration，没有 `add_*` migration**  
   新环境 seeds 有 tag，旧库可能没有；fix 脚本会空更新。要保证 add/seeds/fix 一致。

4. **下游路径不一致**  
   vLLM/llama.cpp 与 TEI 路径可能不同（`/rerank` vs `/v1/rerank`）。Gateway 默认路径要兼容最差情况。

5. **Cherry-pick 半套代码**  
   例如只带了 `TextRanking` 常量，没有 `rerank.go` / 启动脚本 / add-tag migration，编译能过但功能不可用。

6. **专用镜像未写进 config `engine_images`**  
   框架列表里看不到新引擎，或版本对不上。

7. **把只有部分版本支持的 flag 放进共享 `engine_args`**

   引擎收到不认识的 flag 会启动即退出，共享列表必须所有镜像版本都认；按版本差异的放 `engine_images[].extra_engine_args`（见 3.3）。

8. **以为 `engine_args` 里的 `value` 会生效**

   它只是表单默认值，请求里不带这个 key（或带的值和默认值相同）就不会传给引擎。要"默认开"就得写进启动脚本。

---

## 7. 相关代码索引（text-ranking）

| 用途 | 路径 |
|---|---|
| Task 常量 | `common/types/repo.go` |
| Tag migration | `builder/store/database/migrations/20260710023953_add_text_ranking_tag.go` |
| Tag seeds | `builder/store/database/migrations/seeds/tags.yml` |
| Deploy 任务推导 | `component/model.go` → `resolveDeployTask` |
| `HF_TASK` 注入 | `api/workflow/activity/deploy_activity.go` |
| vLLM 配置 | `configs/inference/vllm.json` |
| vLLM 启动 | `docker/inference/vllm/single-node.sh` |
| Qwen3 template | `docker/inference/vllm/qwen3_reranker.jinja` |
| Gateway 路由 | `aigateway/router/aigateway.go` |
| Rerank handler | `aigateway/handler/rerank.go` |
| Request 类型 | `aigateway/handler/requests.go` → `RerankRequest` |
| 计量包装 | `aigateway/handler/response_writer_wrapper_rerank.go` |

镜像构建与运行示例见：`docker/inference/README.md`。

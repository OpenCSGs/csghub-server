# AudioFly 推理镜像

[iFLYTEK AudioFly](https://modelscope.cn/models/iflytek/AudioFly) 是科大讯飞开源的文本生音频（text-to-audio）模型，基于 Latent Diffusion 架构（10 亿参数），根据文本描述生成 44.1 kHz 高保真音效/环境音。

模型仓库自带推理代码（`ldm/` 包）、`config/config.yaml`、`flan-t5-large` 文本编码器和 `models/ldm/model.ckpt` 权重，无法由 vLLM-Omni 等通用引擎承载，因此使用本专用镜像。镜像内置 FastAPI 服务，对外暴露与 vLLM-Omni 一致的 OpenAI 兼容 Speech API，CSGHub aigateway 的 `/v1/audio/speech` 链路可直接代理，无需任何网关改动。

## 工作方式

1. 启动时 `entry.py` 通过 `csghub-sdk` 将模型仓库下载到 `/workspace/$REPO_ID`
2. `serve.sh` 切换到模型仓库根目录（推理代码依赖相对路径），将其加入 `PYTHONPATH`
3. `server.py` 加载模型常驻显存，并在 8000 端口提供 API

## API 端点

### POST /v1/audio/speech

生成音频，返回 `audio/wav` 二进制。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `input` | string | 是 | - | 音频的文本描述（英文效果最佳），如 `Fierce winds howl through the valley` |
| `response_format` | string | 否 | wav | 仅支持 `wav` |
| `cfg` | number | 否 | 3.5 | Guidance scale，控制生成与文本的贴合程度（官方不建议修改） |
| `ddim_steps` | integer | 否 | 200 | 扩散去噪步数，1-1000（官方不建议修改） |
| `model` / `voice` / `speed` | - | 否 | - | 为兼容统一接口而接受，实际忽略（模型无音色/语速概念） |
| `stream` | boolean | 否 | false | 不支持流式，传 `true` 返回 400 |

```bash
curl -X POST http://127.0.0.1:8000/v1/audio/speech \
  -H "Content-Type: application/json" \
  -d '{"input": "Fierce winds howl through the valley"}' \
  --output output.wav
```

### POST /v1/audio/speech/batch

批量生成，`items` 为数组，每项与单次请求参数相同（顶层字段作为各项默认值）。返回 JSON，音频为 base64，结构与 vLLM-Omni batch 响应一致；`usage` 按输入字符数填充，供网关计量。

```bash
curl -X POST http://127.0.0.1:8000/v1/audio/speech/batch \
  -H "Content-Type: application/json" \
  -d '{"items": [{"input": "heavy rain on a tin roof"}, {"input": "a dog barking in the distance"}]}'
```

### GET /v1/audio/voices

返回空音色列表（模型无音色预设）；`POST`/`PUT`/`DELETE` 音色管理返回 405。

### GET /health

探活端点，模型加载完成后返回 `{"status": "ok"}`。

## 本地运行

```bash
docker run -d \
  -e ACCESS_TOKEN=xxx \
  -e REPO_ID="iflytek/AudioFly" \
  -e HF_ENDPOINT=https://hub.opencsg.com \
  --gpus device=0 \
  -p 8000:8000 \
  opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/audiofly:1.0
```

AMD GPU 使用 ROCm 镜像，并将 render/video 组设备映射进容器：

```bash
docker run -d \
  -e ACCESS_TOKEN=xxx \
  -e REPO_ID="iflytek/AudioFly" \
  -e HF_ENDPOINT=https://hub.opencsg.com \
  --device=/dev/kfd \
  --device=/dev/dri \
  --group-add video \
  --group-add render \
  -p 8000:8000 \
  opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/audiofly-rocm:1.0
```

## 注意事项

- 支持 NVIDIA CUDA 12.1 和 AMD ROCm 7.2.2 GPU（扩散采样计算量大，CPU 无实用性）；同一时刻只处理一个生成请求（内部串行化，避免显存溢出）
- 默认 `ddim_steps=200`，单次生成耗时较长（数秒到数十秒），只适合非流式调用
- NVIDIA 和 AMD 引擎配置分别见 `configs/inference/audiofly.json`、`configs/inference/amd-audiofly.json`，按模型名 `AudioFly` 匹配运行时框架；模型仓库需打 `text-to-audio` pipeline 标签，API 仍复用平台的 `/v1/audio/speech` 链路

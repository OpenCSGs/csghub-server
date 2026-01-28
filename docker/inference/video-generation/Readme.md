# LightX2V 视频生成 API 使用指南

基于 [LightX2V](https://github.com/ModelTC/LightX2V) 项目的视频生成 API 使用说明。

## API 端点概览

- `POST /v1/tasks/video` - 提交视频生成任务（JSON 方式）
- `POST /v1/tasks/video/form` - 提交视频生成任务（表单上传方式）
- `GET /v1/tasks/{task_id}/status` - 查询任务状态
- `GET /v1/files/download/{file_path}` - 下载生成结果文件

## 使用流程

### 1. 提交任务

#### 方式一：JSON 提交（推荐）

**文生视频（T2V）示例：**

```bash
POST /v1/tasks/video
Content-Type: application/json

{
  "prompt": "A white cat sitting by the beach under sunset sky",
  "negative_prompt": "blurry, low quality, static",
  "num_fragments": 1,
  "infer_steps": 4,
  "video_duration": 5,
  "seed": 42,
  "height": 480,
  "width": 832
}
```

**图生视频（I2V）示例：**

```bash
POST /v1/tasks/video
Content-Type: application/json

{
  "prompt": "A white cat sitting by the beach under sunset sky",
  "negative_prompt": "blurry, low quality, static",
  "image_path": "/path/to/input_image.jpg",
  "num_fragments": 1,
  "infer_steps": 4,
  "video_duration": 5,
  "seed": 42,
  "height": 480,
  "width": 832
}
```

**响应示例：**

```json
{
  "task_id": "abc123",
  "status": "submitted",
  "message": "Task created successfully"
}
```

#### 方式二：表单上传（适用于需要上传图片的场景）

```bash
POST /v1/tasks/video/form
Content-Type: multipart/form-data

Fields:
  prompt: "A white cat sitting by the beach"
  negative_prompt: "blurry, low quality"
  image_file: <binary file>  # 图生视频时上传图片文件
  infer_steps: 4
  video_duration: 5
  seed: 42
  height: 480
  width: 832
```

### 2. 查询任务状态

使用步骤 1 返回的 `task_id` 轮询任务状态：

```bash
GET /v1/tasks/{task_id}/status
```

**响应示例：**

```json
{
  "task_id": "abc123",
  "status": "running",  # 可能的值: submitted, running, succeed, failed
  "progress": 0.5,      # 0.0 - 1.0
  "message": "Processing..."
}
```

**状态说明：**
- `submitted` - 任务已提交，等待处理
- `running` - 任务正在执行中
- `succeed` - 任务成功完成
- `failed` - 任务执行失败

### 3. 获取结果

当任务状态变为 `succeed` 后，可以通过以下方式获取生成的视频：

#### 方式一：通过任务结果接口（如果支持）

```bash
GET /v1/tasks/{task_id}/result
```

#### 方式二：通过文件下载接口

如果知道生成文件的路径（通常在状态响应中提供），使用下载接口：

```bash
GET /v1/files/download/{file_path}
```

**示例：**

```bash
GET /v1/files/download/outputs/videos/abc123.mp4
```

**响应：** 返回视频文件流（MP4 格式）

## 完整使用示例

### 文生视频（T2V）完整流程

```bash
# 1. 提交任务
curl -X POST http://115.190.78.40:30155/v1/tasks/video \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "A beautiful sunset over the ocean",
    "negative_prompt": "blurry, low quality",
    "infer_steps": 4,
    "video_duration": 5,
    "seed": 42
  }'

# 响应: {"task_id": "abc123", "status": "submitted"}

# 2. 轮询状态（每 10-30 秒查询一次）
curl http://115.190.78.40:30155/v1/tasks/abc123/status

# 3. 当状态为 "succeed" 时，下载结果
curl http://115.190.78.40:30155/v1/files/download/outputs/videos/abc123.mp4 \
  -o output.mp4
```

### 图生视频（I2V）完整流程

```bash
# 方式一：使用 JSON 提交（图片已在服务器上）
curl -X POST http://115.190.78.40:30155/v1/tasks/video \
  -H "Content-Type: application/json" \
  -d '{
    "prompt": "A cat walking on the beach",
    "image_path": "/path/to/input_image.jpg",
    "infer_steps": 4,
    "video_duration": 5,
    "seed": 42
  }'

# 方式二：使用表单上传图片
curl -X POST http://115.190.78.40:30155/v1/tasks/video/form \
  -F "prompt=A cat walking on the beach" \
  -F "image_file=@/local/path/to/image.jpg" \
  -F "infer_steps=4" \
  -F "video_duration=5" \
  -F "seed=42"

# 后续步骤同文生视频
```

## 主要参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `prompt` | string | 是 | 文本提示词，描述要生成的视频内容 |
| `negative_prompt` | string | 否 | 负面提示词，描述不希望出现的内容 |
| `image_path` | string | I2V必填 | 输入图片路径（图生视频时） |
| `infer_steps` | int | 否 | 推理步数，默认 4（使用蒸馏模型） |
| `video_duration` | int | 否 | 视频时长（秒），默认 5 |
| `seed` | int | 否 | 随机种子，用于复现结果 |
| `height` | int | 否 | 视频高度，默认 480 |
| `width` | int | 否 | 视频宽度，默认 832 |
| `num_fragments` | int | 否 | 片段数量，默认 1 |

## 注意事项

1. **任务状态轮询**：建议每 10-30 秒查询一次任务状态，避免过于频繁的请求
2. **文件路径**：下载文件时，`file_path` 需要是服务器上的绝对路径或相对路径
3. **超时处理**：视频生成可能需要较长时间，建议设置合理的超时时间
4. **错误处理**：当状态为 `failed` 时，检查响应中的 `message` 字段获取错误信息
5. **并发限制**：注意 API 的并发请求限制，避免同时提交过多任务

## 参考文档

- [LightX2V GitHub](https://github.com/ModelTC/LightX2V)
- API 文档地址: http://115.190.78.40:30155/docs

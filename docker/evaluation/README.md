# LLM evalution docker image

## Login Container Registry

```bash
OPENCSG_ACR="opencsg-registry.cn-beijing.cr.aliyuncs.com"
OPENCSG_ACR_USERNAME=""
OPENCSG_ACR_PASSWORD=""
echo "$OPENCSG_ACR_PASSWORD" | docker login $OPENCSG_ACR -u $OPENCSG_ACR_USERNAME --password-stdin
```

## Build Multi-Platform Images

```bash
#opencsg-registry.cn-beijing.cr.aliyuncs.com/public/opencompass:0.4.2
export BUILDX_NO_DEFAULT_ATTESTATIONS=1
export IMAGE_TAG=0.4.2
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/public/opencompass:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/public/opencompass:latest \
  -f Dockerfile.opencompass \
  --push .
#opencsg-registry.cn-beijing.cr.aliyuncs.com/public/lm-evaluation-harness:0.4.9
export IMAGE_TAG=0.4.9
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/public/lm-evaluation-harness:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/public/lm-evaluation-harness:latest \
  -f Dockerfile.lm-evaluation-harness \
  --push .

#opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/evalscope:1.4.2-cu124
export IMAGE_TAG=1.4.2-cu124
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/evalscope:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/evalscope:latest \
  -f Dockerfile.evalscope-gpu \
  --push .
#opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/evalscope:1.4.2-cpu
export IMAGE_TAG=1.4.2-cpu
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/evalscope:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/evalscope:latest \
  -f Dockerfile.evalscope-cpu \
  --push .

# opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/claw-eval:1.0.0
export IMAGE_TAG=1.0.0
docker build \
  --build-arg HTTP_PROXY="http://host.docker.internal:7890" \
  --build-arg HTTPS_PROXY="http://host.docker.internal:7890" \
  --build-arg CLAW_EVAL_REPO="https://github.com/claw-eval/claw-eval.git" \
  --build-arg CLAW_EVAL_REF="main" \
  -t ${OPENCSG_ACR}/opencsghq/claw-eval:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/claw-eval:latest \
  -f Dockerfile.claw-eval \
  .
```

_The above command will create `linux/amd64` and `linux/arm64` images with the tags `${IMAGE_TAG}` and `latest` at the same time._

## Test the opencompass Image

```bash
docker run \
  -e ACCESS_TOKEN=xxxx  \
  -e MODEL_ID="OpenCSG/csg-wukong-1B" \
  -e DATASET_IDS="xzgan/hellaswag" \
  -e HF_ENDPOINT=https://hub.opencsg.com \
  -e ASCEND_VISIBLE_DEVICES=7 \
  -e S3_ACCESS_ID="xxxx" \
  -e S3_ACCESS_SECRET="xxxx" \
  -e S3_BUCKET="xxxxx" \
  -e S3_ENDPOINT="xxxxx" \
  -e S3_SSL_ENABLED="true" \
  ${OPENCSG_ACR}/public/opencompass:${IMAGE_TAG}
```

## Test the lm-evaluation-harness Image

```bash
export IMAGE_TAG=0.4.6
docker run \
  --gpus device=1 \
  -e ACCESS_TOKEN=xxxx  \
  -e MODEL_ID="OpenCSG/csg-wukong-1B" \
  -e DATASET_IDS="Rowan/hellaswag" \
  -e HF_ENDPOINT=https://hub.opencsg.com\
  -e S3_ACCESS_ID="xxx" \
  -e S3_ACCESS_SECRET="xxx" \
  -e S3_BUCKET="xxx" \
  -e S3_ENDPOINT="xxx" \
  -e S3_SSL_ENABLED="true" \
  ${OPENCSG_ACR}/public/lm-evaluation-harness:${IMAGE_TAG}
```

## inference image name, version and cuda version

| Latest Image          | Version | CUDA Version |
| --------------------- | ------- | ------------ |
| opencompass           | 0.4.2   | 12.1         |
| lm-evaluation-harness | 0.4.9   | 12.1         |
| claw-eval             | 1.0.0   | N/A (CPU)    |

## Test the claw-eval Image

Claw-eval currently does not support passing multiple discrete task IDs directly. Task selection is controlled via `CLAW_EVAL_TASKS`:

| Value | Description |
| ----- | ----------- |
| `all` | Run all 300 tasks (not recommended on current platform) |
| `normal` | **Recommended.** Run the 129 platform-runnable tasks (excludes web search, multimodal, multi-turn, sandbox-dependent tasks) |
| `general` | Run tasks tagged `general` |
| `multimodal` | Run tasks tagged `multimodal` |
| `multi_turn` | Run tasks tagged `multi_turn` |
| `1-9` | Run tasks in numeric ID range |
| `T009` | Filter match by task id/name |

If `CLAW_EVAL_JUDGE_MODEL` / API `judge_model` is not set, the container defaults to `qwen3.7-max`. Judge traffic uses platform AIGateway credentials injected as `CLAW_EVAL_JUDGE_BASE_URL` and `CLAW_EVAL_JUDGE_API_KEY` (not user-provided). Set `CLAW_EVAL_NO_JUDGE=true` only when you explicitly want to skip grading.

Example:

```bash
export IMAGE_TAG=1.0.0
docker run --rm \
  -e S3_ACCESS_ID="xxxx" \
  -e S3_ACCESS_SECRET="xxxx" \
  -e S3_BUCKET="opencsg-public-resource" \
  -e S3_ENDPOINT="oss-cn-beijing.aliyuncs.com" \
  -e S3_SSL_ENABLED="true" \
  -e CLAW_EVAL_MODEL="glm-5.1" \
  -e CLAW_EVAL_BASE_URL="http://host.docker.internal:11435/v1" \
  -e CLAW_EVAL_API_KEY="sk-local-test" \
  -e CLAW_EVAL_TASKS="1-9" \
  ${OPENCSG_ACR}/opencsghq/claw-eval:${IMAGE_TAG}
```

After the job finishes, `batch_summary.json` and `batch_results.json` under `traces/<model>_<timestamp>/` are uploaded to OSS.

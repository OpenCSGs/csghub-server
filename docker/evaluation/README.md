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

# CSGHUB ChatUI Base Images Building

## Login Container Registry
```bash
OPENCSG_ACR="opencsg-registry.cn-beijing.cr.aliyuncs.com"
OPENCSG_ACR_USERNAME=""
OPENCSG_ACR_PASSWORD=""
echo "$OPENCSG_ACR_PASSWORD" | docker login $OPENCSG_ACR -u $OPENCSG_ACR_USERNAME --password-stdin
```

## Build Multi-Platform Images
```bash
export BUILDX_NO_DEFAULT_ATTESTATIONS=1
export IMAGE_TAG=1.0
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/opencsghq/vllm-cpu-chatui-base:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/vllm-cpu-chatui-base:latest \
  -f Dockerfile.vllm-cpu.base \
  --push .
```
*The above command will create `linux/amd64` and `linux/arm64` images with the tags `${IMAGE_TAG}` and `latest` at the same time.*


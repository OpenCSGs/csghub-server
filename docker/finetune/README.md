# CSGHUB Finetune Images Building

## Base Images
- https://docs.nvidia.com/deeplearning/frameworks/support-matrix/index.html
- https://catalog.ngc.nvidia.com/orgs/nvidia/containers/pytorch/tags

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
export IMAGE_TAG=1.21-cuda12.1-devel-ubuntu22.04-py310-torch2.1.2
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/public/llama-factory:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/public/llama-factory:latest \
  -f Dockerfile.llamafactory \
  --push .
```
*Note: The above command will create `linux/amd64` and `linux/arm64` images with the tags `${IMAGE_TAG}` and `latest` at the same time.*

## Run Finetune Image Locally
```bash
docker run -d \
  -e ACCESS_TOKEN=xxx \
  -e REPO_ID="OpenCSG/csg-wukong-1B" \
  -e HF_ENDPOINT=https://opencsg.com/hf \
  -p 8000:8000 \
  ${OPENCSG_ACR}/public/llama-factory:${IMAGE_TAG}
```
*Note: HF_ENDPOINT should be use the real csghub address.*



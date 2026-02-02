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
#opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/llama-factory:0.9.4
export BUILDX_NO_DEFAULT_ATTESTATIONS=1
export IMAGE_TAG=0.9.4
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/opencsghq/llama-factory:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/llama-factory:latest \
  -f Dockerfile.llamafactory \
  --push .
```

## Build arm64 for npu Images

```bash
export BUILDX_NO_DEFAULT_ATTESTATIONS=1
export IMAGE_TAG=2.1-8.0.rc2.alpha003-910b-ubuntu22.04-py3.8
docker buildx build --platform linux/arm64 \
  -t ${OPENCSG_ACR}/opencsghq/llama-factory:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/llama-factory:latest \
  -f Dockerfile.llamafactory-npu \
  --push .
```

## Build Multi-Platform Images for swift

```bash
#opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/ms-swift:v3.7.2
export BUILDX_NO_DEFAULT_ATTESTATIONS=1
export IMAGE_TAG=v3.11.1
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/opencsghq/ms-swift:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsghq/ms-swift:latest \
  -f Dockerfile.ms-swift \
  --push .
```

_Note: The above command will create `linux/amd64` and `linux/arm64` images with the tags `${IMAGE_TAG}` and `latest` at the same time._

## build gradio whl

```
1. build gradio base image or pick from opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/gradio-build-base:1.0
2. docker run -itd base_image
3. download gradio resource:  git clone https://gitee.com/xzgan/gradio.git --branch 5.32.1 --single-branch
4. build frontend js: bash scripts/build_frontend.sh
5. build whl: python3 -m build -w
6. check whl file in dist folder and upload to https://git-devops.opencsg.com/opensource/gradio/
```

## fintune image name, version and cuda version

| Image Name    | Version | CUDA Version | Fix |
| ------------- | ------- | ------------ | --- |
| llama-factory | 0.9.3   | 12.1         | -   |
| ms-swift      | v3.5.2  | 12.1         | -   |

## Run Finetune Image Locally

```bash
docker run -d \
  --gpus device=7 \
  -e HF_TOKEN=xxx \
  -e REPO_ID="OpenCSG/csg-wukong-1B" \
  -e HF_ENDPOINT=https://hub.opencsg.com/hf \
  -p 30148:8000 \
  ${OPENCSG_ACR}/opencsghq/llama-factory:${IMAGE_TAG}

docker run -d \
  --gpus device=5 \
  -e HF_TOKEN=xxx \
  -e REPO_ID="OpenCSG/csg-wukong-1B" \
  -e HF_ENDPOINT=https://hub.opencsg.com/hf \
  -p 30147:8000 \
  ${OPENCSG_ACR}/opencsghq/ms-swift:${IMAGE_TAG}
```

_Note: HF_ENDPOINT should be use the real csghub address._

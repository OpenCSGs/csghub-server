# space_base_images

## 说明
这些 image 会被 space 中自动为用户应用生成的 Dockerfile 引用，针对不同类型，不同应用会有不同的 base image

生成基础镜像后，会推送到传神社区的 registry 中（当前是`registry.cn-beijing.aliyuncs.com`的`opencsghq`命名空间）

base image 命名格式如下：

`registry.cn-beijing.aliyuncs.com/opencsghq/space-base:[python_version]-[cuda_version]`。

例如：
- `registry.cn-beijing.aliyuncs.com/opencsghq/space-base:python3.10`
- `registry.cn-beijing.aliyuncs.com/opencsghq/space-base:python3.10-cuda11.8.0`

## 构建

### MacOS/Linux

```shell
# Install QEMU (Linux Only/MacOS Support Default)
docker run --privileged --rm tonistiigi/binfmt --install all

# Create builder with driver docker-container
DOCKER_CONTAINERS=$(docker buildx ls | grep docker-container)
if [[ ! -z "$DOCKER_CONTAINERS" ]]; then
BUILDER=$(echo "$DOCKER_CONTAINERS" | awk 'NR==1{gsub(/\*$/, "", $1); print $1}')
docker buildx use ${BUILDER}
else
docker buildx create --name container-builder --driver docker-container --use --bootstrap
fi

# Build base images
## Python only
export DOCKER_BUILDKIT=1
export BUILDX_NO_DEFAULT_ATTESTATIONS=1
docker buildx build \
  --provenance false \
  --platform linux/amd64,linux/arm64 \
  -f Dockerfile-python3.10-base \
  -t opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-base:python3.10-1.0.3 \
  --push .

## Python with cuda
export DOCKER_BUILDKIT=1
export BUILDX_NO_DEFAULT_ATTESTATIONS=1
docker buildx build \
  --provenance false \
  --platform linux/amd64,linux/arm64 \
  -f Dockerfile-python3.10-cuda11.8.0-base \
  -t opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-base:python3.10-cuda11.8.0-1.0.3 \
  --push .

export DOCKER_BUILDKIT=1
export BUILDX_NO_DEFAULT_ATTESTATIONS=1
docker buildx build \
  --provenance false \
  --platform linux/amd64,linux/arm64 \
  -f Dockerfile-python3.10-cuda12.1.0-base \
  -t opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-base:python3.10-cuda12.1.0-1.0.3 \
  --push .
```
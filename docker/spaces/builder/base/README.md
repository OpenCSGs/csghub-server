# space_base_images

## Description
These images will be referenced by the Dockerfile automatically generated for user applications in space. Different types and applications will have different base images.

After the base image is generated, it will be pushed to the registry of the Chuanshen community (currently the `opencsg_space` namespace of `registry.cn-beijing.aliyuncs.com`)

The naming format of the base image is as follows:

`registry.cn-beijing.aliyuncs.com/opencsg_space/space-base:[python_version]-[cuda_version]`.

For example:
- `registry.cn-beijing.aliyuncs.com/opencsg_space/space-base:python3.10`
- `registry.cn-beijing.aliyuncs.com/opencsg_space/space-base:python3.10-cuda11.8.0`

## Build

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
-t opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_space/space-base:python3.10-1.0.1 \ 
--push .

## Python with cuda
export DOCKER_BUILDKIT=1
export BUILDX_NO_DEFAULT_ATTESTATIONS=1
docker buildx build \ 
--provenance false \ 
--platform linux/amd64,linux/arm64 \ 
-f Dockerfile-python3.10-cuda11.8.0-base \ 
-t opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_space/space-base:python3.10-cuda11.8.0-1.0.1 \ 
--push .

export DOCKER_BUILDKIT=1
export BUILDX_NO_DEFAULT_ATTESTATIONS=1
docker buildx build \ 
--provenance false \ 
--platform linux/amd64,linux/arm64 \ 
-f Dockerfile-python3.10-cuda12.1.0-base \ 
-t opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_space/space-base:python3.10-cuda12.1.0-1.0.1 \ 
--push .
```

# 1.0.7
docker buildx build \
--platform linux/amd64,linux/arm64 \
--build-arg BASE_IMAGE=opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-base:python3.10-1.0.4 \
-t opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-runtime:python3.10-1.0.7 \
-f Dockerfile \
--push \
--progress=plain \
.

docker buildx build \
--platform linux/amd64,linux/arm64 \
--build-arg BASE_IMAGE=opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-base:python3.10-cuda11.8.0-1.0.4 \
-t opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-runtime:python3.10-cuda11.8.0-1.0.7 \
-f Dockerfile \
--push \
--progress=plain \
.

docker buildx build \
--platform linux/amd64,linux/arm64 \
--build-arg BASE_IMAGE=opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-base:python3.10-cuda12.1.0-1.0.4 \
-t opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-runtime:python3.10-cuda12.1.0-1.0.7 \
-f Dockerfile \
--push \
--progress=plain \
.


# 1.0.3
docker buildx build \
--platform linux/amd64 \
--build-arg BASE_IMAGE=opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-base:python3.10-1.0.3 \
-t opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-runtime:python3.10-1.0.3 \
-f Dockerfile-1.0.3 \
--push \
--progress=plain \
.

docker buildx build \
--platform linux/amd64 \
--build-arg BASE_IMAGE=opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-base:python3.10-cuda11.8.0-1.0.3 \
-t opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-runtime:python3.10-cuda11.8.0-1.0.3 \
-f Dockerfile-1.0.3 \
--push \
--progress=plain \
.

docker buildx build \
--platform linux/amd64 \
--build-arg BASE_IMAGE=opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-base:python3.10-cuda12.1.0-1.0.3 \
-t opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-runtime:python3.10-cuda12.1.0-1.0.3 \
-f Dockerfile-1.0.3 \
--push \
--progress=plain \
.
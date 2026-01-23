
# 1.0.4
docker buildx build \
--platform linux/amd64 \
--build-arg BASE_IMAGE=opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-base:python3.10-1.0.4 \
-t harbor.opencsg.com/space_stg/space-runtime:python3.10-1.0.4 \
-t harbor.opencsg.com/space_prd/space-runtime:python3.10-1.0.4 \
-f Dockerfile \
--push \
--progress=plain \
.

docker buildx build \
--platform linux/amd64 \
--build-arg BASE_IMAGE=opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-base:python3.10-cuda11.8.0-1.0.4 \
-t harbor.opencsg.com/space_stg/space-runtime:python3.10-cuda11.8.0-1.0.4 \
-t harbor.opencsg.com/space_prd/space-runtime:python3.10-cuda11.8.0-1.0.4 \
-f Dockerfile \
--push \
--progress=plain \
.

docker buildx build \
--platform linux/amd64 \
--build-arg BASE_IMAGE=opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-base:python3.10-cuda12.1.0-1.0.4 \
-t harbor.opencsg.com/space_stg/space-runtime:python3.10-cuda12.1.0-1.0.4 \
-t harbor.opencsg.com/space_prd/space-runtime:python3.10-cuda12.1.0-1.0.4 \
-f Dockerfile \
--push \
--progress=plain \
.


# 1.0.3

docker buildx build \
--platform linux/amd64 \
--build-arg BASE_IMAGE=opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-base:python3.10-1.0.3 \
-t harbor.opencsg.com/space_stg/space-runtime:python3.10-1.0.3 \
-t harbor.opencsg.com/space_prd/space-runtime:python3.10-1.0.3 \
-f Dockerfile \
--push \
--progress=plain \
.

docker buildx build \
--platform linux/amd64 \
--build-arg BASE_IMAGE=opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-base:python3.10-cuda11.8.0-1.0.3 \
-t harbor.opencsg.com/space_stg/space-runtime:python3.10-cuda11.8.0-1.0.3 \
-t harbor.opencsg.com/space_prd/space-runtime:python3.10-cuda11.8.0-1.0.3 \
-f Dockerfile \
--push \
--progress=plain \
.

docker buildx build \
--platform linux/amd64 \
--build-arg BASE_IMAGE=opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/space-base:python3.10-cuda12.1.0-1.0.3 \
-t harbor.opencsg.com/space_stg/space-runtime:python3.10-cuda12.1.0-1.0.3 \
-t harbor.opencsg.com/space_prd/space-runtime:python3.10-cuda12.1.0-1.0.3 \
-f Dockerfile \
--push \
--progress=plain \
.
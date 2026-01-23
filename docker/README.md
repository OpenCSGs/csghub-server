# CSGHUB Server Base Images Building

## Login Container Registry
```bash
OPENCSG_ACR="opencsg-registry.cn-beijing.cr.aliyuncs.com"
OPENCSG_ACR_USERNAME=""
OPENCSG_ACR_PASSWORD=""
echo "$OPENCSG_ACR_PASSWORD" | docker login $OPENCSG_ACR -u $OPENCSG_ACR_USERNAME --password-stdin
```

## Build Multi-Platform Images
### Runtime Base Image
```bash
export IMAGE_TAG=3.1
docker buildx build --provenance false --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/opencsg_public/csghub_server:base-runtime-${IMAGE_TAG} \
  -f docker/Dockerfile-base-runtime \
  --push .
```

### GOLANG Base Image
```bash
export IMAGE_TAG=1.25.5
docker buildx build --provenance false --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/opencsg_public/csghub_server:base-build-${IMAGE_TAG} \
  -f docker/Dockerfile-base-build \
  --push .
```

# CSGHUB Server Base Images Building 

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
  -t ${OPENCSG_ACR}/opencsg_public/csghub_server:base-${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/opencsg_public/csghub_server:base-latest \
  -f Dockerfile.nginx \
  --push .
```
*The above command will create `linux/amd64` and `linux/arm64` images with the tags `base-${IMAGE_TAG}` and `base-latest` at the same time.*


#!/usr/bin/env bash

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]; then
    echo "Usage: $0 <OPENCSG_ACR_USERNAME> <OPENCSG_ACR_PASSWORD> <IMAGE>"
    echo -e "IMAGE example: \n  vllm-local:2.9\n  vllm-cpu:2.5\n  tgi:2.3"
    exit 1
fi

OS=$(uname -s)
echo "Enable docker buildx with QEMU for ${OS}"
if [ "$OS" = "Darwin" ]; then
    echo "QEMU enabled default..."
elif [ "$OS" = "Linux" ]; then
    echo "Install QEMU support..."
    docker run --privileged --rm tonistiigi/binfmt --install all
else
    echo "Unknown OS: $OS"
fi

export DOCKER_BUILDKIT=1
export BUILDX_NO_DEFAULT_ATTESTATIONS=1
DOCKER_CONTAINERS=$(docker buildx ls | grep docker-container)
if [[ ! -z "$DOCKER_CONTAINERS" ]]; then
    BUILDER=$(echo "$DOCKER_CONTAINERS" | awk 'NR==1{gsub(/\*$/, "", $1); print $1}')
    docker buildx use ${BUILDER}
else
    docker buildx create --name container-builder --driver docker-container --use --bootstrap
fi

OPENCSG_ACR_USERNAME=$1
OPENCSG_ACR_PASSWORD=$2
OPENCSG_ACR=${OPENCSG_ACR:-"opencsg-registry.cn-beijing.cr.aliyuncs.com"}
OPENCSG_ACR_NAMESPACE=${OPENCSG_ACR_NAMESPACE:-"public"}
DOCKER_IMAGE_PREFIX="$OPENCSG_ACR/$OPENCSG_ACR_NAMESPACE"

echo "Logging in to OpenCSG ACR..."
echo "$OPENCSG_ACR_PASSWORD" | docker login "$OPENCSG_ACR" -u "$OPENCSG_ACR_USERNAME" --password-stdin

echo "Building images..."
export IMAGE=$3
export PLATFORMS="linux/amd64,linux/arm64"
case "${IMAGE%:*}" in
  vllm-local)
    DOCKERFILE="Dockerfile.vllm"
    ;;
  vllm-cpu)
    DOCKERFILE="Dockerfile.vllm-cpu"
    ;;
  tgi)
    PLATFORMS="linux/amd64"
    DOCKERFILE="Dockerfile.tgi"
    ;;
esac

docker buildx build --platform ${PLATFORMS} \
    -t ${DOCKER_IMAGE_PREFIX}/${IMAGE} \
    -t ${DOCKER_IMAGE_PREFIX}/${IMAGE%:*}:latest \
    -f ${DOCKERFILE} \
    --push .

echo "Done! New image pushed with tag: $NEW_TAG"

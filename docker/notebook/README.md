# CSGHub Notebook Images Building

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
```

### PyTorch CPU

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/opencsghq/notebook:ubuntu-22.04-py311-torch2.3.1 \
  -f Dockerfile.pytorch-cpu \
  --push .
```

### PyTorch CUDA 11.8

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/opencsghq/notebook:ubuntu-24.04-cuda11.8-py313-torch2.7.1 \
  -f Dockerfile.pytorch-cu118 \
  --push .
```

### PyTorch CUDA 12

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/opencsghq/notebook:ubuntu-24.04-cuda12.8-py313-torch2.8.0 \
  -f Dockerfile.pytorch-cu12 \
  --push .
```

### PyTorch AMD ROCm

```bash
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/opencsghq/notebook:rocm7.1_ubuntu22.04_py3.11_pytorch_release_2.9.1 \
  -f Dockerfile.pytorch-amd \
  --push .
```

### TensorFlow GPU

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/opencsghq/notebook:ubuntu-22.04-cuda12.3-py311-tensorflow2.20.0 \
  -f Dockerfile.tensorflow \
  --push .
```

### Unsloth

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/opencsghq/notebook:ubuntu-22.04-cuda12.6-py311-unsloth2025.8.10 \
  -f Dockerfile.unsloth \
  --push .
```

## Notebook Images

| Dockerfile | Image Tag | Framework | CUDA/ROCm | Python | OS |
|---|---|---|---|---|---|
| Dockerfile.pytorch-cpu | `ubuntu-22.04-py311-torch2.3.1` | PyTorch 2.3.1 | CPU only | 3.11 | Ubuntu 22.04 |
| Dockerfile.pytorch-cu118 | `ubuntu-24.04-cuda11.8-py313-torch2.7.1` | PyTorch 2.7.1 | CUDA 11.8 | 3.13 | Ubuntu 24.04 |
| Dockerfile.pytorch-cu12 | `ubuntu-24.04-cuda12.8-py313-torch2.8.0` | PyTorch 2.8.0 | CUDA 12.8 | 3.13 | Ubuntu 24.04 |
| Dockerfile.pytorch-amd | `rocm7.1_ubuntu22.04_py3.11_pytorch_release_2.9.1` | PyTorch 2.9.1 | ROCm 7.1 | 3.11 | Ubuntu 22.04 |
| Dockerfile.tensorflow | `ubuntu-22.04-cuda12.3-py311-tensorflow2.20.0` | TensorFlow 2.20.0 | CUDA 12.3 | 3.11 | Ubuntu 22.04 |
| Dockerfile.unsloth | `ubuntu-22.04-cuda12.6-py311-unsloth2025.8.10` | Unsloth 2025.8.10 | CUDA 12.6 | 3.11 | Ubuntu 22.04 |

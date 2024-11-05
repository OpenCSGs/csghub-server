# CSGHUB Inference Images Building

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

# For vllm: opencsg-registry.cn-beijing.cr.aliyuncs.com/public/vllm-local:2.7
export IMAGE_TAG=2.8
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/public/vllm-local:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/public/vllm-local:latest \
  -f Dockerfile.vllm \
  --push .
  
# For vllm cpu only: opencsg-registry.cn-beijing.cr.aliyuncs.com/public/vllm-cpu:2.3
export IMAGE_TAG=2.4
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/public/vllm-cpu:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/public/vllm-cpu:latest \
  -f Dockerfile.vllm-cpu \
  --push .

# For tgi: opencsg-registry.cn-beijing.cr.aliyuncs.com/public/tgi:2.2
export IMAGE_TAG=2.2
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/public/tgi:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/public/tgi:latest \
  -f Dockerfile.tgi \
  --push .
```
*Note: The above command will create `linux/amd64` and `linux/arm64` images with the tags `${IMAGE_TAG}` and `latest` at the same time.*

## Run Inference Image Locally
```bash
# Run VLLM
docker run -d \
  -e ACCESS_TOKEN=xxx \
  -e REPO_ID="xzgan001/csg-wukong-1B" \
  -e HF_ENDPOINT=https://opencsg.com/hf \
  --gpus device=1 \
  -p 8000:8000 \
  ${OPENCSG_ACR}/public/vllm-local:2.8

# Run TGI
docker run -d \
  -e ACCESS_TOKEN=xxx  \
  -e REPO_ID="xzgan001/csg-wukong-1B" \
  -e HF_ENDPOINT=https://opencsg.com/hf \
  -v llm:/data \
  --gpus device=7 \
  -p 8000:8000
  ${OPENCSG_ACR}/public/tgi:2.2
```
*Note: HF_ENDPOINT should be use the real csghub address.*

## inference image name, version and cuda version
| Image Name | Version | CUDA Version |
| --- | --- | --- |
| vllm | 2.8 | 12.1 |
| vllm | 3.0 | 12.4 |
| vllm-cpu | 2.4 | -|
| tgi | 2.2 | 12.1 |
| tgi | 3.0 | 12.4 |


## API to Call Inference
```
curl -H "Content-type: application/json" -X POST -d '{
  "model": "/data/xzgan/csg-wukong-1B",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "What is deep learning?"
    }
  ],
  "stream": true,
  "max_tokens": 20
}' http://localhost:8000/v1/chat/completions
```
*Note: VLLM and TGI has the same endpoint and request body.*

More reference for TGI: 
- [Text Generation Inference](https://huggingface.github.io/text-generation-inference/)
- [Text Generation Inference Messages API](https://huggingface.co/docs/text-generation-inference/en/messages_api)

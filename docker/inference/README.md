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

# For vllm: opencsg-registry.cn-beijing.cr.aliyuncs.com/public/vllm:v0.8.5
export IMAGE_TAG=v0.8.5
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/public/vllm:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/public/vllm:latest \
  -f Dockerfile.vllm \
  --push .
  
# For vllm cpu only: opencsg-registry.cn-beijing.cr.aliyuncs.com/public/vllm-cpu:2.3
export IMAGE_TAG=2.4
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/public/vllm-cpu:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/public/vllm-cpu:latest \
  -f Dockerfile.vllm-cpu \
  --push .

# For tgi: opencsg-registry.cn-beijing.cr.aliyuncs.com/public/tgi:3.2
export IMAGE_TAG=3.2
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/public/tgi:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/public/tgi:latest \
  -f Dockerfile.tgi \
  --push .

# For sglang: opencsg-registry.cn-beijing.cr.aliyuncs.com/public/sglang:v0.4.6.post1-cu124-srt
export IMAGE_TAG=v0.4.6.post1-cu124-srt
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/public/sglang:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/public/sglang:latest \
  -f Dockerfile.sglang \
  --push .

# For hf-inference-toolkit: opencsg-registry.cn-beijing.cr.aliyuncs.com/public/hf-inference-toolkit:0.5.3
export IMAGE_TAG=0.5.3
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ${OPENCSG_ACR}/public/hf-inference-toolkit:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/public/hf-inference-toolkit:latest \
  -f Dockerfile.hf-inference-toolkit \
  --push .
# For Text Embeddings Inference: opencsg-registry.cn-beijing.cr.aliyuncs.com/public/tei:cpu-1.6
export IMAGE_TAG=cpu-1.6
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/public/tei:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/public/tei:latest \
  -f Dockerfile.tei-cpu \
  --push .
# For Text Embeddings Inference: opencsg-registry.cn-beijing.cr.aliyuncs.com/public/tei:1.6
export IMAGE_TAG=1.6
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/public/tei:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/public/tei:latest \
  -f Dockerfile.tei \
  --push .
# For Text Llama.cpp Inference: opencsg-registry.cn-beijing.cr.aliyuncs.com/public/llama.cpp:b5215
export IMAGE_TAG=b5215
docker buildx build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/public/llama.cpp:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/public/llama.cpp:latest \
  -f Dockerfile.llama.cpp \
  --push .
# For Text ktransformers Inference: opencsg-registry.cn-beijing.cr.aliyuncs.com/public/ktransformers:0.2.1.post1  
export IMAGE_TAG=0.2.3
docker build --platform linux/amd64 \
  -t ${OPENCSG_ACR}/public/ktransformers:${IMAGE_TAG} \
  -t ${OPENCSG_ACR}/public/ktransformers:latest \
  -f Dockerfile.ktransformers \
  --push .
```
*Note: The above command will create `linux/amd64` and `linux/arm64` images with the tags `${IMAGE_TAG}` and `latest` at the same time.*

## Run Inference Image Locally
```bash
# Run VLLM
docker run -d \
  -e ACCESS_TOKEN=xxx \
  -e REPO_ID="xzgan001/csg-wukong-1B" \
  -e HF_ENDPOINT=https://hub.opencsg.com \
  --gpus device=1 \
  -p 8000:8000 \
  ${OPENCSG_ACR}/public/vllm-local:2.8

# Run TGI
docker run -d \
  -e ACCESS_TOKEN=xxx  \
  -e REPO_ID="xzgan001/csg-wukong-1B" \
  -e HF_ENDPOINT=https://hub.opencsg.com \
  -v llm:/workspace \
  --gpus device=7 \
  -p 8000:8000
  ${OPENCSG_ACR}/public/tgi:2.2
```
*Note: HF_ENDPOINT should be use the real csghub address.*

## inference image name, version and cuda version
| Task| Image Name | Version | CUDA Version | Fix
| --- | --- | --- | --- |--- |
|text generation| vllm | 2.8 | 12.1 | - |
|text generation| vllm | v0.8.5 | 12.4 |fix hf hub timestamp|
|text generation| vllm-cpu | 2.4 | -|fix hf hub timestamp |
|text generation| tgi | 2.2 | 12.1 |- |
|text generation| tgi | 3.2 | 12.4 |fix hf hub timestamp|
|image generation| hf-inference-toolkit | 0.5.3 | 12.1 |-|
|text generation| sglang | v0.4.6.post1-cu124-srt| 12.4 |- |
|text generation| mindie | 2.0-csg-1.0.RC2 | 1.0.RC2 |- |
|text generation| llama.cpp | b5215 | - |- |
|text generation| tei | 1.6 | - |- |


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

# CSGHUB inference images

## build images
```bash
docker build -f Dockerfile.vllm .
docker build -f Dockerfile.tgi .
```

## push images
```
docker login registry.cn-beijing.aliyuncs.com
docker push xxx
```
## latest images
#for vllm image
registry.cn-beijing.aliyuncs.com/opencsg/vllm-local:1.3
#for vllm image
registry.cn-beijing.aliyuncs.com/opencsg/tgi-local:1.3

## Run image locally
```
docker run -d -e HF_TOKEN=xxxxx  -e MODEL_ID="xzgan001/csg-wukong-1B" -e HF_ENDPOINT=https://hub-stg.opencsg.com/hf --gpus device=7 registry.cn-beijing.aliyuncs.com/opencsg/vllm-local:1.3

docker run -d -e HF_TOKEN=xxxxx  -e MODEL_ID="xzgan001/csg-wukong-1B" -e HF_ENDPOINT=https://hub-stg.opencsg.com/hf --gpus device=7  rregistry.cn-beijing.aliyuncs.com/opencsg/tgi-local:1.3

```
Note: HF_ENDPOINT should be use the real csghub address
## API to call inference
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
VLLM and TGI has the same endpoint and request body
More reference for tgi: 
https://huggingface.co/docs/text-generation-inference/en/messages_api
https://huggingface.github.io/text-generation-inference/


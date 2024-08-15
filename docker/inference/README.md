# CSGHUB inference images

## build images
```bash
docker build -f Dockerfile.vllm .
docker build -f Dockerfile.tgi .
```

## push images
```
docker login opencsg-registry.cn-beijing.cr.aliyuncs.com
docker push xxx
```
## latest images
```
#for vllm image
opencsg-registry.cn-beijing.cr.aliyuncs.com/public/vllm-local:2.7
#for vllm cpu only
opencsg-registry.cn-beijing.cr.aliyuncs.com/public/vllm-cpu:2.3
#for tgi image
opencsg-registry.cn-beijing.cr.aliyuncs.com/public/tgi-local:1.6
```
## Run image locally
```
docker run -d -e ACCESS_TOKEN=xxx  -e REPO_ID="xzgan001/csg-wukong-1B" -e HF_ENDPOINT=https://hub-stg.opencsg.com/ --gpus device=1  opencsg-registry.cn-beijing.cr.aliyuncs.com/public/vllm-local:2.7

docker run -d -v llm:/data -e ACCESS_TOKEN=xxx  -e REPO_ID="xzgan001/csg-wukong-1B"  -e HF_ENDPOINT=https://hub-stg.opencsg.com/hf --gpus device=7  opencsg-registry.cn-beijing.cr.aliyuncs.com/public/tgi-local:1.6

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


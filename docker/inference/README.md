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
opencsg-registry.cn-beijing.cr.aliyuncs.com/public/vllm-local:2.1
#for vllm cpu only
opencsg-registry.cn-beijing.cr.aliyuncs.com/public/vllm-cpu:2.1
#for tgi image
opencsg-registry.cn-beijing.cr.aliyuncs.com/public/tgi:2.0
```
## Run image locally
```
docker run -d -e ACCESS_TOKEN=c6d57fb71b835d05bd402d2e2ef144bb6e22d27c  -e REPO_ID="xzgan001/csg-wukong-1B" -e HF_ENDPOINT=https://hub-stg.opencsg.com/hf --gpus device=1  opencsg-registry.cn-beijing.cr.aliyuncs.com/public/vllm-local:2.2

docker run -d -e ACCESS_TOKEN=c6d57fb71b835d05bd402d2e2ef144bb6e22d27c  -e REPO_ID="xzgan001/csg-wukong-1B"  -e HF_ENDPOINT=https://hub-stg.opencsg.com/ --gpus 2  opencsg-registry.cn-beijing.cr.aliyuncs.com/public/tgi:2.0

```
Note: HF_ENDPOINT should be use the real csghub address
## API to call inference
```
curl -H "Content-type: application/json" -X POST -d '{
  "model": "xzgan001/csg-wukong-1B",
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


# CSGHUB finetune images

## base image
https://docs.nvidia.com/deeplearning/frameworks/support-matrix/index.html
https://catalog.ngc.nvidia.com/orgs/nvidia/containers/pytorch/tags

## build images
```bash
docker build -f Dockerfile.llamafactory .
```

## push images
```
docker login opencsg-registry.cn-beijing.cr.aliyuncs.com
docker push xxx
```
## latest images
```
#for llama-factory image
opencsg-registry.cn-beijing.cr.aliyuncs.com/public/llama-factory:1.15-cuda12.1-devel-ubuntu22.04-py310-torch2.1.2
```
## Run image locally
```

docker run -d -e ACCESS_TOKEN=xxx -e REPO_ID="OpenCSG/csg-wukong-1B"  -e HF_ENDPOINT=https://hub.opencsg.com/hf  opencsg-registry.cn-beijing.cr.aliyuncs.com/public/llama-factory:1.15-cuda12.1-devel-ubuntu22.04-py310-torch2.1.2

```
Note: HF_ENDPOINT should be use the real csghub address



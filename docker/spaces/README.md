# CSGHUB nginx images

## build images
```bash
docker build -f Dockerfile.nginx .
```

## push images
```
docker login registry.cn-beijing.aliyuncs.com
docker push xxx
```
## environment
```
ACCESS_TOKEN=xxx
REPO_ID=xxx
```
## latest images
registry.cn-beijing.aliyuncs.com/opencsg_space/csg-nginx:1.2
opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_space/csg-nginx:1.2



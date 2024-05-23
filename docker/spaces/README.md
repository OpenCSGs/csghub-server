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
opencsg-registry.cn-beijing.cr.aliyuncs.com/public/csg-nginx:1.1



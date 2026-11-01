#!/bin/sh
# staging 的 GitOps 持续部署链路（.deploy-ci.yml 里的 update:helm:stg -> trigger:deploy:new-stg）。
#
# 这条链路做两件事：
#   1. 把本流水线刚构建的镜像从 $CI_REGISTRY_IMAGE 复制到 ACR 的
#      opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/starhub-server
#      （下游部署项目从 ACR 拉镜像）；
#   2. 把新 tag 提交进 saas-stg helm 仓库的 update.sh-stg / update.sh-a800 / update.sh-a100，
#      再由 trigger:deploy:new-stg 带着 tag 触发 project 58 的部署流水线。
#
# 用法：脚本只定义函数，由 .deploy-ci.yml 的 .update-helm-stg 分两阶段调用 ——
#   before_script: . .gitlab/ci/scripts/update-helm-stg.sh && update_helm_stg_setup
#   script:        update_helm_stg_run
#
# 相比原来 inline 写法的三处有意改动：
#   1. 镜像源从 build job 的 dotenv artifact（IMAGE_PREFIX）改成 $CI_REGISTRY_IMAGE。
#      build 换成 ci-components 的 build 之后不再产出 dotenv，IMAGE_PREFIX 恒为空，
#      `docker pull ":<tag>"` 必然失败；而原脚本没有 set -e，失败后仍会往下把 tag
#      提交进 helm 仓库，让 staging 部署指向一个根本没推上去的镜像。这里改成
#      $CI_REGISTRY_IMAGE，并在 run 开头加 set -e，任何一步失败立即中断。
#   2. 删掉三行把 HELM_REPO_USERNAME / HELM_REPO_TOKEN 原样 echo 进 job 日志的调试输出。
#   3. helm 仓库的 clone/commit 全部用 $CI_PROJECT_DIR 下的绝对路径，不再靠中途 cd 回来。

update_helm_stg_setup() {
  echo "$CI_REGISTRY_PASSWORD" | docker login -u "$CI_REGISTRY_USER" "$CI_REGISTRY" --password-stdin
  echo "$ALI_ACR_PASSWORD" | docker login -u "$ALI_ACR_USER" "$ALI_ACR_REGISTRY" --password-stdin
  sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
  apk add --no-cache git yq openssh-client
  git config --global user.email "ci@gitlab.opencsg.com"
  git config --global user.name "GitLab CI Bot"
}

update_helm_stg_run() {
  set -e

  ACR_REGISTRY_HOST=opencsg-registry.cn-beijing.cr.aliyuncs.com
  ACR_REPO=opencsg_public/starhub-server
  TAG="$IMAGE_TAG"
  SRC_IMAGE="${CI_REGISTRY_IMAGE}:${TAG}"

  if [ -z "$TAG" ]; then
    echo "update helm stg: IMAGE_TAG is empty, refusing to continue"
    exit 1
  fi

  echo "update helm stg: retag ${SRC_IMAGE} -> ${ACR_REGISTRY_HOST}/${ACR_REPO}:${TAG}"
  docker pull "$SRC_IMAGE"
  docker tag "$SRC_IMAGE" "${ACR_REGISTRY_HOST}/${ACR_REPO}:${TAG}"
  docker push "${ACR_REGISTRY_HOST}/${ACR_REPO}:${TAG}"

  if [ -z "$HELM_REPO_USERNAME" ] || [ -z "$HELM_REPO_TOKEN" ]; then
    echo "update helm stg: HELM_REPO_USERNAME / HELM_REPO_TOKEN are required"
    exit 1
  fi

  HELM_REPO="https://${HELM_REPO_USERNAME}:${HELM_REPO_TOKEN}@git-devops.opencsg.com/product/infra/deployment-automation/custom/saas-stg.git"
  HELM_DIR="$CI_PROJECT_DIR/helm-repo"
  UPDATE_FILE_STG="update.sh-stg"
  UPDATE_FILE_A800="update.sh-a800"
  UPDATE_FILE_A100="update.sh-a100"

  rm -rf "$HELM_DIR"
  git clone --quiet "$HELM_REPO" "$HELM_DIR"
  cd "$HELM_DIR"
  git checkout main

  echo "update helm stg: updating image tags in csghub-stg/$UPDATE_FILE_STG"
  cd "$HELM_DIR/csghub-stg"
  sed -i "s/--set global.image.tag=[^ ]*/--set global.image.tag=$TAG/" "$UPDATE_FILE_STG"
  sed -i "s/--set image.tag=[^ ]*/--set image.tag=$TAG/" "$UPDATE_FILE_STG"
  sed -i "s/--set runner.image.tag=[^ ]*/--set runner.image.tag=$TAG/" "$UPDATE_FILE_STG"

  echo "update helm stg: updating runner-a800/$UPDATE_FILE_A800"
  cd "$HELM_DIR/runner-a800"
  sed -i "s/--set image.tag=[^ ]*/--set image.tag=$TAG/" "$UPDATE_FILE_A800"

  echo "update helm stg: updating runner-a100/$UPDATE_FILE_A100"
  cd "$HELM_DIR/runner-a100"
  sed -i "s/--set image.tag=[^ ]*/--set image.tag=$TAG/" "$UPDATE_FILE_A100"

  cd "$HELM_DIR"
  if ! git diff --quiet; then
    git add "csghub-stg/$UPDATE_FILE_STG" "runner-a800/$UPDATE_FILE_A800" "runner-a100/$UPDATE_FILE_A100"
    git commit -m "[skip ci] stg: update starhub-server to ${TAG}"
    git push origin main
    echo "update helm stg: helm values updated and pushed"
  else
    echo "update helm stg: no changes, skipping push"
  fi

  echo "IMAGE_TAG=$TAG" > "$CI_PROJECT_DIR/image.env"
  echo "update helm stg: wrote image.env with IMAGE_TAG=$TAG"
}

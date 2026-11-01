#!/bin/sh
# 【临时】stg 部署 —— chart 就绪前，直接在本流水线里跑老部署脚本。
#
# 逻辑对齐 starhub-deployment 项目 .gitlab-ci.yml 里的 deploy:new-stg-helm：
#   git clone saas-stg → cd saas-stg/csghub-stg → bash update.sh-stg
# 差别只是：那边靠 `rules: $ENV_NAME == "new-stg"` 被本项目触发，这边直接由
# deploy-stg-saas 自己执行：kubeconfig 用本项目已有的 KUBECONFIG_SAAS_STG，clone 仓库的凭据复用项目
# 已有的 HELM_REPO_USERNAME / HELM_REPO_TOKEN（update:helm:stg 就是拿它 clone 同一个 saas-stg 仓库）。
#
# 为什么不用再改 update.sh-stg 里的 tag：
#   needs 依赖的 update:helm:stg 已经把镜像 retag 推到 ACR，并把 saas-stg 仓库里
#   update.sh-stg / -a800 / -a100 的 tag 改成 ${CI_COMMIT_SHORT_SHA}-cn-saas 并 push，
#   这里 clone 到的就是更新后的脚本。
#
# kubeconfig 由 deploy 组件处理：File 变量 KUBECONFIG_SAAS_STG 会被拷成 ~/.kube/config
# 并 export KUBECONFIG，所以下面直接 `bash update.sh-stg` 就能连上集群。
# 末尾 exit 0 会终止 job 的 shell，组件 script 段里基于 chart_set 的 helm upgrade 不执行，
# 因此 chart 没就绪也不会把 job 弄红。
#
# 收尾（chart 就绪后）：
#   1. .gitlab-saas-stage.yml 里 deploy-stg-saas 的 custom_script 换成
#      - . .gitlab/ci/scripts/deploy-saas-chart.sh saas-stg csghub-stg /tmp/saas-stg-chart
#   2. 去掉 tags 和 rules 里的 when: manual；
#   3. 删掉本文件。

set -e

# 组件处理 kubeconfig 的那段代码在它自己 script 段的后半 —— 而我们的 custom_script 在
# script_before，跑在它之前，所以这里自己准备一份。
# KUBECONFIG_SAAS_STG 是 File 类型变量，它的值就是 GitLab 生成的临时文件路径。
if [ -n "$KUBECONFIG_SAAS_STG" ] && [ -f "$KUBECONFIG_SAAS_STG" ]; then
  mkdir -p ~/.kube
  cp "$KUBECONFIG_SAAS_STG" ~/.kube/config
  export KUBECONFIG=~/.kube/config
  echo "deploy-stg legacy: KUBECONFIG prepared from KUBECONFIG_SAAS_STG"
else
  echo "deploy-stg legacy: KUBECONFIG_SAAS_STG is not set or not a readable file"
  exit 1
fi

echo "deploy-stg legacy: helm=$(helm version --short 2>/dev/null || echo unknown) KUBECONFIG=${KUBECONFIG:-unset}"
echo "deploy-stg legacy: branch=${CI_COMMIT_BRANCH} sha=${CI_COMMIT_SHORT_SHA}"

if [ -z "$HELM_REPO_USERNAME" ] || [ -z "$HELM_REPO_TOKEN" ]; then
  echo "deploy-stg legacy: HELM_REPO_USERNAME / HELM_REPO_TOKEN are not set"
  exit 1
fi

apk add --no-cache git >/dev/null 2>&1

WORK="$CI_PROJECT_DIR/.saas-stg"
rm -rf "$WORK"
git clone --quiet \
  "https://${HELM_REPO_USERNAME}:${HELM_REPO_TOKEN}@git-devops.opencsg.com/product/infra/deployment-automation/custom/saas-stg.git" \
  "$WORK"

cd "$WORK/csghub-stg"

# 镜像 tag 直接取本次构建的值（build-stage-saas 的 commit short sha），不依赖 update:helm:stg 是否
# 已经把新 tag 写进仓库 —— 那一步仍然要跑（它负责把镜像 retag 到 ACR），但这里以 CI 的值为准，
# 免得 play 的是一次陈旧 tag。
BEFORE=$(grep -m1 -oE -- '--set image\.tag=[^ ]+' update.sh-stg | cut -d= -f2-)
TAG="${CI_COMMIT_TAG:-${CI_COMMIT_SHORT_SHA}-cn-saas}"
sed -i "s/--set global.image.tag=[^ ]*/--set global.image.tag=$TAG/" update.sh-stg
sed -i "s/--set image.tag=[^ ]*/--set image.tag=$TAG/" update.sh-stg
sed -i "s/--set runner.image.tag=[^ ]*/--set runner.image.tag=$TAG/" update.sh-stg
echo "deploy-stg legacy: image tag $BEFORE -> $TAG"

echo "deploy-stg legacy: running update.sh-stg"

bash update.sh-stg

echo "deploy-stg legacy: deploy finished; the chart_set based helm upgrade is skipped on purpose"
exit 0

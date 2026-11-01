#!/bin/sh
# 为 deploy-stg-saas / deploy-prd-saas 准备 helm chart：把对应环境的 chart 仓库
# 浅克隆到本地目录，deploy 组件随后用 `helm upgrade <release> <本地路径>` 部署。
#
# 由 deploy component 的 custom_script（custom_script_position: script_before）
# 以 source 方式调用：
#   . .gitlab/ci/scripts/deploy-saas-chart.sh <repo> <subdir> <dest>
#   . .gitlab/ci/scripts/deploy-saas-chart.sh saas-stg csghub-stg /tmp/saas-stg-chart
#   . .gitlab/ci/scripts/deploy-saas-chart.sh saas-prd .          /tmp/saas-prd-chart
#
# 三条失败路径都 exit 1，让 job 直接变红，不带病往下 helm upgrade：
#   1. 参数不全；
#   2. chart_set 里引用的 CI 变量没配（组件的 envsubst 会把缺失变量静默展开成空串，
#      所以必须先在这里拦住——否则 helm 会收到空值覆盖掉线上配置）；
#   3. clone 失败或目标目录里没有 Chart.yaml。

saas_chart_vars() {
  # 目前 chart_set 只推 starhub-server 的镜像 tag，值来自预定义变量 CI_COMMIT_TAG，不用断言。
  # 将来若放开 portal / xnet / csgbot / langflow 的镜像 tag，把它们引用的变量名（如
  # PORTAL_IMAGE_TAG）加到下面，就会自动纳入非空校验、缺失直接 exit 1。
  case "$1" in
    saas-stg)
      echo ""
      ;;
    saas-prd)
      echo ""
      ;;
    *)
      echo ""
      ;;
  esac
}

saas_chart_prepare() {
  _repo="$1"
  _subdir="$2"
  _dest="$3"

  if [ -z "$_repo" ] || [ -z "$_subdir" ] || [ -z "$_dest" ]; then
    echo "saas chart: usage: deploy-saas-chart.sh <repo> <subdir> <dest>"
    exit 1
  fi

  echo "saas chart: preparing $_repo/$_subdir -> $_dest"

  _missing=""
  for _v in $(saas_chart_vars "$_repo"); do
    eval "_val=\$$_v"
    if [ -z "$_val" ]; then
      _missing="$_missing $_v"
    fi
  done
  if [ -n "$_missing" ]; then
    echo "saas chart: these CI variables are not set or empty:$_missing"
    echo "saas chart: set them under Settings > CI/CD > Variables (masked), then retry."
    exit 1
  fi
  echo "saas chart: all referenced CI variables are set"

  if [ -z "$HELM_REPO_USERNAME" ] || [ -z "$HELM_REPO_TOKEN" ]; then
    echo "saas chart: HELM_REPO_USERNAME / HELM_REPO_TOKEN are required to clone $_repo"
    exit 1
  fi

  rm -rf "$_dest"
  mkdir -p "$_dest"
  if ! git clone --quiet --depth 1 \
    "https://${HELM_REPO_USERNAME}:${HELM_REPO_TOKEN}@git-devops.opencsg.com/product/infra/deployment-automation/custom/${_repo}.git" \
    "$_dest"; then
    echo "saas chart: git clone of $_repo failed"
    exit 1
  fi

  if [ "$_subdir" != "." ]; then
    _chart="$_dest/$_subdir"
  else
    _chart="$_dest"
  fi

  if [ ! -f "$_chart/Chart.yaml" ]; then
    echo "saas chart: no Chart.yaml under $_chart"
    exit 1
  fi

  echo "saas chart: chart ready at $_chart"
  cat "$_chart/Chart.yaml"
}

saas_chart_prepare "$1" "$2" "$3"


#!/bin/bash

set -euo pipefail

work_dir="${FINETUNE_WORK_DIR:-/workspace}"
test -f "$work_dir/.finetune-download-complete"
export MODEL_PATH="$work_dir/models/$MODEL_ID"
export DATASET_PATH="$work_dir/datasets/$DATASET_ID"
export HF_HOME="$work_dir/.cache"
export SKIP_EXPORT=true
export HF_HUB_OFFLINE=1

cd "$work_dir"
exec /etc/csghub/start-job.sh

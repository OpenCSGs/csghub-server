#!/bin/bash

set -euo pipefail

export NVIDIA_VISIBLE_DEVICES=none
export ASCEND_VISIBLE_DEVICES=void
export ENFLAME_VISIBLE_DEVICES=none
export ROCR_VISIBLE_DEVICES=none
work_dir="${FINETUNE_WORK_DIR:-/workspace}"

echo "Starting finetune work directory cleanup..."

if [ "${KEEP_FINETUNE_WORK_DIR:-false}" = "true" ]; then
    echo "Keeping finetune work directory as requested (KEEP_FINETUNE_WORK_DIR=true)."
    exit 0
fi

if [[ -n "${FINETUNE_WORK_DIR:-}" && "$work_dir" == /workspace/finetune/* ]]; then
    cd /
    rm -rf -- "$work_dir"
    echo "Cleaned finetune work directory after workflow completion."
else
    echo "Work directory does not match /workspace/finetune/* pattern, skipping cleanup."
fi

#!/bin/bash

set -euo pipefail

if [ "${EXPORT_TO_HF:-false}" != "true" ]; then
    echo "Skipping export because EXPORT_TO_HF is not true."
    exit 0
fi

export NVIDIA_VISIBLE_DEVICES=none
export ASCEND_VISIBLE_DEVICES=void
export ENFLAME_VISIBLE_DEVICES=none
export ROCR_VISIBLE_DEVICES=none
work_dir="${FINETUNE_WORK_DIR:-/workspace}"
export SOURCE_MODEL_ID="$MODEL_ID"
export MODEL_PATH="$work_dir/models/$MODEL_ID"
export HF_HOME="$work_dir/.cache"
export HF_HUB_OFFLINE=0

echo "Starting fine-tuned model upload on CPU..."
cd "$work_dir"
/etc/csghub/export-to-csg.sh

if [ "${KEEP_FINETUNE_WORK_DIR:-false}" = "true" ]; then
    echo "Keeping finetune work directory as requested."
    exit 0
fi

if [[ -n "${FINETUNE_WORK_DIR:-}" && "$work_dir" == /workspace/finetune/* ]]; then
    cd /
    rm -rf -- "$work_dir"
    echo "Cleaned finetune work directory after successful upload."
fi

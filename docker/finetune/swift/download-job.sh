#!/bin/bash

set -euo pipefail

export NVIDIA_VISIBLE_DEVICES=none
export ASCEND_VISIBLE_DEVICES=void
export ENFLAME_VISIBLE_DEVICES=none
export ROCR_VISIBLE_DEVICES=none
work_dir="${FINETUNE_WORK_DIR:-/workspace}"
mkdir -p "$work_dir"
export HF_HOME="$work_dir/.cache"

echo "Starting model and dataset download on CPU..."
python3 /etc/csghub/download.py
touch "$work_dir/.finetune-download-complete"
echo "Model and dataset download completed."

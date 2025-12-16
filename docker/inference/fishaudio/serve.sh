#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"
if [ -z "$NPU_NUM" ]; then
    NPU_NUM=1
fi

# Use virtual environment Python directly to avoid uv run sync delays
PYTHON=/app/.venv/bin/python3
$PYTHON /etc/csghub/entry.py
$PYTHON -m tools.api_server \
    --listen 0.0.0.0:8000 \
    --llama-checkpoint-path "/workspace/$REPO_ID" \
    --decoder-checkpoint-path "/workspace/$REPO_ID/codec.pth" \
    --decoder-config-name modded_dac_vq
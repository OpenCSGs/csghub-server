#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py
if [ -z "$GPU_NUM" ]; then
    GPU_NUM=1
fi
python3 -m vllm.entrypoints.openai.api_server --trust-remote-code --model "$REPO_ID" --tensor-parallel-size="$GPU_NUM"
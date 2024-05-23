#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 entry.py "$@"
python3 -m vllm.entrypoints.openai.api_server --model "/data/$REPO_ID"
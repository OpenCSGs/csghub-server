#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py
python3 -m vllm.entrypoints.openai.api_server --trust-remote-code --model "$REPO_ID"
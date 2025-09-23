#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py

ENGINE_ARGS="$ENGINE_ARGS --trust-remote-code --enable-torch-compile --torch-compile-max-bs 16 --host 0.0.0.0 --port 8000 --model-path $REPO_ID --node-rank $LWS_WORKER_INDEX --nnodes $LWS_GROUP_SIZE --tp $TOTAL_GPU --dist-init-addr $LWS_LEADER_ADDRESS:5000"
tokenizer_config="/workspace/$REPO_ID/tokenizer_config.json"
if ! grep -q "chat_template" "$tokenizer_config"; then
    if [ -f "/workspace/$REPO_ID/chat_template.jinja" ]; then
        ENGINE_ARGS="$ENGINE_ARGS --chat-template /workspace/$REPO_ID/chat_template.jinja"
    else
        ENGINE_ARGS="$ENGINE_ARGS --chat-template /etc/csghub/chat_template.jinja"
    fi
fi

echo "start running with args: $ENGINE_ARGS"
python3 -m sglang.launch_server $ENGINE_ARGS

#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py

if [ -z "$GPU_NUM" ]; then
    GPU_NUM=1
fi
LimitedMaxToken=$(($GPU_NUM * 5120))
ENGINE_ARGS="$ENGINE_ARGS --trust-remote-code --enable-mixed-chunk --host 0.0.0.0 --port 8000 --model-path $REPO_ID"
if [[ ! $ENGINE_ARGS == *"--tensor-parallel-size"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --tensor-parallel-size $GPU_NUM"
fi
if [[ ! $ENGINE_ARGS == *"--mem-fraction-static"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --mem-fraction-static 0.8"
fi
configfile="/workspace/$REPO_ID/config.json"
if [[ -f "$configfile" ]] && [[ ! $ENGINE_ARGS == *"--context-length"* ]]; then
    MAX_TOKENS=$(grep '"max_position_embeddings"' $configfile | cut -d":" -f2 | sed 's/[^0-9]*//g')
    # if max_tokens is not set, use 4096
    if [ -z "$MAX_TOKENS" ]; then
        MAX_TOKENS=$LimitedMaxToken
    fi
    if [ ! -z "$MAX_TOKENS" ]; then
        if [ $MAX_TOKENS -gt $LimitedMaxToken ]; then
            MAX_TOKENS=$LimitedMaxToken       
        fi
        ENGINE_ARGS="$ENGINE_ARGS --context-length $MAX_TOKENS"
    fi
fi
tokenizer_config="/workspace/$REPO_ID/tokenizer_config.json"
if ! grep -q "chat_template" "$tokenizer_config"; then
    ENGINE_ARGS="$ENGINE_ARGS --chat-template /etc/csghub/chat_template.jinja"
fi
echo "start running with args: $ENGINE_ARGS"
python3 -m sglang.launch_server $ENGINE_ARGS

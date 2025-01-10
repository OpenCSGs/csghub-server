#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py

if [ -z "$GPU_NUM" ]; then
    GPU_NUM=1
fi
LimitedMaxToken=$(($GPU_NUM * 4096))
args="--tp $GPU_NUM --enable-mixed-chunk --disable-radix-cache --trust-remote-code --enable-p2p-check --model-path $REPO_ID --port 8000 --host 0.0.0.0 --mem-fraction-static 0.8 --enable-torch-compile"
configfile="/workspace/$REPO_ID/config.json"
if [ -f "$configfile" ]; then
    MAX_TOKENS=$(grep '"max_position_embeddings"' $configfile | cut -d":" -f2 | sed 's/[^0-9]*//g')
    # if max_tokens is not set, use 4096
    if [ -z "$MAX_TOKENS" ]; then
        MAX_TOKENS=$LimitedMaxToken
    fi
    if [ ! -z "$MAX_TOKENS" ]; then
        if [ $MAX_TOKENS -gt $LimitedMaxToken ]; then
            MAX_TOKENS=$LimitedMaxToken       
        fi
        args="$args --context-length $MAX_TOKENS"
    fi
fi
tokenizer_config="/workspace/$REPO_ID/tokenizer_config.json"
if ! grep -q "chat_template" "$tokenizer_config"; then
    args="$args --chat-template /etc/csghub/chat_template.jinja"
fi
echo "start running with args: $args"
python3 -m sglang.launch_server $args

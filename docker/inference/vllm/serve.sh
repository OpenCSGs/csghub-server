#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py
if [ -z "$GPU_NUM" ]; then
    GPU_NUM=1
fi
GPU_MEMORY_UTILIZATION=0.9
args="--trust-remote-code --model $REPO_ID --tensor-parallel-size $GPU_NUM --gpu-memory-utilization $GPU_MEMORY_UTILIZATION"
configfile="/workspace/$REPO_ID/config.json"
if [ -f "$configfile" ]; then
    MAX_TOKENS=$(grep "max_position_embeddings" $configfile | cut -d":" -f2 | sed 's/[^0-9]*//g')
    if [ ! -z "$MAX_TOKENS" ]; then
        if [ $MAX_TOKENS -gt 4096 ]; then
            MAX_TOKENS=4096       
        fi
        args="$args --max-model-len $MAX_TOKENS"
    fi
fi
tokenizer_config="/workspace/$REPO_ID/tokenizer_config.json"
if ! grep -q "chat_template" "$tokenizer_config"; then
    args="$args --chat_template /etc/csghub/chat_template.jinja"
fi
    
python3 -m vllm.entrypoints.openai.api_server $args

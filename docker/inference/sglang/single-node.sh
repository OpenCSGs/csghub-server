#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py

# Version gate: SGLang threshold 0.4.4
ENGINE_VER_OK=false
if [[ "${ENGINE_VERSION:-}" =~ ^[^0-9]*([0-9]+)\.([0-9]+)\.([0-9]+) ]]; then
    ENGINE_MAJOR=${BASH_REMATCH[1]}; ENGINE_MINOR=${BASH_REMATCH[2]}; ENGINE_PATCH=${BASH_REMATCH[3]}
    if (( ENGINE_MAJOR > 0 )) || \
       { (( ENGINE_MAJOR == 0 )) && (( ENGINE_MINOR > 4 )); } || \
       { (( ENGINE_MAJOR == 0 )) && (( ENGINE_MINOR == 4 )) && (( ENGINE_PATCH >= 4 )); }; then
        ENGINE_VER_OK=true
    fi
fi

if [ -z "$GPU_NUM" ]; then
    GPU_NUM=1
fi
LimitedMaxToken=$(($GPU_NUM * 5120))
MEM_FRACTION_STATIC=0.8
if $ENGINE_VER_OK && [[ -n "${XPU_MODEL:-}" ]]; then
    case "$(echo "$XPU_MODEL" | tr '[:upper:]' '[:lower:]')" in
        *h100*|*h200*) MEM_FRACTION_STATIC=0.92; LimitedMaxToken=$(($GPU_NUM * 16384)) ;;
        *a100*)        MEM_FRACTION_STATIC=0.90; LimitedMaxToken=$(($GPU_NUM * 10240)) ;;
    esac
fi
ENGINE_ARGS="$ENGINE_ARGS --trust-remote-code --enable-mixed-chunk --host 0.0.0.0 --port 8000 --model-path $REPO_ID"
if $ENGINE_VER_OK && [[ ! $ENGINE_ARGS == *"--enable-torch-compile"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --enable-torch-compile --torch-compile-max-bs 16"
fi
if [[ ! $ENGINE_ARGS == *"--tensor-parallel-size"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --tensor-parallel-size $GPU_NUM"
fi
if [[ ! $ENGINE_ARGS == *"--mem-fraction-static"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --mem-fraction-static $MEM_FRACTION_STATIC"
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
    if [ -f "/workspace/$REPO_ID/chat_template.jinja" ]; then
        ENGINE_ARGS="$ENGINE_ARGS --chat-template /workspace/$REPO_ID/chat_template.jinja"
    else
        ENGINE_ARGS="$ENGINE_ARGS --chat-template /etc/csghub/chat_template.jinja"
    fi
fi
echo "start running with args: $ENGINE_ARGS"
python3 -m sglang.launch_server $ENGINE_ARGS

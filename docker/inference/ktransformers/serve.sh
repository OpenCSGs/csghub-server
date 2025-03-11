#!/bin/bash

# ktransformer required source model config and tokenizer
python3 /etc/csghub/entry.py --repo_id $REPO_ID --model_format gguf 
if [ $? -ne 0 ]; then
    echo "Failed to download gguf model: $REPO_ID"
    exit 1
fi
# download configs
git clone --depth 1 https://opencsg.com/codes/xzgan/kt-configs.git

if grep -q "avx2" /proc/cpuinfo && grep -q "avx512" /proc/cpuinfo; then
    pip install /etc/csghub/ktransformers-fancy-cp310-cp310-linux_x86_64.whl
elif grep -q "avx512" /proc/cpuinfo; then
    pip install /etc/csghub/ktransformers-avx512-cp310-cp310-linux_x86_64.whl
elif grep -q "avx2" /proc/cpuinfo; then
    pip install /etc/csghub/ktransformers-avx2-cp310-cp310-linux_x86_64.whl
fi
GGUF_FILE=$(echo $ENGINE_ARGS | grep -oP '(?<=-m )[^ ]+')
if [ -z "$GGUF_FILE" ]; then
    GGUF_FILE=$GGUF_ENTRY_POINT
fi
GGUF_DIR=$(dirname "$GGUF_FILE")
#get base model path
if [[ ! $ENGINE_ARGS == *"--model_path"* ]]; then
    base_model=$(grep 'base_model:' $REPO_ID/README.md | sed 's/base_model: //')
    if [ -z "$base_model" ]; then
        echo "base_model not found in README.md"
        exit 1
    fi
    echo "$base_model"
    REPO_NAME=$(basename "$base_model")
    ENGINE_ARGS="$ENGINE_ARGS --model_path kt-configs/$REPO_NAME"
fi

echo "ktransformers  --gguf_path $REPO_ID/$GGUF_DIR  --port 8000 $ENGINE_ARGS "
ktransformers --gguf_path $REPO_ID/$GGUF_DIR  --port 8000 $ENGINE_ARGS 

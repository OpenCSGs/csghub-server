#!/bin/bash
#check required args
# offload all to gpu
if [[ ! $ENGINE_ARGS == *"-ngl "* ]] && [[ -n $GPU_NUM ]]; then
    ENGINE_ARGS="$ENGINE_ARGS -ngl -1"
fi
# number of parallel requests
if [[ ! $ENGINE_ARGS == *"-np "* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS -np 1"
fi
#size of the prompt context
if [[ ! $ENGINE_ARGS == *"-c "* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS -c 8192"
fi
#gguf path
if [[ ! $ENGINE_ARGS == *"-m "* ]] && [[ -z $GGUF_ENTRY_POINT ]]; then
    echo "model file name is required, ex: -m DeepSeek-R1-UD-IQ1_M/DeepSeek-R1-UD-IQ1_M-00001-of-00004.gguf"
    exit 1
fi
if [[ ! $ENGINE_ARGS == *"-m"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS -m $GGUF_ENTRY_POINT"
fi
echo $ENGINE_ARGS

python3 /etc/csghub/entry.py

cd $REPO_ID && llama-server $ENGINE_ARGS --port 8000 --host 0.0.0.0 --alias $REPO_ID
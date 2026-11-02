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
# Non-causal decision models (laya, clef, vllm-d3, lfm2-d1-omni) are evaluated
# in a single batch: llama.cpp rejects a prompt larger than the physical batch
# size (-ub) and stalls when it does not fit the logical batch size (-b).
# Decision prompts are bounded by the context size, so both follow -c (default
# 8192 above); DECISION_BATCH_SIZE overrides explicitly.
if [[ "$HF_TASK" == "typed-decisions" ]]; then
    # llama.cpp keeps the last occurrence of a repeated option, so the last
    # -c/--ctx-size value is the effective context size.
    decision_ctx="$(printf '%s' "$ENGINE_ARGS" \
        | grep -oE '(-c|--ctx-size)([[:space:]]+|=)[1-9][0-9]*' \
        | tail -n 1 \
        | grep -oE '[0-9]+$')"
    decision_batch="${DECISION_BATCH_SIZE:-${decision_ctx:-8192}}"
    ENGINE_ARGS="$(printf '%s' "$ENGINE_ARGS" | sed -E \
        -e 's/(^|[[:space:]])(-b|--batch-size|-ub|--ubatch-size)[[:space:]]+[^[:space:]]+/\1/g' \
        -e 's/(^|[[:space:]])(--batch-size|--ubatch-size)=[^[:space:]]+/\1/g')"
    ENGINE_ARGS="$ENGINE_ARGS -b $decision_batch -ub $decision_batch"
fi
# enable embedding mode for embedding-style tasks
if [[ "$HF_TASK" == "feature-extraction" || "$HF_TASK" == "sentence-similarity" ]]; then
    if [[ ! $ENGINE_ARGS == *"--embedding"* ]]; then
        ENGINE_ARGS="$ENGINE_ARGS --embeddings"
    fi
    if [[ ! $ENGINE_ARGS == *"--pooling "* ]] && [[ ! $ENGINE_ARGS == *"--pooling_type "* ]]; then
        ENGINE_ARGS="$ENGINE_ARGS --pooling mean"
    fi
fi
# enable reranking endpoint for text-ranking task
if [[ "$HF_TASK" == "text-ranking" ]]; then
    if [[ ! $ENGINE_ARGS == *"--rerank"* ]]; then
        ENGINE_ARGS="$ENGINE_ARGS --reranking"
    fi
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
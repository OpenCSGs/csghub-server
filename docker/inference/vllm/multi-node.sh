#!/bin/bash

set -euo pipefail

export PYTHONPATH="$(pwd):${PYTHONPATH:-}"

: "${REPO_ID:?REPO_ID is required}"
: "${GPU_NUM:?GPU_NUM is required}"
: "${LWS_GROUP_SIZE:?LWS_GROUP_SIZE is required}"
: "${LWS_WORKER_INDEX:?LWS_WORKER_INDEX is required}"

if [[ ! "$GPU_NUM" =~ ^[1-9][0-9]*$ ]] || [[ ! "$LWS_GROUP_SIZE" =~ ^[1-9][0-9]*$ ]]; then
    echo "GPU_NUM and LWS_GROUP_SIZE must be positive integers" >&2
    exit 1
fi

TOTAL_GPU=${TOTAL_GPU:-$((GPU_NUM * LWS_GROUP_SIZE))}
if [[ ! "$TOTAL_GPU" =~ ^[1-9][0-9]*$ ]]; then
    echo "TOTAL_GPU must be a positive integer" >&2
    exit 1
fi

if [[ "${VLLM_MULTI_NODE_DRY_RUN:-}" != "1" ]]; then
    python3 /etc/csghub/entry.py
fi
GPU_MEMORY_UTILIZATION=0.9
ENGINE_ARGS="${ENGINE_ARGS:-} --trust-remote-code --model $REPO_ID --port 8000"
if [[ ! $ENGINE_ARGS == *"--tensor-parallel-size"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --tensor-parallel-size $TOTAL_GPU"
fi
if [[ ! $ENGINE_ARGS == *"--pipeline-parallel-size"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --pipeline-parallel-size 1"
fi
if [[ ! $ENGINE_ARGS == *"--gpu-memory-utilization"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --gpu-memory-utilization $GPU_MEMORY_UTILIZATION"
fi

if [[ ! $ENGINE_ARGS == *"--max-model-len"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --max-model-len 9016"
fi
tokenizer_config="/workspace/$REPO_ID/tokenizer_config.json"
if [[ -f "$tokenizer_config" ]] && ! grep -q "chat_template" "$tokenizer_config"; then
    if [ -f "/workspace/$REPO_ID/chat_template.jinja" ]; then
        ENGINE_ARGS="$ENGINE_ARGS --chat_template /workspace/$REPO_ID/chat_template.jinja"
    else
        ENGINE_ARGS="$ENGINE_ARGS --chat_template /etc/csghub/chat_template.jinja"
    fi
fi
if { [[ "${VLLM_ENFORCE_EAGER:-}" == "true" ]] || [[ "${VLLM_ENFORCE_EAGER:-}" == "1" ]]; } &&
    [[ ! $ENGINE_ARGS == *"--enforce-eager"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --enforce-eager"
    echo "Enabled --enforce-eager via env var."
fi

get_parallel_size() {
    local arg_name=$1
    if [[ "$ENGINE_ARGS" =~ --${arg_name}=([0-9]+) ]]; then
        echo "${BASH_REMATCH[1]}"
    elif [[ "$ENGINE_ARGS" =~ --${arg_name}[[:space:]]+([0-9]+) ]]; then
        echo "${BASH_REMATCH[1]}"
    else
        echo "1"
    fi
}

tensor_parallel_size=$(get_parallel_size "tensor-parallel-size")
pipeline_parallel_size=$(get_parallel_size "pipeline-parallel-size")
data_parallel_size=$(get_parallel_size "data-parallel-size")
world_size=$((tensor_parallel_size * pipeline_parallel_size * data_parallel_size))
if ((world_size != TOTAL_GPU)); then
    echo "Invalid parallel topology: TP($tensor_parallel_size) * PP($pipeline_parallel_size) * DP($data_parallel_size) = $world_size, expected TOTAL_GPU=$TOTAL_GPU" >&2
    exit 1
fi

echo "ENGINE_ARGS: $ENGINE_ARGS"
if [[ "${VLLM_MULTI_NODE_DRY_RUN:-}" == "1" ]]; then
    exit 0
fi
ray_serving_script=/etc/csghub/multi-node-serving.sh
if [[ "$LWS_WORKER_INDEX" == "0" ]]; then
    "$ray_serving_script" leader --ray_cluster_size="$LWS_GROUP_SIZE"
    exec python3 -m vllm.entrypoints.openai.api_server $ENGINE_ARGS
else
    : "${LWS_LEADER_ADDRESS:?LWS_LEADER_ADDRESS is required for workers}"
    exec "$ray_serving_script" worker --ray_address="$LWS_LEADER_ADDRESS"
fi

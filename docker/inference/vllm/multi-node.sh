#!/bin/bash

set -euo pipefail

export PYTHONPATH="$(pwd):${PYTHONPATH:-}"

# Detect AMD GPU (ROCm) and configure environment for multi-node
if [ -e /dev/kfd ] || command -v rocm-smi &>/dev/null; then
    echo "AMD GPU detected, configuring ROCm multi-node environment"
    # HIP_VISIBLE_DEVICES controls which AMD GPUs are visible to the runtime
    # (analogous to CUDA_VISIBLE_DEVICES for NVIDIA). Build the device list
    # from GPU_NUM so only the assigned GPUs are exposed.
    if [ -z "${HIP_VISIBLE_DEVICES:-}" ] && [ -n "${GPU_NUM:-}" ]; then
        HIP_DEVICES=$(python3 -c "print(','.join(str(i) for i in range($GPU_NUM)))")
        export HIP_VISIBLE_DEVICES="$HIP_DEVICES"
    fi
    # Unset ROCR_VISIBLE_DEVICES: the runner sets it to "none" on non-AMD
    # nodes to hide AMD GPUs. On AMD nodes we must clear it so the devices
    # remain visible for multi-node RCCL communication.
    unset ROCR_VISIBLE_DEVICES
    # Enable P2P for multi-node; the Dockerfile sets NCCL_P2P_DISABLE=1 for
    # single-node safety, but multi-node RCCL needs P2P enabled.
    export NCCL_P2P_DISABLE=0
fi

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

# Version gate: vLLM threshold 0.10
ENGINE_VER_NUM=$(echo "${ENGINE_VERSION:-}" | sed 's/^[^0-9]*//' | cut -d. -f1-2)
ENGINE_VER_OK=false
if [[ "$ENGINE_VER_NUM" =~ ^[0-9]+\.[0-9]+$ ]]; then
    ENGINE_MAJOR=${ENGINE_VER_NUM%.*}; ENGINE_MINOR=${ENGINE_VER_NUM#*.}
    if (( ENGINE_MAJOR > 0 )) || { (( ENGINE_MAJOR == 0 )) && (( ENGINE_MINOR >= 10 )); }; then
        ENGINE_VER_OK=true
    fi
fi

GPU_MEMORY_UTILIZATION=0.9
MAX_NUM_BATCHED_TOKENS=4096
if $ENGINE_VER_OK && [[ -n "${XPU_MODEL:-}" ]]; then
    case "$(echo "$XPU_MODEL" | tr '[:upper:]' '[:lower:]')" in
        *h100*|*h200*)
            GPU_MEMORY_UTILIZATION=0.92
            MAX_NUM_BATCHED_TOKENS=8192
            ;;
        *a100*)
            GPU_MEMORY_UTILIZATION=0.90
            MAX_NUM_BATCHED_TOKENS=8192
            ;;
        *h20*)
            GPU_MEMORY_UTILIZATION=0.90
            MAX_NUM_BATCHED_TOKENS=4096
            ;;
    esac
fi
ENGINE_ARGS="${ENGINE_ARGS:-} --trust-remote-code --model $REPO_ID --port 8000"
if [[ ! $ENGINE_ARGS == *"--tensor-parallel-size"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --tensor-parallel-size $GPU_NUM"
fi
if [[ ! $ENGINE_ARGS == *"--pipeline-parallel-size"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --pipeline-parallel-size $LWS_GROUP_SIZE"
fi
if [[ ! $ENGINE_ARGS == *"--gpu-memory-utilization"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --gpu-memory-utilization $GPU_MEMORY_UTILIZATION"
fi
if [[ ! $ENGINE_ARGS == *"--distributed-executor-backend"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --distributed-executor-backend ray"
fi

if [[ ! $ENGINE_ARGS == *"--max-model-len"* ]]; then
    if $ENGINE_VER_OK; then
        LimitedMaxToken=$(($TOTAL_GPU * 5120))
        if [[ -n "${XPU_MODEL:-}" ]]; then
            case "$(echo "$XPU_MODEL" | tr '[:upper:]' '[:lower:]')" in
                *h100*|*h200*) LimitedMaxToken=$(($TOTAL_GPU * 16384)) ;;
                *a100*)        LimitedMaxToken=$(($TOTAL_GPU * 10240)) ;;
            esac
        fi
        if (( LimitedMaxToken < 9016 )); then
            LimitedMaxToken=9016
        fi
        configfile="/workspace/$REPO_ID/config.json"
        if [[ -f "$configfile" ]]; then
            MODEL_MAX_LEN=$(grep '"max_position_embeddings"' "$configfile" | head -n1 | cut -d":" -f2 | sed 's/[^0-9]*//g' | tr -d '\n\r ') || true
            if [ -n "$MODEL_MAX_LEN" ] && (( MODEL_MAX_LEN < LimitedMaxToken )); then
                LimitedMaxToken=$MODEL_MAX_LEN
            fi
        fi
    else
        LimitedMaxToken=9016
    fi
    ENGINE_ARGS="$ENGINE_ARGS --max-model-len $LimitedMaxToken"
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

# Default async-scheduling (only for NVIDIA GPU + new vLLM + PP<=1, not explicitly disabled)
if $ENGINE_VER_OK && [[ "${ASYNC_SCHEDULING_DISABLED:-}" != "true" ]] && [[ ! $ENGINE_ARGS == *"--async-scheduling"* ]]; then
    PP_SIZE=1
    if [[ "$ENGINE_ARGS" =~ --pipeline-parallel-size[=[:space:]]([0-9]+) ]]; then
        PP_SIZE=${BASH_REMATCH[1]}
    fi
    if (( PP_SIZE <= 1 )) && [[ -n "${XPU_MODEL:-}" ]] && \
       echo "$XPU_MODEL" | grep -iqE 'nvidia|h100|h200|a100|h20|a10|l4|l40|a800'; then
        ENGINE_ARGS="$ENGINE_ARGS --async-scheduling"
    fi
fi

# Default compilation-config (only for NVIDIA GPU + new vLLM)
if $ENGINE_VER_OK && [[ ! $ENGINE_ARGS == *"--compilation-config"* ]]; then
    if [[ -n "${XPU_MODEL:-}" ]] && \
       echo "$XPU_MODEL" | grep -iqE 'nvidia|h100|h200|a100|h20|a10|l4|l40|a800'; then
        ENGINE_ARGS="$ENGINE_ARGS --compilation-config {\"mode\":3}"
    fi
fi

# Default max-num-batched-tokens (tiered by GPU model)
if $ENGINE_VER_OK && [[ ! $ENGINE_ARGS == *"--max-num-batched-tokens"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --max-num-batched-tokens $MAX_NUM_BATCHED_TOKENS"
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

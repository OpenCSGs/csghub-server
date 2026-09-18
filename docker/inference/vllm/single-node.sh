#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py

# Version gate: vLLM threshold 0.10
ENGINE_VER_NUM=$(echo "${ENGINE_VERSION:-}" | sed 's/^[^0-9]*//' | cut -d. -f1-2)
ENGINE_VER_OK=false
if [[ "$ENGINE_VER_NUM" =~ ^[0-9]+\.[0-9]+$ ]]; then
    ENGINE_MAJOR=${ENGINE_VER_NUM%.*}; ENGINE_MINOR=${ENGINE_VER_NUM#*.}
    if (( ENGINE_MAJOR > 0 )) || { (( ENGINE_MAJOR == 0 )) && (( ENGINE_MINOR >= 10 )); }; then
        ENGINE_VER_OK=true
    fi
fi

if [ -z "$GPU_NUM" ]; then
    GPU_NUM=1
fi
#LimitedMaxToken is gpu_num multiplied by 5120
LimitedMaxToken=$(($GPU_NUM * 5120))
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
# GPU model tiering for LimitedMaxToken
if $ENGINE_VER_OK && [[ -n "${XPU_MODEL:-}" ]]; then
    case "$(echo "$XPU_MODEL" | tr '[:upper:]' '[:lower:]')" in
        *h100*|*h200*) LimitedMaxToken=$(($GPU_NUM * 16384)) ;;
        *a100*)        LimitedMaxToken=$(($GPU_NUM * 10240)) ;;
    esac
fi

# text-to-speech models are served by vLLM-Omni (vllm serve --omni), which
# exposes the OpenAI-compatible speech API at /v1/audio/speech.
if [ "$HF_TASK" == "text-to-speech" ]; then
    OMNI_ARGS="--trust-remote-code"
    if [[ ! $ENGINE_ARGS == *"--tensor-parallel-size"* ]]; then
        OMNI_ARGS="$OMNI_ARGS --tensor-parallel-size $GPU_NUM"
    fi
    if [ "${VLLM_ENFORCE_EAGER}" = "true" ] || [ "${VLLM_ENFORCE_EAGER}" = "1" ]; then
        OMNI_ARGS="$OMNI_ARGS --enforce-eager"
        echo "Enabled --enforce-eager via env var."
    fi
    # Do not force --gpu-memory-utilization or --max-model-len here: omni
    # pipelines are multi-stage and manage per-stage memory themselves
    # (tunable via --stage-overrides in custom engine args).
    exec vllm serve "$REPO_ID" --omni $ENGINE_ARGS $OMNI_ARGS
fi

ENGINE_ARGS="$ENGINE_ARGS --trust-remote-code --model $REPO_ID"
if [[ ! $ENGINE_ARGS == *"--tensor-parallel-size"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --tensor-parallel-size $GPU_NUM"
fi
if [[ ! $ENGINE_ARGS == *"--gpu-memory-utilization"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --gpu-memory-utilization $GPU_MEMORY_UTILIZATION"
fi
configfile="/workspace/$REPO_ID/config.json"
if [[ -f "$configfile" ]] && [[ ! $ENGINE_ARGS == *"--max-model-len"* ]]; then
    MAX_TOKENS=$(grep '"max_position_embeddings"' $configfile | cut -d":" -f2 | sed 's/[^0-9]*//g')
    # if max_tokens is not set, use 4096
    if [ -z "$MAX_TOKENS" ]; then
        MAX_TOKENS=$LimitedMaxToken
    fi
    if [ ! -z "$MAX_TOKENS" ]; then
        if [ $MAX_TOKENS -gt $LimitedMaxToken ]; then
            MAX_TOKENS=$LimitedMaxToken       
        fi
        ENGINE_ARGS="$ENGINE_ARGS --max-model-len $MAX_TOKENS"
    fi
fi
# rerank models serve pooling endpoints (/v1/rerank, /score) and have no chat template
if [ "$HF_TASK" == "text-ranking" ]; then
    # The original Qwen3-Reranker ships as Qwen3ForCausalLM and must be manually
    # routed to sequence classification, see vllm examples/pooling/score.
    # Keep the overrides JSON free of spaces: ENGINE_ARGS is expanded unquoted.
    if [[ -f "$configfile" ]] && grep -q '"Qwen3ForCausalLM"' "$configfile" \
        && [[ ! $ENGINE_ARGS == *"--hf-overrides"* ]] && [[ ! $ENGINE_ARGS == *"--hf_overrides"* ]]; then
        ENGINE_ARGS="$ENGINE_ARGS --hf-overrides {\"architectures\":[\"Qwen3ForSequenceClassification\"],\"classifier_from_token\":[\"no\",\"yes\"],\"is_original_qwen3_reranker\":true}"
        if [[ ! $ENGINE_ARGS == *"--chat-template"* ]] && [[ ! $ENGINE_ARGS == *"--chat_template"* ]]; then
            ENGINE_ARGS="$ENGINE_ARGS --chat-template /etc/csghub/qwen3_reranker.jinja"
        fi
    fi
    if [[ ! $ENGINE_ARGS == *"--runner"* ]] && [[ ! $ENGINE_ARGS == *"--task"* ]]; then
        ENGINE_ARGS="$ENGINE_ARGS --runner pooling"
    fi
elif [[ "$HF_TASK" == "feature-extraction" || "$HF_TASK" == "sentence-similarity" ]]; then
    # Embedding models use vLLM's pooling runner and expose /v1/embeddings.
    # Set the task explicitly because some architectures also support generation.
    if [[ ! $ENGINE_ARGS == *"--runner"* ]] && [[ ! $ENGINE_ARGS == *"--task"* ]]; then
        ENGINE_ARGS="$ENGINE_ARGS --runner pooling"
    fi
    if [[ ! $ENGINE_ARGS == *"--pooler-config"* ]]; then
        ENGINE_ARGS="$ENGINE_ARGS --pooler-config.task embed"
    fi
else
    tokenizer_config="/workspace/$REPO_ID/tokenizer_config.json"
    if ! grep -q "chat_template" "$tokenizer_config"; then
        if [ -f "/workspace/$REPO_ID/chat_template.jinja" ]; then
            ENGINE_ARGS="$ENGINE_ARGS --chat_template /workspace/$REPO_ID/chat_template.jinja"
        else
            ENGINE_ARGS="$ENGINE_ARGS --chat_template /etc/csghub/chat_template.jinja"
        fi
    fi
fi

if [ "${VLLM_ENFORCE_EAGER}" = "true" ] || [ "${VLLM_ENFORCE_EAGER}" = "1" ]; then
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
    
python3 -m vllm.entrypoints.openai.api_server $ENGINE_ARGS
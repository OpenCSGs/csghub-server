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
GPU_MEMORY_UTILIZATION=0.9

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

# Default compilation-config (only for NVIDIA GPU + new vLLM)
if $ENGINE_VER_OK && [[ ! $ENGINE_ARGS == *"--compilation-config"* ]]; then
    if [[ -n "${XPU_MODEL:-}" ]] && \
       echo "$XPU_MODEL" | grep -iqE 'nvidia|h100|h200|a100|h20|a10|l4|l40|a800'; then
        ENGINE_ARGS="$ENGINE_ARGS --compilation-config {\"mode\":3}"
    fi
fi

# Atlas 300I Duo has no BF16. An unset dtype, or --dtype auto, follows
# the checkpoint and selects bfloat16, which fails in aclnnInplaceNormal.
# vLLM keeps the last --dtype, so appending float16 replaces auto.
# Any other explicit dtype is left unchanged. An unset max-model-len
# uses the model default (262144 for Qwen3.5); the 310P attention mask
# is allocated as max_model_len squared.
# This path does not add --enforce-eager. Graph mode follows the vLLM
# default: the 310P image sets VLLM_ENFORCE_EAGER=0, so graph capture
# stays on. ACL graph capture can still fail when stream resources are
# exhausted. Deployments that want eager mode must pass --enforce-eager.
if [ "${ASCEND_310P:-}" = "1" ]; then
    last_dtype=$(grep -oE -- '--dtype(=|[[:space:]]+)[^[:space:]]+' <<< "$ENGINE_ARGS" | tail -n 1)
    last_dtype_value=${last_dtype#*dtype}
    last_dtype_value=${last_dtype_value#=}
    last_dtype_value=${last_dtype_value# }
    if [ -z "$last_dtype_value" ] || [ "$last_dtype_value" = "auto" ]; then
        ENGINE_ARGS="$ENGINE_ARGS --dtype float16"
    fi
    if [[ ! $ENGINE_ARGS == *"--max-model-len"* ]] && [[ ! $ENGINE_ARGS == *"--max_model_len"* ]]; then
        ENGINE_ARGS="$ENGINE_ARGS --max-model-len 8192"
    fi
    echo "310P engine args: $ENGINE_ARGS"
fi

python3 -m vllm.entrypoints.openai.api_server $ENGINE_ARGS

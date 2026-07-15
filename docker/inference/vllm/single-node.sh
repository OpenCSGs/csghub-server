#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py
if [ -z "$GPU_NUM" ]; then
    GPU_NUM=1
fi
#LimitedMaxToken is gpu_num multiplied by 4096
LimitedMaxToken=$(($GPU_NUM * 5120))
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
    
python3 -m vllm.entrypoints.openai.api_server $ENGINE_ARGS
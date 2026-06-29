#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py

# ─── Defaults (mirrors the official sglang PD LWS YAML) ───
# These env vars are injected by LWS or set in the pod spec.
# If not set, we provide safe defaults.
#
# ─── SGLang PD parallelism mapping ───
# models.toml sets TP=EP=n, DP=1 uniformly.
# SGLang PD uses: TP=n, EP=n, DP=1 (TP==EP is the recommended setting).
TP_SIZE=${TP_SIZE:-${PD_TP:-2}}
EP_SIZE=${EP_SIZE:-${PD_EP:-2}}
SGLANG_PORT_PREFILL=${SGLANG_PORT_PREFILL:-8000}
SGLANG_PORT_DECODE=${SGLANG_PORT_DECODE:-8200}
DISAGG_TRANSFER_BACKEND=${DISAGG_TRANSFER_BACKEND:-nixl}
LOG_LEVEL=${LOG_LEVEL:-info}

# ─── Auto-detect moe-a2a-backend based on GPU arch ───
# If user explicitly sets MOE_A2A_BACKEND, use it.
# Otherwise: Hopper/Blackwell (cc>=9.0) + ep==tp → deepep; else → none
# deepep/nixl_ep/mooncake require Hopper+ and ep_size==tp_size
# none works on all arch (uses All-Reduce/All-Gather)
if [ -z "$MOE_A2A_BACKEND" ]; then
    CC=$(nvidia-smi --query-gpu=compute_cap --format=csv,noheader,nounits 2>/dev/null | head -1)
    if [ -n "$CC" ] && [ "$CC" -ge 9 ] && [ "$EP_SIZE" -eq "$TP_SIZE" ]; then
        MOE_A2A_BACKEND=deepep
    else
        MOE_A2A_BACKEND=none
    fi
fi

# ─── Shared args (common to both prefill and decode, 1:1 with YAML) ───
# These serve as defaults. ENGINE_ARGS from the pod env comes AFTER and overrides them.
# Both vllm and sglang use last-wins: the last occurrence of a flag takes effect.
SHARED_ARGS=""

# --model-path (YAML: --model-path=Qwen/Qwen3-30B-A3B)
SHARED_ARGS="$SHARED_ARGS --model-path $REPO_ID"

# --host (YAML: --host=0.0.0.0)
SHARED_ARGS="$SHARED_ARGS --host 0.0.0.0"

# --disaggregation-transfer-backend (YAML: --disaggregation-transfer-backend=nixl)
SHARED_ARGS="$SHARED_ARGS --disaggregation-transfer-backend $DISAGG_TRANSFER_BACKEND"

# Distributed topology via LWS env vars (YAML: --nnodes=$(LWS_GROUP_SIZE), --node-rank=$(LWS_WORKER_INDEX), --dist-init-addr=$(LWS_LEADER_ADDRESS):5000)
SHARED_ARGS="$SHARED_ARGS --nnodes $LWS_GROUP_SIZE"
SHARED_ARGS="$SHARED_ARGS --node-rank $LWS_WORKER_INDEX"
SHARED_ARGS="$SHARED_ARGS --dist-init-addr $LWS_LEADER_ADDRESS:5000"

# --tensor-parallel-size (YAML: --tensor-parallel-size=2)
SHARED_ARGS="$SHARED_ARGS --tensor-parallel-size $TP_SIZE"

# --ep (YAML: --ep=2, expert parallelism)
SHARED_ARGS="$SHARED_ARGS --ep $EP_SIZE"

# --moe-a2a-backend (auto-detected or user-specified)
SHARED_ARGS="$SHARED_ARGS --moe-a2a-backend $MOE_A2A_BACKEND"

# --log-level (YAML: --log-level=info)
SHARED_ARGS="$SHARED_ARGS --log-level $LOG_LEVEL"

# --enable-metrics (YAML: --enable-metrics)
SHARED_ARGS="$SHARED_ARGS --enable-metrics"

# --uvicorn-access-log-exclude-prefixes (YAML: --uvicorn-access-log-exclude-prefixes=/metrics)
SHARED_ARGS="$SHARED_ARGS --uvicorn-access-log-exclude-prefixes=/metrics"

# ─── Chat template (existing logic, preserved for compatibility) ───
tokenizer_config="/workspace/$REPO_ID/tokenizer_config.json"
if ! grep -q "chat_template" "$tokenizer_config"; then
    if [ -f "/workspace/$REPO_ID/chat_template.jinja" ]; then
        SHARED_ARGS="$SHARED_ARGS --chat-template /workspace/$REPO_ID/chat_template.jinja"
    else
        SHARED_ARGS="$SHARED_ARGS --chat-template /etc/csghub/chat_template.jinja"
    fi
fi

# ─── Role-specific args ───
if [ "$PD_ROLE" == "prefill" ]; then
    # Prefill: listen on port 8000, disaggregation-mode=prefill
    # SHARED_ARGS as defaults, ENGINE_ARGS overrides (last-wins)
    ENGINE_ARGS="$SHARED_ARGS --port $SGLANG_PORT_PREFILL --disaggregation-mode prefill $ENGINE_ARGS"

elif [ "$PD_ROLE" == "decode" ]; then
    # Decode: listen on port 8200, disaggregation-mode=decode
    # SHARED_ARGS as defaults, ENGINE_ARGS overrides (last-wins)
    ENGINE_ARGS="$SHARED_ARGS --port $SGLANG_PORT_DECODE --disaggregation-mode decode $ENGINE_ARGS"
fi

echo "ENGINE_ARGS: $ENGINE_ARGS"
eval "python3 -m sglang.launch_server $ENGINE_ARGS"

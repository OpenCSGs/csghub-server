#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py

# ─── Defaults (mirrors the official vllm PD LWS YAML) ───
# These env vars are injected by LWS or set in the pod spec.
# If not set, we provide safe defaults matching the YAML.
#
# ─── vLLM PD parallelism mapping ───

# Both vLLM and SGLang use the same TP/DP values from the client.
# TP = tensor parallelism (model weight sharding across GPUs within a pod).
# DP = data parallelism (independent engine replicas).
# TotalGPUs = TP * DP, GPUs per pod = TotalGPUs / PodsSize.
#
# vLLM PD constraint: TP must fit within a single pod (TP ≤ GPUs per pod).
# When DP > 1, each pod runs DP_SIZE_LOCAL = DP / PodsSize DP ranks,
# each with TP tensor-parallel workers.
TP_SIZE=${PD_TP:-1}
DP_SIZE=${PD_DP:-1}
LWS_GROUP_SIZE=${LWS_GROUP_SIZE:-1}

# DP_SIZE_LOCAL = DP ranks per pod = DP / PodsSize
# DP_SIZE_TOTAL = total DP ranks across all pods = DP
if [[ "$LWS_GROUP_SIZE" -gt 0 ]]; then
    DP_SIZE_LOCAL=$(( DP_SIZE / LWS_GROUP_SIZE ))
else
    DP_SIZE_LOCAL=$DP_SIZE
fi
if [[ "$DP_SIZE_LOCAL" -lt 1 ]]; then
    DP_SIZE_LOCAL=1
fi
DP_SIZE_TOTAL=$DP_SIZE

# ─── Auto-detect cross-node TP mode ─────────────────────
# Cross-node TP is needed when DP=1 and there are multiple pods.
# In this mode, we treat all pods as one single TP group across nodes.
CROSS_NODE_TP_MODE=false
if [ "$DP_SIZE" -eq 1 ] && [ "$LWS_GROUP_SIZE" -gt 1 ]; then
    CROSS_NODE_TP_MODE=true
    # Override DP variables to 1 (since we are not using DP)
    DP_SIZE_LOCAL=1
    DP_SIZE_TOTAL=1
    # Derive nnodes and node-rank from LWS environment
    NNODES=$LWS_GROUP_SIZE
    NODE_RANK=${LWS_WORKER_INDEX:-0}
fi

# vLLM PD runtime env vars
export VLLM_SKIP_P2P_CHECK=${VLLM_SKIP_P2P_CHECK:-1}
# VLLM_NIXL_SIDE_CHANNEL_HOST is set by Go code via K8s Downward API (status.podIP)
GPU_MEMORY_UTILIZATION=${GPU_MEMORY_UTILIZATION:-0.94}
API_SERVER_COUNT=${API_SERVER_COUNT:-1}
KV_ROLE=${KV_ROLE:-kv_both}
KV_CONNECTOR=${KV_CONNECTOR:-NixlConnector}
KV_LOAD_FAILURE_POLICY=${KV_LOAD_FAILURE_POLICY:-fail}
VLLM_PORT_PREFILL=${VLLM_PORT_PREFILL:-8000}
VLLM_PORT_DECODE=${VLLM_PORT_DECODE:-8200}
# Default true: enforce-eager is kept ON for PD mode because CUDA Graph + NixlConnector
# KV transfer combination is not yet validated in production. Override with
# VLLM_ENFORCE_EAGER=false at your own risk after PD e2e verification.
# VLLM_ENFORCE_EAGER is not a valid vLLM env var (vLLM warns about it).
# Read it into a local var, then unset the env var so it doesn't leak to vLLM.
ENFORCE_EAGER=${VLLM_ENFORCE_EAGER:-true}
unset VLLM_ENFORCE_EAGER
# Default false: dense models don't need expert parallel.
# Auto-enabled below when PD_EP > 1 (MoE models).
ENABLE_EXPERT_PARALLEL=${ENABLE_EXPERT_PARALLEL:-false}

# ─── Shared args (common to both prefill and decode, 1:1 with YAML) ───
# These serve as defaults. ENGINE_ARGS from the pod env comes AFTER and overrides them.
# Both vllm and sglang use last-wins: the last occurrence of a flag takes effect.
SHARED_ARGS=""

# vllm serve sub-command + model
SHARED_ARGS="$SHARED_ARGS serve $REPO_ID"

# --served-model-name (YAML: --served-model-name openai-mirror/gpt-oss-120b)
SHARED_ARGS="$SHARED_ARGS --served-model-name $REPO_ID"

# --host (YAML: --host 0.0.0.0)
SHARED_ARGS="$SHARED_ARGS --host 0.0.0.0"

# --trust-remote-code (YAML: --trust-remote-code)
SHARED_ARGS="$SHARED_ARGS --trust-remote-code"

# --gpu-memory-utilization (YAML: --gpu-memory-utilization 0.94)
SHARED_ARGS="$SHARED_ARGS --gpu-memory-utilization $GPU_MEMORY_UTILIZATION"

# --api-server-count is added later (after headless check), since it cannot
# be used with --headless mode.

# --disable-access-log-for-endpoints (YAML: --disable-access-log-for-endpoints=/health,/metrics,/v1/models)
SHARED_ARGS="$SHARED_ARGS --disable-access-log-for-endpoints=/health,/metrics,/v1/models"

# --enable-expert-parallel: enabled when user sets it to true/1, OR when PD_EP > 1 (MoE model)
EP_SIZE=${PD_EP:-${EP_SIZE:-0}}
if [[ "${ENABLE_EXPERT_PARALLEL}" == "true" ]] || [[ "${ENABLE_EXPERT_PARALLEL}" == "1" ]] || [[ "$EP_SIZE" -gt 1 ]]; then
    SHARED_ARGS="$SHARED_ARGS --enable-expert-parallel"
fi

# --tensor-parallel-size (vLLM PD: use real TP value from client)
SHARED_ARGS="$SHARED_ARGS --tensor-parallel-size $TP_SIZE"

# Data-parallel or cross-node TP args
if [ "$CROSS_NODE_TP_MODE" = "true" ]; then
    # Cross-node TP: add nnodes, node-rank, master-addr, master-port
    SHARED_ARGS="$SHARED_ARGS --nnodes $NNODES"
    SHARED_ARGS="$SHARED_ARGS --node-rank $NODE_RANK"
    SHARED_ARGS="$SHARED_ARGS --master-addr ${LWS_LEADER_ADDRESS}"
    SHARED_ARGS="$SHARED_ARGS --master-port 5000"
else
    # Data-parallel across LWS group
    SHARED_ARGS="$SHARED_ARGS --data-parallel-size $DP_SIZE_TOTAL"
    SHARED_ARGS="$SHARED_ARGS --data-parallel-size-local $DP_SIZE_LOCAL"
    SHARED_ARGS="$SHARED_ARGS --data-parallel-address ${LWS_LEADER_ADDRESS}"
    SHARED_ARGS="$SHARED_ARGS --data-parallel-rpc-port 5555"

    START_RANK=$(( ${LWS_WORKER_INDEX:-0} * DP_SIZE_LOCAL ))
    SHARED_ARGS="$SHARED_ARGS --data-parallel-start-rank $START_RANK"
fi

# --enforce-eager (YAML: --enforce-eager, always on by default)
# Governance happens in Go (vllmEnforceEagerEnabled): when the client enables
# enforce-eager in ENGINE_ARGS, the platform injects VLLM_ENFORCE_EAGER=1 and
# it is read/unset above, so no shell-side ENGINE_ARGS check is needed here.
if [[ "${ENFORCE_EAGER}" == "true" ]] || [[ "${ENFORCE_EAGER}" == "1" ]]; then
    SHARED_ARGS="$SHARED_ARGS --enforce-eager"
fi

# --kv-transfer-config (YAML: '{"kv_connector":"NixlConnector","kv_role":"kv_both","kv_load_failure_policy":"fail"}')
KV_TRANSFER_CONFIG="{\"kv_connector\":\"${KV_CONNECTOR}\",\"kv_role\":\"${KV_ROLE}\",\"kv_load_failure_policy\":\"${KV_LOAD_FAILURE_POLICY}\"}"
SHARED_ARGS="$SHARED_ARGS --kv-transfer-config ${KV_TRANSFER_CONFIG}"

# --no-disable-hybrid-kv-cache-manager (YAML: --no-disable-hybrid-kv-cache-manager)
SHARED_ARGS="$SHARED_ARGS --no-disable-hybrid-kv-cache-manager"

# --api-server-count (YAML: --api-server-count 1)
# Added to SHARED_ARGS now; will be removed later if headless mode is activated.
SHARED_ARGS="$SHARED_ARGS --api-server-count $API_SERVER_COUNT"

# ─── Role-specific args ───
if [ "$PD_ROLE" == "prefill" ]; then
    # Prefill: listen on port 8000
    ENGINE_ARGS="$SHARED_ARGS --port $VLLM_PORT_PREFILL $ENGINE_ARGS"

    # Prefill: no hybrid-lb, so non-leader workers need --headless
    # Cases:
    # 1) DP mode: non-leader (LWS_WORKER_INDEX > 0) and DP_SIZE_TOTAL > 1
    # 2) Cross-node TP mode: node_rank != 0
    if { [ "$CROSS_NODE_TP_MODE" != "true" ] && [ "$DP_SIZE_TOTAL" -gt 1 ] && [ "${LWS_WORKER_INDEX:-0}" -gt 0 ]; } || \
       { [ "$CROSS_NODE_TP_MODE" = "true" ] && [ "$NODE_RANK" -ne 0 ]; }; then
        ENGINE_ARGS="$ENGINE_ARGS --headless"
        # Remove --api-server-count for headless (no API server in headless mode)
        ENGINE_ARGS="${ENGINE_ARGS//--api-server-count $API_SERVER_COUNT/}"
    fi

elif [ "$PD_ROLE" == "decode" ]; then
    # Decode: listen on port 8200
    ENGINE_ARGS="$SHARED_ARGS --port $VLLM_PORT_DECODE $ENGINE_ARGS"

    # Decode specific: --data-parallel-hybrid-lb (YAML: --data-parallel-hybrid-lb)
    # Only on leader pod when dp_size > 1 and in DP mode.
    if [ "$CROSS_NODE_TP_MODE" != "true" ] && [ "$DP_SIZE_TOTAL" -gt 1 ] && [ "${LWS_WORKER_INDEX:-0}" -eq 0 ]; then
        ENGINE_ARGS="$ENGINE_ARGS --data-parallel-hybrid-lb"
    fi

    # Decode: handle headless and api-server-count removal
    # Cases:
    # 1) Cross-node TP mode: non-leader (node_rank != 0) -> add --headless
    # 2) DP mode: non-leader (LWS_WORKER_INDEX > 0) and DP_SIZE_TOTAL > 1 -> remove api-server-count (no headless)
    if [ "$CROSS_NODE_TP_MODE" = "true" ] && [ "$NODE_RANK" -ne 0 ]; then
        ENGINE_ARGS="$ENGINE_ARGS --headless"
        ENGINE_ARGS="${ENGINE_ARGS//--api-server-count $API_SERVER_COUNT/}"
    elif [ "$CROSS_NODE_TP_MODE" != "true" ] && [ "$DP_SIZE_TOTAL" -gt 1 ] && [ "${LWS_WORKER_INDEX:-0}" -gt 0 ]; then
        ENGINE_ARGS="${ENGINE_ARGS//--api-server-count $API_SERVER_COUNT/}"
    fi

    # Decode specific: --max-num-batched-tokens.
    # ENGINE_ARGS rendered by Go setEngineArgs already carries the framework
    # template default (vllm templates ship 4096) and any customer override;
    # the shell fallback below only applies for frameworks whose template
    # lacks this arg, and is aligned with the template default (4096).
    if [[ ! "$ENGINE_ARGS" == *"--max-num-batched-tokens"* ]]; then
        MAX_NUM_BATCHED_TOKENS=${MAX_NUM_BATCHED_TOKENS:-4096}
        ENGINE_ARGS="$ENGINE_ARGS --max-num-batched-tokens $MAX_NUM_BATCHED_TOKENS"
    fi

    # Decode specific: --max-num-seqs (template default 256 == shell default 256)
    if [[ ! "$ENGINE_ARGS" == *"--max-num-seqs"* ]]; then
        MAX_NUM_SEQS=${MAX_NUM_SEQS:-256}
        ENGINE_ARGS="$ENGINE_ARGS --max-num-seqs $MAX_NUM_SEQS"
    fi
fi

echo "ENGINE_ARGS: $ENGINE_ARGS"
# Direct execution without eval — the JSON in --kv-transfer-config has no spaces,
# so shell word-splitting naturally separates the flag from its JSON value.
# shellcheck disable=SC2086
vllm $ENGINE_ARGS

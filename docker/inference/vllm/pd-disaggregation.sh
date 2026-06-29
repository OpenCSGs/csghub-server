#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py

# ─── Defaults (mirrors the official vllm PD LWS YAML) ───
# These env vars are injected by LWS or set in the pod spec.
# If not set, we provide safe defaults matching the YAML.
#
# ─── vLLM PD parallelism mapping ───
# models.toml sets TP=EP=n, DP=1 uniformly.
# vLLM PD uses: TP=1, DP=n (data-parallel replicas), EP=enabled (no number).
# So we read PD_TP as the GPU count and map it to DP, keeping TP=1.
GPU_COUNT=${PD_TP:-${TP_SIZE:-1}}
TP_SIZE=1

# vLLM PD runtime env vars
export VLLM_SKIP_P2P_CHECK=${VLLM_SKIP_P2P_CHECK:-1}
# VLLM_NIXL_SIDE_CHANNEL_HOST is set by Go code via K8s Downward API (status.podIP)
DP_SIZE_LOCAL=${DP_SIZE_LOCAL:-1}
GPU_MEMORY_UTILIZATION=${GPU_MEMORY_UTILIZATION:-0.94}
API_SERVER_COUNT=${API_SERVER_COUNT:-1}
KV_ROLE=${KV_ROLE:-kv_both}
KV_CONNECTOR=${KV_CONNECTOR:-NixlConnector}
KV_LOAD_FAILURE_POLICY=${KV_LOAD_FAILURE_POLICY:-fail}
VLLM_PORT_PREFILL=${VLLM_PORT_PREFILL:-8000}
VLLM_PORT_DECODE=${VLLM_PORT_DECODE:-8200}
# YAML always sets --enforce-eager; default to true unless explicitly disabled
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

# --tensor-parallel-size (vLLM PD: TP=1, use DP instead)
SHARED_ARGS="$SHARED_ARGS --tensor-parallel-size $TP_SIZE"

# Data-parallel across LWS group (YAML: --data-parallel-size $((LWS_GROUP_SIZE * DP_SIZE_LOCAL)))
# vLLM PD: DP = GPU_COUNT (from PD_TP in models.toml), spread across LWS pods
DP_SIZE_TOTAL=$((GPU_COUNT * DP_SIZE_LOCAL))
SHARED_ARGS="$SHARED_ARGS --data-parallel-size $DP_SIZE_TOTAL"
SHARED_ARGS="$SHARED_ARGS --data-parallel-size-local $DP_SIZE_LOCAL"
SHARED_ARGS="$SHARED_ARGS --data-parallel-address ${LWS_LEADER_ADDRESS}"
SHARED_ARGS="$SHARED_ARGS --data-parallel-rpc-port 5555"

# --data-parallel-start-rank (YAML: START_RANK=$(( ${LWS_WORKER_INDEX:-0} * DP_SIZE_LOCAL )))
START_RANK=$(( ${LWS_WORKER_INDEX:-0} * DP_SIZE_LOCAL ))
SHARED_ARGS="$SHARED_ARGS --data-parallel-start-rank $START_RANK"

# --enforce-eager (YAML: --enforce-eager, always on by default)
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
    # vLLM: "Remote engine N must use --headless unless in external or hybrid dp lb mode"
    if [ "$DP_SIZE_TOTAL" -gt 1 ] && [ "${LWS_WORKER_INDEX:-0}" -gt 0 ]; then
        ENGINE_ARGS="$ENGINE_ARGS --headless"
        # Remove --api-server-count for headless (no API server in headless mode)
        ENGINE_ARGS="${ENGINE_ARGS//--api-server-count $API_SERVER_COUNT/}"
    fi

elif [ "$PD_ROLE" == "decode" ]; then
    # Decode: listen on port 8200
    ENGINE_ARGS="$SHARED_ARGS --port $VLLM_PORT_DECODE $ENGINE_ARGS"

    # Decode specific: --data-parallel-hybrid-lb (YAML: --data-parallel-hybrid-lb)
    # Only on leader pod when dp_size > 1.
    if [ "$DP_SIZE_TOTAL" -gt 1 ] && [ "${LWS_WORKER_INDEX:-0}" -eq 0 ]; then
        ENGINE_ARGS="$ENGINE_ARGS --data-parallel-hybrid-lb"
    fi

    # Decode with hybrid-lb: workers must NOT use --headless
    # vLLM: "Remote engine N must not use --headless in external or hybrid dp lb mode"
    # Decode without hybrid-lb (dp_size=1): no headless needed anyway
    # So for decode: never add --headless, but remove --api-server-count for workers
    # when dp_size > 1 (workers in hybrid-lb mode don't run API server)
    if [ "$DP_SIZE_TOTAL" -gt 1 ] && [ "${LWS_WORKER_INDEX:-0}" -gt 0 ]; then
        ENGINE_ARGS="${ENGINE_ARGS//--api-server-count $API_SERVER_COUNT/}"
    fi

    # Decode specific: --max-num-batched-tokens (YAML: --max-num-batched-tokens 256)
    MAX_NUM_BATCHED_TOKENS=${MAX_NUM_BATCHED_TOKENS:-256}
    ENGINE_ARGS="$ENGINE_ARGS --max-num-batched-tokens $MAX_NUM_BATCHED_TOKENS"

    # Decode specific: --max-num-seqs (YAML: --max-num-seqs 256)
    MAX_NUM_SEQS=${MAX_NUM_SEQS:-256}
    ENGINE_ARGS="$ENGINE_ARGS --max-num-seqs $MAX_NUM_SEQS"
fi

echo "ENGINE_ARGS: $ENGINE_ARGS"
# Direct execution without eval — the JSON in --kv-transfer-config has no spaces,
# so shell word-splitting naturally separates the flag from its JSON value.
# shellcheck disable=SC2086
vllm $ENGINE_ARGS

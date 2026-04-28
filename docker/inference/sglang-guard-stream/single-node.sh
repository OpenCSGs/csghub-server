#!/bin/bash

export PYTHONPATH="/sgl-workspace/sglang/python:$PYTHONPATH"

# Download model
python3 /etc/csghub/entry.py

# Default GPU count
if [ -z "$GPU_NUM" ]; then
    GPU_NUM=1
fi

# Model path (REPO_ID is set by platform)
MODEL_PATH="/workspace/$REPO_ID"

# Engine parameters
CONTEXT_LENGTH="${CONTEXT_LENGTH:-10000}"
TP_SIZE="${TP_SIZE:-$GPU_NUM}"
MEM_FRACTION_STATIC="${MEM_FRACTION_STATIC:-0.6}"
CHUNKED_PREFILL_SIZE="${CHUNKED_PREFILL_SIZE:-131072}"
PAGE_SIZE="${PAGE_SIZE:-1}"

echo "=========================================="
echo "Starting Qwen3Guard-Stream Server"
echo "=========================================="
echo "Model Path: $MODEL_PATH"
echo "Context Length: $CONTEXT_LENGTH"
echo "TP Size: $TP_SIZE"
echo "Mem Fraction Static: $MEM_FRACTION_STATIC"
echo "Chunked Prefill Size: $CHUNKED_PREFILL_SIZE"
echo "Page Size: $PAGE_SIZE"
echo "GPU Num: $GPU_NUM"
echo "=========================================="

# Start the guard server using Engine API
cd /sgl-workspace/sglang
exec python3 /etc/csghub/start_engine.py
#!/bin/bash
set -euo pipefail

python /etc/csghub/entry.py

PORT="${PORT:-8000}"
DEVICE="${DIFFUSERS_DEVICE:-${DEVICE:-cuda}}"
MODEL_PATH="${DIFFUSERS_MODEL:-/workspace/${REPO_ID}}"

echo "Starting diffusers runtime with local model: ${MODEL_PATH}"

exec python /etc/csghub/server.py \
    --host 0.0.0.0 \
    --port "${PORT}" \
    --device "${DEVICE}" \
    --model-path "${MODEL_PATH}" \
    --model-id "${REPO_ID}" \
    ${ENGINE_ARGS:-}

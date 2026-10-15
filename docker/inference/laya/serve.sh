#!/bin/bash
set -euo pipefail

python /etc/csghub/entry.py

PORT="${PORT:-8000}"

exec python /etc/csghub/server.py \
    --host 0.0.0.0 \
    --port "${PORT}" \
    ${ENGINE_ARGS:-}

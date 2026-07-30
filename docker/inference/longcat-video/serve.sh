#!/usr/bin/env bash
set -euo pipefail

python3 /etc/csghub/entry.py

exec uvicorn server:app \
  --app-dir /etc/csghub \
  --host 0.0.0.0 \
  --port "${PORT:-8000}" \
  --workers 1

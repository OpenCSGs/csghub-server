#!/bin/bash
set -e

# Download the model repository (inference code + weights) from CSGHub.
python3 /etc/csghub/entry.py

# The AudioFly repo ships its own `ldm` package and references
# ./models/... and ./config/... relatively, so run from the repo root.
cd "/workspace/$REPO_ID"
export PYTHONPATH="$(pwd):$PYTHONPATH"

exec uvicorn server:app --app-dir /etc/csghub --host 0.0.0.0 --port 8000

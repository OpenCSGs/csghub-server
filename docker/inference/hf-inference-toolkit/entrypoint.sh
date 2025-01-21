#!/bin/bash

# Define the default port
python /etc/csghub/entry.py
PORT=8000
export HF_MODEL_DIR="/workspace/$REPO_ID"
export HF_TRUST_REMOTE_CODE="1"

# Check if HF_MODEL_DIR is set and if not skip installing custom dependencies
if [[ ! -z "${HF_MODEL_DIR}" ]]; then
    # Check if requirements.txt exists and if so install dependencies
    if [ -f "${HF_MODEL_DIR}/requirements.txt" ]; then
        echo "Installing custom dependencies from ${HF_MODEL_DIR}/requirements.txt"
        pip install -r ${HF_MODEL_DIR}/requirements.txt --no-cache-dir
    fi
fi

# Start the server
exec uvicorn webservice_starlette:app --host 0.0.0.0 --port ${PORT}
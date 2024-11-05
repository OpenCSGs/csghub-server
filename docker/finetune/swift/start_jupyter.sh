#!/bin/bash

context_path="/"
if [ "x${CONTEXT_PATH}" != "x" ]; then
    context_path=${CONTEXT_PATH}
fi

sleep 10 && jupyter lab --ip=0.0.0.0 --port=8000 --no-browser --ServerApp.base_url=$context_path --allow-root --config=/root/.jupyter/jupyter_notebook_config.py
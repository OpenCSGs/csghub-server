#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py

ENGINE_ARGS="$ENGINE_ARGS --trust-remote-code --enable-torch-compile --torch-compile-max-bs 16 --host 0.0.0.0 --port 8000 --model-path $REPO_ID --node-rank $LWS_WORKER_INDEX --nnodes $LWS_GROUP_SIZE --tp $TOTAL_GPU --dist-init-addr $LWS_LEADER_ADDRESS:5000"

echo "start running with args: $ENGINE_ARGS"
python3 -m sglang.launch_server $ENGINE_ARGS

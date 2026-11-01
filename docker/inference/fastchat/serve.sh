#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"
if [ -z "$NPU_NUM" ]; then
    NPU_NUM=1
fi
python3.10 /etc/csghub/entry.py
source /usr/local/Ascend/ascend-toolkit/set_env.sh
python3 -m fastchat.serve.controller --host 0.0.0.0 --port 21001 &
python3 -m fastchat.serve.openai_api_server --host 0.0.0.0 --port 8000 --controller-address http://localhost:21001 &
python3 -m fastchat.serve.model_worker --num-gpus $NPU_NUM  --model-path "$REPO_ID" --model-names "$REPO_ID" --device npu --controller http://localhost:21001 --host 0.0.0.0 --port 31001 --worker http://localhost:31001
#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py
GPU_MEMORY_UTILIZATION=0.9
ENGINE_ARGS="$ENGINE_ARGS --trust-remote-code --model $REPO_ID --port 8000 --pipeline-parallel-size $LWS_GROUP_SIZE --enforce-eager"
if [[ ! $ENGINE_ARGS == *"--tensor-parallel-size"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --tensor-parallel-size $GPU_NUM"
fi
if [[ ! $ENGINE_ARGS == *"--gpu-memory-utilization"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --gpu-memory-utilization $GPU_MEMORY_UTILIZATION"
fi

if [[ ! $ENGINE_ARGS == *"--max-model-len"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --max-model-len 9016"
fi
echo "ENGINE_ARGS: $ENGINE_ARGS"
if [ "$LWS_WORKER_INDEX" == "0" ]; then
    /vllm-workspace/examples/online_serving/multi-node-serving.sh leader --ray_cluster_size=$LWS_GROUP_SIZE;python3 -m vllm.entrypoints.openai.api_server $ENGINE_ARGS
else
    /vllm-workspace/examples/online_serving/multi-node-serving.sh worker --ray_address=$LWS_LEADER_ADDRESS
fi


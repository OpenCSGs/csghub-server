#!/bin/bash

# set path and first
lightx2v_path=/app/LightX2V
model_path=/workspace/$REPO_ID

# set environment variables
source ${lightx2v_path}/scripts/base/base.sh
if [ -z "$GPU_NUM" ]; then
    GPU_NUM=1
fi

# Start API server with distributed inference service
if [ "$GPU_NUM" -gt 1 ]; then
    config_file="wan_moe_t2v_distill_lora_ parallel.json"
    sed -i "s/\"\$GPU_NUM\"/$GPU_NUM/g" /etc/csghub/"${config_file}"
    sed -i "s|\$REPO_ID|$REPO_ID|g" /etc/csghub/"${config_file}"
    torchrun --nproc_per_node=$GPU_NUM -m lightx2v.server \
    --model_cls wan2.2_moe_distill \
    --task t2v \
    --model_path $model_path \
    --config_json /etc/csghub/"${config_file}" \
    --port 8000
else
    config_file="wan_moe_t2v_distill_lora.json"
    sed -i "s/\"\$GPU_NUM\"/$GPU_NUM/g" /etc/csghub/${config_file}
    sed -i "s|\$REPO_ID|$REPO_ID|g" /etc/csghub/${config_file}
    python -m lightx2v.server \
    --model_cls wan2.2_moe_distill \
    --task t2v \
    --model_path $model_path \
    --config_json /etc/csghub/${config_file} \
    --port 8000
fi
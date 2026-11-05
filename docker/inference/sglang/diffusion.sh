#!/bin/bash
# SGLang Diffusion runtime for joint video/audio models such as MiniMax-H3.
# Serves the OpenAI-style async video API: POST /v1/videos,
# GET /v1/videos/{id} and GET /v1/videos/{id}/content. video_compat.py also
# serves the LightX2V routes (POST /v1/tasks/video, GET /v1/tasks/{id}/status,
# GET /v1/files/download/outputs/videos/{id}.mp4) on the same port.
set -e

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 /etc/csghub/entry.py

if [ -z "$GPU_NUM" ]; then
    GPU_NUM=1
fi
# MiniMax-H3 ships one DiT per checkpoint partition; fl2va serves t2va and
# fl2va requests, ref2va serves omni-reference requests.
MODEL_VARIANT=${MODEL_VARIANT:-fl2va}

# ENGINE_ARGS is rendered from the LLM engine_args in sglang.json, which
# `sglang serve` rejects in diffusion mode; keep only the other options, such
# as diffusion flags passed through custom-options.
LLM_VALUE_ARGS=" --context-length --tensor-parallel-size --chunked-prefill-size --cpu-offload-gb --dp-size --mem-fraction-static --tool-call-parser "
LLM_FLAG_ARGS=" --enable-dp-attention "
DIFFUSION_ARGS=""
skip_next=false
for arg in $ENGINE_ARGS; do
    if $skip_next; then
        skip_next=false
        continue
    fi
    name=${arg%%=*}
    if [[ $LLM_VALUE_ARGS == *" $name "* ]]; then
        [[ $arg != *=* ]] && skip_next=true
        continue
    fi
    if [[ $LLM_FLAG_ARGS == *" $name "* ]]; then
        continue
    fi
    DIFFUSION_ARGS="$DIFFUSION_ARGS $arg"
done
ENGINE_ARGS=$DIFFUSION_ARGS

MODEL_DIR=/workspace/$REPO_ID
ENGINE_ARGS="$ENGINE_ARGS --host 0.0.0.0 --port 8000 --model-path $MODEL_DIR --num-gpus $GPU_NUM"
# SGLang recognizes MiniMax-H3 releases only by their Hugging Face repo names,
# which CSGHub repo names need not follow, so select them from repo metadata.
if grep -Eq '"model_id": *"FastVideo/FastVideo-FastH3-8-Step-V2"' "$MODEL_DIR/fastvideo_inference.json" 2>/dev/null; then
    # FastH3 ships flat Diffusers weights that SGLang materializes into the H3
    # layout with an overlay it would fetch from Hugging Face. Use the bundled
    # copy, keyed by this local path, and keep the materialized model next to
    # the weights so restarts skip re-serializing the ~10GB video VAE.
    export SGLANG_DIFFUSION_MODEL_OVERLAY_REGISTRY="{\"$MODEL_DIR\": {\"overlay_repo_id\": \"/etc/csghub/overlays/fasth3-8step-v2\", \"overlay_revision\": \"308e2dd627218c40ae759a93745429800771a625\"}}"
    export SGLANG_DIFFUSION_CACHE_ROOT=${SGLANG_DIFFUSION_CACHE_ROOT:-/workspace/.sgl_diffusion}
    ENGINE_ARGS="$ENGINE_ARGS --backend sglang --model-id FastVideo-FastH3-8-Step-V2"
    # FastH3 ships a single t2va partition and rejects --model-variant.
    MODEL_VARIANT=""
elif [[ ! $ENGINE_ARGS == *"--pipeline-class-name"* ]] && grep -Eq '"_class_name": *"MiniMaxH3ModularPipeline"' "$MODEL_DIR/model_index.json" 2>/dev/null; then
    # The H3 repo root's modular model_index.json also fails SGLang's local
    # check, so name the native pipeline instead of falling back to diffusers.
    ENGINE_ARGS="$ENGINE_ARGS --backend sglang --pipeline-class-name MiniMaxH3Pipeline"
fi
if [[ -n $MODEL_VARIANT && ! $ENGINE_ARGS == *"--model-variant"* ]]; then
    ENGINE_ARGS="$ENGINE_ARGS --model-variant $MODEL_VARIANT"
fi
# BF16 weights are ~135GB per variant (61.7GB DiT + 62.1GB Qwen3-VL encoder +
# VAEs), so a single GPU must stream layers from host memory, while multi-GPU
# deployments split the DiT with TP2 and the sequence with Ulysses.
if [[ ! $ENGINE_ARGS == *"--tp-size"* && ! $ENGINE_ARGS == *"--ulysses-degree"* ]]; then
    if (( GPU_NUM == 1 )); then
        ENGINE_ARGS="$ENGINE_ARGS --performance-mode memory --layerwise-offload-components dit,text_encoder"
    elif (( GPU_NUM % 2 == 0 )); then
        ENGINE_ARGS="$ENGINE_ARGS --tp-size 2 --ulysses-degree $((GPU_NUM / 2)) --encoder-parallel auto"
    else
        ENGINE_ARGS="$ENGINE_ARGS --ulysses-degree $GPU_NUM --encoder-parallel auto"
    fi
fi

echo "start running with args: $ENGINE_ARGS"
exec python3 /etc/csghub/video_compat.py $ENGINE_ARGS

#!/bin/bash
set -euo pipefail

python /etc/csghub/entry.py

PORT="${PORT:-8000}"
DEVICE="${PADDLEOCR_DEVICE:-${DEVICE:-cpu}}"
MODEL_DIR="/workspace/${REPO_ID}"
MODEL_SOURCE="${PADDLEOCR_MODEL_SOURCE:-hub}"

# Default ("hub"): sub-models are resolved by name from the HF-compatible
# endpoint given by HF_ENDPOINT (PaddleX's HuggingFace hoster), with
# PaddleX's built-in failover to its other official hosters when a sub-model
# is missing there. "local-only" is the strict offline mode for air-gapped
# deployments: no external sources, and the model repo must be a self-
# contained PaddleX pipeline bundle.
if [ "${MODEL_SOURCE}" = "local-only" ]; then
  export PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK=True
elif [ "${MODEL_SOURCE}" = "hub" ]; then
  # CSGHub serves its HuggingFace-compatible API under the /hf subpath, so
  # PaddleX's HF hoster must use ${HF_ENDPOINT}/hf, not ${HF_ENDPOINT}.
  export PADDLE_PDX_HUGGING_FACE_ENDPOINT="${PADDLE_PDX_HUGGING_FACE_ENDPOINT:-${HF_ENDPOINT%/}/hf}"
  export PADDLE_PDX_MODEL_SOURCE=huggingface
  export HF_TOKEN="${HF_TOKEN:-${ACCESS_TOKEN}}"
else
  echo "ERROR: unknown PADDLEOCR_MODEL_SOURCE '${MODEL_SOURCE}' (expected hub|local-only)" >&2
  exit 1
fi

PIPELINE="${PADDLEX_PIPELINE:-}"
if [ -z "${PIPELINE}" ]; then
  if [ -f "${MODEL_DIR}/pipeline.yaml" ]; then
    PIPELINE="${MODEL_DIR}/pipeline.yaml"
  elif [ -f "${MODEL_DIR}/OCR.yaml" ]; then
    PIPELINE="${MODEL_DIR}/OCR.yaml"
  elif [ "${MODEL_SOURCE}" = "local-only" ]; then
    echo "ERROR: ${MODEL_DIR} contains no pipeline.yaml/OCR.yaml and PADDLEOCR_MODEL_SOURCE=local-only." >&2
    echo "The model repo must be a PaddleX pipeline bundle (pipeline.yaml + local model_dir subdirs)." >&2
    exit 1
  else
    REC_NAME="$(basename "${REPO_ID}")"
    if [ -f "${MODEL_DIR}/inference.pdiparams" ] && [[ "${REC_NAME}" == *_rec ]]; then
      # The repo is a single recognition model: generate the OCR pipeline
      # config and point TextRecognition at the local weights (det/cls
      # sub-models are still resolved by name from the model sources).
      GEN_DIR="${MODEL_DIR}/.csghub"
      paddlex --get_pipeline_config OCR --save_path "${GEN_DIR}"
      PIPELINE="$(find "${GEN_DIR}" -name '*.yaml' | head -1)"
      python /etc/csghub/gen_pipeline.py --config "${PIPELINE}" --rec-name "${REC_NAME}" --rec-dir "${MODEL_DIR}"
    else
      # Sub-models are resolved by name from the configured model sources.
      PIPELINE="OCR"
    fi
  fi
fi

echo "Starting PaddleX serving: pipeline=${PIPELINE} device=${DEVICE} port=${PORT} model_source=${MODEL_SOURCE}"

exec paddlex --serve \
    --pipeline "${PIPELINE}" \
    --host 0.0.0.0 \
    --port "${PORT}" \
    --device "${DEVICE}" \
    ${ENGINE_ARGS:-}

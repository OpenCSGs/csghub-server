#!/bin/bash
set -euo pipefail

RUNTIME_MODE="${1:-}"

case "${RUNTIME_MODE}" in
  paddleocr|paddleocr-vl)
    ;;
  "")
    echo "ERROR: image runtime mode is required (expected paddleocr|paddleocr-vl)" >&2
    exit 1
    ;;
  *)
    echo "ERROR: unknown image runtime mode '${RUNTIME_MODE}' (expected paddleocr|paddleocr-vl)" >&2
    exit 1
    ;;
esac

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
  # PaddleX health-checks the HF hoster with HEAD on the endpoint root, which
  # 404s for the /hf subpath and would silently drop the hoster; skip it.
  export PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK=True
  # The platform sets HF_HUB_OFFLINE=1, which makes hf_hub refuse all HTTP;
  # re-enable it here since the endpoint is our own hub, not huggingface.co.
  export HF_HUB_OFFLINE=0
  export HF_TOKEN="${HF_TOKEN:-${ACCESS_TOKEN}}"
else
  echo "ERROR: unknown PADDLEOCR_MODEL_SOURCE '${MODEL_SOURCE}' (expected hub|local-only)" >&2
  exit 1
fi

PIPELINE="${PADDLEX_PIPELINE:-}"
if [ -z "${PIPELINE}" ]; then
  if [ -f "${MODEL_DIR}/pipeline.yaml" ]; then
    PIPELINE="${MODEL_DIR}/pipeline.yaml"
  elif [ "${RUNTIME_MODE}" = "paddleocr" ] && [ -f "${MODEL_DIR}/OCR.yaml" ]; then
    PIPELINE="${MODEL_DIR}/OCR.yaml"
  elif [ "${MODEL_SOURCE}" = "local-only" ]; then
    echo "ERROR: ${MODEL_DIR} contains no pipeline config for ${RUNTIME_MODE} and PADDLEOCR_MODEL_SOURCE=local-only." >&2
    echo "The model repo must be a PaddleX pipeline bundle (pipeline.yaml + local model_dir subdirs)." >&2
    exit 1
  elif [ "${RUNTIME_MODE}" = "paddleocr-vl" ]; then
    # Repository names may contain flattened namespace prefixes, so use the
    # model's own metadata as the authoritative identity for pipeline selection.
    MODEL_METADATA="${MODEL_DIR}/inference.yml"
    [ -f "${MODEL_METADATA}" ] || {
      echo "ERROR: PaddleOCR-VL model metadata not found at ${MODEL_METADATA}" >&2
      exit 1
    }
    MODEL_NAME="$(python /etc/csghub/model_metadata.py "${MODEL_METADATA}")" || {
      echo "ERROR: failed to read model metadata from ${MODEL_METADATA}" >&2
      exit 1
    }
    if [ "${MODEL_NAME}" = "PaddleOCR-VL-0.9B" ]; then
      PIPELINE_NAME="PaddleOCR-VL"
    elif [[ "${MODEL_NAME}" =~ ^(PaddleOCR-VL-[0-9]+(\.[0-9]+)*)-0\.9B$ ]]; then
      PIPELINE_NAME="${BASH_REMATCH[1]}"
    else
      echo "ERROR: unsupported PaddleOCR-VL model '${MODEL_NAME}' from ${MODEL_METADATA}" >&2
      exit 1
    fi
    GEN_DIR="${MODEL_DIR}/.csghub"
    PIPELINE="${GEN_DIR}/${PIPELINE_NAME}.yaml"
    rm -f "${PIPELINE}"
    paddlex --get_pipeline_config "${PIPELINE_NAME}" --save_path "${GEN_DIR}"
    [ -f "${PIPELINE}" ] || {
      echo "ERROR: failed to generate PaddleX pipeline config at ${PIPELINE}" >&2
      exit 1
    }
    python /etc/csghub/gen_pipeline.py \
      --pipeline paddleocr-vl \
      --config "${PIPELINE}" \
      --rec-name "${MODEL_NAME}" \
      --rec-dir "${MODEL_DIR}"
  else
    REC_NAME="$(basename "${REPO_ID}")"
    if [ -f "${MODEL_DIR}/inference.pdiparams" ] && [[ "${REC_NAME}" == *_rec ]]; then
      # The repo is a single recognition model: generate the OCR pipeline
      # config and point TextRecognition at the local weights (det/cls
      # sub-models are still resolved by name from the model sources).
      GEN_DIR="${MODEL_DIR}/.csghub"
      PIPELINE="${GEN_DIR}/OCR.yaml"
      # /workspace persists across restarts; remove the stale copy or paddlex
      # prompts to overwrite and crashes on EOF (no TTY in the pod).
      rm -f "${PIPELINE}"
      paddlex --get_pipeline_config OCR --save_path "${GEN_DIR}"
      [ -f "${PIPELINE}" ] || {
        echo "ERROR: failed to generate PaddleX pipeline config at ${PIPELINE}" >&2
        exit 1
      }
      python /etc/csghub/gen_pipeline.py --pipeline ocr --config "${PIPELINE}" --rec-name "${REC_NAME}" --rec-dir "${MODEL_DIR}"
    else
      # Sub-models are resolved by name from the configured model sources.
      PIPELINE="OCR"
    fi
  fi
fi

echo "Starting PaddleX serving: runtime_mode=${RUNTIME_MODE} pipeline=${PIPELINE} device=${DEVICE} port=${PORT} model_source=${MODEL_SOURCE}"

exec paddlex --serve \
    --pipeline "${PIPELINE}" \
    --host 0.0.0.0 \
    --port "${PORT}" \
    --device "${DEVICE}" \
    ${ENGINE_ARGS:-}

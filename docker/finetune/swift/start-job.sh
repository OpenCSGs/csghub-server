#!/bin/bash

# Set default EPOCHS if not provided
EPOCHS="${EPOCHS:-3}"
LEARNING_RATE="${LEARNING_RATE:-0.0001}"
SWIFT_COMMAND="${SWIFT_COMMAND:-sft}"
DATASET_ARG="$DATASET_ID"
CUSTOM_DATASET_INFO_ARG=()

case "$SWIFT_COMMAND" in
    sft|rlhf|pt)
        ;;
    *)
        echo "Unsupported SWIFT_COMMAND: $SWIFT_COMMAND. Expected one of: sft, rlhf, pt"
        exit 1
        ;;
esac

if [ -z "${NPROC_PER_NODE:-}" ] && [ -n "${GPU_NUM:-}" ] && [ "$GPU_NUM" != "0" ]; then
    export NPROC_PER_NODE="$GPU_NUM"
fi

if [ -n "${DATASET_REVISION:-}" ]; then
    CUSTOM_DATASET_NAME="${CUSTOM_DATASET_NAME:-csghub_finetune_dataset}"
    CUSTOM_DATASET_INFO="${CUSTOM_DATASET_INFO:-/tmp/csghub_dataset_info.json}"
    export CUSTOM_DATASET_NAME CUSTOM_DATASET_INFO DATASET_ID DATASET_REVISION

    python3 - <<'PY'
import json
import os

dataset_info = [
    {
        "hf_dataset_id": os.environ["DATASET_ID"],
        "hf_revision": os.environ["DATASET_REVISION"],
        "dataset_name": os.environ["CUSTOM_DATASET_NAME"],
    }
]

with open(os.environ["CUSTOM_DATASET_INFO"], "w", encoding="utf-8") as f:
    json.dump(dataset_info, f)
PY

    DATASET_ARG="$CUSTOM_DATASET_NAME"
    CUSTOM_DATASET_INFO_ARG=(--custom_dataset_info "$CUSTOM_DATASET_INFO")
    echo "Using DATASET_REVISION: $DATASET_REVISION"
fi

# Run fine-tuning
echo "Starting fine-tuning process..."
echo "Using SWIFT_COMMAND: $SWIFT_COMMAND"
echo "Using EPOCHS: $EPOCHS"
if [ -n "${NPROC_PER_NODE:-}" ]; then
    echo "Using NPROC_PER_NODE: $NPROC_PER_NODE"
fi

CMD=(swift "$SWIFT_COMMAND" --model "$MODEL_ID" "${CUSTOM_DATASET_INFO_ARG[@]}" --dataset "$DATASET_ARG" --use_hf true)

case "$SWIFT_COMMAND" in
    sft|rlhf)
        CMD+=(--num_train_epochs "$EPOCHS" --learning_rate "$LEARNING_RATE" --tuner_type lora)
        ;;
esac

if [ -n "${CUSTOM_ARGS:-}" ]; then
    CMD+=($CUSTOM_ARGS)
fi

"${CMD[@]}"

# Check if fine-tuning was successful
if [ $? -eq 0 ]; then
    echo "Fine-tuning completed successfully!"
    
    # Check if Hugging Face export is requested
    echo "Starting export to CSGHUB..."
    /etc/csghub/export-to-csg.sh
    
    if [ $? -eq 0 ]; then
        echo "Export to CSGHUB completed successfully!"
    else
        echo "Export to CSGHUB failed!"
        exit 1
    fi
else
    echo "Fine-tuning failed!"
    exit 1
fi
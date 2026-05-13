#!/bin/bash

# Set default EPOCHS if not provided
EPOCHS="${EPOCHS:-3}"
LEARNING_RATE="${LEARNING_RATE:-0.0001}"
DATASET_ARG="$DATASET_ID"
CUSTOM_DATASET_INFO_ARG=()

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
echo "Using EPOCHS: $EPOCHS"
swift sft --model "$MODEL_ID" "${CUSTOM_DATASET_INFO_ARG[@]}" --dataset "$DATASET_ARG" --num_train_epochs "$EPOCHS" --learning_rate "$LEARNING_RATE" --use_hf true --tuner_type lora $CUSTOM_ARGS

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
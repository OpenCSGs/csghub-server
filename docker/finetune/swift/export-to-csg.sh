#!/bin/bash

# Export fine-tuned model to CSGHUB using API
# This script should be run after fine-tuning completes

set -e

echo "Starting export to CSGHUB using API..."

if [ -z "$HF_TOKEN" ]; then
    echo "Error: HF_TOKEN environment variable is required"
    exit 1
fi

if [ -z "$HF_ENDPOINT" ]; then
    echo "Error: HF_ENDPOINT environment variable is required"
    exit 1
fi

# Set default values for optional variables
HF_COMMIT_MESSAGE="${HF_COMMIT_MESSAGE:-"Fine-tuned model exported from ms-swift"}"
LATEST_CKP=`find output/*/v*/checkpoint-* -type d | grep -v 'merged' | sort -V | tail -n 1`
EXPORT_DIR="${EXPORT_DIR:-/workspace/$LATEST_CKP}"

# Create the full repository name
CURRENT_TIME=$(date +%Y%m%d_%H%M%S)
MODEL_NAME=$(echo "$MODEL_ID" | cut -d'/' -f2)
#use FINETUNED_MODEL_NAME if provided, otherwise use MODEL_ID
REPO_NAME_DEFAULT="$MODEL_NAME-finetuned-$CURRENT_TIME"
REPO_NAME="${FINETUNED_MODEL_NAME:-$REPO_NAME_DEFAULT}"
FULL_REPO_NAME="$HF_USERNAME/$REPO_NAME"

echo "Export configuration:"
echo "  Model Name: $MODEL_ID"
echo "  Repository Name: $FULL_REPO_NAME"
echo "  HF Endpoint: $HF_ENDPOINT"
echo "  Export Directory: $EXPORT_DIR"

# Check if the fine-tuned model exists
if [ ! -d "$EXPORT_DIR" ]; then
    echo "Error: Export directory $EXPORT_DIR does not exist"
    echo "Make sure fine-tuning has completed successfully"
    exit 1
fi

# Detect model type if not provided
if [ -z "$MODEL_TYPE" ]; then
    echo "Detecting model type..."
    if [ -n "$MODEL_ID" ]; then
        MODEL_NAME=$(echo "$MODEL_ID" | cut -d'/' -f2)
        model_template=`python /etc/csghub/get_model_info_clean.py $MODEL_NAME`
        echo "Model template info: $model_template"
        IFS=',' read -ra item_types <<< $model_template
        MODEL_TYPE=${item_types[0]}
        echo "Detected model type: $MODEL_TYPE"
    else
        echo "Error: MODEL_TYPE not provided and MODEL_ID not available for detection"
        exit 1
    fi
fi

# Export the model using swift export command with API endpoint
echo "Exporting model to CSGHUB ..."
echo "Using model type: $MODEL_TYPE"
swift export \
    --model "$MODEL_ID" \
    --ckpt_dir "$EXPORT_DIR" \
    --model_type "$MODEL_TYPE" \
    --merge_lora true \
    --push_to_hub true \
    --output_dir $REPO_NAME \
    --hub_model_id "$FULL_REPO_NAME" \
    --commit_message "$HF_COMMIT_MESSAGE" \
    --use_hf true \
    --exist_ok true

echo "Export completed successfully!"
echo "Model is now available at: $HF_ENDPOINT/$FULL_REPO_NAME"

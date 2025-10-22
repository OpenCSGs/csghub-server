#!/bin/bash

# Set default EPOCHS if not provided
EPOCHS="${EPOCHS:-3}"
LEARNING_RATE="${LEARNING_RATE:-0.0001}"

# Run fine-tuning
echo "Starting fine-tuning process..."
echo "Using EPOCHS: $EPOCHS"
swift sft --model $MODEL_ID --dataset $DATASET_ID --num_train_epochs $EPOCHS --learning_rate $LEARNING_RATE --use_hf true --train_type lora $CUSTOM_ARGS

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
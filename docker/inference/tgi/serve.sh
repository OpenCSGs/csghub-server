#!/bin/bash

export PYTHONPATH="$(pwd):$PYTHONPATH"

python3 entry.py "$@"
text-generation-launcher --model-id "/data/$MODEL_ID"